package config

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	DatabaseURL string

	JWTPrivateKey   *rsa.PrivateKey
	JWTPublicKey    *rsa.PublicKey
	AccessTokenTTL  time.Duration // 15 min
	RefreshTokenTTL time.Duration // 7 days

	Port string

	CORSOrigins []string
}

func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL:     requireEnv("DATABASE_URL"),
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 7 * 24 * time.Hour,
		Port:            getEnvOr("PORT", "8080"),
	}

	// Parse RSA keys from base64-encoded PEM (set via k8s secret / .env)
	privateKey, err := loadRSAPrivateKey(requireEnv("JWT_PRIVATE_KEY_B64"))
	if err != nil {
		return nil, fmt.Errorf("load JWT private key: %w", err)
	}
	cfg.JWTPrivateKey = privateKey
	cfg.JWTPublicKey = &privateKey.PublicKey

	// Optional: override token TTLs
	if v := os.Getenv("ACCESS_TOKEN_TTL_MINUTES"); v != "" {
		mins, err := strconv.Atoi(v)
		if err == nil {
			cfg.AccessTokenTTL = time.Duration(mins) * time.Minute
		}
	}

	cfg.CORSOrigins = []string{
		getEnvOr("CORS_ORIGIN_1", "https://fin.jcrlabs.net"),
		getEnvOr("CORS_ORIGIN_2", "https://fin-test.jcrlabs.net"),
	}
	if dev := os.Getenv("CORS_ORIGIN_DEV"); dev != "" {
		cfg.CORSOrigins = append(cfg.CORSOrigins, dev)
	}

	return cfg, nil
}

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic(fmt.Sprintf("required environment variable %q is not set", key))
	}
	return v
}

func getEnvOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func loadRSAPrivateKey(b64 string) (*rsa.PrivateKey, error) {
	pemBytes, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %w", err)
	}
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		// Try PKCS1 as fallback
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("key is not RSA")
	}
	return rsaKey, nil
}
