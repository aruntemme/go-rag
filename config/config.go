package config

import (
	"encoding/json"
	"log"
	"os"
)

type Config struct {
	ServerPort      string `json:"server_port"`
	LlamaCPPBaseURL string `json:"llamacpp_base_url"`
	EmbeddingModel  string `json:"embedding_model"`
	ChatModel       string `json:"chat_model"`
	VectorDBPath    string `json:"vector_db_path"` // For SQLite
	DefaultTopK     int    `json:"default_top_k"`
}

var AppConfig Config

func LoadConfig(path string) error {
	file, err := os.ReadFile(path)
	if err != nil {
		// Fallback to default config if file not found or error
		log.Println("Config file not found or error reading, using default config:", err)
		AppConfig = DefaultConfig()
		return nil // Or return err if config file is mandatory
	}
	err = json.Unmarshal(file, &AppConfig)
	if err != nil {
		log.Println("Error unmarshalling config, using default config:", err)
		AppConfig = DefaultConfig()
		return err
	}
	return nil
}

func DefaultConfig() Config {
	return Config{
		ServerPort:      "8080",                     // Gin server port
		LlamaCPPBaseURL: "http://localhost:8091/v1", // Your OpenAI-compatible API
		EmbeddingModel:  "nomic-embed-text-v1.5",    // Specify model if LlamaCPP needs it
		ChatModel:       "qwen3:8b",                 // Specify model for LlamaCPP
		VectorDBPath:    "./rag_database.db",
		DefaultTopK:     3,
	}
}
