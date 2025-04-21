package config

import (
	"os"
)

type Config struct {
	Environment string
	ServerPort  string
	DBHost      string
	DBPort      int
	DBUser      string
	DBPassword  string
	DBName      string
	JWTSecret   string
}

func Load() (*Config, error) {
	return &Config{
		Environment: getEnv("ENVIRONMENT", "development"),
		ServerPort:  getEnv("PORT", "8080"),
		DBHost:      getEnv("DB_HOST", "localhost"),
		DBPort:      5432,
		DBUser:      getEnv("DB_USER", "annamugu"),
		DBPassword:  getEnv("DB_PASSWORD", ""),
		DBName:      getEnv("DB_NAME", "unicorn_app"),
		JWTSecret:   getEnv("JWT_SECRET", "your-secret-key"),
	}, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
