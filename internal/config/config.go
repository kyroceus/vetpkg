package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	Analyzer AnalyzerConfig `json:"analyzer"`
	Claude   ClaudeConfig   `json:"claude"`
	Ollama   OllamaConfig   `json:"ollama"`
	General  GeneralConfig  `json:"general"`
}

type AnalyzerConfig struct {
	Backend string `json:"backend"` // "claude" | "ollama" | "multi" | "none"
}

type ClaudeConfig struct {
	Model  string `json:"model"`
	APIKey string `json:"api_key"` // overridden by ANTHROPIC_API_KEY env var
}

type OllamaConfig struct {
	Endpoint string `json:"endpoint"`
	Model    string `json:"model"`
}

type GeneralConfig struct {
	// MakepkgPath optionally pins the real makepkg binary to an exact path.
	// Leave empty to resolve "makepkg" from $PATH at run time (the default —
	// safe because vetpkg is a distinct binary name, so there's no risk of
	// it resolving back to itself).
	MakepkgPath        string `json:"makepkg_path"`
	AutoApproveLowRisk bool   `json:"auto_approve_low_risk"`
}

func Load() (*Config, error) {
	cfg := &Config{
		Analyzer: AnalyzerConfig{Backend: "claude"},
		Claude:   ClaudeConfig{Model: "claude-sonnet-4-6"},
		Ollama:   OllamaConfig{Endpoint: "http://localhost:11434", Model: "llama3.1"},
		General:  GeneralConfig{},
	}

	path := filepath.Join(os.Getenv("HOME"), ".config", "vetpkg", "config.json")
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return applyEnv(cfg), nil
		}
		return nil, err
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(cfg); err != nil {
		return nil, err
	}
	return applyEnv(cfg), nil
}

func applyEnv(cfg *Config) *Config {
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		cfg.Claude.APIKey = key
	}
	return cfg
}
