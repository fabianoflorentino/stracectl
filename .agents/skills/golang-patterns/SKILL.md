---
name: golang-patterns
description: Idiomatic Go patterns, best practices, and conventions for building robust, efficient, and maintainable Go applications.
origin: ECC
---

# Go Development Patterns

Idiomatic Go patterns and best practices for building robust, efficient, and maintainable applications.

## When to Activate

- Writing new Go code
- Reviewing Go code
- Refactoring existing Go code
- Designing Go packages/modules

## Core Principles

### 1. Simplicity and Clarity

Go favors simplicity over cleverness. Code should be obvious and easy to read.

```go
// Good: Clear and direct
func GetUser(id string) (*User, error) {
    user, err := db.FindUser(id)
    if err != nil {
        return nil, fmt.Errorf("get user %s: %w", id, err)
    }
    return user, nil
}

// Bad: Overly clever
func GetUser(id string) (*User, error) {
    return func() (*User, error) {
        if u, e := db.FindUser(id); e == nil {
            return u, nil
        } else {
            return nil, e
        }
    }()
}
```

### 2. Make the Zero Value Useful

Design types so their zero value is immediately usable without initialization.

```go
// Good: Zero value is useful
type Counter struct {
    mu    sync.Mutex
    count int // zero value is 0, ready to use
}

func (c *Counter) Inc() {
    c.mu.Lock()
    c.count++
    c.mu.Unlock()
}

// Good: bytes.Buffer works with zero value
var buf bytes.Buffer
buf.WriteString("hello")

// Bad: Requires initialization
type BadCounter struct {
    counts map[string]int // nil map will panic
}
```

### 3. Accept Interfaces, Return Structs

Functions should accept interface parameters and return concrete types.

```go
// Good: Accepts interface, returns concrete type
func ProcessData(r io.Reader) (*Result, error) {
    data, err := io.ReadAll(r)
    if err != nil {
        return nil, err
    }
    return &Result{Data: data}, nil
}

// Bad: Returns interface (hides implementation details unnecessarily)
func ProcessData(r io.Reader) (io.Reader, error) {
    // ...
}
```

## Error Handling Patterns

### Error Wrapping with Context

```go
// Good: Wrap errors with context
func LoadConfig(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("load config %s: %w", path, err)
    }

    var cfg Config
    if err := json.Unmarshal(data, &cfg); err != nil {
        return nil, fmt.Errorf("parse config %s: %w", path, err)
    }

    return &cfg, nil
}
```

### Custom Error Types

```go
// Define domain-specific errors
type ValidationError struct {
    Field   string
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation failed on %s: %s", e.Field, e.Message)
}

// Sentinel errors for common cases
var (
    ErrNotFound     = errors.New("resource not found")
    ErrUnauthorized = errors.New("unauthorized")
    ErrInvalidInput = errors.New("invalid input")
)
```

### Error Checking with errors.Is and errors.As

```go
func HandleError(err error) {
    // Check for specific error
    if errors.Is(err, sql.ErrNoRows) {
        log.Println("No records found")
        return
    }

    // Check for error type
    var validationErr *ValidationError
    if errors.As(err, &validationErr) {
        log.Printf("Validation error on field %s: %s",
            validationErr.Field, validationErr.Message)
        return
    }

    // Unknown error
    log.Printf("Unexpected error: %v", err)
}
```

### Never Ignore Errors

```go
// Bad: Ignoring error with blank identifier
result, _ := doSomething()

// Good: Handle or explicitly document why it's safe to ignore
result, err := doSomething()
if err != nil {
    return err
}

// Acceptable: When error truly doesn't matter (rare)
_ = writer.Close() // Best-effort cleanup, error logged elsewhere
```

## Concurrency Patterns

### sync.RWMutex with *Locked Convention

Use `sync.RWMutex` for read-heavy structs. Mark internal helpers that require the caller to already hold the lock with a `*Locked` suffix and document the invariant explicitly.

