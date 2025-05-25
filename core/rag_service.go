package core

import (
	"fmt"
	"log"
	"math"
	"os"
	"rag-go-app/models"
	"sort"
	"strings"
	"time"
)

// EmbeddingService wraps the embedding functionality
type EmbeddingService struct{}

func NewEmbeddingService() *EmbeddingService {
	return &EmbeddingService{}
}

func (e *EmbeddingService) GetEmbedding(text string) ([]float32, error) {
	embeddings, err := GetEmbeddings([]string{text}, "")
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}
	return embeddings[0], nil
}

func (e *EmbeddingService) GetEmbeddings(texts []string) ([][]float32, error) {
	return GetEmbeddings(texts, "")
}

// LLMService wraps the LLM functionality
type LLMService struct{}

func NewLLMService() *LLMService {
	return &LLMService{}
}

func (l *LLMService) GenerateResponse(prompt string) (string, error) {
	messages := []models.ChatCompletionMessage{
		{Role: "user", Content: prompt},
	}
	return GenerateChatCompletion(messages, "")
}

type RAGService struct {
	vectorDB        *VectorDB
	embeddingClient *EmbeddingService
	llmClient       *LLMService
}

func NewRAGService(vectorDB *VectorDB, embeddingClient *EmbeddingService, llmClient *LLMService) *RAGService {
	return &RAGService{
		vectorDB:        vectorDB,
		embeddingClient: embeddingClient,
		llmClient:       llmClient,
	}
}

// ReadFileContent reads a file and returns its content as string
func ReadFileContent(filePath string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", filePath, err)
	}
	return string(content), nil
}

func (r *RAGService) AddDocument(collectionName string, req *models.AddDocumentRequest) error {
	startTime := time.Now()

	// Read content
	var content string
	var err error

	if req.FilePath != "" {
		content, err = ReadFileContent(req.FilePath)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}
	} else if req.Content != "" {
		content = req.Content
	} else {
		return fmt.Errorf("either file_path or content must be provided")
	}

	if len(content) == 0 {
		return fmt.Errorf("document content is empty")
	}

	// Process document with enhanced chunking
	doc, err := ProcessDocumentContent(content, req.Source, req.DocType, req.ChunkingConfig)
	if err != nil {
		return fmt.Errorf("failed to process document: %w", err)
	}

	log.Printf("Document processed: %d chunks created using %s strategy",
		len(doc.Chunks), doc.Metadata["chunking_strategy"])

	// Generate embeddings for all chunks
	log.Printf("Generating embeddings for %d chunks...", len(doc.Chunks))
	if err := r.generateEmbeddings(doc.Chunks); err != nil {
		return fmt.Errorf("failed to generate embeddings: %w", err)
	}

	// Store document and chunks in vector database
	if err := r.vectorDB.AddDocument(collectionName, doc); err != nil {
		return fmt.Errorf("failed to add document to database: %w", err)
	}

	// Store embeddings
	if err := r.vectorDB.AddEmbeddings(doc.Chunks); err != nil {
		return fmt.Errorf("failed to add embeddings: %w", err)
	}

	log.Printf("Document '%s' added successfully in %v with %d chunks",
		doc.Source, time.Since(startTime), len(doc.Chunks))

	return nil
}

