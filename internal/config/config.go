package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds the application configuration
type Config struct {
	Server   ServerConfig
	Weaviate WeaviateConfig
	Ollama   OllamaConfig
}

// ServerConfig holds server-specific configuration
type ServerConfig struct {
	Port string
	Host string
}

// WeaviateConfig holds Weaviate-specific configuration
type WeaviateConfig struct {
	Scheme string
	Host   string
	APIKey string
}

// OllamaConfig holds Ollama-specific configuration
type OllamaConfig struct {
	URL string
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Port: getEnv("PORT", "8080"),
			Host: getEnv("HOST", ""),
		},
		Weaviate: WeaviateConfig{
			Scheme: getEnv("WEAVIATE_SCHEME", "http"),
			Host:   getEnv("WEAVIATE_HOST", "localhost:8000"),
			APIKey: getEnv("WEAVIATE_API_KEY", ""),
		},
		Ollama: OllamaConfig{
			URL: getEnv("OLLAMA_URL", "http://localhost:11434"),
		},
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate server port
	if c.Server.Port != "" {
		port, err := strconv.Atoi(c.Server.Port)
		if err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("invalid port: %s", c.Server.Port)
		}
	}

	// Validate Weaviate configuration
	if c.Weaviate.Host == "" {
		return fmt.Errorf("WEAVIATE_HOST is required")
	}

	if c.Weaviate.Scheme != "http" && c.Weaviate.Scheme != "https" {
		return fmt.Errorf("WEAVIATE_SCHEME must be http or https")
	}

	return nil
}

// getEnv gets an environment variable with a fallback default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
