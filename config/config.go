package config

import (
	"os"
)

type Config struct {
	Port      string
	MongoURI  string
	JWTSecret string
}

func Load() *Config {
	return &Config{
		Port:      getEnv("PORT", ""),
		MongoURI:  getEnv("MONGO_URI", ""),
		JWTSecret: getEnv("JWT_SECRET", ""),
	}
}

func getEnv(key, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultVal
}
