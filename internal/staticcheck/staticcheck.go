package staticcheck

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

type Finding struct {
	Severity string // "high" | "medium" | "low"
	Rule     string
	Detail   string
}

func (f Finding) String() string {
	return fmt.Sprintf("[%s] %s: %s", strings.ToUpper(f.Severity), f.Rule, f.Detail)
}

var rules = []struct {
	name     string
	severity string
	pat      *regexp.Regexp
	note     string
}{
	{
		name:     "pipe-to-shell",
		severity: "high",
		pat:      regexp.MustCompile(`(?i)(curl|wget)\s+[^|]*\|\s*(bash|sh|zsh|fish|python|perl|ruby)`),
		note:     "downloads piped directly into a shell interpreter",
	},
	{
		name:     "eval-base64",
		severity: "high",
		pat:      regexp.MustCompile(`(?i)eval\s*[\$\(].*base64`),
		note:     "eval of base64-decoded content",
	},
	{
		name:     "base64-eval",
		severity: "high",
		pat:      regexp.MustCompile(`(?i)base64\s+--?decode[^|]*\|[^|]*eval`),
		note:     "base64 decoded content passed to eval",
	},
	{
		name:     "base64-pipe-shell",
		severity: "high",
		pat:      regexp.MustCompile(`(?i)base64\s+--?decode[^|]*\|\s*(bash|sh|zsh|python|perl|ruby)`),
		note:     "base64 decoded content piped to shell",
	},
	{
		name:     "sudo-in-build",
		severity: "high",
		pat:      regexp.MustCompile(`(?m)^\s*sudo\s+`),
		note:     "sudo usage inside PKGBUILD (should never be needed)",
	},
	{
		name:     "rm-rf-root",
		severity: "high",
		pat:      regexp.MustCompile(`rm\s+(-\w*r\w*f\w*|-\w*f\w*r\w*)\s+(\/\s|\/\*|"\/"|'/')`),
		note:     "recursive force-delete of root or near-root path",
	},
	{
		name:     "home-exfil",
		severity: "high",
		pat:      regexp.MustCompile(`(?i)(curl|wget|nc|ncat)\s+[^#]*(\$HOME|~/|\.ssh|\.gnupg|\.config)`),
		note:     "potential exfiltration of home directory contents",
	},
	{
		name:     "env-exfil",
		severity: "medium",
		pat:      regexp.MustCompile(`(?i)(curl|wget)\s+[^#]*\$\{?(HOME|USER|LOGNAME|PATH|SHELL|SSH_|GPG_|TOKEN|SECRET|PASSWORD|PASS|KEY|API)\b`),
		note:     "potential exfiltration of sensitive environment variables",
	},
	{
		name:     "npm-postinstall",
		severity: "medium",
		pat:      regexp.MustCompile(`(?i)(npm|bun|yarn|pnpm)\s+(install|i|add)\b`),
		note:     "npm/bun/yarn install may trigger postinstall hooks",
	},
	{
		name:     "hex-encoded-payload",
		severity: "medium",
		pat:      regexp.MustCompile(`(?i)(echo|printf)\s+['"\\x]*([0-9a-f]{20,})['"\\x]*\s*\|\s*(bash|sh|python|perl|ruby)`),
		note:     "hex-encoded content piped to interpreter",
	},
	{
		name:     "network-in-prepare",
		severity: "low",
		pat:      regexp.MustCompile(`(?i)(curl|wget|fetch)\b`),
		note:     "network access during build (may be intentional for some packages)",
	},
}

// knownGoodDomains are commonly trusted source hosts for AUR packages.
var knownGoodDomains = map[string]bool{
	"github.com":           true,
	"raw.githubusercontent.com": true,
	"codeload.github.com":  true,
	"gitlab.com":           true,
	"bitbucket.org":        true,
	"pypi.org":             true,
	"files.pythonhosted.org": true,
	"crates.io":            true,
	"static.crates.io":     true,
	"registry.npmjs.org":   true,
	"download.gnome.org":   true,
	"download.kde.org":     true,
	"ftp.gnu.org":          true,
	"kernel.org":           true,
	"xorg.freedesktop.org": true,
	"freedesktop.org":      true,
	"sourceforge.net":      true,
	"downloads.sourceforge.net": true,
	"launchpad.net":        true,
	"cpan.org":             true,
	"search.cpan.org":      true,
	"metacpan.org":         true,
	"rubygems.org":         true,
	"hackage.haskell.org":  true,
	"cabal.haskell.org":    true,
	"archive.org":          true,
}

var sourceURLPat = regexp.MustCompile(`"(https?://[^"]+)"`)
var pkgnamePat = regexp.MustCompile(`(?m)^pkgname\s*=\s*['"]?([^\s'"#]+)`)

// Run checks the PKGBUILD content and returns any findings.
func Run(content string) []Finding {
	var findings []Finding

	// pattern-based rules
	for _, r := range rules {
		if r.pat.MatchString(content) {
			findings = append(findings, Finding{
				Severity: r.severity,
				Rule:     r.name,
				Detail:   r.note,
			})
		}
	}

	// source= URL domain check
	findings = append(findings, checkSourceDomains(content)...)

	return findings
}

func checkSourceDomains(content string) []Finding {
	// find source=(...) block
	sourceBlockPat := regexp.MustCompile(`(?s)source\s*=\s*\([^)]*\)`)
	block := sourceBlockPat.FindString(content)
	if block == "" {
		return nil
	}

	var findings []Finding
	for _, match := range sourceURLPat.FindAllStringSubmatch(block, -1) {
		rawURL := match[1]
		u, err := url.Parse(rawURL)
		if err != nil {
			continue
		}
		host := strings.ToLower(u.Hostname())
		if host == "" {
			continue
		}
		// strip www. prefix
		host = strings.TrimPrefix(host, "www.")
		if !knownGoodDomains[host] {
			findings = append(findings, Finding{
				Severity: "medium",
				Rule:     "unknown-source-domain",
				Detail:   fmt.Sprintf("source URL from unfamiliar domain: %s", host),
			})
		}
	}
	return findings
}

// HighestSeverity returns the highest severity level across all findings,
// or "" if there are none.
func HighestSeverity(findings []Finding) string {
	level := 0
	for _, f := range findings {
		switch f.Severity {
		case "high":
			if level < 3 {
				level = 3
			}
		case "medium":
			if level < 2 {
				level = 2
			}
		case "low":
			if level < 1 {
				level = 1
			}
		}
	}
	switch level {
	case 3:
		return "high"
	case 2:
		return "medium"
	case 1:
		return "low"
	}
	return ""
}
