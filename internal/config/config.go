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
	VAPIDPublicKey    string
	VAPIDPrivateKey   string
	VAPIDSubject      string
}

func Load() *Config {
	// Critical: JWT_SECRET and ADMIN_PASS must be set via environment variables
	// No defaults are provided for security reasons
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		panic("JWT_SECRET environment variable is required")
	}

	adminPass := os.Getenv("ADMIN_PASS")
	if adminPass == "" {
		panic("ADMIN_PASS environment variable is required")
	}

	return &Config{
		Port:              getEnv("PORT", "9800"),
		DatabaseURL:       getEnv("DATABASE_URL", "postgres://chris@localhost/agent_im?sslmode=disable"),
		JWTSecret:         jwtSecret,
		AdminUser:         getEnv("ADMIN_USER", "chris"),
		AdminPass:         adminPass,
		ServerURL:         getEnv("SERVER_URL", "http://localhost:9800"),
		AutoApproveAgents: getEnv("AUTO_APPROVE_AGENTS", "") == "true",
		VAPIDPublicKey:    getEnv("VAPID_PUBLIC_KEY", ""),
		VAPIDPrivateKey:   getEnv("VAPID_PRIVATE_KEY", ""),
		VAPIDSubject:      getEnv("VAPID_SUBJECT", "mailto:admin@example.com"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
