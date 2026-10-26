package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var ErrInvalidToken = errors.New("auth: token inválido")

const (
	audience = "authenticated"
	leeway   = 5 * time.Second
)

// The algorithms of Supabase's asymmetric signing keys. Anything else,
// including "none" and the legacy HS256 secret, is rejected.
var allowedAlgorithms = []string{"ES256", "RS256"}

type User struct {
	ID    uuid.UUID
	Email string
}

type claims struct {
	jwt.RegisteredClaims
	Email string `json:"email"`
}

// KeysFunc resolves the verification key of a token, bound to the request ctx.
type KeysFunc func(ctx context.Context) jwt.Keyfunc

type Verifier struct {
	keys   KeysFunc
	parser *jwt.Parser
}

func NewVerifier(issuer string, keys KeysFunc) *Verifier {
	return &Verifier{
		keys: keys,
		parser: jwt.NewParser(
			jwt.WithValidMethods(allowedAlgorithms),
			jwt.WithIssuer(issuer),
			jwt.WithAudience(audience),
			jwt.WithExpirationRequired(),
			jwt.WithLeeway(leeway),
		),
	}
}

// NewSupabaseVerifier validates tokens issued by the Supabase project at
// supabaseURL. The JWKS is cached and refreshed in the background until ctx
// ends; a key id it does not know triggers a rate-limited refresh. With
// requireKeys, failing to fetch the JWKS now is an error; otherwise it is
// logged and tokens are rejected until a refresh succeeds.
func NewSupabaseVerifier(ctx context.Context, supabaseURL string, requireKeys bool) (*Verifier, error) {
	issuer := strings.TrimSuffix(supabaseURL, "/") + "/auth/v1"
	jwks, err := keyfunc.NewDefaultOverrideCtx(ctx, []string{issuer + "/.well-known/jwks.json"}, keyfunc.Override{
		HTTPTimeout:               jwksTimeout,
		NoErrorReturnFirstHTTPReq: new(!requireKeys),
		RefreshErrorHandlerFunc:   logRefreshError,
	})
	if err != nil {
		return nil, fmt.Errorf("auth: jwks: %w", err)
	}
	return NewVerifier(issuer, jwks.KeyfuncCtx), nil
}

const jwksTimeout = 10 * time.Second

func logRefreshError(url string) func(context.Context, error) {
	return func(ctx context.Context, err error) {
		slog.ErrorContext(ctx, "falha ao buscar as chaves do Supabase", "url", url, "err", err)
	}
}

func (v *Verifier) Verify(ctx context.Context, token string) (User, error) {
	var c claims
	if _, err := v.parser.ParseWithClaims(token, &c, v.keyfunc(ctx)); err != nil {
		return User{}, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}
	id, err := uuid.Parse(c.Subject)
	if err != nil {
		return User{}, fmt.Errorf("%w: sub não é um uuid", ErrInvalidToken)
	}
	return User{ID: id, Email: c.Email}, nil
}

// keyfunc requires a kid: without one the JWKS library would try every key.
func (v *Verifier) keyfunc(ctx context.Context) jwt.Keyfunc {
	keys := v.keys(ctx)
	return func(t *jwt.Token) (any, error) {
		if kid, _ := t.Header["kid"].(string); kid == "" {
			return nil, errors.New("token sem kid")
		}
		return keys(t)
	}
}
