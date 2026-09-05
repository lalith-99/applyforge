package aiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// generateDocument posts a ResumeProfile and returns the raw rendered
// document bytes (PDF or DOCX depending on path).
func (c *Client) generateDocument(ctx context.Context, path string, profile ResumeProfile) ([]byte, error) {
	reqBody, err := json.Marshal(profile)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call ai-worker %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ai-worker %s failed: %s: %s", path, resp.Status, readBody(resp.Body))
	}

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// GeneratePDF renders a resume profile to PDF bytes.
func (c *Client) GeneratePDF(ctx context.Context, profile ResumeProfile) ([]byte, error) {
	return c.generateDocument(ctx, "/v1/documents/pdf", profile)
}

// GenerateDOCX renders a resume profile to DOCX bytes.
func (c *Client) GenerateDOCX(ctx context.Context, profile ResumeProfile) ([]byte, error) {
	return c.generateDocument(ctx, "/v1/documents/docx", profile)
}
