---
name: golang-testing
description: Go testing patterns including table-driven tests, subtests, benchmarks, fuzzing, and test coverage. Follows TDD methodology with idiomatic Go practices.
origin: ECC
---

# Go Testing Patterns

Comprehensive Go testing patterns for writing reliable, maintainable tests following TDD methodology.

## When to Activate

- Writing new Go functions or methods
- Adding test coverage to existing code
- Creating benchmarks for performance-critical code
- Implementing fuzz tests for input validation
- Following TDD workflow in Go projects

## Testing Philosophy for This Project

**No third-party test frameworks.** Use only the standard library `testing` package. No `testify`, no `gomock`, no `ginkgo`. This keeps the test dependency surface minimal and the patterns explicit.

**Fake implementations via local structs** — not code-generated mocks. Define small structs in test files that satisfy the interface being tested.

**Inject dependencies at construction time** — either via exported `New()` constructors or unexported `newWith*()` variants reserved for tests.

## White-box vs Black-box Testing

Choose the test package name deliberately:

```go
// White-box (same package): access unexported helpers
// File: aggregator_test.go
package aggregator

func TestAddLocked(t *testing.T) {
    a := newWithClock(func() time.Time { return fixedTime })
    // can call unexported: a.addLocked(), a.snapshotLocked(), etc.
}

// Black-box (external package): only public API
// File: server_test.go
package server_test

import "github.com/fabianoflorentino/stracectl/internal/server"

func TestServerRoutes(t *testing.T) {
    srv := server.New(agg)
    // only public API accessible
}
```

**Rule of thumb:**
- Use white-box (`package foo`) when the unit under test has meaningful internal behavior worth verifying directly (e.g., aggregator internals, locked helpers).
- Use black-box (`package foo_test`) when testing contracts and HTTP handlers — prevents coupling tests to implementation details.

## TDD Workflow for Go

### The RED-GREEN-REFACTOR Cycle

```
RED     → Write a failing test first
GREEN   → Write minimal code to pass the test
REFACTOR → Improve code while keeping tests green
REPEAT  → Continue with next requirement
```

### Step-by-Step TDD in Go

```go
// Step 1: Define the signature
func Add(a, b int) int {
    panic("not implemented")
}

// Step 2: Write failing test (RED)
func TestAdd(t *testing.T) {
    got := Add(2, 3)
    want := 5
    if got != want {
        t.Errorf("Add(2, 3) = %d; want %d", got, want)
    }
}

// Step 3: Implement minimal code (GREEN)
func Add(a, b int) int {
    return a + b
}

// Step 4: Refactor if needed, verify tests still pass
```

## Table-Driven Tests

The standard pattern for comprehensive coverage with minimal code.

```go
func TestParseEvent(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    *Event
        wantErr bool
    }{
        {
            name:  "valid syscall",
            input: `read(3, "data", 4096) = 10`,
            want:  &Event{Name: "read", Args: `3, "data", 4096`, Ret: 10},
        },
        {
            name:    "empty input",
            input:   "",
            wantErr: true,
        },
        {
            name:    "malformed",
            input:   "not a syscall",
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := ParseEvent(tt.input)

            if tt.wantErr {
                if err == nil {
                    t.Error("expected error, got nil")
                }
                return
            }

            if err != nil {
                t.Fatalf("unexpected error: %v", err)
            }

            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("got %+v; want %+v", got, tt.want)
            }
        })
    }
}
```

## Clock Injection for Deterministic Tests

Never use `time.Sleep` in tests. Instead, inject time via a `nowFunc` field and advance the fake clock in tests.

```go
// production code
type Aggregator struct {
    nowFunc func() time.Time
    // ...
}

func New() *Aggregator {
    return &Aggregator{nowFunc: time.Now}
}

// newWithClock is unexported — only for tests in the same package
func newWithClock(fn func() time.Time) *Aggregator {
    return &Aggregator{nowFunc: fn}
}
```

```go
// aggregator_test.go (package aggregator — white-box)
func TestRateAfterAdvancingClock(t *testing.T) {
    var mu sync.Mutex
    fakeTime := time.Now()

    a := newWithClock(func() time.Time {
        mu.Lock()
        defer mu.Unlock()
        return fakeTime
    })

    a.Add(event("read"))
    a.Add(event("read"))

    // Advance time past the 500ms rate-update threshold
    mu.Lock()
    fakeTime = fakeTime.Add(600 * time.Millisecond)
    mu.Unlock()

    a.Add(event("read")) // triggers rate recalculation

    rate := a.Rate("read")
    if rate <= 0 {
        t.Errorf("expected positive rate, got %f", rate)
    }
}
```

## var-Swap for Function Injection

When a dependency is a function (not an interface), use a package-level `var` that tests can replace. Always restore the original in `t.Cleanup`.

