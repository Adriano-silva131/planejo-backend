package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"reflect"
	"strconv"

	"github.com/danielgtaylor/huma/v2"

	"github.com/Adriano-silva131/planejo-backend/internal/shared/domainerr"
)

func init() {
	translateValidationMessages()
	huma.NewError = func(status int, _ string, errs ...error) huma.StatusError {
		return newError(context.Background(), status, errs)
	}
	huma.NewErrorWithContext = func(hctx huma.Context, status int, _ string, errs ...error) huma.StatusError {
		ctx := context.Background()
		if hctx != nil {
			ctx = hctx.Context()
		}
		return newError(ctx, status, errs)
	}
}

type ErrorResponse struct {
	status int
	Err    ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code    string              `json:"code" doc:"Identificador estável do erro"`
	Message string              `json:"message" doc:"Mensagem para o usuário"`
	Fields  map[string][]string `json:"fields,omitempty" doc:"Erros de validação por campo"`
}

func (e *ErrorResponse) Error() string {
	return e.Err.Message
}

func (e *ErrorResponse) GetStatus() int {
	return e.status
}

var statusErrors = map[int]ErrorBody{
	http.StatusBadRequest:            {Code: "bad_request", Message: "Requisição inválida."},
	http.StatusUnauthorized:          {Code: "unauthorized", Message: "Faça login para continuar."},
	http.StatusForbidden:             {Code: "forbidden", Message: "Você não tem permissão para esta ação."},
	http.StatusNotFound:              {Code: "not_found", Message: "Recurso não encontrado."},
	http.StatusMethodNotAllowed:      {Code: "method_not_allowed", Message: "Método não permitido."},
	http.StatusNotAcceptable:         {Code: "not_acceptable", Message: "Formato de resposta não suportado."},
	http.StatusRequestTimeout:        {Code: "request_timeout", Message: "A requisição demorou demais para ser enviada."},
	http.StatusConflict:              {Code: "conflict", Message: "A operação conflita com o estado atual."},
	http.StatusRequestEntityTooLarge: {Code: "payload_too_large", Message: "A requisição é grande demais."},
	http.StatusUnsupportedMediaType:  {Code: "unsupported_media_type", Message: "Formato de conteúdo não suportado."},
	http.StatusUnprocessableEntity:   {Code: "validation_failed", Message: "Dados inválidos."},
	http.StatusTooManyRequests:       {Code: "rate_limited", Message: "Muitas requisições. Tente novamente em instantes."},
	http.StatusInternalServerError:   {Code: "internal_error", Message: "Erro inesperado. Tente novamente."},
	http.StatusServiceUnavailable:    {Code: "timeout", Message: "A requisição demorou demais. Tente novamente."},
}

var statusByKind = map[domainerr.Kind]int{
	domainerr.NotFound: http.StatusNotFound,
	domainerr.Conflict: http.StatusConflict,
	domainerr.Invalid:  http.StatusUnprocessableEntity,
}

func newError(ctx context.Context, status int, errs []error) *ErrorResponse {
	if status == http.StatusInternalServerError {
		err := errors.Join(errs...)
		if resp, ok := knownError(ctx, err); ok {
			return resp
		}
		slog.ErrorContext(ctx, "erro inesperado", "err", err)
	}
	resp := statusError(status)
	resp.Err.Fields = fieldErrors(errs)
	return resp
}

// knownError maps errors returned by handlers that are not failures of the API.
// A context error only counts when it comes from the request itself ending: an
// internal timeout, such as a slow query with its own deadline, is a failure.
func knownError(ctx context.Context, err error) (*ErrorResponse, bool) {
	var de *domainerr.Error
	if errors.As(err, &de) {
		if status, ok := statusByKind[de.Kind]; ok {
			return &ErrorResponse{status: status, Err: ErrorBody{Code: de.Code, Message: de.Message}}, true
		}
	}
	if ctx.Err() != nil && errors.Is(err, ctx.Err()) {
		return statusError(http.StatusServiceUnavailable), true
	}
	return nil, false
}

func statusError(status int) *ErrorResponse {
	body, ok := statusErrors[status]
	if !ok {
		body = statusErrors[http.StatusInternalServerError]
		if status < http.StatusInternalServerError {
			body = statusErrors[http.StatusBadRequest]
		}
	}
	return &ErrorResponse{status: status, Err: body}
}

// documentErrors adds error responses to the OpenAPI contract from a group
// modifier, which runs after huma has already turned op.Errors into responses.
func documentErrors(api huma.API, statuses ...int) func(*huma.Operation) {
	schema := api.OpenAPI().Components.Schemas.Schema(reflect.TypeFor[ErrorResponse](), true, "")
	return func(op *huma.Operation) {
		if op.Responses == nil {
			op.Responses = map[string]*huma.Response{}
		}
		for _, status := range statuses {
			code := strconv.Itoa(status)
			if _, ok := op.Responses[code]; ok {
				continue
			}
			op.Responses[code] = &huma.Response{
				Description: http.StatusText(status),
				Content:     map[string]*huma.MediaType{"application/json": {Schema: schema}},
			}
		}
	}
}

// writeError answers with the API error format outside huma operations.
func writeError(w http.ResponseWriter, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(statusError(status))
}
