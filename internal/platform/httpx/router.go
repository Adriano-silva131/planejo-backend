package httpx

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Options struct {
	Production     bool
	AllowedOrigins []string
	RequestTimeout time.Duration
	Verifier       TokenVerifier
	// Requests per minute. Authenticated routes are limited per client IP
	// before the token is checked, so invalid tokens also count, and per user
	// after it.
	PublicRateLimit          int
	AuthenticatedIPRateLimit int
	UserRateLimit            int
}

// Router holds the HTTP handler and the groups, both under /v1, where the
// domains register their operations.
type Router struct {
	Handler       http.Handler
	Public        huma.API
	Authenticated huma.API
}

func New(opts Options, log *slog.Logger) Router {
	r := chi.NewRouter()
	r.Use(
		requestID,
		middleware.ClientIPFromRemoteAddr,
		requestLogger(log),
		recoverer(log),
		timeout(opts.RequestTimeout),
		securityHeaders(opts.Production),
		corsHandler(opts.AllowedOrigins),
	)
	r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound)
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusMethodNotAllowed)
	})

	api := humachi.New(r, humaConfig(opts.Production))
	registerHealth(api)

	public := huma.NewGroup(api, "/v1")
	public.UseSimpleModifier(documentErrors(api, http.StatusTooManyRequests))
	public.UseMiddleware(rateLimit(api, opts.PublicRateLimit, clientIPKey))

	authenticated := huma.NewGroup(api, "/v1")
	authenticated.UseSimpleModifier(requireBearer)
	authenticated.UseSimpleModifier(documentErrors(api, http.StatusUnauthorized, http.StatusTooManyRequests))
	authenticated.UseMiddleware(
		rateLimit(api, opts.AuthenticatedIPRateLimit, clientIPKey),
		authenticate(api, opts.Verifier),
		rateLimit(api, opts.UserRateLimit, userKey),
	)

	return Router{Handler: r, Public: public, Authenticated: authenticated}
}

func humaConfig(production bool) huma.Config {
	cfg := huma.DefaultConfig("Planejo API", "1.0.0")
	// The default hook adds the `$schema` field and the Link header to responses.
	cfg.CreateHooks = nil
	cfg.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		bearerScheme: {Type: "http", Scheme: "bearer", BearerFormat: "JWT"},
	}
	if production {
		cfg.DocsPath = ""
	}
	return cfg
}

type healthOutput struct {
	Body struct {
		Status string `json:"status" example:"ok"`
	}
}

func registerHealth(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "health",
		Method:      http.MethodGet,
		Path:        "/health",
		Summary:     "Verifica se a API está no ar",
		Tags:        []string{"Infra"},
	}, func(context.Context, *struct{}) (*healthOutput, error) {
		out := &healthOutput{}
		out.Body.Status = "ok"
		return out, nil
	})
}
