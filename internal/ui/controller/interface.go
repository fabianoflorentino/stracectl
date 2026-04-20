package controller

import (
	"time"

	"github.com/fabianoflorentino/stracectl/internal/aggregator"
	umodel "github.com/fabianoflorentino/stracectl/internal/ui/model"
)

// UIController exposes a minimal surface needed by renderers to build UI views
// without depending on the concrete TUI implementation. Implemented by *ui.model.
type UIController interface {
	Width() int
	Height() int
	Agg() umodel.AggregatorView
	SortBy() aggregator.SortField
	Filter() string
	Editing() bool
	ProcessDone() bool
	Cursor() int
	LogOffsetPtr() *int
	FilesOffsetPtr() *int
	Started() time.Time
	Target() string
	PerPID() bool
}
