package cmd

import (
	"strings"

	predact "github.com/fabianoflorentino/stracectl/internal/privacy/redactor"
)

// derivedPrivacyNoArgs reports whether arguments should be suppressed for the
// configured privacy level.
func derivedPrivacyNoArgs() bool {
	return privacyNoArgs || privacyPrivacyLevel == "high"
}

// privacyRedactionPatterns returns the user-provided redaction patterns.
func privacyRedactionPatterns() []string {
	if privacyRedactPatterns == "" {
		return nil
	}

	patterns := make([]string, 0)
	for _, s := range strings.Split(privacyRedactPatterns, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			patterns = append(patterns, s)
		}
	}
	return patterns
}

// newPrivacyRedactor builds a redactor from the global privacy flags.
func newPrivacyRedactor() (*predact.Redactor, error) {
	return predact.New(predact.Config{
		NoArgs:     derivedPrivacyNoArgs(),
		MaxArgSize: privacyMaxArgSize,
		Patterns:   privacyRedactionPatterns(),
	})
}
