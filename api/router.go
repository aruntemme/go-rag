package api

import (
	"github.com/gin-gonic/gin"
	// Import your handlers package if it were separate, e.g.:
	// "rag-go-app/api/handlers"
)

func SetupRoutes() *gin.Engine {
	r := gin.Default()
	// Middleware for logging, recovery, CORS etc. can be added here
	// r.Use(gin.Logger())
	// r.Use(gin.Recovery())
	// Example for CORS (needs import "github.com/gin-contrib/cors"):
	// config := cors.DefaultConfig()
	// config.AllowOrigins = []string{"http://localhost:3000"} // Adjust for your Electron app's origin
	// r.Use(cors.New(config))

	// Health check
	r.GET("/health", HealthHandler)

	// API v1 routes
	v1 := r.Group("/api/v1")
	{
		// Collection management
		v1.POST("/collections", CreateCollectionHandler)
		v1.GET("/collections", ListCollectionsHandler)
		v1.GET("/collections/:name", GetCollectionStatsHandler)
		v1.DELETE("/collections/:name", DeleteCollectionHandler)

		// Document management
		v1.POST("/documents", AddDocumentHandler)
		v1.GET("/collections/:name/documents", ListDocumentsHandler)
		v1.DELETE("/documents/:id", DeleteDocumentHandler)
		v1.DELETE("/collections/:name/documents", DeleteAllDocumentsHandler)

		// Query endpoints
		v1.POST("/query", QueryHandler)   // Full RAG with LLM generation
		v1.POST("/search", SearchHandler) // Search-only without LLM
		v1.POST("/analyze", AnalyzeDocumentHandler)

		// Chunking strategy comparison
		v1.POST("/compare-chunking", CompareChunkingHandler)
	}

	return r
}
