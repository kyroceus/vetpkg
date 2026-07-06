package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"syscall"
	"time"

	"vetpkg/internal/analyzer"
	"vetpkg/internal/cache"
	"vetpkg/internal/config"
	"vetpkg/internal/diff"
	"vetpkg/internal/staticcheck"
)

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorGreen  = "\033[32m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fatalf("vetpkg: load config: %v\n", err)
	}

	pkgbuild, err := os.ReadFile("PKGBUILD")
	if err != nil {
		// No PKGBUILD in cwd — pass through directly (e.g. vetpkg --version)
		execMakepkg(cfg.General.MakepkgPath, os.Args[1:])
	}

	content := string(pkgbuild)
	pkgname := extractPkgname(content)
	if pkgname == "" {
		pkgname = "unknown"
	}

	hash := cache.Hash(content)
	approvedHash, err := cache.GetApprovedHash(pkgname)
	if err != nil {
		fatalf("vetpkg: read cache: %v\n", err)
	}

	if hash == approvedHash {
		// Unchanged — skip review and build
		execMakepkg(cfg.General.MakepkgPath, os.Args[1:])
	}

	// --- Changed or new package: run the review gate ---
	previousContent, err := cache.GetApprovedContent(pkgname)
	if err != nil {
		fatalf("vetpkg: read cached content: %v\n", err)
	}

	unified := diff.Unified(previousContent, content)
	findings := staticcheck.Run(content)
	highestStatic := staticcheck.HighestSeverity(findings)

	printHeader(pkgname, previousContent == "")
	printDiff(unified, previousContent == "")
	printStaticFindings(findings)

	// LLM analysis
	var llmResult *analyzer.Result
	if cfg.Analyzer.Backend != "none" {
		llmResult = runLLM(cfg, content, unified)
		if llmResult != nil {
			printLLMResult(llmResult)
		}
	}

	// Determine combined risk
	overallRisk := combineRisk(highestStatic, llmResult)

	printRiskSummary(overallRisk)

	// Auto-approve low risk if configured (never for high)
	if cfg.General.AutoApproveLowRisk && overallRisk == "low" {
		fmt.Printf("%svetpkg: auto-approving low-risk package %q%s\n", colorGreen, pkgname, colorReset)
		saveAndExec(cfg, pkgname, content)
	}

	// Interactive prompt
	approved := promptUser(pkgname, overallRisk, content)
	if !approved {
		fmt.Fprintf(os.Stderr, "%svetpkg: build declined by user%s\n", colorRed, colorReset)
		os.Exit(1)
	}

	saveAndExec(cfg, pkgname, content)
}

func printHeader(pkgname string, isNew bool) {
	status := "updated"
	if isNew {
		status = "new (no previous approved version)"
	}
	fmt.Printf("\n%s%s=== vetpkg review: %s (%s) ===%s\n\n",
		colorBold, colorCyan, pkgname, status, colorReset)
}

func printDiff(unified string, isNew bool) {
	if isNew {
		fmt.Printf("%s[new package — no diff available]%s\n\n", colorDim, colorReset)
		return
	}
	if unified == "" {
		return
	}
	fmt.Printf("%s%s--- PKGBUILD diff ---%s\n", colorBold, colorDim, colorReset)
	for _, line := range strings.Split(unified, "\n") {
		switch {
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			fmt.Printf("%s%s%s\n", colorGreen, line, colorReset)
		case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
			fmt.Printf("%s%s%s\n", colorRed, line, colorReset)
		case strings.HasPrefix(line, "@@"):
			fmt.Printf("%s%s%s\n", colorCyan, line, colorReset)
		default:
			fmt.Println(line)
		}
	}
	fmt.Println()
}

func printStaticFindings(findings []staticcheck.Finding) {
	if len(findings) == 0 {
		fmt.Printf("%s[static checks] no issues found%s\n\n", colorGreen, colorReset)
		return
	}
	fmt.Printf("%s%s[static checks] %d finding(s):%s\n", colorBold, colorYellow, len(findings), colorReset)
	for _, f := range findings {
		color := colorYellow
		if f.Severity == "high" {
			color = colorRed
		} else if f.Severity == "low" {
			color = colorDim
		}
		fmt.Printf("  %s%s%s\n", color, f.String(), colorReset)
	}
	fmt.Println()
}

