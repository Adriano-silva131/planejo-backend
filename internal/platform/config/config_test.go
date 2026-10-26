package config

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testDatabaseURL = "postgres://localhost:5432/db"
	testOrigin      = "http://localhost:5173"
	testSupabaseURL = "https://test.supabase.co"
)

func setRequired(t *testing.T) {
	t.Setenv("DATABASE_URL", testDatabaseURL)
	t.Setenv("CORS_ALLOWED_ORIGINS", testOrigin)
	t.Setenv("SUPABASE_URL", testSupabaseURL)
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("PORT", "")
	t.Setenv("LOG_LEVEL", "")
	setRequired(t)

	cfg, err := Load()

	require.NoError(t, err)
	assert.Equal(t, Development, cfg.Env)
	assert.Equal(t, 8080, cfg.Port)
	assert.Equal(t, slog.LevelInfo, cfg.LogLevel)
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("PORT", "9000")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("DATABASE_URL", testDatabaseURL)
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://planejoai.com,https://www.planejoai.com")
	t.Setenv("SUPABASE_URL", testSupabaseURL)

	cfg, err := Load()

	require.NoError(t, err)
	assert.Equal(t, Production, cfg.Env)
	assert.Equal(t, 9000, cfg.Port)
	assert.Equal(t, slog.LevelDebug, cfg.LogLevel)
	assert.Equal(t, testDatabaseURL, cfg.DatabaseURL)
	assert.Equal(t, []string{"https://planejoai.com", "https://www.planejoai.com"}, cfg.CORSAllowedOrigins)
	assert.Equal(t, testSupabaseURL, cfg.SupabaseURL)
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	cases := map[string]struct{ key, value string }{
		"environment":      {"APP_ENV", "staging"},
		"port text":        {"PORT", "abc"},
		"port range":       {"PORT", "70000"},
		"log level":        {"LOG_LEVEL", "verbose"},
		"database url":     {"DATABASE_URL", ""},
		"cors empty":       {"CORS_ALLOWED_ORIGINS", ""},
		"cors any":         {"CORS_ALLOWED_ORIGINS", "*"},
		"cors commas":      {"CORS_ALLOWED_ORIGINS", " , "},
		"cors no scheme":   {"CORS_ALLOWED_ORIGINS", "planejoai.com"},
		"cors path":        {"CORS_ALLOWED_ORIGINS", "https://planejoai.com/app"},
		"cors any in list": {"CORS_ALLOWED_ORIGINS", "https://planejoai.com,*"},
		"supabase":         {"SUPABASE_URL", ""},
		"supabase url":     {"SUPABASE_URL", "test.supabase.co"},
		"supabase ftp":     {"SUPABASE_URL", "ftp://test.supabase.co"},
		"supabase path":    {"SUPABASE_URL", testSupabaseURL + "/auth/v1"},
		"supabase query":   {"SUPABASE_URL", testSupabaseURL + "?x=1"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			setRequired(t)
			t.Setenv(tc.key, tc.value)

			_, err := Load()

			assert.Error(t, err)
		})
	}
}

func TestSupabaseURLAllowsHTTPOnlyInDevelopment(t *testing.T) {
	setRequired(t)
	t.Setenv("SUPABASE_URL", "http://127.0.0.1:54321")

	t.Setenv("APP_ENV", "development")
	_, devErr := Load()
	t.Setenv("APP_ENV", "production")
	_, prodErr := Load()

	assert.NoError(t, devErr)
	assert.Error(t, prodErr)
}

func TestSupabaseURLAcceptsTrailingSlash(t *testing.T) {
	setRequired(t)
	t.Setenv("SUPABASE_URL", testSupabaseURL+"/")

	_, err := Load()

	assert.NoError(t, err)
}

func TestCORSOriginsAreNormalized(t *testing.T) {
	setRequired(t)
	t.Setenv("CORS_ALLOWED_ORIGINS", " https://planejoai.com/ , https://www.planejoai.com,")

	cfg, err := Load()

	require.NoError(t, err)
	assert.Equal(t, []string{"https://planejoai.com", "https://www.planejoai.com"}, cfg.CORSAllowedOrigins)
}

func TestCORSOriginsAllowHTTPOnlyInDevelopment(t *testing.T) {
	setRequired(t)
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://planejoai.com,http://localhost:5173")

	t.Setenv("APP_ENV", "development")
	_, devErr := Load()
	t.Setenv("APP_ENV", "production")
	_, prodErr := Load()

	assert.NoError(t, devErr)
	assert.Error(t, prodErr)
}