func (r *RAGService) Query(req *models.QueryRequest) (*models.QueryResponse, error) {
	startTime := time.Now()

	// Set defaults
	if req.TopK <= 0 {
		req.TopK = 5
	}

	// Query expansion
	query := req.Query
	if req.QueryExpansion {
		expandedQuery := r.expandQuery(req.Query)
		if expandedQuery != req.Query {
			query = expandedQuery
			log.Printf("Query expanded: '%s' -> '%s'", req.Query, query)
		}
	}

	// Generate query embedding
	queryEmbedding, err := r.embeddingClient.GetEmbedding(query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	// Build metadata filters
	filters := make(map[string]interface{})
	for key, value := range req.MetadataFilters {
		filters[key] = value
	}

	// Search for similar chunks
	chunks, scores, err := r.vectorDB.QuerySimilarChunks(
		req.CollectionName,
		queryEmbedding,
		req.TopK*2, // Get more for re-ranking
		filters,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to search similar chunks: %w", err)
	}

	if len(chunks) == 0 {
		return &models.QueryResponse{
			Answer:         "I couldn't find any relevant information for your query.",
			ProcessingTime: time.Since(startTime).Seconds(),
			MetadataUsed:   len(req.MetadataFilters) > 0,
		}, nil
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
			return &models.QueryResponse{
				Answer:         "No chunks met the semantic similarity threshold.",
				ProcessingTime: time.Since(startTime).Seconds(),
				MetadataUsed:   len(req.MetadataFilters) > 0,
			}, nil
		}
	}

	// Include parent chunks if requested
	if req.IncludeParents {
		chunks, scores = r.includeParentChunks(chunks, scores)
	}

	// Re-ranking
	var rerankedScores []float64
	if req.RerankerEnabled && len(chunks) > 1 {
		chunks, rerankedScores = r.rerankChunks(query, chunks, scores)
	}

	// Limit to requested TopK after re-ranking
	if len(chunks) > req.TopK {
		chunks = chunks[:req.TopK]
		scores = scores[:req.TopK]
		if len(rerankedScores) > req.TopK {
			rerankedScores = rerankedScores[:req.TopK]
		}
	}

	// Prepare context for LLM
	context := r.prepareContext(chunks)

	// Generate answer using LLM
	answer, err := r.generateAnswer(req.Query, context)
	if err != nil {
		return nil, fmt.Errorf("failed to generate answer: %w", err)
	}

	// Prepare response
	response := &models.QueryResponse{
		Answer:           answer,
		RetrievedContext: r.extractChunkTexts(chunks),
		EnhancedChunks:   chunks,
		SimilarityScores: scores,
		ProcessingTime:   time.Since(startTime).Seconds(),
		MetadataUsed:     len(req.MetadataFilters) > 0,
	}

	if len(rerankedScores) > 0 {
		response.RerankedScores = rerankedScores
	}

	return response, nil
}

func (r *RAGService) generateEmbeddings(chunks []*models.EnhancedChunk) error {
	texts := make([]string, len(chunks))
	for i, chunk := range chunks {
		texts[i] = chunk.Text
	}

	embeddings, err := r.embeddingClient.GetEmbeddings(texts)
	if err != nil {
		return err
	}

	for i, embedding := range embeddings {
		chunks[i].Embedding = embedding
	}

	return nil
}

func (r *RAGService) expandQuery(query string) string {
	// Simple query expansion - could be enhanced with synonyms, related terms, etc.
	words := strings.Fields(strings.ToLower(query))

	// Add some common synonyms and related terms
	expansions := map[string][]string{
		"experience":     {"work", "job", "employment", "career", "role", "position", "background"},
		"skills":         {"abilities", "competencies", "expertise", "knowledge", "proficiency", "technologies"},
		"education":      {"degree", "university", "college", "learning", "academic", "study", "qualification"},
		"project":        {"initiative", "work", "development", "implementation", "assignment", "task"},
		"manage":         {"lead", "supervise", "oversee", "direct", "coordinate", "administer", "manage"},
		"develop":        {"create", "build", "design", "implement", "construct", "establish", "code"},
		"lead":           {"manage", "direct", "supervise", "coordinate", "oversee", "team lead", "leadership"},
		"team":           {"group", "team", "squad", "unit", "crew", "staff"},
		"position":       {"role", "job", "employment", "work", "career", "title"},
		"role":           {"position", "job", "employment", "work", "responsibility"},
		"senior":         {"experienced", "advanced", "lead", "principal", "expert"},
		"manager":        {"lead", "supervisor", "director", "head", "team lead"},
		"engineer":       {"developer", "programmer", "architect", "technical", "software"},
		"developer":      {"engineer", "programmer", "coder", "software", "technical"},
		"technical":      {"technology", "programming", "software", "engineering", "development"},
		"programming":    {"coding", "development", "software", "technical", "engineering"},
		"responsibility": {"duty", "task", "role", "function", "accountability"},
		"achievement":    {"accomplishment", "success", "result", "outcome", "milestone"},
	}

	var expandedTerms []string
	expandedTerms = append(expandedTerms, query) // Always include original query

	for _, word := range words {
		if synonyms, exists := expansions[word]; exists {
			// Add one or two most relevant synonyms
			for i, synonym := range synonyms {
				if i >= 2 { // Limit to avoid too much expansion
					break
				}
				if !contains(expandedTerms, synonym) {
					expandedTerms = append(expandedTerms, synonym)
				}
			}
		}
	}

	if len(expandedTerms) > 1 {
		return strings.Join(expandedTerms, " ")
	}

	return query
}

