package overlays

import "github.com/fabianoflorentino/stracectl/internal/ui/model"

// Overlay is a UI overlay (help, detail, log, files). Implementations render
// themselves given the UI model state and return the rendered string.
type Overlay interface {
	Render(m model.AggregatorView) string
}
