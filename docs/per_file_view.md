# Per-file view — implementation specification

This document describes a concrete implementation plan for the *Per-file view*
feature: identify and surface the file paths most frequently opened by the
traced process. It lists the files to change, includes example code snippets to
implement the feature, test ideas, and the observable results this feature
will bring to users (TUI and sidecar API).

**Summary**

- Purpose: show which file paths are opened most often (hot files), both in the
  TUI (toggle `f`) and via the HTTP sidecar (`GET /api/files`).
- Scope: implement cheap, reliable path extraction from `open` / `openat` calls
  (no expensive ptrace string dereferencing). Cap cardinality and truncate
  large values to remain safe for high-volume workloads.

**Files to change**

- `internal/aggregator/aggregator.go` — add `FileStats` storage, record paths
  inside `Add()` and expose a `TopFiles()` accessor.
- `internal/server/server.go` — register endpoint `/api/files` and return top
  file stats as JSON.
- `internal/ui/tui.go` — add a simple overlay toggled with `f` that lists the
  top files and counts.
- `internal/aggregator/aggregator_test.go` — unit tests to validate counting and
  limits.
- `internal/parser/parser.go` (optional) — if more robust parsing is desired,
  add a small helper there; otherwise implement extraction in the aggregator.

Design choices and constraints
-----------------------------

- Parse only the first path-like argument from `open` and `openat` syscall
  argument strings (the same format produced by the `strace` backend parser).
- Avoid dereferencing user-space pointers via `ptrace` (too expensive). Keep
  this as a cheap heuristic working with `strace` and eBPF outputs.
- Cap the number of tracked distinct file paths to a safe limit (default
  `fileStatsCap = 10000`) to avoid memory blowups on pathological workloads.
- Truncate path strings longer than `maxPathLen = 1024`.

Aggregator changes (conceptual code)
-----------------------------------

Add fields to `Aggregator`:

```go
// inside internal/aggregator/aggregator.go
type Aggregator struct {
    mu       sync.RWMutex
    stats    map[string]*SyscallStat
    // ... existing fields ...
    fileStats map[string]int64 // counts per path
}

const (
    fileStatsCap = 10_000
    maxPathLen   = 1024
)
```

Initialize `fileStats` in `New()`:

```go
func New() *Aggregator {
    now := time.Now()
    return &Aggregator{
        stats:    make(map[string]*SyscallStat),
        fileStats: make(map[string]int64),
        started:  now,
        prevRate: rateSnapshot{total: 0, at: now},
    }
}
```

Record file paths inside `Add()` when syscall is `open` or `openat`:

```go
func (a *Aggregator) Add(e models.SyscallEvent) {
    a.mu.Lock()
    defer a.mu.Unlock()

    // existing aggregations ...

    // cheap extraction: only for open/openat
    switch e.Name {
    case "open", "openat":
        if p := extractPathFromArgs(e.Name, e.Args); p != "" {
            if len(p) > maxPathLen { p = p[:maxPathLen] }
            if a.fileStats == nil { a.fileStats = make(map[string]int64) }
            if len(a.fileStats) < fileStatsCap || a.fileStats[p] > 0 {
                a.fileStats[p]++
            }
        }
    }
}
```

Add a public accessor `TopFiles(n int)`:

```go
type FileStat struct {
    Path  string `json:"path"`
    Count int64  `json:"count"`
}

func (a *Aggregator) TopFiles(n int) []FileStat {
    a.mu.RLock()
    defer a.mu.RUnlock()

    out := make([]FileStat, 0, len(a.fileStats))
    for p, c := range a.fileStats {
        out = append(out, FileStat{Path: p, Count: c})
    }
    sort.Slice(out, func(i, j int) bool { return out[i].Count > out[j].Count })
    if n > 0 && len(out) > n { out = out[:n] }
    return out
}
```

Path extraction helper (robust heuristic)
----------------------------------------

Implement a small helper to get the first path-like argument from the
`SyscallEvent.Args` string (works for common `strace`-style formatting):

```go
func extractPathFromArgs(name, args string) string {
    // 1) look for a quoted string: "..."
    if i := strings.Index(args, "\""); i >= 0 {
        if j := strings.Index(args[i+1:], "\""); j >= 0 {
            s := args[i+1 : i+1+j]
            return unescapePath(s)
        }
    }

    // 2) fallback: split by commas and pick the likely argument
    parts := strings.SplitN(args, ",", 3)
    switch name {
    case "open":
        if len(parts) >= 1 { return strings.TrimSpace(parts[0]) }
    case "openat":
        if len(parts) >= 2 { return strings.TrimSpace(parts[1]) }
    }
    return ""
}

func unescapePath(s string) string {
    // Attempt strconv.Unquote to handle C-style escapes, fallback to raw.
    if unq, err := strconv.Unquote("\"" + s + "\""); err == nil {
        return unq
    }
    return s
}
```

Server API
----------

Register a route in `internal/server/server.go` during server initialization:

```go
s.registerRoute("/api/files", s.handleFiles, "Top opened files")
```

Handler implementation:

```go
func (s *Server) handleFiles(w http.ResponseWriter, r *http.Request) {
    limit := 50
    if l := r.URL.Query().Get("limit"); l != "" {
        if v, err := strconv.Atoi(l); err == nil && v > 0 { limit = v }
    }
    files := s.agg.TopFiles(limit)
    writeJSON(w, files)
}
```

### Dashboard integration (sidecar web UI)

- Files to edit: `internal/server/static/dashboard.html` and `internal/server/static/dashboard.js`.
- UI: add a "Files" tab/button in the dashboard header and a panel with id `panel-files` containing controls and a table. Minimal HTML markup to add inside the dashboard template:

