package domain

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAreaColorNormalizes(t *testing.T) {
	c, err := NewAreaColor("  Saúde ", " #E08A5C ")

	require.NoError(t, err)
	assert.Equal(t, AreaColor{Area: "Saúde", Color: "#e08a5c"}, c)
}

func TestNewAreaColorRejects(t *testing.T) {
	for name, tc := range map[string]struct {
		area, color string
		want        error
	}{
		"empty area":   {"  ", "#e08a5c", ErrAreaRequired},
		"long area":    {strings.Repeat("a", MaxAreaLength+1), "#e08a5c", ErrAreaTooLong},
		"named color":  {"Saúde", "red", ErrInvalidColor},
		"short hex":    {"Saúde", "#e08", ErrInvalidColor},
		"missing hash": {"Saúde", "e08a5c", ErrInvalidColor},
		"with alpha":   {"Saúde", "#e08a5cff", ErrInvalidColor},
		"no color":     {"Saúde", "", ErrInvalidColor},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := NewAreaColor(tc.area, tc.color)

			assert.ErrorIs(t, err, tc.want)
		})
	}
}