```go
type Aggregator struct {
    mu   sync.RWMutex
    data map[string]*Stat
}

// Public method: acquires lock
func (a *Aggregator) Add(name string, val int) {
    a.mu.Lock()
    defer a.mu.Unlock()
    a.addLocked(name, val)
}

// addLocked must only be called while a.mu is held.
func (a *Aggregator) addLocked(name string, val int) {
    if a.data[name] == nil {
        a.data[name] = &Stat{}
    }
    a.data[name].Count += val
}
```

### Copy-on-Read Snapshot Pattern

For sorted/computed results, acquire a read lock, take a deep copy of the data, release the lock, then perform expensive operations (sort, percentile calc) outside the lock to minimize contention.

```go
func (a *Aggregator) Sorted() []Stat {
    a.mu.RLock()
    snapshot := a.snapshotLocked() // deep copy under lock
    a.mu.RUnlock()

    // Sort and compute on the copy — no lock held
    sort.Slice(snapshot, func(i, j int) bool {
        return snapshot[i].Count > snapshot[j].Count
    })
    return snapshot
}

// snapshotLocked must only be called while a.mu.RLock is held.
func (a *Aggregator) snapshotLocked() []Stat {
    out := make([]Stat, 0, len(a.data))
    for _, v := range a.data {
        out = append(out, *v) // copy struct value
    }
    return out
}
```

### Worker Pool

```go
func WorkerPool(jobs <-chan Job, results chan<- Result, numWorkers int) {
    var wg sync.WaitGroup

    for i := 0; i < numWorkers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for job := range jobs {
                results <- process(job)
            }
        }()
    }

    wg.Wait()
    close(results)
}
```

### Context for Cancellation and Timeouts

```go
func FetchWithTimeout(ctx context.Context, url string) ([]byte, error) {
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return nil, fmt.Errorf("create request: %w", err)
    }

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("fetch %s: %w", url, err)
    }
    defer resp.Body.Close()

    return io.ReadAll(resp.Body)
}
```

### Graceful Shutdown

```go
func GracefulShutdown(server *http.Server) {
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

    <-quit
    log.Println("Shutting down server...")

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    if err := server.Shutdown(ctx); err != nil {
        log.Fatalf("Server forced to shutdown: %v", err)
    }

    log.Println("Server exited")
}
```

### errgroup for Coordinated Goroutines

```go
import "golang.org/x/sync/errgroup"

func FetchAll(ctx context.Context, urls []string) ([][]byte, error) {
    g, ctx := errgroup.WithContext(ctx)
    results := make([][]byte, len(urls))

    for i, url := range urls {
        i, url := i, url // Capture loop variables
        g.Go(func() error {
            data, err := FetchWithTimeout(ctx, url)
            if err != nil {
                return err
            }
            results[i] = data
            return nil
        })
    }

    if err := g.Wait(); err != nil {
        return nil, err
    }
    return results, nil
}
```

### Avoiding Goroutine Leaks

```go
// Bad: Goroutine leak if context is cancelled
func leakyFetch(ctx context.Context, url string) <-chan []byte {
    ch := make(chan []byte)
    go func() {
        data, _ := fetch(url)
        ch <- data // Blocks forever if no receiver
    }()
    return ch
}

// Good: Properly handles cancellation
func safeFetch(ctx context.Context, url string) <-chan []byte {
    ch := make(chan []byte, 1) // Buffered channel
    go func() {
        data, err := fetch(url)
        if err != nil {
            return
        }
        select {
        case ch <- data:
        case <-ctx.Done():
        }
    }()
    return ch
}
```

## BubbleTea (TUI) Pattern

For terminal UIs using [charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea), follow the Elm architecture strictly: Model holds state, Update handles messages, View renders — no side effects in View.