```html
<div id="panel-files" class="panel" style="display:none">
  <div class="panel-controls">
    <label>Limit: <input id="files-limit" type="number" min="1" max="500" value="50"></label>
    <input id="files-filter" placeholder="filter paths..." />
    <button id="files-refresh">Refresh</button>
  </div>
  <table id="files-table" class="table">
    <thead><tr><th>Path</th><th>Count</th></tr></thead>
    <tbody></tbody>
  </table>
</div>
```

- JS: implement `fetchFiles(limit, filter)` that calls `/api/files?limit=N`, renders rows into `#files-table tbody`, and sets `title` attributes for full paths. Wire `#tab-files` click to show the panel and start a polling interval (e.g. 2s) while visible. Provide click-to-copy on path cells and tooltip for full path.

Example minimal JS (add to `dashboard.js`):

```js
async function fetchFiles(limit=50, filter='') {
  try {
    const res = await fetch(`/api/files?limit=${limit}`);
    if (!res.ok) return;
    const files = await res.json();
    const tbody = document.querySelector('#files-table tbody');
    tbody.innerHTML = '';
    for (const f of files) {
      if (filter && !f.path.includes(filter)) continue;
      const tr = document.createElement('tr');
      const tdPath = document.createElement('td');
      tdPath.textContent = f.path;
      tdPath.title = f.path;
      tdPath.addEventListener('click', () => navigator.clipboard.writeText(f.path));
      const tdCount = document.createElement('td');
      tdCount.textContent = f.count;
      tr.appendChild(tdPath);
      tr.appendChild(tdCount);
      tbody.appendChild(tr);
    }
  } catch (err) {
    console.error('failed to fetch files', err);
  }
}

// UI wiring (run on DOM ready)
document.getElementById('tab-files').addEventListener('click', () => {
  showPanel('files'); // implement showPanel to hide other panels
  fetchFiles(Number(document.getElementById('files-limit').value), document.getElementById('files-filter').value);
});
document.getElementById('files-refresh').addEventListener('click', () => {
  fetchFiles(Number(document.getElementById('files-limit').value), document.getElementById('files-filter').value);
});
```

- Consider WebSocket alternative: if you already push stats on `/stream`, extend the snapshot to include top files to avoid polling.

- UX details: truncate visible path column with CSS (e.g. max-width + ellipsis), show full path in tooltip, allow click-to-copy, and add accessibility attributes.


TUI overlay
-----------

Add a minimal overlay in the TUI toggled by `f`. The overlay renders the
top N files with counts and supports simple scrolling. Implementation touches
in `internal/ui/tui.go`:

- Add `filesOverlay bool` and `filesOffset int` to the model struct.
- Handle `case "f": toggle overlay`. When rendering, call `agg.TopFiles(100)`
  and display `Path` and `Count` in two columns.

Example render snippet (conceptual):

```go
func (m model) renderFiles() string {
    files := m.agg.TopFiles(100)
    var b strings.Builder
    b.WriteString(detailTitleStyle.Render(" top files ") + "\n")
    for _, f := range files {
        line := fmt.Sprintf("%-80s %6s", f.Path, formatCount(f.Count))
        b.WriteString(detailDimStyle.Render(line) + "\n")
    }
    return b.String()
}
```

Testing
-------

- Unit tests for `extractPathFromArgs()` using representative `strace`-style
  argument strings (quoted paths, escaped characters, relative paths, missing
  args).
- Aggregator unit test: create an `Aggregator`, call `Add()` with several
  `models.SyscallEvent` objects (open/openat) and assert `TopFiles()` returns
  the expected counts and order.
- Integration test (optional): produce a small `strace -T -o trace.log` from a
  short workload and run `stracectl stats trace.log` to ensure API/TUI output
  contains expected top files.

Observability & limits
----------------------

- Cardinality cap: `fileStatsCap` prevents unbounded memory growth. For
  workloads that legitimately open many distinct files (e.g. directory
  crawlers), consider sampling or a configurable cap.
- Path truncation: `maxPathLen` protects against pathological long strings.
- Security: the aggregator stores only strings extracted from the traced
  process; do not log or display secrets. Consider redacting paths that look
  like tokens (optional enhancement).

User-visible results
--------------------

- TUI: pressing `f` opens a Top Files overlay showing the most opened file
  paths and counts — useful to debug excessive file I/O, repeated ENOENTs or
  to find configuration or credential files being probed.
- Sidecar API: `GET /api/files?limit=20` returns a JSON array of `{path,count}`
  allowing automated tooling to detect hotspots and create alerts.
- Reports: the existing `--report` HTML export can be extended to include the
  top files table (low-effort follow-up).

Example API response
--------------------

```json
[
  { "path": "/etc/ld.so.cache", "count": 124 },
  { "path": "/etc/hosts", "count": 42 },
  { "path": "/home/user/.cache/foo", "count": 11 }
]
```

Try-it commands
---------------

Run and serve a command, then query the API:

```bash
sudo ./stracectl run --serve :8080 curl -s https://example.com
curl -s http://localhost:8080/api/files?limit=20 | jq .
```

Estimated effort
----------------

- Aggregator + API: ~3–6 hours.
- TUI overlay + tests + polish: additional 0.5–1 day.

Next steps
----------

1. Implement the `fileStats` field and `TopFiles()` accessor in
   `internal/aggregator/aggregator.go`.
2. Add the `/api/files` handler and route registration in
   `internal/server/server.go`.
3. Add the TUI overlay in `internal/ui/tui.go` and unit tests under
   `internal/aggregator`.

If you want I can implement steps 1–2 now and open a PR with tests. Reply
"implement" to proceed.
