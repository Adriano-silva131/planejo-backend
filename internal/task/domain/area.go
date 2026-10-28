package domain

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// AreaColor is the color the user chose for an area, as #rrggbb. Every task
// in the area uses it; clients derive the shades of each theme from it.
type AreaColor struct {
	Area  string
	Color string
}

var hexColor = regexp.MustCompile(`^#[0-9a-f]{6}$`)

func NewAreaColor(area, color string) (AreaColor, error) {
	area = strings.TrimSpace(area)
	color = strings.ToLower(strings.TrimSpace(color))
	switch {
	case area == "":
		return AreaColor{}, ErrAreaRequired
	case utf8.RuneCountInString(area) > MaxAreaLength:
		return AreaColor{}, ErrAreaTooLong
	case !hexColor.MatchString(color):
		return AreaColor{}, ErrInvalidColor
	}
	return AreaColor{Area: area, Color: color}, nil
}
