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
)

// GenerateChatCompletion sends a prompt to the LlamaCPP server.
func GenerateChatCompletion(messages []models.ChatCompletionMessage, modelName string) (string, error) {
	if modelName == "" {
		modelName = config.AppConfig.ChatModel
	}

	reqPayload := models.ChatCompletionRequest{
		Model:    modelName,
		Messages: messages,
		Stream:   false, // Set to true if you want to handle streaming
	}
	payloadBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal chat completion request: %w", err)
	}

	apiURL := fmt.Sprintf("%s/chat/completions", config.AppConfig.LlamaCPPBaseURL)
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create chat completion request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	// Add Authorization header if needed
	// req.Header.Set("Authorization", "Bearer YOUR_API_KEY")

	resp, err := httpClient.Do(req) // httpClient from embedding_service.go or a new one
	if err != nil {
		return "", fmt.Errorf("failed to call chat completion API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errBodyBytes []byte
		if resp.Body != nil {
			// Assuming ReadAll is available from embedding_service.go (which now uses io.ReadAll)
			// If embedding_service.go's ReadAll is not exported or httpClient is different,
			// then io.ReadAll should be used directly here as well.
			// For now, assuming embedding_service.ReadAll is accessible if needed, or direct io.ReadAll is preferred.
			errBodyBytes, _ = io.ReadAll(resp.Body)
		}
		log.Printf("Chat completion API error response body: %s", string(errBodyBytes))
		return "", fmt.Errorf("chat completion API request failed with status %s: %s", resp.Status, string(errBodyBytes))
	}

	var completionResp models.ChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&completionResp); err != nil {
		return "", fmt.Errorf("failed to decode chat completion API response: %w", err)
	}

	if len(completionResp.Choices) == 0 {
		return "", fmt.Errorf("no choices returned from chat completion API")
	}

	return completionResp.Choices[0].Message.Content, nil
}
