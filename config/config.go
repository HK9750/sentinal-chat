package config

import (
	"os"
	"strconv"
)

type Config struct {
	AppPort       string
	DBHost        string
	DBUser        string
	DBPassword    string
	DBName        string
	DBPort        string
	RedisHost     string
	RedisPort     string
	RedisPassword string
}

func LoadConfig() *Config {
	return &Config{
		AppPort:       getEnv("APP_PORT", "8080"),
		DBHost:        getEnv("DB_HOST", "localhost"),
		DBUser:        getEnv("DB_USER", "postgres"),
		DBPassword:    getEnv("DB_PASSWORD", "postgres"),
		DBName:        getEnv("DB_NAME", "sentinal_chat"),
		DBPort:        getEnv("DB_PORT", "5432"),
		RedisHost:     getEnv("REDIS_HOST", "localhost"),
		RedisPort:     getEnv("REDIS_PORT", "6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func getEnvAsInt(key string, fallback int) int {
	valueStr := getEnv(key, "")
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return fallback
}
