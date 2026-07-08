package config

import (
	"log"
	"os"
	"strconv"
)

type Config struct {
	JWTAccessSecret  string
	JWTRefreshSecret string
	HMACSecret       string
	QRSecret         string
	Port             string
	DBHost           string
	DBPort           string
	DBUser           string
	DBPassword       string
	DBName           string
	JWTSecret        string
	Backend1URL      string
}

const minSecretLen = 32

func Load() *Config {
	cfg := &Config{
		Port:             getEnv("PORT", "3001"),
		DBHost:           getEnv("DB_HOST", "localhost"),
		DBPort:           getEnv("DB_PORT", "5432"),
		DBUser:           getEnv("DB_USER", "postgres"),
		DBPassword:       getEnv("DB_PASSWORD", "1234"),
		DBName:           getEnv("DB_NAME", "absensi_karyawan"),
		JWTAccessSecret:  mustGetSecret("JWT_ACCESS_SECRET"),
		JWTRefreshSecret: mustGetSecret("JWT_REFRESH_SECRET"),
		HMACSecret:       mustGetSecret("HMAC_SECRET"),
		QRSecret:         mustGetSecret("QR_SECRET"),
		Backend1URL:      getEnv("BACKEND1_URL", "http://localhost:3000"),
	}

	validateNoDuplicateSecrets(cfg)

	log.Printf("Config loaded: port=%s db=%s@%s:%s/%s", cfg.Port, cfg.DBUser, cfg.DBHost, cfg.DBPort, cfg.DBName)
	return cfg
}

func mustGetSecret(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("FATAL: environment variable %s wajib diisi dan tidak boleh kosong", key)
	}
	if len(val) < minSecretLen {
		log.Fatalf("FATAL: environment variable %s terlalu pendek (min %d karakter, dapat %d)", key, minSecretLen, len(val))
	}
	return val
}

func validateNoDuplicateSecrets(cfg *Config) {
	secrets := map[string]string{
		"JWT_ACCESS_SECRET":  cfg.JWTAccessSecret,
		"JWT_REFRESH_SECRET": cfg.JWTRefreshSecret,
		"HMAC_SECRET":        cfg.HMACSecret,
		"QR_SECRET":          cfg.QRSecret,
	}
	seen := map[string]string{}
	for name, val := range secrets {
		if prev, ok := seen[val]; ok {
			log.Fatalf("FATAL: secret %s memiliki nilai yang sama persis dengan %s", name, prev)
		}
		seen[val] = name
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}
