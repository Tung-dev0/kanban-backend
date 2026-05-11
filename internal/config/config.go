package config

import (
	"os"
	"strings"
	"time"
)

type Config struct {
	Addr               string
	DatabaseURL        string
	MigrationsDir      string
	JWTSecret          []byte
	JWTTTL             time.Duration
	AllowOrigins       []string
	FrontendURL        string
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string
}

func Load() Config {
	origins := envOr("CORS_ORIGIN", "http://localhost:5173")
	return Config{
		Addr:               envOr("ADDR", ":8080"),
		DatabaseURL:        envOr("DATABASE_URL", "postgres://todo:todo@localhost:5432/todo?sslmode=disable"),
		MigrationsDir:      envOr("MIGRATIONS_DIR", "migrations"),
		JWTSecret:          []byte(envOr("JWT_SECRET", "dev-only-change-me-please-32bytes-min")),
		JWTTTL:             24 * time.Hour,
		AllowOrigins:       splitCSV(origins),
		FrontendURL:        envOr("FRONTEND_URL", "http://localhost:5173"),
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURL:  envOr("GOOGLE_REDIRECT_URL", "http://localhost:8080/api/auth/google/callback"),
	}
}

func (c Config) OAuthEnabled() bool {
	return c.GoogleClientID != "" && c.GoogleClientSecret != ""
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
