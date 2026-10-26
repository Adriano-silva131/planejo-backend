package httpx

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"github.com/Adriano-silva131/planejo-backend/internal/platform/auth"
	"github.com/Adriano-silva131/planejo-backend/internal/platform/logger"
)

const bearerScheme = "bearer"

type TokenVerifier interface {
	Verify(ctx context.Context, token string) (auth.User, error)
}

func authenticate(api huma.API, verifier TokenVerifier) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		token, ok := bearerToken(ctx.Header("Authorization"))
		if !ok {
			unauthorized(api, ctx)
			return
		}
		user, err := verifier.Verify(ctx.Context(), token)
		if err != nil {
			slog.DebugContext(ctx.Context(), "token rejeitado", "err", err)
			unauthorized(api, ctx)
			return
		}

		c := auth.WithUser(ctx.Context(), user)
		c = logger.With(c, slog.String("userId", user.ID.String()))
		next(huma.WithContext(ctx, c))
	}
}

func bearerToken(header string) (string, bool) {
	scheme, token, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(scheme, bearerScheme) || token == "" {
		return "", false
	}
	return token, true
}

func unauthorized(api huma.API, ctx huma.Context) {
	ctx.SetHeader("WWW-Authenticate", "Bearer")
	_ = huma.WriteErr(api, ctx, http.StatusUnauthorized, "")
}

// requireBearer documents the token requirement in the OpenAPI contract.
func requireBearer(op *huma.Operation) {
	op.Security = append(op.Security, map[string][]string{bearerScheme: {}})
}
