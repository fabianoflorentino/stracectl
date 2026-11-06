package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/fabianoflorentino/stracectl/internal/models"
	"github.com/fabianoflorentino/stracectl/internal/ndjson"
	p "github.com/fabianoflorentino/stracectl/internal/privacy"
	pfilters "github.com/fabianoflorentino/stracectl/internal/privacy/filters"
	predact "github.com/fabianoflorentino/stracectl/internal/privacy/redactor"
)

var saveEventsPath string

// saveEventsSink writes parsed events as NDJSON after applying the configured
// privacy filter and redaction settings.
type saveEventsSink struct {
	writer   *ndjson.Writer
	filter   *pfilters.FilterSet
	redactor *predact.Redactor
}

func newSaveEventsSink(path string) (*saveEventsSink, error) {
	writer, err := ndjson.NewFile(path)
	if err != nil {
		return nil, err
	}

	redactor, err := newPrivacyRedactor()
	if err != nil {
		_ = writer.Close()
		return nil, fmt.Errorf("initializing save-events redactor: %w", err)
	}

	return &saveEventsSink{
		writer:   writer,
		filter:   pfilters.New(privacySyscalls, privacyExclude, nil, nil),
		redactor: redactor,
	}, nil
}

// Write applies the privacy filter/redaction pipeline and writes one event.
func (s *saveEventsSink) Write(event models.SyscallEvent) error {
	te := p.NewTraceEventFromModel(event)
	te.Args = []p.Arg{{Name: "raw", Value: []byte(event.Args)}}

	if s.filter != nil && !s.filter.Allow(&te) {
		return nil
	}

	if s.redactor != nil {
		if err := s.redactor.Redact(&te); err != nil {
			return err
		}
	}

	if len(te.Args) == 0 {
		event.Args = ""
	} else {
		event.Args = string(te.Args[0].Value)
	}

	return s.writer.Write(event)
}

// Close flushes and closes the underlying file.
func (s *saveEventsSink) Close() error {
	return s.writer.Close()
}

// openSaveEventsSink returns nil when --save-events is not set.
func openSaveEventsSink() (*saveEventsSink, error) {
	if saveEventsPath == "" {
		return nil, nil
	}
	return newSaveEventsSink(saveEventsPath)
}

// closeSaveEventsSink closes the sink and reports close failures without
// interrupting the trace shutdown path.
func closeSaveEventsSink(s *saveEventsSink) {
	if s == nil {
		return
	}
	if err := s.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: closing save-events file: %v\n", err)
	}
}

// writeSavedEvent writes one event and disables the sink after a write error
// to avoid flooding stderr for the rest of the trace.
func writeSavedEvent(s *saveEventsSink, event models.SyscallEvent) *saveEventsSink {
	if s == nil {
		return nil
	}
	if err := s.Write(event); err != nil {
		fmt.Fprintf(os.Stderr, "warning: save-events write failed: %v\n", err)
		_ = s.Close()
		return nil
	}
	return s
}

func addSaveEventsFlag(cmd *cobra.Command) {
	cmd.Flags().StringVar(&saveEventsPath, "save-events", "", "write parsed syscall events as NDJSON to this file")
}
