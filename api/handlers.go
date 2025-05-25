package api

import (
	"fmt"
	"log"
	"net/http"
	"rag-go-app/core"
	"rag-go-app/models"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	vectorDB   *core.VectorDB
	ragService *core.RAGService
)

func InitializeServices(dbPath string) error {
	var err error

	// Initialize vector database
	vectorDB, err = core.NewVectorDB(dbPath)
	if err != nil {
		return fmt.Errorf("failed to initialize vector database: %w", err)
	}

	// Initialize services
	embeddingService := core.NewEmbeddingService()
	llmService := core.NewLLMService()
	ragService = core.NewRAGService(vectorDB, embeddingService, llmService)

	log.Println("Services initialized successfully")
	return nil
}

func CreateCollectionHandler(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := vectorDB.CreateCollection(req.Name, req.Description)
	if err != nil {
		log.Printf("Error creating collection: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create collection"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":     "Collection created successfully",
		"name":        req.Name,
		"description": req.Description,
	})
}

func AddDocumentHandler(c *gin.Context) {
	var req models.AddDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Set default chunking strategy if none provided
	if req.ChunkingConfig == nil {
		req.ChunkingConfig = &models.ChunkingConfig{
			Strategy:           models.StructuralStrategy,
			FixedSize:          500,
			Overlap:            50,
			MinChunkSize:       100,
			MaxChunkSize:       2000,
			PreserveParagraphs: true,
			ExtractKeywords:    true,
		}
	}

	// Document type is stored for metadata but doesn't affect chunking strategy
	// All documents use the configured or default strategy

	err := ragService.AddDocument(req.CollectionName, &req)
	if err != nil {
		log.Printf("Error adding document to collection %s: %v", req.CollectionName, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add document"})
		return
	}

	response := gin.H{
		"message":           "Document added successfully",
		"collection_name":   req.CollectionName,
		"chunking_strategy": string(req.ChunkingConfig.Strategy),
	}

	if req.Source != "" {
		response["source"] = req.Source
	}
	if req.FilePath != "" {
		response["file_path"] = req.FilePath
	}

	c.JSON(http.StatusCreated, response)
}

func QueryHandler(c *gin.Context) {
	var req models.QueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Set defaults for enhanced features
	if req.TopK <= 0 {
		req.TopK = 5
	}

	response, err := ragService.Query(&req)
	if err != nil {
		log.Printf("Error processing query for collection %s: %v", req.CollectionName, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process query"})
		return
	}

	c.JSON(http.StatusOK, response)
}

// SearchHandler performs only retrieval without LLM generation
// Returns all context and metadata needed for external LLM processing
func SearchHandler(c *gin.Context) {
	var req models.QueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Set defaults
	if req.TopK <= 0 {
		req.TopK = 5
	}

	startTime := time.Now()

	// Use the original query (query expansion disabled for search-only mode)
	query := req.Query

	// Generate query embedding
	embeddingClient := core.NewEmbeddingService()
	queryEmbedding, err := embeddingClient.GetEmbedding(query)
	if err != nil {
		log.Printf("Error generating query embedding: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate query embedding"})
		return
	}

	// Build metadata filters
	filters := make(map[string]interface{})
	for key, value := range req.MetadataFilters {
		filters[key] = value
	}

	// Search for similar chunks
	chunks, scores, err := vectorDB.QuerySimilarChunks(
		req.CollectionName,
		queryEmbedding,
		req.TopK*2, // Get more for potential re-ranking
		filters,
	)
	if err != nil {
		log.Printf("Error searching similar chunks: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search similar chunks"})
		return
	}

	if len(chunks) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"query":           req.Query,
			"expanded_query":  query,
			"collection_name": req.CollectionName,
			"chunks_found":    0,
			"chunks":          []interface{}{},
			"context":         "",
			"processing_time": time.Since(startTime).Seconds(),
			"metadata": gin.H{
				"semantic_threshold": req.SemanticThreshold,
				"metadata_filters":   req.MetadataFilters,
				"query_expansion":    req.QueryExpansion,
				"include_parents":    req.IncludeParents,
				"reranker_enabled":   req.RerankerEnabled,
			},
		})
		return
	}

	// Apply semantic threshold filtering
	if req.SemanticThreshold > 0 {
		filteredChunks := make([]*models.EnhancedChunk, 0)
		filteredScores := make([]float64, 0)

		for i, score := range scores {
			if score >= req.SemanticThreshold {
				filteredChunks = append(filteredChunks, chunks[i])
				filteredScores = append(filteredScores, score)
			}
		}

		chunks = filteredChunks
		scores = filteredScores

		if len(chunks) == 0 {
			c.JSON(http.StatusOK, gin.H{
				"query":           req.Query,
				"expanded_query":  query,
				"collection_name": req.CollectionName,
				"chunks_found":    0,
				"chunks":          []interface{}{},
				"context":         "",
				"message":         "No chunks met the semantic similarity threshold",
				"processing_time": time.Since(startTime).Seconds(),
				"metadata": gin.H{
					"semantic_threshold": req.SemanticThreshold,
					"metadata_filters":   req.MetadataFilters,
					"query_expansion":    req.QueryExpansion,
					"include_parents":    req.IncludeParents,
					"reranker_enabled":   req.RerankerEnabled,
				},
			})
			return
		}
	}

	// Note: Advanced features like parent inclusion and re-ranking
	// are available in the full query endpoint (/api/v1/query)
	// This search endpoint provides basic retrieval functionality

	// Limit to TopK results
	if len(chunks) > req.TopK {
		chunks = chunks[:req.TopK]
		scores = scores[:req.TopK]
	}

	// Prepare response with detailed chunk information
	responseChunks := make([]gin.H, len(chunks))
	for i, chunk := range chunks {
		chunkInfo := gin.H{
			"id":               chunk.ID,
			"document_id":      chunk.DocumentID,
			"text":             chunk.Text,
			"section":          chunk.Section,
			"subsection":       chunk.Subsection,
			"chunk_type":       chunk.ChunkType,
			"start_pos":        chunk.StartPos,
			"end_pos":          chunk.EndPos,
			"chunk_index":      chunk.ChunkIndex,
			"keywords":         chunk.Keywords,
			"confidence":       chunk.Confidence,
			"similarity_score": scores[i],
		}

		// Add parent/child relationship info
		if chunk.ParentChunkID != nil {
			chunkInfo["parent_chunk_id"] = *chunk.ParentChunkID
		}
		if len(chunk.ChildChunkIDs) > 0 {
			chunkInfo["child_chunk_ids"] = chunk.ChildChunkIDs
		}

		// Add metadata if available
		if chunk.Metadata != nil {
			chunkInfo["metadata"] = chunk.Metadata
		}

		responseChunks[i] = chunkInfo
	}

	// Prepare context string (concatenated chunks for easy LLM processing)
	contextStrings := make([]string, len(chunks))
	for i, chunk := range chunks {
		contextStrings[i] = fmt.Sprintf("Context %d: %s", i+1, chunk.Text)
	}
	context := strings.Join(contextStrings, "\n\n")

	// Build comprehensive response
	response := gin.H{
		"query":           req.Query,
		"expanded_query":  query,
		"collection_name": req.CollectionName,
		"chunks_found":    len(chunks),
		"chunks":          responseChunks,
		"context":         context,
		"context_strings": contextStrings, // Alternative format for easier processing
		"processing_time": time.Since(startTime).Seconds(),
		"metadata": gin.H{
			"semantic_threshold": req.SemanticThreshold,
			"metadata_filters":   req.MetadataFilters,
			"filters_applied":    len(req.MetadataFilters) > 0,
			"note":               "Advanced features available in /api/v1/query endpoint",
		},
	}

	// Add statistics
	if len(scores) > 0 {
		minScore := scores[0]
		maxScore := scores[0]
		totalScore := 0.0
		for _, score := range scores {
			if score < minScore {
				minScore = score
			}
			if score > maxScore {
				maxScore = score
			}
			totalScore += score
		}

		response["score_statistics"] = gin.H{
			"min_similarity": minScore,
			"max_similarity": maxScore,
			"avg_similarity": totalScore / float64(len(scores)),
			"total_scores":   len(scores),
		}
	}

	c.JSON(http.StatusOK, response)
}