```go
// model.go
type model struct {
    data      []Item
    cursor    int
    width     int
    height    int
    tickCount int
}

// Init starts the tick loop
func (m model) Init() tea.Cmd {
    return tea.Tick(200*time.Millisecond, func(t time.Time) tea.Msg {
        return tickMsg(t)
    })
}

// Update is the only place state mutates
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "q", "ctrl+c":
            return m, tea.Quit
        case "j", "down":
            if m.cursor < len(m.data)-1 {
                m.cursor++
            }
        case "k", "up":
            if m.cursor > 0 {
                m.cursor--
            }
        }
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
    case tickMsg:
        m.tickCount++
        return m, tea.Tick(200*time.Millisecond, func(t time.Time) tea.Msg {
            return tickMsg(t)
        })
    }
    return m, nil
}

// View is pure — only reads from m, never writes
func (m model) View() string {
    return render.RenderView(m) // delegate to pure render functions
}

type tickMsg time.Time
```

**Decouple renderers from the model using an interface:**

```go
// controller/interface.go
type UIController interface {
    Data()   []Item
    Cursor() int
    Width()  int
    Height() int
}

// render/view.go — depends only on the interface
func RenderView(ctrl UIController) string {
    // ...
}

// In tests, implement UIController with a simple struct
type fakeCtrl struct {
    data   []Item
    cursor int
}
func (f fakeCtrl) Data() []Item { return f.data }
func (f fakeCtrl) Cursor() int  { return f.cursor }
// ...
```

## Dependency Injection via Embedded Functions

When a full interface is too heavy, use a package-level `var` pointing to the real function. Tests override the var to inject stubs. Document clearly that the var exists for testability.

```go
// tracer.go
// selectTracer is a package-level var so tests can replace it.
var selectTracer = tracer.Select

func run(pid int) error {
    t, err := selectTracer(pid)
    // ...
}
```

```go
// tracer_test.go
func TestRunWithFakeTracer(t *testing.T) {
    original := selectTracer
    t.Cleanup(func() { selectTracer = original })

    selectTracer = func(pid int) (Tracer, error) {
        return &fakeTracer{}, nil
    }

    err := run(42)
    // assertions...
}
```

## Clock Injection for Deterministic Tests

Inject time via a `nowFunc` field instead of calling `time.Now()` directly. This avoids `time.Sleep` in tests.

```go
type Aggregator struct {
    nowFunc func() time.Time
    // ...
}

// New returns production instance using real clock
func New() *Aggregator {
    return &Aggregator{nowFunc: time.Now}
}

// newWithClock is unexported — only for tests
func newWithClock(fn func() time.Time) *Aggregator {
    return &Aggregator{nowFunc: fn}
}

func (a *Aggregator) ratePerSecond() float64 {
    now := a.nowFunc()
    elapsed := now.Sub(a.lastReset).Seconds()
    // ...
}
```

## Embedding Static Assets with //go:embed

Use `//go:embed` to bundle frontend assets directly into the binary, making it fully self-contained.

```go
package server

import _ "embed"

//go:embed static/dashboard.html
var dashboardHTML []byte

//go:embed static/dashboard.js
var dashboardJS []byte

// Serve embedded assets
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/html; charset=utf-8")
    _, _ = w.Write(dashboardHTML)
}
```

For a directory of assets:

```go
import "embed"

//go:embed static
var staticFiles embed.FS

func (s *Server) staticHandler() http.Handler {
    sub, _ := fs.Sub(staticFiles, "static")
    return http.FileServer(http.FS(sub))
}
```

## iota Enumerations with String() and JSON

Use `iota` for typed enumerations. Implement `String()` for human-readable output and custom `MarshalJSON`/`UnmarshalJSON` when the enum must serialize as a string in JSON responses.