func (r *RAGService) includeParentChunks(chunks []*models.EnhancedChunk, scores []float64) ([]*models.EnhancedChunk, []float64) {
	var enhancedChunks []*models.EnhancedChunk
	var enhancedScores []float64

	seen := make(map[string]bool)

	for i, chunk := range chunks {
		// Add the original chunk if not seen
		if !seen[chunk.ID] {
			enhancedChunks = append(enhancedChunks, chunk)
			enhancedScores = append(enhancedScores, scores[i])
			seen[chunk.ID] = true
		}

		// Add parent chunks if they exist
		if chunk.ParentChunkID != nil {
			parentChunks, err := r.vectorDB.GetChunkWithParents(*chunk.ParentChunkID)
			if err == nil {
				for _, parent := range parentChunks {
					if !seen[parent.ID] {
						enhancedChunks = append(enhancedChunks, parent)
						// Give parent chunks slightly lower score
						enhancedScores = append(enhancedScores, scores[i]*0.9)
						seen[parent.ID] = true
					}
				}
			}
		}
	}

	return enhancedChunks, enhancedScores
}

func (r *RAGService) rerankChunks(query string, chunks []*models.EnhancedChunk, originalScores []float64) ([]*models.EnhancedChunk, []float64) {
	type ChunkScore struct {
		chunk    *models.EnhancedChunk
		score    float64
		reranked float64
		index    int
	}

	var chunkScores []ChunkScore

	// Calculate re-ranking scores based on multiple factors
	for i, chunk := range chunks {
		rerankedScore := r.calculateRerankedScore(query, chunk, originalScores[i])

		chunkScores = append(chunkScores, ChunkScore{
			chunk:    chunk,
			score:    originalScores[i],
			reranked: rerankedScore,
			index:    i,
		})
	}

	// Sort by re-ranked score
	sort.Slice(chunkScores, func(i, j int) bool {
		return chunkScores[i].reranked > chunkScores[j].reranked
	})

	// Extract sorted chunks and scores
	rerankedChunks := make([]*models.EnhancedChunk, len(chunkScores))
	rerankedScores := make([]float64, len(chunkScores))

	for i, cs := range chunkScores {
		rerankedChunks[i] = cs.chunk
		rerankedScores[i] = cs.reranked
	}

	return rerankedChunks, rerankedScores
}

