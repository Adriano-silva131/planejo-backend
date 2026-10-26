package domainerr

type Kind int

const (
	NotFound Kind = iota + 1
	Conflict
	Invalid
)

// Error is a business rule failure the user can act on. Code is stable so
// clients can branch on it; Message is shown to the user in pt-BR.
type Error struct {
	Kind    Kind
	Code    string
	Message string
}

func New(kind Kind, code, message string) *Error {
	return &Error{Kind: kind, Code: code, Message: message}
}

func (e *Error) Error() string {
	return e.Code
}
