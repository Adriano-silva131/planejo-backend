package httpx

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidationErrorsByField(t *testing.T) {
	rec := newTestServer(t, false).postItem(t, `{"title":"ab","count":0,"extra":1,"tags":[{}]}`)

	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.Equal(t, ErrorBody{
		Code:    "validation_failed",
		Message: "Dados inválidos.",
		Fields: map[string][]string{
			"title":        {"O tamanho mínimo é 3."},
			"count":        {"O valor mínimo é 1."},
			"extra":        {"Campo não permitido."},
			"tags[0].name": {"Campo obrigatório."},
		},
	}, decodeError(t, rec))
}

func TestValidationErrorForMissingBodyField(t *testing.T) {
	rec := newTestServer(t, false).postItem(t, `{"count":1}`)

	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	assert.Equal(t, map[string][]string{"title": {"Campo obrigatório."}}, decodeError(t, rec).Fields)
}

func TestValidationMessageDropsParserDetail(t *testing.T) {
	rec := newTestServer(t, false).postItem(t, `{"title":"abc","count":1,"email":"nope"}`)

	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	assert.Equal(t, map[string][]string{"email": {"E-mail inválido."}}, decodeError(t, rec).Fields)
}

func TestValidationErrorsForQueryParameters(t *testing.T) {
	rec := newTestServer(t, false).get(t, "/v1/items?limit=abc")

	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	assert.Equal(t, map[string][]string{
		"day":   {"Campo obrigatório."},
		"limit": {"Informe um número inteiro."},
	}, decodeError(t, rec).Fields)
}

func TestMalformedBody(t *testing.T) {
	rec := newTestServer(t, false).postItem(t, `{"title":`)

	assert.GreaterOrEqual(t, rec.Code, 400)
	assert.Less(t, rec.Code, 500)
	body := decodeError(t, rec)
	assert.NotEmpty(t, body.Code)
	assert.NotEmpty(t, body.Message)
}

func TestDomainErrorMapping(t *testing.T) {
	rec := newTestServer(t, false).get(t, "/v1/items/missing")

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, ErrorBody{Code: "item_not_found", Message: "Item não encontrado."}, decodeError(t, rec))
}

func TestUnexpectedErrorHidesDetailAndLogsIt(t *testing.T) {
	srv := newTestServer(t, false)

	rec := srv.get(t, "/v1/items/boom")

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, ErrorBody{Code: "internal_error", Message: "Erro inesperado. Tente novamente."}, decodeError(t, rec))
	assert.NotContains(t, rec.Body.String(), "secret database detail")
	assert.Contains(t, srv.logs.String(), "secret database detail")
	assert.Contains(t, srv.logs.String(), rec.Header().Get("X-Request-Id"))
}

func TestPanicIsRecovered(t *testing.T) {
	srv := newTestServer(t, false)

	rec := srv.get(t, "/v1/items/panic")

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, "internal_error", decodeError(t, rec).Code)
	assert.Contains(t, srv.logs.String(), "kaboom")
}

func TestPanicAfterResponseStartedAbortsTheResponse(t *testing.T) {
	h := recoverer(slog.New(slog.DiscardHandler))(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"partial":`))
		panic("kaboom")
	}))
	rec := httptest.NewRecorder()

	assert.PanicsWithValue(t, http.ErrAbortHandler, func() {
		h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	})
	assert.Equal(t, `{"partial":`, rec.Body.String())
}

func TestHandlerTimeout(t *testing.T) {
	rec := newTestServer(t, false).get(t, "/v1/items/slow")

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Equal(t, "timeout", decodeError(t, rec).Code)
}

func TestInternalTimeoutIsUnexpectedError(t *testing.T) {
	srv := newTestServer(t, false)

	rec := srv.get(t, "/v1/items/internal-timeout")

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, "internal_error", decodeError(t, rec).Code)
	assert.Contains(t, srv.logs.String(), "slow query")
}

func TestTimeoutWritesErrorWhenHandlerWritesNothing(t *testing.T) {
	h := timeout(10 * time.Millisecond)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Equal(t, "timeout", decodeError(t, rec).Code)
}

func TestUnknownRouteAndMethod(t *testing.T) {
	srv := newTestServer(t, false)

	notFound := srv.get(t, "/v1/nothing")
	notAllowed := srv.do(t, httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/v1/items", nil))

	assert.Equal(t, http.StatusNotFound, notFound.Code)
	assert.Equal(t, "not_found", decodeError(t, notFound).Code)
	assert.Equal(t, http.StatusMethodNotAllowed, notAllowed.Code)
	assert.Equal(t, "method_not_allowed", decodeError(t, notAllowed).Code)
}
