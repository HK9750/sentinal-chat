package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	AppPort            string
	AppMode            string
	DBHost             string
	DBUser             string
	DBPassword         string
	DBName             string
	DBPort             string
	JWTSecret          string
	JWTExpiryHours     int
	RefreshExpiry      int
	CookieSecure       bool
	CookieDomain       string
	FrontendURL        string
	RedisHost          string
	RedisPort          string
	RedisPassword      string
	S3Region           string
	S3Bucket           string
	S3AccessKeyID      string
	S3SecretKey        string
	S3Endpoint         string
	S3PublicBase       string
	S3PresignTTL       int
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURI  string
	GithubClientID     string
	GithubClientSecret string
	GithubRedirectURI  string
}

func LoadConfig() *Config {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	appMode := getEnv("APP_MODE", "debug")

	return &Config{
		AppPort:            getEnv("APP_PORT", "8080"),
		AppMode:            appMode,
		DBHost:             getEnv("DB_HOST", "localhost"),
		DBUser:             getEnv("DB_USER", "postgres"),
		DBPassword:         getEnv("DB_PASSWORD", "postgres"),
		DBName:             getEnv("DB_NAME", "sentinal_chat"),
		DBPort:             getEnv("DB_PORT", "5432"),
		JWTSecret:          getEnv("JWT_SECRET", "change-me"),
		JWTExpiryHours:     getEnvAsInt("JWT_EXPIRY_HOURS", 1),
		RefreshExpiry:      getEnvAsInt("REFRESH_EXPIRY_DAYS", 14),
		CookieSecure:       getEnvAsBool("COOKIE_SECURE", appMode == "release"),
		CookieDomain:       getEnv("COOKIE_DOMAIN", ""),
		FrontendURL:        getEnv("FRONTEND_URL", "http://localhost:3000"),
		RedisHost:          getEnv("REDIS_HOST", "localhost"),
		RedisPort:          getEnv("REDIS_PORT", "6379"),
		RedisPassword:      getEnv("REDIS_PASSWORD", ""),
		S3Region:           getEnv("S3_REGION", ""),
		S3Bucket:           getEnv("S3_BUCKET", ""),
		S3AccessKeyID:      getEnv("S3_ACCESS_KEY_ID", ""),
		S3SecretKey:        getEnv("S3_SECRET_ACCESS_KEY", ""),
		S3Endpoint:         getEnv("S3_ENDPOINT", ""),
		S3PublicBase:       getEnv("S3_PUBLIC_BASE_URL", ""),
		S3PresignTTL:       getEnvAsInt("S3_PRESIGN_TTL_SECONDS", 900),
		GoogleClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
		GoogleRedirectURI:  getEnv("GOOGLE_REDIRECT_URI", ""),
		GithubClientID:     getEnv("GITHUB_CLIENT_ID", ""),
		GithubClientSecret: getEnv("GITHUB_CLIENT_SECRET", ""),
		GithubRedirectURI:  getEnv("GITHUB_REDIRECT_URI", ""),
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

func getEnvAsBool(key string, fallback bool) bool {
	value := getEnv(key, "")
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
