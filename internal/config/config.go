package config

import "os"

type Config struct {
	Port              string
	DatabaseURL       string
	JWTSecret         string
	AdminUser         string
	AdminPass         string
	ServerURL         string
	AutoApproveAgents bool
}

func Load() *Config {
	return &Config{
		Port:              getEnv("PORT", "9800"),
		DatabaseURL:       getEnv("DATABASE_URL", "postgres://chris@localhost/agent_im?sslmode=disable"),
		JWTSecret:         getEnv("JWT_SECRET", "dev-secret-change-in-production"),
		AdminUser:         getEnv("ADMIN_USER", "chris"),
		AdminPass:         getEnv("ADMIN_PASS", "admin123"),
		ServerURL:         getEnv("SERVER_URL", "http://localhost:9800"),
		AutoApproveAgents: getEnv("AUTO_APPROVE_AGENTS", "") == "true",
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
