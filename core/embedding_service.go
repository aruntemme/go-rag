package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"rag-go-app/config"
	"rag-go-app/models"
	"strings"
	"time"
)

var httpClient = &http.Client{Timeout: 180 * time.Second} // Increased timeout, but batching is key

const (
	defaultEmbeddingBatchSize = 32   // Default number of texts to send in one batch
	maxTokensPerBatch         = 8000 // Maximum tokens per batch (conservative estimate)
	maxCharsPerToken          = 4    // Rough estimation: 1 token ≈ 4 characters
	maxBatchSizeLimit         = 64   // Hard limit on batch size
	minBatchSize              = 1    // Minimum batch size
)

// GetEmbeddings sends text(s) to the LlamaCPP server's embedding endpoint with adaptive batching.
func GetEmbeddings(texts []string, modelName string) ([][]float32, error) {
	if modelName == "" {
		modelName = config.AppConfig.EmbeddingModel
	}

	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	allEmbeddings := make([][]float32, len(texts))

	// Create adaptive batches
	batches := createAdaptiveBatches(texts)

	log.Printf("Processing %d texts in %d adaptive batches", len(texts), len(batches))

	for batchIndex, batch := range batches {
		embeddings, err := processBatchWithRetry(batch, modelName, batchIndex)
		if err != nil {
			return nil, fmt.Errorf("failed to process batch %d: %w", batchIndex, err)
		}

		// Place embeddings in correct positions
		for i, embedding := range embeddings {
			globalIndex := batch.StartIndex + i
			if globalIndex < len(allEmbeddings) {
				allEmbeddings[globalIndex] = embedding
			}
		}

		log.Printf("Successfully processed batch %d (%d texts)", batchIndex, len(batch.Texts))
	}

	// Final validation
	for idx, emb := range allEmbeddings {
		if len(emb) == 0 {
			return nil, fmt.Errorf("embedding for text at index %d was not populated", idx)
		}
	}

	return allEmbeddings, nil
}

// EmbeddingBatch represents a batch of texts to be processed
type EmbeddingBatch struct {
	Texts      []string
	StartIndex int
	TotalChars int
}

// createAdaptiveBatches creates optimally sized batches based on content size
func createAdaptiveBatches(texts []string) []EmbeddingBatch {
	var batches []EmbeddingBatch

	i := 0
	for i < len(texts) {
		batch := EmbeddingBatch{
			StartIndex: i,
		}

		currentChars := 0
		batchSize := 0

		// Add texts to batch while staying within limits
		for i+batchSize < len(texts) && batchSize < maxBatchSizeLimit {
			textChars := len(texts[i+batchSize])
			estimatedTokens := (currentChars + textChars) / maxCharsPerToken

			// Check if adding this text would exceed token limit
			if estimatedTokens > maxTokensPerBatch && batchSize > 0 {
				break
			}

			// Check if single text is too large
			if textChars/maxCharsPerToken > maxTokensPerBatch {
				log.Printf("Warning: Text at index %d is very large (%d chars, ~%d tokens), processing individually",
					i+batchSize, textChars, textChars/maxCharsPerToken)
				// Process this large text alone
				if batchSize == 0 {
					batch.Texts = append(batch.Texts, texts[i+batchSize])
					batch.TotalChars = textChars
					batchSize = 1
				}
				break
			}

			batch.Texts = append(batch.Texts, texts[i+batchSize])
			currentChars += textChars
			batchSize++
		}

		batch.TotalChars = currentChars
		batches = append(batches, batch)
		i += batchSize
	}

	return batches
}

// Add a function to determine embedding dimension based on model
func getEmbeddingDimension(modelName string) int {
	// Map of known models to their dimensions
	modelDimensions := map[string]int{
		"mxbai-embed-large":       1024,
		"mxbai-embed-large:large": 1024,
		"nomic-embed-text-v1.5":   768,
		"text-embedding-ada-002":  1536,
		"text-embedding-3-small":  1536,
		"text-embedding-3-large":  3072,
		// Add more models as needed
	}

	if dim, exists := modelDimensions[modelName]; exists {
		return dim
	}

	// Default to 1024 for unknown models (mxbai-embed-large is common)
	log.Printf("Unknown model %s, defaulting to 1024 dimensions", modelName)
	return 1024
}

