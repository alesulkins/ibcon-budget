package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Минимальная длина ключа подписи токенов. 32 символа — та же длина,
// что у ключа в .env: короче подбирается перебором.
const minJWTSecretLen = 32

type Config struct {
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	JWTSecret      string
	JWTExpiryHours int

	ServerPort string

	// AllowedOrigins — адреса, которым разрешены запросы из браузера
	// (ALLOWED_ORIGINS через запятую). Пусто — запросы только со своего
	// адреса, обычный рабочий случай; см. middleware.CORS.
	AllowedOrigins []string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	expiryHours, err := strconv.Atoi(getEnv("JWT_EXPIRY_HOURS", "60"))
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_EXPIRY_HOURS: %w", err)
	}

	// Ключ подписи токенов обязателен и не имеет значения по умолчанию.
	secret := os.Getenv("JWT_SECRET")
	if len(secret) < minJWTSecretLen {
		return nil, fmt.Errorf(
			"JWT_SECRET не задан или короче %d символов: сервер с предсказуемым "+
				"ключом подписи запускать нельзя", minJWTSecretLen)
	}

	return &Config{
		DBHost:         getEnv("DB_HOST", "localhost"),
		DBPort:         getEnv("DB_PORT", "5432"),
		DBUser:         getEnv("DB_USER", "ibcon"),
		DBPassword:     getEnv("DB_PASSWORD", "ibcon_secret"),
		DBName:         getEnv("DB_NAME", "ibcon_budget"),
		DBSSLMode:      getEnv("DB_SSLMODE", "disable"),
		AllowedOrigins: splitList(getEnv("ALLOWED_ORIGINS", "")),
		JWTSecret:      secret,
		JWTExpiryHours: expiryHours,
		ServerPort:     getEnv("SERVER_PORT", "8080"),
	}, nil
}

func (c *Config) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName, c.DBSSLMode,
	)
}

// splitList разбирает список через запятую, отбрасывая пробелы и пустые
// значения: «a, b,» — это два адреса, а не три.
func splitList(v string) []string {
	var out []string
	for _, part := range strings.Split(v, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
