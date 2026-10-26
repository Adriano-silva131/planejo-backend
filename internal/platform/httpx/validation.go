package httpx

import (
	"errors"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/validation"
)

const (
	msgRequired = "Campo obrigatório."
	msgInteger  = "Informe um número inteiro."
	msgNumber   = "Informe um número."
	msgBoolean  = "Informe verdadeiro ou falso."
	msgInvalid  = "Valor inválido."
)

// huma reports a missing property at its parent's location with the name only
// inside the message; the marker lets fieldErrors move it to the field itself.
const requiredMarker = "required-property:"

// paramMessages translates the fixed messages huma writes when parsing
// path, query and header parameters.
var paramMessages = map[string]string{
	"invalid integer":         msgInteger,
	"invalid float":           msgNumber,
	"invalid boolean":         msgBoolean,
	"unknown query parameter": "Parâmetro não permitido.",
	"expected at most one value, but received multiple values": "Informe um único valor.",
}

// validationMessages replaces huma's messages. A "%.0v" or "%.0s" verb
// consumes an argument huma always passes (usually an English parser error)
// without printing it.
var validationMessages = map[*string]string{
	&validation.MsgUnexpectedProperty:                 "Campo não permitido.",
	&validation.MsgExpectedRFC3339DateTime:            "Data e hora inválidas; use o formato 2026-10-20T07:30:00-03:00.",
	&validation.MsgExpectedRFC1123DateTime:            "Data e hora inválidas.",
	&validation.MsgExpectedRFC3339Date:                "Data inválida; use o formato 2026-10-20.",
	&validation.MsgExpectedRFC3339Time:                "Horário inválido.",
	&validation.MsgExpectedRFC5322Email:               "E-mail inválido.%.0v",
	&validation.MsgExpectedRFC5322EmailBare:           "E-mail inválido.",
	&validation.MsgExpectedRFC5890Hostname:            "Nome de host inválido.",
	&validation.MsgExpectedRFC2673IPv4:                "Endereço IPv4 inválido.",
	&validation.MsgExpectedRFC2373IPv6:                "Endereço IPv6 inválido.",
	&validation.MsgExpectedRFCIPAddr:                  "Endereço IP inválido.",
	&validation.MsgExpectedRFC3986URI:                 "URL inválida.%.0v",
	&validation.MsgExpectedRFC3986AbsoluteURI:         "URL inválida; informe o endereço completo.",
	&validation.MsgExpectedRFC4122UUID:                "Identificador inválido.%.0v",
	&validation.MsgExpectedRFC6570URITemplate:         msgInvalid,
	&validation.MsgExpectedRFC6901JSONPointer:         msgInvalid,
	&validation.MsgExpectedRFC6901RelativeJSONPointer: msgInvalid,
	&validation.MsgExpectedRegexp:                     "Expressão regular inválida.%.0v",
	&validation.MsgExpectedMatchAtLeastOneSchema:      msgInvalid,
	&validation.MsgExpectedMatchExactlyOneSchema:      msgInvalid,
	&validation.MsgExpectedNotMatchSchema:             msgInvalid,
	&validation.MsgExpectedPropertyNameInObject:       msgInvalid,
	&validation.MsgExpectedBoolean:                    msgBoolean,
	&validation.MsgExpectedDuration:                   "Duração inválida.%.0v",
	&validation.MsgExpectedNumber:                     msgNumber,
	&validation.MsgExpectedInteger:                    msgInteger,
	&validation.MsgExpectedString:                     "Informe um texto.",
	&validation.MsgExpectedBase64String:               "Valor em base64 inválido.",
	&validation.MsgExpectedArray:                      "Informe uma lista.",
	&validation.MsgExpectedObject:                     "Informe um objeto.",
	&validation.MsgExpectedArrayItemsUnique:           "A lista não pode ter itens repetidos.",
	&validation.MsgExpectedOneOf:                      "Use um destes valores: %s.",
	&validation.MsgExpectedConst:                      "Use o valor %v.",
	&validation.MsgExpectedMinimumNumber:              "O valor mínimo é %v.",
	&validation.MsgExpectedExclusiveMinimumNumber:     "Informe um valor maior que %v.",
	&validation.MsgExpectedMaximumNumber:              "O valor máximo é %v.",
	&validation.MsgExpectedExclusiveMaximumNumber:     "Informe um valor menor que %v.",
	&validation.MsgExpectedNumberBeMultipleOf:         "Informe um múltiplo de %v.",
	&validation.MsgExpectedMinLength:                  "O tamanho mínimo é %d.",
	&validation.MsgExpectedMaxLength:                  "O tamanho máximo é %d.",
	&validation.MsgExpectedBePattern:                  "Formato inválido; use %s.",
	&validation.MsgExpectedMatchPattern:               "Formato inválido.%.0s",
	&validation.MsgExpectedMinItems:                   "Quantidade mínima de itens: %d.",
	&validation.MsgExpectedMaxItems:                   "Quantidade máxima de itens: %d.",
	&validation.MsgExpectedMinProperties:              "Quantidade mínima de campos: %d.",
	&validation.MsgExpectedMaxProperties:              "Quantidade máxima de campos: %d.",
	&validation.MsgExpectedRequiredProperty:           requiredMarker + "%s",
	&validation.MsgExpectedDependentRequiredProperty:  requiredMarker + "%s%.0s",
}

// translateValidationMessages must run before any operation is registered:
// huma formats the messages when it builds each schema.
func translateValidationMessages() {
	for target, message := range validationMessages {
		*target = message
	}
}

// fieldErrors groups huma's validation details by field. Details about the
// request as a whole are left out: the top-level message already covers them.
func fieldErrors(errs []error) map[string][]string {
	var fields map[string][]string
	for _, err := range errs {
		var detail *huma.ErrorDetail
		if !errors.As(err, &detail) {
			continue
		}
		field, message := describe(detail)
		if field == "" {
			continue
		}
		if fields == nil {
			fields = map[string][]string{}
		}
		fields[field] = append(fields[field], message)
	}
	return fields
}

func describe(detail *huma.ErrorDetail) (field, message string) {
	// Locations look like "body.items[0].title" or "query.page".
	_, field, _ = strings.Cut(detail.Location, ".")

	if name, ok := strings.CutPrefix(detail.Message, requiredMarker); ok {
		if field == "" {
			return name, msgRequired
		}
		return field + "." + name, msgRequired
	}
	if translated, ok := paramMessages[detail.Message]; ok {
		return field, translated
	}
	if strings.HasPrefix(detail.Message, "required ") && strings.HasSuffix(detail.Message, " parameter is missing") {
		return field, msgRequired
	}
	return field, detail.Message
}
