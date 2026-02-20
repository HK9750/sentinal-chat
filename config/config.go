package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	AppPort        string
	AppMode        string
	DBHost         string
	DBUser         string
	DBPassword     string
	DBName         string
	DBPort         string
	JWTSecret      string
	JWTExpiryHours int
	RefreshExpiry  int
	RedisHost      string
	RedisPort      string
	RedisPassword  string
	S3Region       string
	S3Bucket       string
	S3AccessKeyID  string
	S3SecretKey    string
	S3Endpoint     string
	S3PublicBase   string
	S3PresignTTL   int
}

func LoadConfig() *Config {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	return &Config{
		AppPort:        getEnv("APP_PORT", "8080"),
		AppMode:        getEnv("APP_MODE", "debug"),
		DBHost:         getEnv("DB_HOST", "localhost"),
		DBUser:         getEnv("DB_USER", "postgres"),
		DBPassword:     getEnv("DB_PASSWORD", "postgres"),
		DBName:         getEnv("DB_NAME", "sentinal_chat"),
		DBPort:         getEnv("DB_PORT", "5432"),
		JWTSecret:      getEnv("JWT_SECRET", "change-me"),
		JWTExpiryHours: getEnvAsInt("JWT_EXPIRY_HOURS", 12),
		RefreshExpiry:  getEnvAsInt("REFRESH_EXPIRY_DAYS", 14),
		RedisHost:      getEnv("REDIS_HOST", "localhost"),
		RedisPort:      getEnv("REDIS_PORT", "6379"),
		RedisPassword:  getEnv("REDIS_PASSWORD", ""),
		S3Region:       getEnv("S3_REGION", ""),
		S3Bucket:       getEnv("S3_BUCKET", ""),
		S3AccessKeyID:  getEnv("S3_ACCESS_KEY_ID", ""),
		S3SecretKey:    getEnv("S3_SECRET_ACCESS_KEY", ""),
		S3Endpoint:     getEnv("S3_ENDPOINT", ""),
		S3PublicBase:   getEnv("S3_PUBLIC_BASE_URL", ""),
		S3PresignTTL:   getEnvAsInt("S3_PRESIGN_TTL_SECONDS", 900),
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
