package main

import (
	"log"
	"os"
	"os/signal"
	"rag-go-app/api"
	"rag-go-app/config"
	"syscall"
)

func main() {
	// Load configuration
	config.LoadConfig("config.json")
	log.Printf("Configuration loaded: Server will run on port %s", config.AppConfig.ServerPort)
	log.Printf("Vector DB path: %s", config.AppConfig.VectorDBPath)
	log.Printf("LlamaCPP Base URL: %s", config.AppConfig.LlamaCPPBaseURL)

	// Initialize services
	err := api.InitializeServices(config.AppConfig.VectorDBPath)
	if err != nil {
		log.Fatalf("Failed to initialize services: %v", err)
	}

	// Setup graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		log.Println("Shutting down gracefully...")
		api.Cleanup()
		os.Exit(0)
	}()

	// Setup and start router
	router := api.SetupRoutes()

	log.Printf("RAG server starting on port %s...", config.AppConfig.ServerPort)
	log.Println("Available endpoints:")
	log.Println("  GET  /health                           - Health check")
	log.Println("")
	log.Println("📚 Collection Management:")
	log.Println("  POST   /api/v1/collections             - Create collection")
	log.Println("  GET    /api/v1/collections             - List all collections")
	log.Println("  GET    /api/v1/collections/:name       - Get collection statistics")
	log.Println("  DELETE /api/v1/collections/:name       - Delete collection")
	log.Println("")
	log.Println("📄 Document Management:")
	log.Println("  POST   /api/v1/documents               - Add document")
	log.Println("  GET    /api/v1/collections/:name/documents - List documents in collection")
	log.Println("  DELETE /api/v1/documents/:id           - Delete specific document")
	log.Println("  DELETE /api/v1/collections/:name/documents - Delete all documents (requires ?confirm=true)")
	log.Println("")
	log.Println("🔍 Query & Analysis:")
	log.Println("  POST   /api/v1/query                   - Query documents")
	log.Println("  POST   /api/v1/analyze                 - Analyze document with metadata")
	log.Println("  POST   /api/v1/compare-chunking        - Compare chunking strategies")
	log.Println()
	log.Println("Enhanced features available:")
	log.Println("  ✓ Intelligent structural chunking with automatic section detection")
	log.Println("  ✓ Experience-aware job entry extraction")
	log.Println("  ✓ Semantic and sentence-window chunking strategies")
	log.Println("  ✓ Parent-child chunk relationships")
	log.Println("  ✓ Query expansion and advanced re-ranking")
	log.Println("  ✓ Metadata filtering and keyword extraction")
	log.Println("  ✓ Position-aware query enhancement")

	if err := router.Run(":" + config.AppConfig.ServerPort); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
