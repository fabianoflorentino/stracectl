package pipeline

import (
	"github.com/fabianoflorentino/stracectl/internal/privacy"
)

// Process runs the pipeline for a single TraceEvent using provided components.
func Process(e *privacy.TraceEvent, filter privacy.Filter, redactor privacy.Redactor, formatter privacy.Formatter, output privacy.Output) error {
	if filter != nil {
		if !filter.Allow(e) {
			return nil
		}
	}

	if redactor != nil {
		if err := redactor.Redact(e); err != nil {
			return err
		}
	}

	if formatter == nil || output == nil {
		// Nothing to do — allow callers to compose incrementally.
		return nil
	}

	b, err := formatter.Format(e)
	if err != nil {
		return err
	}

	return output.Write(b)
}
