package httpx

import (
	"context"
	"crypto/rand"
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/Adriano-silva131/planejo-backend/internal/platform/logger"
)

const requestIDHeader = "X-Request-Id"

// requestID always generates the id: one sent by the client is not trusted.
func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := rand.Text()
		w.Header().Set(requestIDHeader, id)
		ctx := logger.With(r.Context(), slog.String("requestId", id))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// requestLogger never logs headers, body or query string: they may carry
// tokens or personal data.
func requestLogger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)

			status := ww.Status()
			if status == 0 {
				status = http.StatusOK
			}
			log.InfoContext(r.Context(), "requisição",
				"method", r.Method,
				"route", route(r),
				"status", status,
				"duration", time.Since(start),
			)
		})
	}
}

func route(r *http.Request) string {
	if rctx := chi.RouteContext(r.Context()); rctx != nil && rctx.RoutePattern() != "" {
		return rctx.RoutePattern()
	}
	return r.URL.Path
}

func recoverer(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}
				if err, ok := rec.(error); ok && errors.Is(err, http.ErrAbortHandler) {
					panic(rec)
				}
				log.ErrorContext(r.Context(), "panic", "panic", rec, "stack", string(debug.Stack()))
				if ww.Status() != 0 {
					// The response already started: abort the connection so the
					// client does not take the partial body as complete.
					panic(http.ErrAbortHandler)
				}
				writeError(w, http.StatusInternalServerError)
			}()
			next.ServeHTTP(ww, r)
		})
	}
}

// timeout answers in the API error format when the handler gives up after
// the deadline without writing a response.
func timeout(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()

			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r.WithContext(ctx))
			if ww.Status() == 0 && errors.Is(ctx.Err(), context.DeadlineExceeded) {
				writeError(w, http.StatusServiceUnavailable)
			}
		})
	}
}

// securityHeaders sends CSP and HSTS only in production: /docs, available in
// development, loads its scripts from a CDN and the API runs over plain HTTP.
func securityHeaders(production bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			h.Set("Referrer-Policy", "no-referrer")
			if production {
				h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
				h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}

func corsHandler(allowedOrigins []string) func(http.Handler) http.Handler {
	return cors.Handler(cors.Options{
		AllowedOrigins: allowedOrigins,
		AllowedMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete},
		AllowedHeaders: []string{"Authorization", "Content-Type", "Time-Zone"},
		ExposedHeaders: []string{requestIDHeader},
		MaxAge:         600,
	})
}
