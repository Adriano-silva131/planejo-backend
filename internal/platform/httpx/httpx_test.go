package httpx

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Adriano-silva131/planejo-backend/internal/platform/config"
	"github.com/Adriano-silva131/planejo-backend/internal/platform/logger"
	"github.com/Adriano-silva131/planejo-backend/internal/shared/domainerr"
)

const testOrigin = "http://localhost:5173"

var errItemNotFound = domainerr.New(domainerr.NotFound, "item_not_found", "Item não encontrado.")

type itemOutput struct {
	Body struct {
		ID string `json:"id"`
	}
}

type getItemInput struct {
	ID string `path:"id"`
}

type createItemInput struct {
	Body struct {
		Title string `json:"title" minLength:"3"`
		Count int    `json:"count" minimum:"1"`
		Email string `json:"email,omitempty" format:"email"`
		Tags  []struct {
			Name string `json:"name"`
		} `json:"tags,omitempty"`
	}
}

type listItemsInput struct {
	Day   string `query:"day" required:"true"`
	Limit int    `query:"limit"`
}

func registerItems(api huma.API) {
	huma.Get(api, "/items/{id}", func(ctx context.Context, in *getItemInput) (*itemOutput, error) {
		switch in.ID {
		case "missing":
			return nil, errItemNotFound
		case "boom":
			return nil, errors.New("secret database detail")
		case "panic":
			panic("kaboom")
		case "slow":
			<-ctx.Done()
			return nil, ctx.Err()
		case "internal-timeout":
			return nil, fmt.Errorf("slow query: %w", context.DeadlineExceeded)
		}
		out := &itemOutput{}
		out.Body.ID = in.ID
		return out, nil
	})
	huma.Post(api, "/items", func(context.Context, *createItemInput) (*itemOutput, error) {
		return &itemOutput{}, nil
	})
	huma.Get(api, "/items", func(context.Context, *listItemsInput) (*itemOutput, error) {
		return &itemOutput{}, nil
	})
}

type testServer struct {
	handler http.Handler
	logs    *bytes.Buffer
}

func testOptions(production bool) Options {
	return Options{
		Production:               production,
		AllowedOrigins:           []string{testOrigin},
		RequestTimeout:           50 * time.Millisecond,
		Verifier:                 fakeVerifier{},
		PublicRateLimit:          1000,
		AuthenticatedIPRateLimit: 1000,
		UserRateLimit:            1000,
	}
}

func newTestServer(t *testing.T, production bool) testServer {
	t.Helper()
	return newTestServerWith(t, testOptions(production))
}

func newTestServerWith(t *testing.T, opts Options) testServer {
	t.Helper()
	logs := &bytes.Buffer{}
	log := logger.New(logs, config.Production, slog.LevelDebug)

	prev := slog.Default()
	slog.SetDefault(log)
	t.Cleanup(func() { slog.SetDefault(prev) })

	router := New(opts, log)
	registerItems(router.Public)
	registerMe(router.Authenticated)
	return testServer{handler: router.Handler, logs: logs}
}

func (s testServer) do(t *testing.T, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	s.handler.ServeHTTP(rec, req)
	return rec
}

func (s testServer) get(t *testing.T, target string) *httptest.ResponseRecorder {
	t.Helper()
	return s.do(t, httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil))
}

func (s testServer) postItem(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/items", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return s.do(t, req)
}

func (s testServer) logLines(t *testing.T) []map[string]any {
	t.Helper()
	var lines []map[string]any
	sc := bufio.NewScanner(bytes.NewReader(s.logs.Bytes()))
	for sc.Scan() {
		var line map[string]any
		require.NoError(t, json.Unmarshal(sc.Bytes(), &line))
		lines = append(lines, line)
	}
	return lines
}

func decodeError(t *testing.T, rec *httptest.ResponseRecorder) ErrorBody {
	t.Helper()
	var resp struct {
		Error ErrorBody `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp), rec.Body.String())
	return resp.Error
}

func TestHealth(t *testing.T) {
	rec := newTestServer(t, false).get(t, "/health")

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"status":"ok"}`, rec.Body.String())
}

func TestRoutesLiveUnderV1(t *testing.T) {
	srv := newTestServer(t, false)

	assert.Equal(t, http.StatusOK, srv.get(t, "/v1/items/1").Code)
	assert.Equal(t, http.StatusNotFound, srv.get(t, "/items/1").Code)
}

func TestResponsesHaveNoSchemaLink(t *testing.T) {
	rec := newTestServer(t, false).get(t, "/v1/items/1")

	assert.JSONEq(t, `{"id":"1"}`, rec.Body.String())
	assert.Empty(t, rec.Header().Get("Link"))
}

func TestOpenAPIAndDocs(t *testing.T) {
	dev := newTestServer(t, false)
	prod := newTestServer(t, true)

	spec := prod.get(t, "/openapi.json")
	require.Equal(t, http.StatusOK, spec.Code)
	assert.Contains(t, spec.Body.String(), `"/v1/items/{id}"`)
	assert.Contains(t, spec.Body.String(), `"/health"`)

	assert.Equal(t, http.StatusOK, dev.get(t, "/docs").Code)
	assert.Equal(t, http.StatusNotFound, prod.get(t, "/docs").Code)
}

func TestRequestIDAndRequestLog(t *testing.T) {
	srv := newTestServer(t, false)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/items/42?token=secret", nil)
	req.Header.Set("X-Request-Id", "forged")

	rec := srv.do(t, req)

	id := rec.Header().Get("X-Request-Id")
	require.NotEmpty(t, id)
	assert.NotEqual(t, "forged", id)
	lines := srv.logLines(t)
	require.Len(t, lines, 1)
	assert.Equal(t, "requisição", lines[0]["msg"])
	assert.Equal(t, id, lines[0]["requestId"])
	assert.Equal(t, "GET", lines[0]["method"])
	assert.Equal(t, "/v1/items/{id}", lines[0]["route"])
	assert.InDelta(t, http.StatusOK, lines[0]["status"], 0)
	assert.NotContains(t, srv.logs.String(), "secret")
}

func TestSecurityHeaders(t *testing.T) {
	dev := newTestServer(t, false).get(t, "/health")
	prod := newTestServer(t, true).get(t, "/health")

	for _, rec := range []*httptest.ResponseRecorder{dev, prod} {
		assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
		assert.Equal(t, "DENY", rec.Header().Get("X-Frame-Options"))
		assert.Equal(t, "no-referrer", rec.Header().Get("Referrer-Policy"))
	}
	assert.Empty(t, dev.Header().Get("Strict-Transport-Security"))
	assert.NotEmpty(t, prod.Header().Get("Strict-Transport-Security"))
	assert.NotEmpty(t, prod.Header().Get("Content-Security-Policy"))
}

func TestCORS(t *testing.T) {
	srv := newTestServer(t, false)
	preflight := func(origin string) *httptest.ResponseRecorder {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodOptions, "/v1/items", nil)
		req.Header.Set("Origin", origin)
		req.Header.Set("Access-Control-Request-Method", http.MethodPost)
		req.Header.Set("Access-Control-Request-Headers", "Authorization, Content-Type, Time-Zone")
		return srv.do(t, req)
	}

	allowed := preflight(testOrigin)
	denied := preflight("https://evil.example")

	assert.Equal(t, testOrigin, allowed.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, allowed.Header().Get("Access-Control-Allow-Headers"), "Time-Zone")
	assert.Empty(t, allowed.Header().Get("Access-Control-Allow-Credentials"))
	assert.Empty(t, denied.Header().Get("Access-Control-Allow-Origin"))
}