```go
// cmd/run.go
var selectTracer = tracer.Select // var exists for testability

func runCmd(pid int) error {
    t, err := selectTracer(pid)
    // ...
}
```

```go
// cmd/run_test.go
func TestRunWithFakeTracer(t *testing.T) {
    original := selectTracer
    t.Cleanup(func() { selectTracer = original })

    selectTracer = func(pid int) (tracer.Tracer, error) {
        return &fakeTracer{events: testEvents}, nil
    }

    err := runCmd(42)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
}
```

## Fake Implementations (No Code Generation)

Prefer small local structs that implement the interface. Name them clearly and keep them minimal.

```go
// controller/interface.go
type UIController interface {
    Data()   []Item
    Cursor() int
    Width()  int
}

// render/view_test.go
type fakeCtrl struct {
    data   []Item
    cursor int
    width  int
}

func (f fakeCtrl) Data() []Item { return f.data }
func (f fakeCtrl) Cursor() int  { return f.cursor }
func (f fakeCtrl) Width() int   { return f.width }

func TestRenderView(t *testing.T) {
    ctrl := fakeCtrl{
        data:   []Item{{Name: "read", Count: 100}},
        cursor: 0,
        width:  80,
    }
    got := RenderView(ctrl)
    if !strings.Contains(got, "read") {
        t.Errorf("expected 'read' in output, got:\n%s", got)
    }
}
```

For pipeline stages, compose multiple fakes:

```go
type dummyFilter struct{}
func (dummyFilter) Filter(e Event) bool { return true }

type recordingOutput struct {
    events []Event
}
func (r *recordingOutput) Write(e Event) { r.events = append(r.events, e) }

func TestPipeline(t *testing.T) {
    out := &recordingOutput{}
    p := NewPipeline(dummyFilter{}, out)
    p.Process(Event{Name: "read"})

    if len(out.events) != 1 {
        t.Errorf("expected 1 event, got %d", len(out.events))
    }
}
```

## Test File Organization

Split tests across focused files rather than one large `*_test.go`. Each file tests a logical concern:

```
aggregator/
├── aggregator.go
├── aggregator_test.go      # core Add, Rate, Concurrent
├── types_test.go           # AvgTime, ErrPct, TopErrors
├── parse_test.go           # path/arg extraction helpers
├── lathist_test.go         # latency histogram / percentiles
├── errwindow_test.go       # rolling error window
├── sorted_test.go          # Sorted(), snapshot
└── extra_test.go           # coverage-gap tests added later
```

Use `extra_test.go` or `coverage_test.go` for tests added after the fact to close coverage gaps — keeps the original test files clean.

## Test Helpers

### Helper Functions with t.Helper()

```go
func assertNoError(t *testing.T, err error) {
    t.Helper()
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
}

func assertEqual[T comparable](t *testing.T, got, want T) {
    t.Helper()
    if got != want {
        t.Errorf("got %v; want %v", got, want)
    }
}
```

### Event Builders for Readability

Define compact builder functions in test files to reduce noise in table-driven tests:

```go
// aggregator_test.go
func event(name string) SyscallEvent {
    return SyscallEvent{Name: name, Duration: time.Microsecond}
}

func ok(name string) SyscallEvent {
    return SyscallEvent{Name: name, Ret: 0}
}

func fail(name string) SyscallEvent {
    return SyscallEvent{Name: name, Ret: -1, Err: "ENOENT"}
}

func newPopulatedAgg() *Aggregator {
    a := New()
    for range 50 {
        a.Add(ok("read"))
    }
    for range 5 {
        a.Add(fail("open"))
    }
    return a
}
```

### Temporary Files and Directories

```go
func TestFileProcessing(t *testing.T) {
    tmpDir := t.TempDir() // automatically cleaned up

    testFile := filepath.Join(tmpDir, "test.txt")
    if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
        t.Fatalf("failed to create test file: %v", err)
    }

    result, err := ProcessFile(testFile)
    if err != nil {
        t.Fatalf("ProcessFile failed: %v", err)
    }
    _ = result
}
```

## Concurrency Tests

Test concurrent safety with multiple goroutines and a `sync.WaitGroup`. Always run with `-race`.

```go
func TestAddConcurrent(t *testing.T) {
    a := New()
    const goroutines = 20
    const addsEach = 500

    var wg sync.WaitGroup
    wg.Add(goroutines)

    for range goroutines {
        go func() {
            defer wg.Done()
            for range addsEach {
                a.Add(event("read"))
            }
        }()
    }

    wg.Wait()

    got := a.Count("read")
    want := goroutines * addsEach
    if got != want {
        t.Errorf("got %d; want %d", got, want)
    }
}
```

Run with race detector:

```bash
go test -race ./...
```

## HTTP Handler Testing

Use `httptest` for unit-level handler tests without a real listener.

