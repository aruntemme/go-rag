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
		ServerPort:      ":8080",                     // Gin server port
		LlamaCPPBaseURL: "http://localhost:8091/v1",  // Your LlamaCPP OpenAI-compatible API
		EmbeddingModel:  "your-embedding-model-name", // Specify model if LlamaCPP needs it
		ChatModel:       "your-chat-model-name",      // Specify model for LlamaCPP
		VectorDBPath:    "./rag_database.db",
		DefaultTopK:     3,
	}
}

func init() {
	// Load config on package initialization or explicitly in main
	if err := LoadConfig("config.json"); err != nil {
		log.Printf("Warning: Could not load config.json, using default values. Error: %v", err)
	}
	if AppConfig.LlamaCPPBaseURL == "" { // Ensure defaults are set if file was empty/partial
		AppConfig = DefaultConfig()
		log.Println("AppConfig was not fully loaded, applied defaults.")
	}
}
