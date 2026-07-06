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

const defaultOllamaTimeout = 5 * time.Minute

type OllamaAnalyzer struct {
	Endpoint string
	Model    string
	Timeout  time.Duration // defaults to 5 minutes if zero
}

func (o *OllamaAnalyzer) Name() string { return "ollama" }

func (o *OllamaAnalyzer) Analyze(pkgbuild, diff string) (*Result, error) {
	endpoint := strings.TrimRight(o.Endpoint, "/") + "/api/generate"

	request := map[string]any{
		"model":  o.Model,
		"prompt": systemPrompt + "\n\n" + buildPrompt(pkgbuild, diff),
		"stream": false,
		"think": false,
	}
	reqBody, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("content-type", "application/json")

	timeout := o.Timeout
	if timeout <= 0 {
		timeout = defaultOllamaTimeout
	}
	client := &http.Client{Timeout: timeout}
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
		Response string `json:"response"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("parse ollama response: %w", err)
	}

	text := strings.TrimSpace(apiResp.Response)
	result, err := parseResult(text)
	if err != nil {
		return nil, fmt.Errorf("parse ollama JSON output: %w (raw: %s)", err, text)
	}
	result.Backend = o.Name()
	return result, nil
}
