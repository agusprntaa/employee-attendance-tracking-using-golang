package config

import (
	"log"
	"os"
	"strconv"
)

type Config struct {
	Port        string
	DBHost      string
	DBPort      string
	DBUser      string
	DBPassword  string
	DBName      string
	JWTSecret   string
	Backend1URL string
}

func Load() *Config {
	cfg := &Config{
		Port:        getEnv("PORT", "3001"),
		DBHost:      getEnv("DB_HOST", "localhost"),
		DBPort:      getEnv("DB_PORT", "5432"),
		DBUser:      getEnv("DB_USER", "postgres"),
		DBPassword:  getEnv("DB_PASSWORD", "1234"),
		DBName:      getEnv("DB_NAME", "absensi_karyawan"),
		JWTSecret:   getEnv("JWT_SECRET", "SUPER_SECRET_KEY"),
		Backend1URL: getEnv("BACKEND1_URL", "http://localhost:3000"),
	}
	log.Printf("Config loaded: port=%s db=%s@%s:%s/%s", cfg.Port, cfg.DBUser, cfg.DBHost, cfg.DBPort, cfg.DBName)
	return cfg
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
