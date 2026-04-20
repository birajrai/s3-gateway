package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"s3server/auth"
)

type Config struct {
	Port          int
	Host          string
	Debug         bool
	DataDir       string
	AccessKeys    auth.Secrets
	PresignExpiry int
}

func Load() (*Config, error) {
	portStr := getEnv("S3_PORT", "8000")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("invalid port: %w", err)
	}

	debugStr := getEnv("S3_DEBUG", "false")
	debug := strings.ToLower(debugStr) == "true" || debugStr == "1"

	accessKey := getEnv("S3_ACCESS_KEY", "minioadmin")
	secretKey := getEnv("S3_SECRET_KEY", "minioadmin")

	accessKeys := auth.Secrets{
		accessKey: secretKey,
	}

	cfg := &Config{
		Port:          port,
		Host:          getEnv("S3_HOST", "0.0.0.0"),
		Debug:         debug,
		DataDir:       getEnv("S3_DATA_DIR", "./data"),
		AccessKeys:    accessKeys,
		PresignExpiry: 3600,
	}

	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data dir: %w", err)
	}

	auth.SetSecrets(accessKeys)

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
