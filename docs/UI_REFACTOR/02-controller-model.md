UI refactor — Controller and model interfaces

This commit introduces a small interface layer to decouple renderers from the
Bubble Tea model:

- `internal/ui/controller` defines `UIController` interface implemented by the
  TUI model.
- `internal/ui/model` contains `AggregatorView` interface allowing renderers to
  query aggregator state without depending on concrete aggregator types.

Rationale:
- Separation of concerns: rendering code depends on small, well-defined
  interfaces instead of a large model struct.
- Facilitates unit testing of renderers using lightweight stubs.