// processBatchWithRetry processes a batch with retry logic for oversized batches
func processBatchWithRetry(batch EmbeddingBatch, modelName string, batchIndex int) ([][]float32, error) {
	currentBatch := batch
	maxRetries := 3

	for attempt := 0; attempt < maxRetries; attempt++ {
		log.Printf("Batch %d attempt %d: %d texts, %d chars (~%d tokens)",
			batchIndex, attempt+1, len(currentBatch.Texts), currentBatch.TotalChars, currentBatch.TotalChars/maxCharsPerToken)

		embeddings, err := sendEmbeddingRequest(currentBatch.Texts, modelName)
		if err == nil {
			return embeddings, nil
		}

		// Check if error indicates batch is too large
		if isOversizedBatchError(err) {
			// If this is a single text that's too large, we need to handle it differently
			if len(currentBatch.Texts) == 1 {
				log.Printf("Single text at batch %d is too large (%d chars), skipping", batchIndex, currentBatch.TotalChars)
				// Return a placeholder embedding for the oversized text
				// Determine the correct dimension based on the model
				dimension := getEmbeddingDimension(modelName)
				placeholder := make([]float32, dimension)
				return [][]float32{placeholder}, nil
			}

			if len(currentBatch.Texts) > minBatchSize {
				log.Printf("Batch %d is too large, splitting in half (attempt %d)", batchIndex, attempt+1)

				// Split batch in half
				midpoint := len(currentBatch.Texts) / 2

				// Calculate total chars for first half
				firstHalfChars := 0
				for _, text := range currentBatch.Texts[:midpoint] {
					firstHalfChars += len(text)
				}

				// Calculate total chars for second half
				secondHalfChars := 0
				for _, text := range currentBatch.Texts[midpoint:] {
					secondHalfChars += len(text)
				}

				firstHalf := EmbeddingBatch{
					Texts:      currentBatch.Texts[:midpoint],
					StartIndex: currentBatch.StartIndex,
					TotalChars: firstHalfChars,
				}
				secondHalf := EmbeddingBatch{
					Texts:      currentBatch.Texts[midpoint:],
					StartIndex: currentBatch.StartIndex + midpoint,
					TotalChars: secondHalfChars,
				}

				// Process each half
				firstEmbeddings, err1 := processBatchWithRetry(firstHalf, modelName, batchIndex)
				if err1 != nil {
					return nil, fmt.Errorf("failed to process first half of split batch: %w", err1)
				}

				secondEmbeddings, err2 := processBatchWithRetry(secondHalf, modelName, batchIndex)
				if err2 != nil {
					return nil, fmt.Errorf("failed to process second half of split batch: %w", err2)
				}

				// Combine results
				combined := append(firstEmbeddings, secondEmbeddings...)
				return combined, nil
			}
		}

		// If not an oversized batch error, or we can't split further, return the error
		if attempt == maxRetries-1 || len(currentBatch.Texts) <= minBatchSize {
			return nil, fmt.Errorf("failed after %d attempts: %w", attempt+1, err)
		}

		// Wait a bit before retry
		time.Sleep(time.Second * time.Duration(attempt+1))
	}

	return nil, fmt.Errorf("exceeded maximum retry attempts")
}

// sendEmbeddingRequest sends a single embedding request
func sendEmbeddingRequest(texts []string, modelName string) ([][]float32, error) {
	reqPayload := models.EmbeddingRequest{
		Input: texts,
		Model: modelName,
	}

	payloadBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal embedding request: %w", err)
	}

	apiURL := fmt.Sprintf("%s/embeddings", config.AppConfig.LlamaCPPBaseURL)
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create embedding request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call embedding API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errBodyBytes []byte
		if resp.Body != nil {
			errBodyBytes, _ = io.ReadAll(resp.Body)
		}
		return nil, fmt.Errorf("embedding API request failed with status %s: %s", resp.Status, string(errBodyBytes))
	}

	var embeddingResp models.EmbeddingAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&embeddingResp); err != nil {
		return nil, fmt.Errorf("failed to decode embedding API response: %w", err)
	}

	if len(embeddingResp.Data) != len(texts) {
		return nil, fmt.Errorf("mismatch in number of embeddings returned (%d) vs texts sent (%d)", len(embeddingResp.Data), len(texts))
	}

	// Convert response to embeddings array
	embeddings := make([][]float32, len(texts))
	for _, data := range embeddingResp.Data {
		if data.Index >= 0 && data.Index < len(embeddings) {
			embeddings[data.Index] = data.Embedding
		} else {
			return nil, fmt.Errorf("embedding data index out of bounds: %d", data.Index)
		}
	}

	return embeddings, nil
}

// isOversizedBatchError checks if the error indicates the batch is too large
func isOversizedBatchError(err error) bool {
	errorStr := strings.ToLower(err.Error())
	oversizedIndicators := []string{
		"too large",
		"input is too large",
		"increase the physical batch size",
		"context length exceeded",
		"maximum context length",
		"token limit",
		"input size",
	}

	for _, indicator := range oversizedIndicators {
		if strings.Contains(errorStr, indicator) {
			return true
		}
	}

	return false
}

/*
func ReadAll(r io.Reader) ([]byte, error) { // Original ReadAll, ensure io.ReadAll is used or this is removed if not needed elsewhere
    var b bytes.Buffer
    _, err := io.Copy(&b, r)
    return b.Bytes(), err
}
*/
