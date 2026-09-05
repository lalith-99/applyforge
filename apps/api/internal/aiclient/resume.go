// Package aiclient is a thin HTTP client for the Python AI/document worker.
// Business logic in the Go API should call through this package rather than
// constructing requests to the AI worker ad hoc, so the integration surface
// stays centralized and easy to swap/extend (see MASTER_REQUIREMENTS.md §45).
package aiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"time"
)

// Client calls the AI worker's HTTP API.
type Client struct {
	baseURL string
	http    *http.Client
}

// New builds a Client pointed at the given AI worker base URL.
func New(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		http:    &http.Client{Timeout: 60 * time.Second},
	}
}

// ContactInfo mirrors app/resume/models.py ContactInfo.
type ContactInfo struct {
	Name     *string `json:"name"`
	Email    *string `json:"email"`
	Phone    *string `json:"phone"`
	Location *string `json:"location"`
}

// ExperienceEntry mirrors app/resume/models.py ExperienceEntry.
type ExperienceEntry struct {
	Company        *string  `json:"company"`
	Title          *string  `json:"title"`
	StartDate      *string  `json:"start_date"`
	EndDate        *string  `json:"end_date"`
	Location       *string  `json:"location"`
	Bullets        []string `json:"bullets"`
	DetectedSkills []string `json:"detected_skills"`
	Technologies   []string `json:"technologies"`
}

// ResumeProfile mirrors app/resume/models.py ResumeProfile.
type ResumeProfile struct {
	Contact        ContactInfo       `json:"contact"`
	Summary        *string           `json:"summary"`
	Skills         []string          `json:"skills"`
	Experiences    []ExperienceEntry `json:"experiences"`
	Education      []string          `json:"education"`
	Certifications []string          `json:"certifications"`
}

// ExtractResumeText uploads a resume file and returns its selectable text.
func (c *Client) ExtractResumeText(ctx context.Context, filename, mimeType string, fileBytes []byte) (string, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename=%q`, filename))
	header.Set("Content-Type", mimeType)
	part, err := writer.CreatePart(header)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(fileBytes); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/resumes/extract", &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("call ai-worker extract: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ai-worker extract failed: %s: %s", resp.Status, readBody(resp.Body))
	}

	var out struct {
		RawText string `json:"raw_text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.RawText, nil
}

// ParseResume sends extracted resume text and returns a structured profile.
func (c *Client) ParseResume(ctx context.Context, rawText string) (ResumeProfile, error) {
	reqBody, err := json.Marshal(map[string]string{"raw_text": rawText})
	if err != nil {
		return ResumeProfile{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/resumes/parse", bytes.NewReader(reqBody))
	if err != nil {
		return ResumeProfile{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return ResumeProfile{}, fmt.Errorf("call ai-worker parse: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ResumeProfile{}, fmt.Errorf("ai-worker parse failed: %s: %s", resp.Status, readBody(resp.Body))
	}

	var out struct {
		Profile ResumeProfile `json:"profile"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return ResumeProfile{}, err
	}
	return out.Profile, nil
}

func readBody(r io.Reader) string {
	b, _ := io.ReadAll(io.LimitReader(r, 2048))
	return string(b)
}
