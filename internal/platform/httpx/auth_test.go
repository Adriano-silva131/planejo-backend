package httpx

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Adriano-silva131/planejo-backend/internal/platform/auth"
)

var (
	userA = auth.User{ID: uuid.MustParse("0b9c3f0e-1111-4a8e-9a5b-2f1d7c3e8b90"), Email: "a@example.com"}
	userB = auth.User{ID: uuid.MustParse("0b9c3f0e-2222-4a8e-9a5b-2f1d7c3e8b90"), Email: "b@example.com"}
)

type fakeVerifier struct{}

func (fakeVerifier) Verify(_ context.Context, token string) (auth.User, error) {
	switch token {
	case "token-a":
		return userA, nil
	case "token-b":
		return userB, nil
	}
	return auth.User{}, auth.ErrInvalidToken
}

type meOutput struct {
	Body struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	}
}

func registerMe(api huma.API) {
	huma.Get(api, "/me", func(ctx context.Context, _ *struct{}) (*meOutput, error) {
		user, _ := auth.UserFrom(ctx)
		out := &meOutput{}
		out.Body.ID = user.ID.String()
		out.Body.Email = user.Email
		return out, nil
	})
}

func (s testServer) getMe(t *testing.T, authorization string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/me", nil)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	return s.do(t, req)
}

func TestAuthenticatedRouteRejectsMissingOrInvalidToken(t *testing.T) {
	srv := newTestServer(t, false)

	for name, header := range map[string]string{
		"missing":       "",
		"other scheme":  "Basic dXNlcjpwYXNz",
		"empty bearer":  "Bearer ",
		"invalid token": "Bearer forged",
	} {
		t.Run(name, func(t *testing.T) {
			rec := srv.getMe(t, header)

			assert.Equal(t, http.StatusUnauthorized, rec.Code)
			assert.Equal(t, "Bearer", rec.Header().Get("WWW-Authenticate"))
			assert.Equal(t, ErrorBody{Code: "unauthorized", Message: "Faça login para continuar."}, decodeError(t, rec))
		})
	}
}

func TestAuthenticatedRoutePutsUserInContext(t *testing.T) {
	srv := newTestServer(t, false)

	rec := srv.getMe(t, "bearer token-a")

	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"id":"`+userA.ID.String()+`","email":"a@example.com"}`, rec.Body.String())
}

func TestCurrentUser(t *testing.T) {
	id, err := CurrentUser(auth.WithUser(t.Context(), userA))
	require.NoError(t, err)
	assert.Equal(t, userA.ID, id)

	_, err = CurrentUser(t.Context())
	var status huma.StatusError
	require.ErrorAs(t, err, &status)
	assert.Equal(t, http.StatusUnauthorized, status.GetStatus())
}

func TestPublicRouteNeedsNoToken(t *testing.T) {
	rec := newTestServer(t, false).get(t, "/v1/items/1")

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestUserRateLimitIsPerUser(t *testing.T) {
	opts := testOptions(false)
	opts.UserRateLimit = 2
	srv := newTestServerWith(t, opts)

	for range 2 {
		require.Equal(t, http.StatusOK, srv.getMe(t, "Bearer token-a").Code)
	}
	limited := srv.getMe(t, "Bearer token-a")
	other := srv.getMe(t, "Bearer token-b")

	assert.Equal(t, http.StatusTooManyRequests, limited.Code)
	assert.Equal(t, "rate_limited", decodeError(t, limited).Code)
	assert.NotEmpty(t, limited.Header().Get("Retry-After"))
	assert.Equal(t, http.StatusOK, other.Code)
}

func TestPublicRateLimitIsPerClientIP(t *testing.T) {
	opts := testOptions(false)
	opts.PublicRateLimit = 2
	srv := newTestServerWith(t, opts)
	getFrom := func(remoteAddr string) int {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/items/1", nil)
		req.RemoteAddr = remoteAddr
		req.Header.Set("X-Forwarded-For", "203.0.113.9")
		return srv.do(t, req).Code
	}

	assert.Equal(t, http.StatusOK, getFrom("198.51.100.1:1111"))
	assert.Equal(t, http.StatusOK, getFrom("198.51.100.1:2222"))
	assert.Equal(t, http.StatusTooManyRequests, getFrom("198.51.100.1:3333"))
	assert.Equal(t, http.StatusOK, getFrom("198.51.100.2:1111"))
}

func TestAuthenticatedIPRateLimitCountsInvalidTokens(t *testing.T) {
	opts := testOptions(false)
	opts.AuthenticatedIPRateLimit = 2
	srv := newTestServerWith(t, opts)
	getMeFrom := func(remoteAddr, token string) int {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/me", nil)
		req.RemoteAddr = remoteAddr
		req.Header.Set("Authorization", "Bearer "+token)
		return srv.do(t, req).Code
	}

	assert.Equal(t, http.StatusUnauthorized, getMeFrom("198.51.100.1:1111", "forged"))
	assert.Equal(t, http.StatusUnauthorized, getMeFrom("198.51.100.1:2222", "forged"))
	assert.Equal(t, http.StatusTooManyRequests, getMeFrom("198.51.100.1:3333", "token-a"))
	assert.Equal(t, http.StatusOK, getMeFrom("198.51.100.2:1111", "token-a"))
}

func TestOpenAPIDocumentsBearerOnlyOnAuthenticatedRoutes(t *testing.T) {
	rec := newTestServer(t, false).get(t, "/openapi.json")
	var spec struct {
		Paths map[string]map[string]struct {
			Security  []map[string][]string `json:"security"`
			Responses map[string]any        `json:"responses"`
		} `json:"paths"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &spec))

	me := spec.Paths["/v1/me"]["get"]
	item := spec.Paths["/v1/items/{id}"]["get"]
	assert.Equal(t, []map[string][]string{{"bearer": {}}}, me.Security)
	assert.Contains(t, me.Responses, "401")
	assert.Contains(t, me.Responses, "429")
	assert.Empty(t, item.Security)
	assert.Contains(t, item.Responses, "429")
}
