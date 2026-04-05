package asr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Client struct {
	baseURL      string
	apiKey       string
	defaultModel string
	httpClient   *http.Client
}

type TranscribeInput struct {
	FilePath string
	Model    string
}

type Segment struct {
	ID    int     `json:"id"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

type TranscribeOutput struct {
	Text     string    `json:"text"`
	Segments []Segment `json:"segments"`
	Duration float64   `json:"duration"`
}

type apiError struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

func NewClient(baseURL, apiKey, defaultModel string) *Client {
	return &Client{
		baseURL:      strings.TrimRight(baseURL, "/"),
		apiKey:       apiKey,
		defaultModel: defaultModel,
		httpClient:   &http.Client{Timeout: 180 * time.Second},
	}
}

func (c *Client) Transcribe(ctx context.Context, in TranscribeInput) (TranscribeOutput, error) {
	model := strings.TrimSpace(in.Model)
	if model == "" {
		model = c.defaultModel
	}
	if strings.TrimSpace(in.FilePath) == "" {
		return TranscribeOutput{}, fmt.Errorf("file path is required")
	}

	file, err := os.Open(in.FilePath)
	if err != nil {
		return TranscribeOutput{}, fmt.Errorf("open file failed: %w", err)
	}
	defer file.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", filepath.Base(in.FilePath))
	if err != nil {
		return TranscribeOutput{}, fmt.Errorf("create file part failed: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return TranscribeOutput{}, fmt.Errorf("copy file content failed: %w", err)
	}

	_ = writer.WriteField("model", model)
	_ = writer.WriteField("response_format", "verbose_json")
	_ = writer.WriteField("timestamp_granularities[]", "segment")

	if err := writer.Close(); err != nil {
		return TranscribeOutput{}, fmt.Errorf("close multipart writer failed: %w", err)
	}

	url := c.baseURL + "/v1/audio/transcriptions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &body)
	if err != nil {
		return TranscribeOutput{}, fmt.Errorf("build request failed: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return TranscribeOutput{}, fmt.Errorf("call asr failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return TranscribeOutput{}, fmt.Errorf("read asr response failed: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var ae apiError
		if err := json.Unmarshal(respBytes, &ae); err == nil && strings.TrimSpace(ae.Error.Message) != "" {
			return TranscribeOutput{}, fmt.Errorf("asr request failed: %s", ae.Error.Message)
		}
		return TranscribeOutput{}, fmt.Errorf("asr request failed: status=%d body=%s", resp.StatusCode, truncate(string(respBytes), 600))
	}

	var out TranscribeOutput
	if err := json.Unmarshal(respBytes, &out); err != nil {
		return TranscribeOutput{}, fmt.Errorf("parse asr response failed: %w, body=%s", err, truncate(string(respBytes), 600))
	}

	if len(out.Segments) == 0 {
		return TranscribeOutput{}, fmt.Errorf("asr response has no segments, model may not support verbose_json/segment")
	}

	if out.Duration <= 0 {
		last := out.Segments[len(out.Segments)-1]
		if last.End > 0 {
			out.Duration = last.End
		}
	}

	return out, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
