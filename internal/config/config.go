package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Cache    CacheConfig
	Auth     AuthConfig
	JWT      JWTConfig
	Storage  StorageConfig
}

type ServerConfig struct {
	Port            string
	Mode            string
	ReadTimeout     int
	WriteTimeout    int
	ShutdownTimeout int
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
	MaxIdle  int
	MaxOpen  int
}

type CacheConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
	Enabled  bool
}

type AuthConfig struct {
	Username string
	Password string
}

type JWTConfig struct {
	Secret      string
	ExpiryHours int
}

type StorageConfig struct {
	Endpoint        string // S3-compatible endpoint, e.g. "https://is3.cloudhost.id" — leave empty for real AWS
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	PublicBaseURL   string // base URL for public file access, e.g. "https://bucket.is3.cloudhost.id"
	ForcePathStyle  bool   // true for most non-AWS providers (idcloudhost, Minio, etc.)
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, reading from environment")
	}

	return &Config{
		Server: ServerConfig{
			Port:            getEnv("SERVER_PORT", "8080"),
			Mode:            getEnv("GIN_MODE", "debug"),
			ReadTimeout:     getEnvInt("SERVER_READ_TIMEOUT", 30),
			WriteTimeout:    getEnvInt("SERVER_WRITE_TIMEOUT", 30),
			ShutdownTimeout: getEnvInt("SERVER_SHUTDOWN_TIMEOUT", 5),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", ""),
			Name:     getEnv("DB_NAME", "pawonwarga"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
			MaxIdle:  getEnvInt("DB_MAX_IDLE_CONNS", 10),
			MaxOpen:  getEnvInt("DB_MAX_OPEN_CONNS", 100),
		},
		Cache: CacheConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnv("REDIS_PORT", "6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvInt("REDIS_DB", 0),
			Enabled:  getEnvBool("REDIS_ENABLED", true),
		},
		Auth: AuthConfig{
			Username: getEnv("AUTH_USERNAME", "admin"),
			Password: getEnv("AUTH_PASSWORD", "secret"),
		},
		JWT: JWTConfig{
			Secret:      getEnv("JWT_SECRET", "change-this-secret-in-production"),
			ExpiryHours: getEnvInt("JWT_EXPIRY_HOURS", 24),
		},
		Storage: StorageConfig{
			Endpoint:        getEnv("STORAGE_ENDPOINT", ""),
			Region:          getEnv("STORAGE_REGION", "us-east-1"),
			Bucket:          getEnv("STORAGE_BUCKET", ""),
			AccessKeyID:     getEnv("STORAGE_ACCESS_KEY_ID", ""),
			SecretAccessKey: getEnv("STORAGE_SECRET_ACCESS_KEY", ""),
			PublicBaseURL:   getEnv("STORAGE_PUBLIC_BASE_URL", ""),
			ForcePathStyle:  getEnvBool("STORAGE_FORCE_PATH_STYLE", false),
		},
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}
	return n
}

func getEnvBool(key string, defaultVal bool) bool {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	b, err := strconv.ParseBool(val)
	if err != nil {
		return defaultVal
	}
	return b
}
