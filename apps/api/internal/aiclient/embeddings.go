package aiclient

import (
	"context"
)

// EmbedResponse mirrors app/api/embeddings.py's EmbedResponse.
type EmbedResponse struct {
	Embedding  []float32 `json:"embedding"`
	Model      string    `json:"model"`
	Dimensions int       `json:"dimensions"`
}

// Embed requests a semantic embedding vector for text (Phase E: job and
// candidate embeddings for pgvector-backed retrieval).
func (c *Client) Embed(ctx context.Context, text string) (EmbedResponse, error) {
	var out EmbedResponse
	err := c.postJSON(ctx, "embed_text", "/v1/embeddings", map[string]string{"text": text}, &out)
	return out, err
}