// Enhanced query endpoint with chunking strategy analysis
func AnalyzeDocumentHandler(c *gin.Context) {
	var req struct {
		CollectionName string `json:"collection_name" binding:"required"`
		Query          string `json:"query" binding:"required"`
		ShowMetadata   bool   `json:"show_metadata"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Query with metadata and enhanced features enabled
	queryReq := &models.QueryRequest{
		CollectionName:    req.CollectionName,
		Query:             req.Query,
		TopK:              10,
		RerankerEnabled:   true,
		IncludeParents:    true,
		QueryExpansion:    true,
		SemanticThreshold: 0.1,
	}

	response, err := ragService.Query(queryReq)
	if err != nil {
		log.Printf("Error analyzing document for collection %s: %v", req.CollectionName, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to analyze document"})
		return
	}

	// Prepare analysis response
	analysis := gin.H{
		"query":                  req.Query,
		"answer":                 response.Answer,
		"processing_time":        response.ProcessingTime,
		"chunks_found":           len(response.EnhancedChunks),
		"reranking_applied":      len(response.RerankedScores) > 0,
		"parent_chunks_included": queryReq.IncludeParents,
		"query_expansion":        queryReq.QueryExpansion,
	}

	if req.ShowMetadata && response.EnhancedChunks != nil {
		chunkAnalysis := make([]gin.H, 0, len(response.EnhancedChunks))

		for i, chunk := range response.EnhancedChunks {
			chunkInfo := gin.H{
				"chunk_id":         chunk.ID,
				"chunk_type":       chunk.ChunkType,
				"section":          chunk.Section,
				"subsection":       chunk.Subsection,
				"text_length":      len(chunk.Text),
				"keywords":         chunk.Keywords,
				"similarity_score": response.SimilarityScores[i],
			}

			if len(response.RerankedScores) > i {
				chunkInfo["reranked_score"] = response.RerankedScores[i]
			}

			if chunk.ParentChunkID != nil {
				chunkInfo["has_parent"] = true
				chunkInfo["parent_chunk_id"] = *chunk.ParentChunkID
			}

			if len(chunk.ChildChunkIDs) > 0 {
				chunkInfo["child_count"] = len(chunk.ChildChunkIDs)
			}

			chunkAnalysis = append(chunkAnalysis, chunkInfo)
		}

		analysis["chunk_analysis"] = chunkAnalysis
	}

	c.JSON(http.StatusOK, analysis)
}

// Endpoint to test different chunking strategies
func CompareChunkingHandler(c *gin.Context) {
	var req struct {
		Content    string                    `json:"content" binding:"required"`
		DocType    string                    `json:"doc_type"`
		Strategies []models.ChunkingStrategy `json:"strategies"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(req.Strategies) == 0 {
		req.Strategies = []models.ChunkingStrategy{
			models.FixedSizeStrategy,
			models.StructuralStrategy,
			models.SemanticStrategy,
			models.SentenceWindowStrategy,
			models.ParentDocumentStrategy,
		}
	}

	results := make([]gin.H, 0, len(req.Strategies))

	for _, strategy := range req.Strategies {
		config := &models.ChunkingConfig{
			Strategy:           strategy,
			FixedSize:          500,
			Overlap:            50,
			MinChunkSize:       100,
			MaxChunkSize:       2000,
			PreserveParagraphs: true,
			ExtractKeywords:    true,
		}

		doc, err := core.ProcessDocumentContent(req.Content, "test_content", req.DocType, config)
		if err != nil {
			results = append(results, gin.H{
				"strategy": string(strategy),
				"error":    err.Error(),
			})
			continue
		}

		strategyResult := gin.H{
			"strategy":    string(strategy),
			"chunk_count": len(doc.Chunks),
			"chunks":      make([]gin.H, 0, len(doc.Chunks)),
		}

		for _, chunk := range doc.Chunks {
			chunkInfo := gin.H{
				"id":          chunk.ID,
				"text_length": len(chunk.Text),
				"chunk_type":  chunk.ChunkType,
				"section":     chunk.Section,
				"subsection":  chunk.Subsection,
				"keywords":    chunk.Keywords,
			}

			if chunk.ParentChunkID != nil {
				chunkInfo["has_parent"] = true
			}

			if len(chunk.ChildChunkIDs) > 0 {
				chunkInfo["child_count"] = len(chunk.ChildChunkIDs)
			}

			strategyResult["chunks"] = append(strategyResult["chunks"].([]gin.H), chunkInfo)
		}

		results = append(results, strategyResult)
	}

	c.JSON(http.StatusOK, gin.H{
		"content_length": len(req.Content),
		"doc_type":       req.DocType,
		"strategies":     results,
	})
}

// Health check endpoint
func HealthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "rag-go-app",
	})
}

