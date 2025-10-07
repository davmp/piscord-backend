package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port      string
	MongoURI  string
	JWTSecret string
}

func Load() *Config {
	godotenv.Load()

	return &Config{
		Port:      getEnv("PORT", "8000"),
		MongoURI:  getEnv("MONGO_URI", "mongodb://localhost:27017/piscord"),
		JWTSecret: getEnv("JWT_SECRET", "your-secret-key"),
	}
}

func getEnv(key, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultVal
}
