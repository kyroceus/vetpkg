package analyzer

import "fmt"

// MultiAnalyzer runs several backends and escalates to the highest risk.
// It never averages risk down — any high finding makes the result high.
type MultiAnalyzer struct {
	Backends []Analyzer
}

func (m *MultiAnalyzer) Name() string { return "multi" }

func (m *MultiAnalyzer) Analyze(pkgbuild, diff string) (*Result, error) {
	if len(m.Backends) == 0 {
		return nil, fmt.Errorf("MultiAnalyzer has no backends configured")
	}

	combined := &Result{Risk: RiskLow}

	for _, backend := range m.Backends {
		r, err := backend.Analyze(pkgbuild, diff)
		if err != nil {
			combined.Reasons = append(combined.Reasons,
				fmt.Sprintf("[%s] analysis failed: %v", backend.Name(), err))
			continue
		}
		combined.Risk = Higher(combined.Risk, r.Risk)
		for _, reason := range r.Reasons {
			combined.Reasons = append(combined.Reasons,
				fmt.Sprintf("[%s] %s", backend.Name(), reason))
		}
	}

	combined.Backend = m.Name()
	return combined, nil
}