```go
type Category int

const (
    CategoryIO Category = iota
    CategoryFS
    CategoryNet
    CategoryProc
)

func (c Category) String() string {
    switch c {
    case CategoryIO:
        return "I/O"
    case CategoryFS:
        return "FS"
    case CategoryNet:
        return "NET"
    case CategoryProc:
        return "PROC"
    default:
        return "UNKNOWN"
    }
}

func (c Category) MarshalJSON() ([]byte, error) {
    return json.Marshal(c.String())
}

func (c *Category) UnmarshalJSON(data []byte) error {
    var s string
    if err := json.Unmarshal(data, &s); err != nil {
        return err
    }
    switch s {
    case "I/O":
        *c = CategoryIO
    case "FS":
        *c = CategoryFS
    // ...
    default:
        return fmt.Errorf("unknown category: %s", s)
    }
    return nil
}
```

## HTTP Server Patterns

### Self-documenting Route Registration

Keep a registry of all routes for automatic API discovery:

```go
type routeInfo struct {
    Method string
    Path   string
    Desc   string
}

type Server struct {
    mux    *http.ServeMux
    routes []routeInfo
}

func (s *Server) handle(method, path, desc string, h http.HandlerFunc) {
    s.routes = append(s.routes, routeInfo{method, path, desc})
    s.mux.HandleFunc(path, h)
}

// GET /api returns all registered routes
func (s *Server) handleAPIIndex(w http.ResponseWriter, r *http.Request) {
    writeJSON(w, s.routes)
}
```

### Single JSON Helper

```go
func writeJSON(w http.ResponseWriter, v any) {
    w.Header().Set("Content-Type", "application/json")
    data, err := json.MarshalIndent(v, "", "  ")
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    _, _ = w.Write(data) // write errors to ResponseWriter are intentionally discarded
}
```

### WebSocket with Write Deadline

```go
func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        return
    }
    defer conn.Close()

    ticker := time.NewTicker(time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
            if err := conn.WriteJSON(s.agg.Sorted()); err != nil {
                return
            }
        case <-r.Context().Done():
            return
        }
    }
}
```

## Interface Design

### Small, Focused Interfaces

```go
// Good: Single-method interfaces
type Reader interface {
    Read(p []byte) (n int, err error)
}

type Writer interface {
    Write(p []byte) (n int, err error)
}

type Closer interface {
    Close() error
}

// Compose interfaces as needed
type ReadWriteCloser interface {
    Reader
    Writer
    Closer
}
```

### Define Interfaces Where They're Used

```go
// In the consumer package, not the provider
package service

// UserStore defines what this service needs
type UserStore interface {
    GetUser(id string) (*User, error)
    SaveUser(user *User) error
}

type Service struct {
    store UserStore
}

// Concrete implementation can be in another package
// It doesn't need to know about this interface
```

## Package Organization

### Standard Project Layout

```text
myproject/
├── cmd/
│   └── myapp/
│       └── main.go           # Entry point
├── internal/
│   ├── aggregator/           # Core data model + thread-safe accumulator
│   ├── server/               # HTTP + WebSocket + Prometheus
│   │   └── static/           # Embedded frontend assets
│   ├── ui/                   # TUI (BubbleTea)
│   │   ├── render/           # Pure render functions
│   │   ├── controller/       # UIController interface
│   │   ├── styles/           # lipgloss constants
│   │   └── widgets/          # Reusable TUI components
│   ├── tracer/               # Backend abstraction (strace / eBPF)
│   └── models/               # Shared types
├── go.mod
├── go.sum
└── Makefile
```

### Package Naming

```go
// Good: Short, lowercase, no underscores
package aggregator
package render
package server

// Bad: Verbose, mixed case, or redundant
package httpHandler
package renderUtils
package serverService
```

### Avoid Package-Level State

```go
// Bad: Global mutable state
var db *sql.DB

func init() {
    db, _ = sql.Open("postgres", os.Getenv("DATABASE_URL"))
}

// Good: Dependency injection
type Server struct {
    db *sql.DB
}

func NewServer(db *sql.DB) *Server {
    return &Server{db: db}
}
```

## Struct Design

### Functional Options Pattern

