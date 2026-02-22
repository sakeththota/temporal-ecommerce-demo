package config

import "os"

type Config struct {
	DatabaseURL     string
	Port            string
	OllamaURL       string
	TemporalAddress string
}

func Load() Config {
	return Config{
		DatabaseURL:     getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/embeddings?sslmode=disable"),
		Port:            getEnv("PORT", "8080"),
		OllamaURL:       getEnv("OLLAMA_URL", "http://localhost:11434"),
		TemporalAddress: getEnv("TEMPORAL_ADDRESS", "localhost:7233"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