func (r *RAGService) calculateRerankedScore(query string, chunk *models.EnhancedChunk, originalScore float64) float64 {
	score := originalScore
	queryLower := strings.ToLower(query)

	// Boost score based on chunk type (some types are more valuable)
	switch chunk.ChunkType {
	case "section", "paragraph":
		score *= 1.2 // Boost structural chunks
	case "job_entry":
		score *= 1.4 // Strong boost for job entries
	case "section_part":
		score *= 1.1 // Slight boost for section parts
	case "parent":
		score *= 1.3 // Boost parent chunks (more context)
	}

	// Extra boost for experience-related sections when query mentions positions/roles
	if r.isPositionQuery(queryLower) && r.isExperienceRelated(chunk) {
		score *= 1.5
	}

	// Boost score based on section relevance
	if chunk.Section != "" {
		sectionLower := strings.ToLower(chunk.Section)
		if r.isPositionQuery(queryLower) && strings.Contains(sectionLower, "experience") {
			score *= 1.4
		}
		if strings.Contains(queryLower, "skill") && strings.Contains(sectionLower, "skill") {
			score *= 1.4
		}
		if strings.Contains(queryLower, "education") && strings.Contains(sectionLower, "education") {
			score *= 1.4
		}
	}

	// Boost score based on keyword matches
	queryWords := strings.Fields(queryLower)
	keywordMatches := 0

	for _, keyword := range chunk.Keywords {
		keywordLower := strings.ToLower(keyword)
		for _, queryWord := range queryWords {
			if strings.Contains(keywordLower, queryWord) ||
				strings.Contains(queryWord, keywordLower) {
				keywordMatches++
			}
		}
	}

	if keywordMatches > 0 {
		keywordBoost := 1.0 + (float64(keywordMatches) * 0.15)
		score *= keywordBoost
	}

	// Check for position-related metadata
	if metadata := chunk.Metadata; metadata != nil {
		if position, exists := metadata["position"]; exists {
			if posStr, ok := position.(string); ok && posStr != "" {
				if r.isPositionQuery(queryLower) {
					score *= 1.3 // Boost chunks with position metadata for position queries
				}
			}
		}
	}

	// Boost score based on text length (moderate length is often better)
	textLength := len(chunk.Text)
	if textLength >= 100 && textLength <= 1000 {
		score *= 1.1 // Boost moderate-length chunks
	} else if textLength > 2000 {
		score *= 0.9 // Slight penalty for very long chunks
	}

	// Boost score for chunks with metadata confidence
	if chunk.Confidence > 0 {
		score *= (1.0 + chunk.Confidence*0.2)
	}

	return math.Min(score, 1.0) // Cap at 1.0
}

// isPositionQuery checks if the query is asking about positions or roles
func (r *RAGService) isPositionQuery(query string) bool {
	positionKeywords := []string{
		"position", "role", "job", "title", "lead", "manager", "director",
		"senior", "junior", "principal", "team lead", "leadership",
	}

	for _, keyword := range positionKeywords {
		if strings.Contains(query, keyword) {
			return true
		}
	}
	return false
}

// isExperienceRelated checks if chunk is related to work experience
func (r *RAGService) isExperienceRelated(chunk *models.EnhancedChunk) bool {
	if chunk.ChunkType == "job_entry" {
		return true
	}

	if chunk.Section != "" {
		sectionLower := strings.ToLower(chunk.Section)
		experienceTerms := []string{"experience", "employment", "career", "work", "professional"}
		for _, term := range experienceTerms {
			if strings.Contains(sectionLower, term) {
				return true
			}
		}
	}

	return false
}

func (r *RAGService) prepareContext(chunks []*models.EnhancedChunk) string {
	var contextParts []string

	for i, chunk := range chunks {
		var contextPart strings.Builder

		// Add metadata information if available
		if chunk.Section != "" || chunk.Subsection != "" {
			contextPart.WriteString(fmt.Sprintf("[Context %d", i+1))
			if chunk.Section != "" {
				contextPart.WriteString(fmt.Sprintf(" - %s", chunk.Section))
			}
			if chunk.Subsection != "" {
				contextPart.WriteString(fmt.Sprintf(" - %s", chunk.Subsection))
			}
			contextPart.WriteString("]\n")
		} else {
			contextPart.WriteString(fmt.Sprintf("[Context %d]\n", i+1))
		}

		contextPart.WriteString(chunk.Text)
		contextParts = append(contextParts, contextPart.String())
	}

	return strings.Join(contextParts, "\n\n")
}

func (r *RAGService) generateAnswer(query, context string) (string, error) {
	prompt := fmt.Sprintf(`You are a helpful AI assistant. Based on the provided context, answer the user's question accurately and comprehensively. If the context doesn't contain enough information to answer the question, say so clearly.

Context:
%s

Question: %s

Answer:`, context, query)

	return r.llmClient.GenerateResponse(prompt)
}

func (r *RAGService) extractChunkTexts(chunks []*models.EnhancedChunk) []string {
	texts := make([]string, len(chunks))
	for i, chunk := range chunks {
		texts[i] = chunk.Text
	}
	return texts
}

// Helper function to check if slice contains string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