```go
type Server struct {
    addr    string
    timeout time.Duration
    logger  *log.Logger
}

type Option func(*Server)

func WithTimeout(d time.Duration) Option {
    return func(s *Server) {
        s.timeout = d
    }
}

func WithLogger(l *log.Logger) Option {
    return func(s *Server) {
        s.logger = l
    }
}

func NewServer(addr string, opts ...Option) *Server {
    s := &Server{
        addr:    addr,
        timeout: 30 * time.Second, // default
        logger:  log.Default(),    // default
    }
    for _, opt := range opts {
        opt(s)
    }
    return s
}

// Usage
server := NewServer(":8080",
    WithTimeout(60*time.Second),
    WithLogger(customLogger),
)
```

### Embedding for Composition

```go
type Logger struct {
    prefix string
}

func (l *Logger) Log(msg string) {
    fmt.Printf("[%s] %s\n", l.prefix, msg)
}

type Server struct {
    *Logger // Embedding - Server gets Log method
    addr    string
}

func NewServer(addr string) *Server {
    return &Server{
        Logger: &Logger{prefix: "SERVER"},
        addr:   addr,
    }
}
```

## Memory and Performance

### Preallocate Slices When Size is Known

```go
// Bad: Grows slice multiple times
func processItems(items []Item) []Result {
    var results []Result
    for _, item := range items {
        results = append(results, process(item))
    }
    return results
}

// Good: Single allocation
func processItems(items []Item) []Result {
    results := make([]Result, 0, len(items))
    for _, item := range items {
        results = append(results, process(item))
    }
    return results
}
```

### Use sync.Pool for Frequent Allocations

```go
var bufferPool = sync.Pool{
    New: func() interface{} {
        return new(bytes.Buffer)
    },
}

func ProcessRequest(data []byte) []byte {
    buf := bufferPool.Get().(*bytes.Buffer)
    defer func() {
        buf.Reset()
        bufferPool.Put(buf)
    }()

    buf.Write(data)
    return buf.Bytes()
}
```

## Go Tooling Integration

### Essential Commands

```bash
# Build and run
go build ./...
go run ./cmd/myapp

# Testing
go test ./...
go test -race ./...
go test -cover ./...

# Static analysis
go vet ./...
staticcheck ./...
golangci-lint run

# Module management
go mod tidy
go mod verify

# Formatting
gofmt -w .
goimports -w .
```

## Quick Reference: Go Idioms

| Idiom | Description |
|-------|-------------|
| Accept interfaces, return structs | Functions accept interface params, return concrete types |
| Errors are values | Treat errors as first-class values, not exceptions |
| Don't communicate by sharing memory | Use channels for coordination between goroutines |
| Make the zero value useful | Types should work without explicit initialization |
| `*Locked` suffix | Methods that require the mutex to already be held by the caller |
| `nowFunc` injection | Replace `time.Now` with a field for deterministic tests |
| `var` swap | Package-level var pointing to real function; tests replace it |
| `//go:embed` | Bundle static assets into the binary |
| Clear is better than clever | Prioritize readability over cleverness |
| gofmt is no one's favorite but everyone's friend | Always format with gofmt/goimports |

## Anti-Patterns to Avoid

```go
// Bad: Naked returns in long functions
func process() (result int, err error) {
    // ... 50 lines ...
    return // What is being returned?
}

// Bad: Using panic for control flow
func GetUser(id string) *User {
    user, err := db.Find(id)
    if err != nil {
        panic(err) // Don't do this
    }
    return user
}

// Bad: Passing context in struct
type Request struct {
    ctx context.Context // Context should be first param
    ID  string
}

// Good: Context as first parameter
func ProcessRequest(ctx context.Context, id string) error {
    // ...
}

// Bad: Mixing value and pointer receivers on the same type
type Counter struct{ n int }
func (c Counter) Value() int  { return c.n } // Value receiver
func (c *Counter) Inc()       { c.n++ }      // Pointer receiver — pick one style
```

**Remember**: Go code should be boring in the best way — predictable, consistent, and easy to understand. When in doubt, keep it simple.