// Collection management handlers

// ListCollectionsHandler returns all collections with metadata
func ListCollectionsHandler(c *gin.Context) {
	collections, err := vectorDB.ListCollections()
	if err != nil {
		log.Printf("Error listing collections: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list collections"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"collections": collections,
		"total":       len(collections),
	})
}

// DeleteCollectionHandler deletes a collection and all its documents
func DeleteCollectionHandler(c *gin.Context) {
	collectionName := c.Param("name")
	if collectionName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Collection name is required"})
		return
	}

	err := vectorDB.DeleteCollection(collectionName)
	if err != nil {
		log.Printf("Error deleting collection %s: %v", collectionName, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete collection"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":         "Collection deleted successfully",
		"collection_name": collectionName,
	})
}

// GetCollectionStatsHandler returns detailed statistics for a collection
func GetCollectionStatsHandler(c *gin.Context) {
	collectionName := c.Param("name")
	if collectionName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Collection name is required"})
		return
	}

	stats, err := vectorDB.GetCollectionStats(collectionName)
	if err != nil {
		log.Printf("Error getting collection stats for %s: %v", collectionName, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get collection statistics"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// Document management handlers

// ListDocumentsHandler returns all documents in a collection
func ListDocumentsHandler(c *gin.Context) {
	collectionName := c.Param("name")
	if collectionName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Collection name is required"})
		return
	}

	documents, err := vectorDB.ListDocuments(collectionName)
	if err != nil {
		log.Printf("Error listing documents in collection %s: %v", collectionName, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list documents"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"collection_name": collectionName,
		"documents":       documents,
		"total":           len(documents),
	})
}

// DeleteDocumentHandler deletes a specific document by ID
func DeleteDocumentHandler(c *gin.Context) {
	documentID := c.Param("id")
	if documentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Document ID is required"})
		return
	}

	err := vectorDB.DeleteDocument(documentID)
	if err != nil {
		log.Printf("Error deleting document %s: %v", documentID, err)
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete document"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "Document deleted successfully",
		"document_id": documentID,
	})
}

// DeleteAllDocumentsHandler deletes all documents in a collection
func DeleteAllDocumentsHandler(c *gin.Context) {
	collectionName := c.Param("name")
	if collectionName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Collection name is required"})
		return
	}

	// Optional confirmation parameter
	confirm := c.Query("confirm")
	if confirm != "true" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "This operation will delete all documents in the collection",
			"message": "To confirm, add '?confirm=true' to the request",
		})
		return
	}

	err := vectorDB.DeleteAllDocumentsInCollection(collectionName)
	if err != nil {
		log.Printf("Error deleting all documents in collection %s: %v", collectionName, err)
		if strings.Contains(err.Error(), "no documents found") {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete documents"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":         "All documents deleted successfully",
		"collection_name": collectionName,
	})
}

// Cleanup function
func Cleanup() {
	if vectorDB != nil {
		vectorDB.Close()
	}
}
