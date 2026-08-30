package config

import (
	"fmt"
	"os"
	"strconv"

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
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	expiryHours, err := strconv.Atoi(getEnv("JWT_EXPIRY_HOURS", "60"))
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_EXPIRY_HOURS: %w", err)
	}

	// Ключ подписи токенов обязателен и не имеет значения по умолчанию.
	//
	// Раньше здесь стояло «changeme»: сервер, поднятый без JWT_SECRET,
	// молча подписывал токены общеизвестной строкой — кто угодно мог
	// подписать себе токен главного экономиста и получить все проекты.
	// Отсутствие ключа должно ронять запуск, а не проходить незаметно.
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

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
