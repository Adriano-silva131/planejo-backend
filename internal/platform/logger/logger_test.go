package logger

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Adriano-silva131/planejo-backend/internal/platform/config"
)

func TestNewWritesJSONInProduction(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, config.Production, slog.LevelInfo)

	log.Info("hello", "port", 8080)

	var line map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &line))
	assert.Equal(t, "hello", line["msg"])
	assert.InDelta(t, 8080, line["port"], 0)
}

func TestNewWritesTextInDevelopment(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, config.Development, slog.LevelInfo)

	log.Info("hello", "port", 8080)

	assert.Contains(t, buf.String(), "msg=hello port=8080")
}

func TestNewAddsAttrsFromContext(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, config.Development, slog.LevelInfo)
	ctx := With(t.Context(), slog.String("requestId", "abc"))
	ctx = With(ctx, slog.String("userId", "u1"))

	log.InfoContext(ctx, "hello")
	log.InfoContext(t.Context(), "plain")

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	require.Len(t, lines, 2)
	assert.Contains(t, lines[0], "requestId=abc userId=u1")
	assert.NotContains(t, lines[1], "requestId")
}

func TestNewFiltersBelowLevel(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, config.Development, slog.LevelWarn)

	log.Info("ignored")
	log.Warn("kept")

	assert.NotContains(t, buf.String(), "ignored")
	assert.Contains(t, buf.String(), "kept")
}
