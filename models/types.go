package models

import "time"

// Document represents a text document to be processed.
type Document struct {
	ID        string                 `json:"id"`
	Content   string                 `json:"content"`
	Chunks    []*EnhancedChunk       `json:"-"`                  // Enhanced chunks with metadata
	Source    string                 `json:"source,omitempty"`   // e.g., filename
	Metadata  map[string]interface{} `json:"metadata,omitempty"` // Document-level metadata
	DocType   string                 `json:"doc_type,omitempty"` // e.g., "resume", "bible", "article"
	CreatedAt time.Time              `json:"created_at"`
}

// EnhancedChunk represents a piece of a document with rich metadata and relationships.
type EnhancedChunk struct {
	ID         string    `json:"id"`
	DocumentID string    `json:"document_id"`
	Text       string    `json:"text"`
	Embedding  []float32 `json:"-"`

	// Hierarchical information
	ParentChunkID *string  `json:"parent_chunk_id,omitempty"` // For parent-child relationships
	ChildChunkIDs []string `json:"child_chunk_ids,omitempty"` // Child chunks

	// Structural metadata
	Section    string `json:"section,omitempty"`    // e.g., "Professional Summary", "Experience"
	Subsection string `json:"subsection,omitempty"` // e.g., specific job, skill category
	ChunkType  string `json:"chunk_type"`           // e.g., "sentence", "paragraph", "section", "parent"

	// Position and context
	StartPos   int `json:"start_pos"`   // Character position in original document
	EndPos     int `json:"end_pos"`     // End character position
	ChunkIndex int `json:"chunk_index"` // Sequential index in document

	// Semantic metadata
	Keywords   []string               `json:"keywords,omitempty"`   // Extracted keywords
	Metadata   map[string]interface{} `json:"metadata,omitempty"`   // Flexible metadata
	Confidence float64                `json:"confidence,omitempty"` // Relevance confidence for retrieval
}

// DocumentChunk represents a piece of a larger document (backwards compatibility).
type DocumentChunk struct {
	ID         string    `json:"id"` // Unique ID for the chunk
	DocumentID string    `json:"document_id"`
	Text       string    `json:"text"`
	Embedding  []float32 `json:"-"` // Internal, not for JSON response
}

// ChunkingStrategy defines different approaches to text chunking.
type ChunkingStrategy string

const (
	FixedSizeStrategy      ChunkingStrategy = "fixed_size"
	SemanticStrategy       ChunkingStrategy = "semantic"
	StructuralStrategy     ChunkingStrategy = "structural"
	SentenceWindowStrategy ChunkingStrategy = "sentence_window"
	ParentDocumentStrategy ChunkingStrategy = "parent_document"
)

// ChunkingConfig contains parameters for different chunking strategies.
type ChunkingConfig struct {
	Strategy           ChunkingStrategy `json:"strategy"`
	FixedSize          int              `json:"fixed_size,omitempty"`           // For fixed size chunking
	Overlap            int              `json:"overlap,omitempty"`              // Overlap between chunks
	SentenceWindowSize int              `json:"sentence_window_size,omitempty"` // For sentence window strategy
	MinChunkSize       int              `json:"min_chunk_size,omitempty"`       // Minimum chunk size
	MaxChunkSize       int              `json:"max_chunk_size,omitempty"`       // Maximum chunk size
	PreserveParagraphs bool             `json:"preserve_paragraphs,omitempty"`  // Try to keep paragraphs intact
	ExtractKeywords    bool             `json:"extract_keywords,omitempty"`     // Extract keywords from chunks
}

// AddDocumentRequest is the structure for requests to add a new document.
type AddDocumentRequest struct {
	CollectionName string          `json:"collection_name" binding:"required"`
	FilePath       string          `json:"file_path,omitempty"`       // For server-side file access
	Content        string          `json:"content,omitempty"`         // For direct content submission
	Source         string          `json:"source,omitempty"`          // e.g. filename if content is direct
	DocType        string          `json:"doc_type,omitempty"`        // Document type for strategy selection
	ChunkingConfig *ChunkingConfig `json:"chunking_config,omitempty"` // Custom chunking configuration
}

// QueryRequest is the structure for requests to query the RAG system.
type QueryRequest struct {
	CollectionName    string                 `json:"collection_name" binding:"required"`
	Query             string                 `json:"query" binding:"required"`
	TopK              int                    `json:"top_k,omitempty"`
	RerankerEnabled   bool                   `json:"reranker_enabled,omitempty"`   // Enable re-ranking
	MetadataFilters   map[string]interface{} `json:"metadata_filters,omitempty"`   // Filter by metadata
	IncludeParents    bool                   `json:"include_parents,omitempty"`    // Include parent chunks in results
	QueryExpansion    bool                   `json:"query_expansion,omitempty"`    // Expand query with synonyms/related terms
	SemanticThreshold float64                `json:"semantic_threshold,omitempty"` // Minimum similarity threshold
}

// QueryResponse is the structure for the RAG system's answer.
type QueryResponse struct {
	Answer           string           `json:"answer"`
	RetrievedContext []string         `json:"retrieved_context,omitempty"`
	EnhancedChunks   []*EnhancedChunk `json:"enhanced_chunks,omitempty"`   // Full chunk metadata
	SimilarityScores []float64        `json:"similarity_scores,omitempty"` // Similarity scores for chunks
	RerankedScores   []float64        `json:"reranked_scores,omitempty"`   // Re-ranking scores
	ProcessingTime   float64          `json:"processing_time,omitempty"`   // Query processing time
	MetadataUsed     bool             `json:"metadata_used,omitempty"`     // Whether metadata filtering was applied
}

// EmbeddingRequest is the structure for requesting embeddings from an OpenAI-compatible API.
type EmbeddingRequest struct {
	Input interface{} `json:"input"` // string or []string
	Model string      `json:"model"` // e.g., "text-embedding-ada-002" or your local model name
}

// EmbeddingResponseData holds a single embedding vector and its metadata.
type EmbeddingResponseData struct {
	Embedding []float32 `json:"embedding"`
	Index     int       `json:"index"`
	Object    string    `json:"object"`
}

// EmbeddingAPIResponse is the top-level structure for responses from the embedding API.
type EmbeddingAPIResponse struct {
	Data   []EmbeddingResponseData `json:"data"`
	Model  string                  `json:"model"`
	Object string                  `json:"object"`
	Usage  struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

// ChatCompletionMessage represents a single message in a chat completion request/response.
type ChatCompletionMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionRequest is the structure for requesting chat completions from an OpenAI-compatible API.
type ChatCompletionRequest struct {
	Model    string                  `json:"model"`
	Messages []ChatCompletionMessage `json:"messages"`
	Stream   bool                    `json:"stream,omitempty"`
}

// ChatChoice represents one of the completion choices from the API.
type ChatChoice struct {
	Index        int                   `json:"index"`
	Message      ChatCompletionMessage `json:"message"`
	FinishReason string                `json:"finish_reason"`
}

// ChatCompletionResponse is the top-level structure for responses from the chat completion API.
type ChatCompletionResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []ChatChoice `json:"choices"`
	// Usage   UsageInfo    `json:"usage"` // If applicable
}
