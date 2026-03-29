package redactor

import (
	"bytes"
	"regexp"

	"github.com/fabianoflorentino/stracectl/internal/privacy"
)

// Config holds redaction configuration.
type Config struct {
	NoArgs     bool
	MaxArgSize int // bytes, 0 means no truncation
	Patterns   []string
}

// Redactor implements the privacy.Redactor interface.
type Redactor struct {
	noArgs     bool
	maxArgSize int
	regexes    []*regexp.Regexp
}

// New creates a Redactor with compiled patterns.
func New(cfg Config) (*Redactor, error) {
	r := &Redactor{
		noArgs:     cfg.NoArgs,
		maxArgSize: cfg.MaxArgSize,
	}

	// Default patterns if none provided.
	if len(cfg.Patterns) == 0 {
		cfg.Patterns = []string{
			`([A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+)`, // JWT
			`[\w.+-]+@[\w-]+\.[\w.-]+`,                         // email
			`(?i)(?:api[_-]?key|token|secret|password|passwd)=\s*([^&\s]+)`,
		}
	}

	for _, p := range cfg.Patterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, err
		}
		r.regexes = append(r.regexes, re)
	}

	return r, nil
}

// mask replaces matches with a fixed mask preserving length where sensible.
func (r *Redactor) mask(match []byte) []byte {
	// Simple deterministic mask: replace with '*' same length but at least 4 chars.
	l := len(match)
	if l < 4 {
		l = 4
	}
	return bytes.Repeat([]byte("*"), l)
}

// applyPatterns applies all configured regex replacements to data.
func (r *Redactor) applyPatterns(data []byte) []byte {
	out := data
	for _, re := range r.regexes {
		out = re.ReplaceAllFunc(out, r.mask)
	}
	return out
}

// Redact implements privacy.Redactor. It modifies the TraceEvent in-place.
func (r *Redactor) Redact(e *privacy.TraceEvent) error {
	if e == nil {
		return nil
	}

	if r.noArgs {
		e.Args = nil
		e.RawPayload = nil
		return nil
	}

	for i := range e.Args {
		v := e.Args[i].Value
		if r.maxArgSize > 0 && len(v) > r.maxArgSize {
			v = v[:r.maxArgSize]
		}
		v = r.applyPatterns(v)
		e.Args[i].Value = v
	}

	if len(e.RawPayload) > 0 {
		rp := e.RawPayload
		if r.maxArgSize > 0 && len(rp) > r.maxArgSize {
			rp = rp[:r.maxArgSize]
		}
		e.RawPayload = r.applyPatterns(rp)
	}

	return nil
}
