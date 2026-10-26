package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testIssuer = "https://test.supabase.co/auth/v1"
	testKID    = "test-key"
)

var testUserID = uuid.MustParse("6f1b5d1e-6a3c-4a8e-9a5b-2f1d7c3e8b90")

func newECKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	return key
}

func jwksJSON(t *testing.T, key *ecdsa.PrivateKey) []byte {
	t.Helper()
	b64 := func(n *big.Int) string {
		return base64.RawURLEncoding.EncodeToString(n.FillBytes(make([]byte, 32)))
	}
	raw, err := json.Marshal(map[string]any{"keys": []map[string]string{{
		"kty": "EC", "crv": "P-256", "alg": "ES256", "use": "sig", "kid": testKID,
		"x": b64(key.X), "y": b64(key.Y),
	}}})
	require.NoError(t, err)
	return raw
}

func newTestVerifier(t *testing.T, key *ecdsa.PrivateKey) *Verifier {
	t.Helper()
	jwks, err := keyfunc.NewJWKSetJSON(jwksJSON(t, key))
	require.NoError(t, err)
	return NewVerifier(testIssuer, jwks.KeyfuncCtx)
}

func validClaims() jwt.MapClaims {
	now := time.Now()
	return jwt.MapClaims{
		"iss":   testIssuer,
		"aud":   "authenticated",
		"sub":   testUserID.String(),
		"email": "ana@example.com",
		"role":  "authenticated",
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	}
}

func sign(t *testing.T, method jwt.SigningMethod, key any, kid string, c jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(method, c)
	if kid != "" {
		token.Header["kid"] = kid
	}
	signed, err := token.SignedString(key)
	require.NoError(t, err)
	return signed
}

func TestVerifyAcceptsValidToken(t *testing.T) {
	key := newECKey(t)

	user, err := newTestVerifier(t, key).Verify(t.Context(), sign(t, jwt.SigningMethodES256, key, testKID, validClaims()))

	require.NoError(t, err)
	assert.Equal(t, User{ID: testUserID, Email: "ana@example.com"}, user)
}

func TestVerifyToleratesSmallClockSkew(t *testing.T) {
	key := newECKey(t)
	c := validClaims()
	c["exp"] = time.Now().Add(-2 * time.Second).Unix()

	_, err := newTestVerifier(t, key).Verify(t.Context(), sign(t, jwt.SigningMethodES256, key, testKID, c))

	assert.NoError(t, err)
}

func TestVerifyRejectsInvalidTokens(t *testing.T) {
	key := newECKey(t)
	otherKey := newECKey(t)
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	with := func(field string, value any) jwt.MapClaims {
		c := validClaims()
		if value == nil {
			delete(c, field)
		} else {
			c[field] = value
		}
		return c
	}
	es256 := func(c jwt.MapClaims) string { return sign(t, jwt.SigningMethodES256, key, testKID, c) }

	cases := map[string]string{
		"wrong signature":   sign(t, jwt.SigningMethodES256, otherKey, testKID, validClaims()),
		"expired":           es256(with("exp", time.Now().Add(-time.Minute).Unix())),
		"missing exp":       es256(with("exp", nil)),
		"not yet valid":     es256(with("nbf", time.Now().Add(time.Minute).Unix())),
		"other issuer":      es256(with("iss", "https://other.supabase.co/auth/v1")),
		"other audience":    es256(with("aud", "anon")),
		"sub not uuid":      es256(with("sub", "admin")),
		"missing kid":       sign(t, jwt.SigningMethodES256, key, "", validClaims()),
		"unknown kid":       sign(t, jwt.SigningMethodES256, key, "other-key", validClaims()),
		"alg none":          sign(t, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, testKID, validClaims()),
		"alg HS256":         sign(t, jwt.SigningMethodHS256, []byte("legacy-jwt-secret"), testKID, validClaims()),
		"alg RS256 vs JWKS": sign(t, jwt.SigningMethodRS256, rsaKey, testKID, validClaims()),
		"garbage":           "not.a.jwt",
	}

	verifier := newTestVerifier(t, key)
	for name, token := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := verifier.Verify(t.Context(), token)

			assert.ErrorIs(t, err, ErrInvalidToken)
		})
	}
}

func TestNewSupabaseVerifierUsesProjectJWKS(t *testing.T) {
	key := newECKey(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/v1/.well-known/jwks.json" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(jwksJSON(t, key))
	}))
	t.Cleanup(srv.Close)

	verifier, err := NewSupabaseVerifier(t.Context(), srv.URL+"/", true)
	require.NoError(t, err)

	c := validClaims()
	c["iss"] = srv.URL + "/auth/v1"
	user, err := verifier.Verify(t.Context(), sign(t, jwt.SigningMethodES256, key, testKID, c))

	require.NoError(t, err)
	assert.Equal(t, testUserID, user.ID)
}

func TestNewSupabaseVerifierWhenJWKSIsUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(srv.Close)
	key := newECKey(t)
	c := validClaims()
	c["iss"] = srv.URL + "/auth/v1"
	token := sign(t, jwt.SigningMethodES256, key, testKID, c)

	_, requiredErr := NewSupabaseVerifier(t.Context(), srv.URL, true)
	lenient, lenientErr := NewSupabaseVerifier(t.Context(), srv.URL, false)

	require.Error(t, requiredErr)
	require.NoError(t, lenientErr)
	_, err := lenient.Verify(t.Context(), token)
	assert.ErrorIs(t, err, ErrInvalidToken)
}
