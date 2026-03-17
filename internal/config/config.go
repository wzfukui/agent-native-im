package config

import "os"

type Config struct {
	Port              string
	DatabaseURL       string
	JWTSecret         string
	JWTTTLHours       int
	AdminUser         string
	AdminPass         string
	ServerURL         string
	AutoApproveAgents bool
	VAPIDPublicKey    string
	VAPIDPrivateKey   string
	VAPIDSubject      string
	FileRetentionDays int
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
		JWTTTLHours:       getEnvInt("JWT_TTL_HOURS", 24),
		AdminUser:         getEnv("ADMIN_USER", "chris"),
		AdminPass:         adminPass,
		ServerURL:         getEnv("SERVER_URL", "http://localhost:9800"),
		AutoApproveAgents: getEnv("AUTO_APPROVE_AGENTS", "") == "true",
		VAPIDPublicKey:    getEnv("VAPID_PUBLIC_KEY", ""),
		VAPIDPrivateKey:   getEnv("VAPID_PRIVATE_KEY", ""),
		VAPIDSubject:      getEnv("VAPID_SUBJECT", "mailto:admin@example.com"),
		FileRetentionDays: getEnvInt("FILE_RETENTION_DAYS", 180),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n := 0
	for _, ch := range v {
		if ch < '0' || ch > '9' {
			return fallback
		}
		n = n*10 + int(ch-'0')
	}
	if n <= 0 {
		return fallback
	}
	return n
}
