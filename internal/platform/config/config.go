package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"github.com/caarlos0/env/v11"
)

type Environment string

const (
	Development Environment = "development"
	Production  Environment = "production"
)

type Config struct {
	Env      Environment `env:"APP_ENV" envDefault:"development"`
	Port     int         `env:"PORT" envDefault:"8080"`
	LogLevel slog.Level  `env:"LOG_LEVEL" envDefault:"info"`

	DatabaseURL string `env:"DATABASE_URL,required,notEmpty"`

	CORSAllowedOrigins []string `env:"CORS_ALLOWED_ORIGINS,required,notEmpty" envSeparator:","`

	// Project URL, e.g. https://<project>.supabase.co
	SupabaseURL string `env:"SUPABASE_URL,required,notEmpty"`
}

func Load() (Config, error) {
	cfg, err := env.ParseAs[Config]()
	if err != nil {
		return Config{}, fmt.Errorf("config: %w", err)
	}
	cfg.CORSAllowedOrigins = normalizeOrigins(cfg.CORSAllowedOrigins)
	if err := cfg.validate(); err != nil {
		return Config{}, fmt.Errorf("config: %w", err)
	}
	return cfg, nil
}

func (c Config) validate() error {
	if c.Env != Development && c.Env != Production {
		return fmt.Errorf("APP_ENV inválido %q: use %q ou %q", c.Env, Development, Production)
	}
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("PORT inválida %d", c.Port)
	}
	if err := c.validateOrigins(); err != nil {
		return err
	}
	return c.validateSupabaseURL()
}

// normalizeOrigins drops spaces around the commas and a trailing slash, which
// browsers never send in the Origin header.
func normalizeOrigins(origins []string) []string {
	var out []string
	for _, o := range origins {
		o = strings.TrimSuffix(strings.TrimSpace(o), "/")
		if o != "" {
			out = append(out, o)
		}
	}
	return out
}

// validateOrigins accepts http only in development: in production a plaintext
// page at an allowed origin could be tampered with to call the API.
func (c Config) validateOrigins() error {
	if len(c.CORSAllowedOrigins) == 0 {
		return errors.New("CORS_ALLOWED_ORIGINS vazia: informe a origem do frontend")
	}
	for _, o := range c.CORSAllowedOrigins {
		if o == "*" {
			return errors.New("CORS_ALLOWED_ORIGINS não aceita \"*\": informe a origem do frontend")
		}
		u, ok := parseHTTPURL(o)
		if !ok || u.Path != "" {
			return fmt.Errorf("origem inválida %q em CORS_ALLOWED_ORIGINS: use o formato https://dominio[:porta]", o)
		}
		if u.Scheme == "http" && c.Env == Production {
			return fmt.Errorf("origem inválida %q em CORS_ALLOWED_ORIGINS: use https em produção", o)
		}
	}
	return nil
}

// validateSupabaseURL accepts http only in development, for the local Supabase.
func (c Config) validateSupabaseURL() error {
	u, ok := parseHTTPURL(c.SupabaseURL)
	if !ok || (u.Path != "" && u.Path != "/") {
		return fmt.Errorf("SUPABASE_URL inválida %q: use só a URL do projeto, ex.: https://<projeto>.supabase.co", c.SupabaseURL)
	}
	if u.Scheme == "http" && c.Env == Production {
		return fmt.Errorf("SUPABASE_URL inválida %q: use https em produção", c.SupabaseURL)
	}
	return nil
}

// parseHTTPURL accepts an http or https URL with a host and no credentials,
// query or fragment; the path is left to the caller.
func parseHTTPURL(raw string) (*url.URL, bool) {
	u, err := url.Parse(raw)
	ok := err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != "" &&
		u.User == nil && u.RawQuery == "" && u.Fragment == ""
	return u, ok
}
