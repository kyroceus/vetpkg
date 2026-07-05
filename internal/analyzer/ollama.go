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

type OllamaAnalyzer struct {
	Endpoint string
	Model    string
}

func (o *OllamaAnalyzer) Name() string { return "ollama" }

func (o *OllamaAnalyzer) Analyze(pkgbuild, diff string) (*Result, error) {
	endpoint := strings.TrimRight(o.Endpoint, "/") + "/api/chat"

	reqBody, err := json.Marshal(map[string]any{
		"model":  o.Model,
		"stream": false,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": buildPrompt(pkgbuild, diff)},
		},
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("content-type", "application/json")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama error %d: %s", resp.StatusCode, body)
	}

	var apiResp struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("parse ollama response: %w", err)
	}

	text := strings.TrimSpace(apiResp.Message.Content)
	result, err := parseResult(text)
	if err != nil {
		return nil, fmt.Errorf("parse ollama JSON output: %w (raw: %s)", err, text)
	}
	result.Backend = o.Name()
	return result, nil
}
