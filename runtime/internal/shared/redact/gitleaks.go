package redact

import (
	"sort"
	"strings"
	"sync"

	"github.com/zricethezav/gitleaks/v8/detect"
	"github.com/zricethezav/gitleaks/v8/report"
)

var (
	gitleaksOnce    sync.Once
	gitleaksDet     *detect.Detector
	errGitleaksInit error
)

func gitleaksDetector() (*detect.Detector, error) {
	gitleaksOnce.Do(func() {
		gitleaksDet, errGitleaksInit = detect.NewDetectorDefaultConfig()
	})
	return gitleaksDet, errGitleaksInit
}

func applyGitleaks(s string) (string, bool) {
	d, err := gitleaksDetector()
	if err != nil || d == nil {
		return s, false
	}
	findings := d.Detect(detect.Fragment{Raw: s})
	if len(findings) == 0 {
		return s, false
	}
	return replaceFindings(s, findings), true
}

func replaceFindings(s string, findings []report.Finding) string {
	replacements := make(map[string]struct{})
	for _, f := range findings {
		if f.Secret != "" {
			replacements[f.Secret] = struct{}{}
		}
		if f.Match != "" {
			replacements[f.Match] = struct{}{}
		}
	}
	keys := make([]string, 0, len(replacements))
	for k := range replacements {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return len(keys[i]) > len(keys[j])
	})

	out := s
	for _, k := range keys {
		out = strings.ReplaceAll(out, k, credentialPlaceholder)
	}
	return out
}
