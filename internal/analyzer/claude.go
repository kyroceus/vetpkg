package analyzer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const claudeEndpoint = "https://api.anthropic.com/v1/messages"
const anthropicVersion = "2023-06-01"

type ClaudeAnalyzer struct {
	APIKey string
	Model  string
}

func (c *ClaudeAnalyzer) Name() string { return "claude" }

func (c *ClaudeAnalyzer) Analyze(pkgbuild, diff string) (*Result, error) {
	if c.APIKey == "" {
		return nil, ErrNoKey
	}

	userContent := buildPrompt(pkgbuild, diff)

	reqBody, err := json.Marshal(map[string]any{
		"model":      c.Model,
		"max_tokens": 512,
		"system":     systemPrompt,
		"messages": []map[string]string{
			{"role": "user", "content": userContent},
		},
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, claudeEndpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", c.APIKey)
	req.Header.Set("anthropic-version", anthropicVersion)
	req.Header.Set("content-type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("claude API error %d: %s", resp.StatusCode, body)
	}

	var apiResp struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("parse claude response: %w", err)
	}
	if len(apiResp.Content) == 0 {
		return nil, fmt.Errorf("empty response from claude")
	}

	text := strings.TrimSpace(apiResp.Content[0].Text)
	result, err := parseResult(text)
	if err != nil {
		return nil, fmt.Errorf("parse claude JSON output: %w (raw: %s)", err, text)
	}
	result.Backend = c.Name()
	return result, nil
}

func buildPrompt(pkgbuild, diff string) string {
	var sb strings.Builder
	sb.WriteString("PKGBUILD:\n```\n")
	sb.WriteString(pkgbuild)
	sb.WriteString("\n```\n")
	if diff != "" {
		sb.WriteString("\nDiff from previous approved version:\n```diff\n")
		sb.WriteString(diff)
		sb.WriteString("\n```\n")
	}
	return sb.String()
}

func parseResult(text string) (*Result, error) {
	// strip any accidental markdown fences
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)

	var r Result
	if err := json.Unmarshal([]byte(text), &r); err != nil {
		return nil, err
	}
	if r.Risk != RiskLow && r.Risk != RiskMedium && r.Risk != RiskHigh {
		return nil, fmt.Errorf("invalid risk value %q", r.Risk)
	}
	return &r, nil
}