```go
func TestHandleStats(t *testing.T) {
    agg := newPopulatedAgg()
    srv := New(agg)

    req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
    w := httptest.NewRecorder()

    srv.handleStats(w, req)

    if w.Code != http.StatusOK {
        t.Errorf("got status %d; want %d", w.Code, http.StatusOK)
    }

    var result []SyscallStat
    if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
        t.Fatalf("failed to decode response: %v", err)
    }

    if len(result) == 0 {
        t.Error("expected non-empty stats")
    }
}
```

For WebSocket tests that require a real TCP connection, use `httptest.NewServer`:

```go
func TestWebSocketStream(t *testing.T) {
    agg := newPopulatedAgg()
    srv := New(agg)

    ts := httptest.NewServer(srv)
    t.Cleanup(ts.Close)

    wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/stream"
    conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
    if err != nil {
        t.Fatalf("dial failed: %v", err)
    }
    defer conn.Close()

    var stats []SyscallStat
    if err := conn.ReadJSON(&stats); err != nil {
        t.Fatalf("read failed: %v", err)
    }

    if len(stats) == 0 {
        t.Error("expected non-empty stats over WebSocket")
    }
}
```

## Subtests and Parallel Tests

```go
func TestUser(t *testing.T) {
    db := setupTestDB(t)

    t.Run("Create", func(t *testing.T) {
        // ...
    })

    t.Run("Get", func(t *testing.T) {
        // ...
    })
}

// Parallel subtests — capture range variable
func TestParallel(t *testing.T) {
    tests := []struct{ name, input string }{
        {"case1", "input1"},
        {"case2", "input2"},
    }

    for _, tt := range tests {
        tt := tt // capture
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            _ = Process(tt.input)
        })
    }
}
```

## Benchmarks

```go
func BenchmarkAggregatorAdd(b *testing.B) {
    a := New()
    e := event("read")
    b.ResetTimer()

    for range b.N {
        a.Add(e)
    }
}

func BenchmarkSorted(b *testing.B) {
    a := newPopulatedAgg()
    b.ResetTimer()

    for range b.N {
        _ = a.Sorted()
    }
}
```

```bash
go test -bench=. -benchmem ./...
```

## Fuzzing (Go 1.18+)

```go
func FuzzParseEvent(f *testing.F) {
    f.Add(`read(3, "hello", 4096) = 10`)
    f.Add(`write(1, "ok", 2) = 2`)
    f.Add(``)

    f.Fuzz(func(t *testing.T, input string) {
        event, err := ParseEvent(input)
        if err != nil {
            return // invalid input is expected
        }
        // Round-trip: marshal then re-parse
        out, err := json.Marshal(event)
        if err != nil {
            t.Errorf("marshal failed after successful parse: %v", err)
        }
        _ = out
    })
}
```

```bash
go test -fuzz=FuzzParseEvent -fuzztime=30s ./...
```

## Test Coverage

```bash
# Basic coverage
go test -cover ./...

# Generate profile
go test -coverprofile=coverage.out ./...

# View in browser
go tool cover -html=coverage.out

# View by function
go tool cover -func=coverage.out

# Coverage + race detector (recommended for CI)
go test -race -coverprofile=coverage.out ./...
```

### Coverage Targets

| Code Type | Target |
|-----------|--------|
| Core aggregation logic | 90%+ |
| HTTP handlers | 85%+ |
| TUI render functions | 80%+ |
| Generated / embed code | Exclude |

## Testing Commands

```bash
# Run all tests
go test ./...

# Verbose
go test -v ./...

# Specific test
go test -run TestAggregator ./...

# Subtest pattern
go test -run "TestAggregator/Concurrent" ./...

# Race detector (always in CI)
go test -race ./...

# Coverage
go test -cover -coverprofile=coverage.out ./...

# Short tests only (skip slow integration tests)
go test -short ./...

# Timeout
go test -timeout 60s ./...

# Benchmarks
go test -bench=. -benchmem ./...

# Fuzz
go test -fuzz=FuzzParse -fuzztime=30s ./...

# Repeat (flaky test detection)
go test -count=10 ./...
```

## Best Practices

**DO:**
- Write tests FIRST (TDD)
- Use stdlib `testing` only — no testify, no gomock
- Use table-driven tests for comprehensive coverage
- Define small local fakes, not generated mocks
- Inject time via `nowFunc`, not `time.Sleep`
- Use `t.Helper()` in helper functions
- Use `t.Cleanup()` for resource teardown
- Split test files by concern, not just by source file
- White-box test internals that matter; black-box test public contracts
- Always run with `-race` in CI

**DON'T:**
- Use `time.Sleep` in tests (use clock injection or channels)
- Use global test state without cleanup
- Import third-party test frameworks
- Test private functions directly when white-box access isn't necessary
- Ignore flaky tests — fix or remove them
- Skip error path testing