func runLLM(cfg *config.Config, content, unified string) *analyzer.Result {
	fmt.Printf("%s[llm] running analysis via %s...%s\n", colorDim, cfg.Analyzer.Backend, colorReset)

	var a analyzer.Analyzer
	switch cfg.Analyzer.Backend {
	case "claude":
		a = &analyzer.ClaudeAnalyzer{APIKey: cfg.Claude.APIKey, Model: cfg.Claude.Model}
	case "ollama":
		a = &analyzer.OllamaAnalyzer{Endpoint: cfg.Ollama.Endpoint, Model: cfg.Ollama.Model, Timeout: time.Duration(cfg.Ollama.TimeoutSeconds) * time.Second}
	case "multi":
		a = &analyzer.MultiAnalyzer{
			Backends: []analyzer.Analyzer{
				&analyzer.ClaudeAnalyzer{APIKey: cfg.Claude.APIKey, Model: cfg.Claude.Model},
				&analyzer.OllamaAnalyzer{Endpoint: cfg.Ollama.Endpoint, Model: cfg.Ollama.Model, Timeout: time.Duration(cfg.Ollama.TimeoutSeconds) * time.Second},
			},
		}
	default:
		fmt.Printf("%s[llm] unknown backend %q, skipping%s\n", colorYellow, cfg.Analyzer.Backend, colorReset)
		return nil
	}

	result, err := a.Analyze(content, unified)
	if err != nil {
		fmt.Printf("%s[llm] analysis failed: %v%s\n", colorYellow, err, colorReset)
		return nil
	}
	return result
}

func printLLMResult(r *analyzer.Result) {
	color := colorGreen
	if r.Risk == analyzer.RiskHigh {
		color = colorRed
	} else if r.Risk == analyzer.RiskMedium {
		color = colorYellow
	}
	fmt.Printf("%s%s[llm/%s] risk: %s%s\n", colorBold, color, r.Backend, r.Risk, colorReset)
	for _, reason := range r.Reasons {
		fmt.Printf("  %s- %s%s\n", color, reason, colorReset)
	}
	fmt.Println()
}

func printRiskSummary(overall string) {
	color := colorGreen
	if overall == "high" {
		color = colorRed
	} else if overall == "medium" {
		color = colorYellow
	}
	fmt.Printf("%s%s[overall risk: %s]%s\n\n", colorBold, color, strings.ToUpper(overall), colorReset)
}

func combineRisk(staticSeverity string, llm *analyzer.Result) string {
	risks := map[string]int{"": 0, "low": 1, "medium": 2, "high": 3}
	best := staticSeverity
	if llm != nil && risks[string(llm.Risk)] > risks[best] {
		best = string(llm.Risk)
	}
	if best == "" {
		best = "low"
	}
	return best
}

func promptUser(pkgname, overallRisk, fullContent string) bool {
	reader := bufio.NewReader(os.Stdin)
	for {
		if overallRisk == "high" {
			fmt.Printf("%s%s[!] High risk detected. Approve anyway? [y/N/s=show full PKGBUILD]: %s",
				colorBold, colorRed, colorReset)
		} else {
			fmt.Printf("%s[vetpkg] Approve build of %q? [y/N/s=show full PKGBUILD]: %s",
				colorBold, pkgname, colorReset)
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			return false
		}
		choice := strings.ToLower(strings.TrimSpace(line))
		switch choice {
		case "y", "yes":
			return true
		case "s", "show":
			fmt.Printf("\n%s--- full PKGBUILD ---%s\n%s\n%s--- end PKGBUILD ---%s\n\n",
				colorDim, colorReset, fullContent, colorDim, colorReset)
		default:
			return false
		}
	}
}

func saveAndExec(cfg *config.Config, pkgname, content string) {
	if err := cache.SaveApproved(pkgname, content); err != nil {
		fatalf("vetpkg: save cache: %v\n", err)
	}
	execMakepkg(cfg.General.MakepkgPath, os.Args[1:])
}

// execMakepkg hands off to the real makepkg. vetpkg is a distinct binary
// name, so resolving "makepkg" via $PATH here can never recurse back into
// vetpkg itself — unlike the old shadow-binary design, no pre-pinned path
// is required.
func execMakepkg(configuredPath string, args []string) {
	binary := configuredPath
	if binary == "" {
		var err error
		binary, err = exec.LookPath("makepkg")
		if err != nil {
			fatalf("vetpkg: could not find makepkg in $PATH: %v\n", err)
		}
	}
	argv := append([]string{binary}, args...)
	if err := syscall.Exec(binary, argv, os.Environ()); err != nil {
		fatalf("vetpkg: exec %s: %v\n", binary, err)
	}
}

var pkgnamePat = regexp.MustCompile(`(?m)^pkgname\s*=\s*['"]?([^\s'"#(]+)`)

func extractPkgname(content string) string {
	m := pkgnamePat.FindStringSubmatch(content)
	if len(m) < 2 {
		return ""
	}
	return strings.Trim(m[1], `'"`)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format, args...)
	os.Exit(1)
}
