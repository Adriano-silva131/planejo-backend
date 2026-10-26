package httpx

import (
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"

	"github.com/Adriano-silva131/planejo-backend/internal/platform/auth"
)

func rateLimit(api huma.API, perMinute int, key func(huma.Context) string) func(huma.Context, func(huma.Context)) {
	limiter := httprate.NewRateLimiter(perMinute, time.Minute)
	return func(ctx huma.Context, next func(huma.Context)) {
		r, w := humachi.Unwrap(ctx)
		if limiter.OnLimit(w, r, key(ctx)) {
			_ = huma.WriteErr(api, ctx, http.StatusTooManyRequests, "")
			return
		}
		next(ctx)
	}
}

func clientIPKey(ctx huma.Context) string {
	return "ip:" + middleware.GetClientIP(ctx.Context())
}

// userKey runs after authenticate, so the user is always in the context.
func userKey(ctx huma.Context) string {
	user, _ := auth.UserFrom(ctx.Context())
	return "user:" + user.ID.String()
}
