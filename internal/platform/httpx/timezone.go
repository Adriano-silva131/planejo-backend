package httpx

import (
	"time"

	"github.com/danielgtaylor/huma/v2"
)

// ParseTimeZone reads the Time-Zone header, an IANA zone name such as
// "America/Sao_Paulo". "Local" is refused: it would be the server's zone, not
// the user's.
func ParseTimeZone(name string) (*time.Location, error) {
	loc, err := time.LoadLocation(name)
	if err != nil || name == "" || name == "Local" {
		return nil, huma.Error422UnprocessableEntity("", &huma.ErrorDetail{
			Location: "header.Time-Zone",
			Message:  "Fuso horário inválido; use um nome como America/Sao_Paulo.",
		})
	}
	return loc, nil
}
