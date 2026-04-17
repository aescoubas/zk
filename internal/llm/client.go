package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
)

type Client struct {
	BaseURL string
	Model   string
}

func NewClient(baseURL, model string) *Client {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	if model == "" {
		model = "nomic-embed-text" // Default embedding model
	}
	return &Client{BaseURL: baseURL, Model: model}
}

// Single-text embedding request (legacy /api/embeddings endpoint).
type EmbeddingRequest struct {
	Model   string                 `json:"model"`
	Prompt  string                 `json:"prompt"`
	Options map[string]interface{} `json:"options,omitempty"`
}

type EmbeddingResponse struct {
	Embedding []float64 `json:"embedding"`
}

// Batch embedding request (/api/embed endpoint).
type BatchEmbedRequest struct {
	Model   string                 `json:"model"`
	Input   []string               `json:"input"`
	Options map[string]interface{} `json:"options,omitempty"`
}

type BatchEmbedResponse struct {
	Embeddings [][]float64 `json:"embeddings"`
}

func (c *Client) options() map[string]interface{} {
	numThreads := runtime.NumCPU() - 2
	if numThreads < 1 {
		numThreads = 1
	}
	return map[string]interface{}{
		"num_ctx":    8192,
		"num_thread": numThreads,
	}
}

// Embed generates an embedding for a single text.
func (c *Client) Embed(text string) ([]float64, error) {
	reqBody := EmbeddingRequest{
		Model:   c.Model,
		Prompt:  text,
		Options: c.options(),
	}
	jsonBody, _ := json.Marshal(reqBody)

	resp, err := http.Post(c.BaseURL+"/api/embeddings", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama API error: %s - %s", resp.Status, string(body))
	}

	var embeddingResp EmbeddingResponse
	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &embeddingResp); err != nil {
		return nil, err
	}
	return embeddingResp.Embedding, nil
}

// EmbedBatch generates embeddings for multiple texts in a single request
// using the /api/embed batch endpoint. Returns one vector per input text.
func (c *Client) EmbedBatch(texts []string) ([][]float64, error) {
	reqBody := BatchEmbedRequest{
		Model:   c.Model,
		Input:   texts,
		Options: c.options(),
	}
	jsonBody, _ := json.Marshal(reqBody)

	resp, err := http.Post(c.BaseURL+"/api/embed", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama API error: %s - %s", resp.Status, string(body))
	}

	var batchResp BatchEmbedResponse
	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &batchResp); err != nil {
		return nil, err
	}

	if len(batchResp.Embeddings) != len(texts) {
		return nil, fmt.Errorf("expected %d embeddings, got %d", len(texts), len(batchResp.Embeddings))
	}
	return batchResp.Embeddings, nil
}
