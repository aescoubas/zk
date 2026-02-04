package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

type EmbeddingRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type EmbeddingResponse struct {
	Embedding []float64 `json:"embedding"`
}

func main() {
	url := "http://localhost:11434/api/embeddings"
	model := "nomic-embed-text"
	
	content, err := os.ReadFile("zettels/braindump.md")
	if err != nil {
		panic(err)
	}
	
	text := string(content)
	fmt.Printf("Embedding text of length: %d\n", len(text))

	reqBody := EmbeddingRequest{
		Model:  model,
		Prompt: text,
	}
	jsonBody, _ := json.Marshal(reqBody)

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Status: %s\n", resp.Status)
	fmt.Printf("Body: %s\n", string(body))
}