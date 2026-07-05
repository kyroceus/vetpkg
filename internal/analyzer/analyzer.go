package analyzer

import "errors"

// Risk levels ordered low < medium < high.
type Risk string

const (
	RiskLow    Risk = "low"
	RiskMedium Risk = "medium"
	RiskHigh   Risk = "high"
)

// Result is the structured output from an LLM analysis.
type Result struct {
	Risk    Risk     `json:"risk"`
	Reasons []string `json:"reasons"`
	Backend string   `json:"-"` // set by the implementation, not from JSON
}

var ErrNoKey = errors.New("no API key configured")

// Analyzer is the interface every backend must implement.
type Analyzer interface {
	Analyze(pkgbuild, diff string) (*Result, error)
	Name() string
}

func rankRisk(r Risk) int {
	switch r {
	case RiskHigh:
		return 3
	case RiskMedium:
		return 2
	case RiskLow:
		return 1
	}
	return 0
}

// Higher returns whichever risk level is more severe.
func Higher(a, b Risk) Risk {
	if rankRisk(a) >= rankRisk(b) {
		return a
	}
	return b
}

const systemPrompt = `You are a security analyst reviewing Arch Linux PKGBUILD scripts for supply-chain attack indicators.

Analyze the provided PKGBUILD and diff for malicious or suspicious behaviour. Focus on:
- Exfiltration of credentials, SSH keys, environment variables, or home directory contents
- Execution of downloaded or dynamically generated code (pipe to shell, eval, etc.)
- Persistence mechanisms (systemd units, cron, shell profile modification)
- Network connections during the build phase to unexpected hosts
- Obfuscated payloads (base64, hex, compressed blobs)
- Suspicious source= URLs or integrity mismatches

Respond with ONLY valid JSON in this exact shape — no markdown, no explanation:
{"risk":"low|medium|high","reasons":["reason1","reason2"]}`
