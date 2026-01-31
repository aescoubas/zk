package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

type EmbeddingRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type EmbeddingResponse struct {
	Embedding []float64 `json:"embedding"`
}

func (c *Client) Embed(text string) ([]float64, error) {
	reqBody := EmbeddingRequest{
		Model:  c.Model,
		Prompt: text,
	}
	jsonBody, _ := json.Marshal(reqBody)

	resp, err := http.Post(c.BaseURL+"/api/embeddings", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("ollama API error: %s", resp.Status)
	}

	var embeddingResp EmbeddingResponse
	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &embeddingResp); err != nil {
		return nil, err
	}
	return embeddingResp.Embedding, nil
}
