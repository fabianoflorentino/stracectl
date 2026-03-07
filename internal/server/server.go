// Package server exposes aggregator data over HTTP for the sidecar mode.
//
// Endpoints: / (dashboard), /healthz (healthcheck), /api/stats (JSON),
// /api/categories (JSON), /stream (WebSocket), /metrics (Prometheus).
package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/fabianoflorentino/stracectl/internal/aggregator"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Server wraps an HTTP server and exposes aggregator data.
type Server struct {
	agg      *aggregator.Aggregator
	mux      *http.ServeMux
	httpSrv  *http.Server
	registry *prometheus.Registry
}

// New creates a Server listening on addr (e.g. ":8080").
func New(addr string, agg *aggregator.Aggregator) *Server {
	reg := prometheus.NewRegistry()
	s := &Server{agg: agg, registry: reg, mux: http.NewServeMux()}

	s.registerMetrics(reg)

	s.mux.HandleFunc("/", s.handleDashboard)
	s.mux.HandleFunc("/healthz", s.handleHealthz)
	s.mux.HandleFunc("/api/stats", s.handleStats)
	s.mux.HandleFunc("/api/categories", s.handleCategories)
	s.mux.HandleFunc("/stream", s.handleStream)
	s.mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

	s.httpSrv = &http.Server{
		Addr:              addr,
		Handler:           s.mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
	}
	return s
}

// ServeHTTP implements http.Handler so Server can be used with httptest.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// Start begins listening. It blocks until ctx is cancelled.
func (s *Server) Start(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		if err := s.httpSrv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.httpSrv.Shutdown(shutCtx)
	}
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(dashboardHTML))
}

func (s *Server) handleStats(w http.ResponseWriter, _ *http.Request) {
	stats := s.agg.Sorted(aggregator.SortByCount)
	writeJSON(w, stats)
}

func (s *Server) handleCategories(w http.ResponseWriter, _ *http.Request) {
	bd := s.agg.CategoryBreakdown()
	out := make(map[string]aggregator.CategoryStats, len(bd))
	for cat, cs := range bd {
		out[cat.String()] = cs
	}
	writeJSON(w, out)
}

// handleStream upgrades to WebSocket and pushes a JSON stats snapshot every second.
func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer func() { _ = conn.Close() }()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			stats := s.agg.Sorted(aggregator.SortByCount)
			if err := conn.WriteJSON(stats); err != nil {
				return
			}
		}
	}
}

// ── Prometheus metrics ─────────────────────────────────────────────────────────

type promCollector struct {
	agg        *aggregator.Aggregator
	descCount  *prometheus.Desc
	descErrors *prometheus.Desc
	descTotal  *prometheus.Desc
	descAvg    *prometheus.Desc
	descMax    *prometheus.Desc
	descRate   *prometheus.Desc
}

func (s *Server) registerMetrics(reg *prometheus.Registry) {
	c := &promCollector{
		agg: s.agg,
		descCount: prometheus.NewDesc(
			"stracectl_syscall_calls_total",
			"Total number of syscall invocations.",
			[]string{"syscall", "category"}, nil,
		),
		descErrors: prometheus.NewDesc(
			"stracectl_syscall_errors_total",
			"Total number of failed syscall invocations.",
			[]string{"syscall", "category"}, nil,
		),
		descTotal: prometheus.NewDesc(
			"stracectl_syscall_duration_seconds_total",
			"Total time spent in kernel for this syscall.",
			[]string{"syscall", "category"}, nil,
		),
		descAvg: prometheus.NewDesc(
			"stracectl_syscall_duration_avg_seconds",
			"Average time spent in kernel per call.",
			[]string{"syscall", "category"}, nil,
		),
		descMax: prometheus.NewDesc(
			"stracectl_syscall_duration_max_seconds",
			"Maximum observed kernel time per call.",
			[]string{"syscall", "category"}, nil,
		),
		descRate: prometheus.NewDesc(
			"stracectl_syscalls_per_second",
			"Recent syscall rate (syscalls/s).",
			nil, nil,
		),
	}
	reg.MustRegister(c)
}

func (c *promCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.descCount
	ch <- c.descErrors
	ch <- c.descTotal
	ch <- c.descAvg
	ch <- c.descMax
	ch <- c.descRate
}

func (c *promCollector) Collect(ch chan<- prometheus.Metric) {
	stats := c.agg.Sorted(aggregator.SortByCount)
	for i := range stats {
		s := &stats[i]
		cat := s.Category.String()
		ch <- prometheus.MustNewConstMetric(c.descCount, prometheus.CounterValue,
			float64(s.Count), s.Name, cat)
		ch <- prometheus.MustNewConstMetric(c.descErrors, prometheus.CounterValue,
			float64(s.Errors), s.Name, cat)
		ch <- prometheus.MustNewConstMetric(c.descTotal, prometheus.CounterValue,
			s.TotalTime.Seconds(), s.Name, cat)
		ch <- prometheus.MustNewConstMetric(c.descAvg, prometheus.GaugeValue,
			s.AvgTime().Seconds(), s.Name, cat)
		ch <- prometheus.MustNewConstMetric(c.descMax, prometheus.GaugeValue,
			s.MaxTime.Seconds(), s.Name, cat)
	}
	ch <- prometheus.MustNewConstMetric(c.descRate, prometheus.GaugeValue,
		c.agg.Rate())
}

// ── helpers ───────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, v any) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		http.Error(w, "encoding error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data)
}

// dashboardHTML is the single-page live dashboard served at /.
const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>stracectl</title>
<style>
  *{box-sizing:border-box;margin:0;padding:0}
  body{font-family:'Segoe UI',system-ui,sans-serif;background:#0d1117;color:#e6edf3;min-height:100vh}
  header{background:#161b22;border-bottom:1px solid #30363d;padding:12px 24px;display:flex;align-items:center;gap:16px}
  header h1{font-size:1.1rem;font-weight:600;color:#58a6ff}
  #meta{margin-left:auto;font-size:.8rem;color:#8b949e;display:flex;gap:20px}
  #meta span b{color:#e6edf3}
  .bar-wrap{background:#161b22;border-bottom:1px solid #30363d;padding:10px 24px;display:flex;gap:20px;flex-wrap:wrap}
  .cat-pill{font-size:.75rem;padding:2px 8px;border-radius:10px;background:#21262d;font-weight:500}
  .cat-IO{color:#79c0ff}.cat-FS{color:#56d364}.cat-NET{color:#f0883e}.cat-MEM{color:#d2a8ff}
  .cat-PROC{color:#ff7b72}.cat-SIG{color:#8b949e}.cat-OTHER{color:#6e7681}
  .wrap{padding:20px 24px;overflow-x:auto}
  table{width:100%;border-collapse:collapse;font-size:.82rem}
  thead th{text-align:left;padding:6px 10px;color:#8b949e;font-weight:600;border-bottom:1px solid #30363d;white-space:nowrap;cursor:pointer;user-select:none}
  thead th:hover{color:#e6edf3}
  thead th.asc::after{content:" ▲"}
  thead th.desc::after{content:" ▼"}
  tbody tr{border-bottom:1px solid #161b22;transition:background .1s}
  tbody tr:hover{background:#161b22}
  td{padding:5px 10px;white-space:nowrap}
  td.name{font-family:monospace;color:#79c0ff}
  td.num{text-align:right;font-variant-numeric:tabular-nums}
  td.err{text-align:right;color:#f85149}
  td.slow{color:#e3b341}
  .spark{display:inline-block;width:80px;height:8px;background:#21262d;border-radius:2px;vertical-align:middle}
  .spark-fill{height:100%;background:#1f6feb;border-radius:2px}
  #status{padding:8px 24px;font-size:.75rem;color:#8b949e;border-top:1px solid #30363d;position:fixed;bottom:0;width:100%;background:#0d1117}
  #status.err{color:#f85149}
  .tag{font-size:.7rem;padding:1px 6px;border-radius:8px;margin-left:4px}
</style>
</head>
<body>
<header>
  <h1>stracectl</h1>
  <div id="meta">
    <span>syscalls: <b id="m-total">—</b></span>
    <span>rate: <b id="m-rate">—</b>/s</span>
    <span>errors: <b id="m-errors">—</b></span>
    <span>unique: <b id="m-unique">—</b></span>
  </div>
</header>
<div class="bar-wrap" id="cat-bar"></div>
<div class="wrap">
<table>
  <thead>
    <tr>
      <th data-col="Name">SYSCALL</th>
      <th data-col="Category">CAT</th>
      <th data-col="Count" class="desc">CALLS</th>
      <th data-col="_bar"></th>
      <th data-col="AvgTime">AVG</th>
      <th data-col="MaxTime">MAX</th>
      <th data-col="TotalTime">TOTAL</th>
      <th data-col="Errors">ERRORS</th>
      <th data-col="_erp">ERR%</th>
    </tr>
  </thead>
  <tbody id="tbody"></tbody>
</table>
</div>
<div id="status">Connecting…</div>

<script>
const fmtDur = ns => {
  if (!ns) return '—';
  if (ns < 1e3) return ns + 'ns';
  if (ns < 1e6) return (ns/1e3).toFixed(1) + 'µs';
  if (ns < 1e9) return (ns/1e6).toFixed(1) + 'ms';
  return (ns/1e9).toFixed(2) + 's';
};
const fmtN = n => {
  if (n >= 1e6) return (n/1e6).toFixed(1) + 'M';
  if (n >= 1e3) return (n/1e3).toFixed(1) + 'k';
  return '' + n;
};
const catClass = c => 'cat-' + c.replace('/','');
const CAT_ORDER = ['I/O','FS','NET','MEM','PROC','SIG','OTHER'];

let sortCol = 'Count', sortDir = -1;
let lastData = [];

function sortData(rows) {
  return [...rows].sort((a,b) => {
    let av = a[sortCol], bv = b[sortCol];
    if (sortCol === '_erp') { av = a.Count ? a.Errors/a.Count : 0; bv = b.Count ? b.Errors/b.Count : 0; }
    if (sortCol === 'AvgTime') { av = a.Count ? a.TotalTime/a.Count : 0; bv = b.Count ? b.TotalTime/b.Count : 0; }
    if (sortCol === '_bar') { av = a.Count; bv = b.Count; }
    if (sortCol === 'Category') { av = CAT_ORDER.indexOf(a.Category); bv = CAT_ORDER.indexOf(b.Category); }
    if (typeof av === 'string') return sortDir * av.localeCompare(bv);
    return sortDir * ((av||0) - (bv||0));
  });
}

function render(rows) {
  const maxCount = rows.reduce((m,r) => Math.max(m, r.Count), 0);
  const tbody = document.getElementById('tbody');
  const sorted = sortData(rows);
  tbody.innerHTML = sorted.map(r => {
    const errPct = r.Count ? (r.Errors / r.Count * 100) : 0;
    const avgNs  = r.Count ? Math.round(r.TotalTime / r.Count) : 0;
    const pct    = maxCount ? Math.round(r.Count / maxCount * 100) : 0;
    const slow   = avgNs >= 5e6;
    return '<tr>' +
      '<td class="name">' + r.Name + '</td>' +
      '<td><span class="cat-pill ' + catClass(r.Category) + '">' + r.Category + '</span></td>' +
      '<td class="num">' + fmtN(r.Count) + '</td>' +
      '<td><div class="spark"><div class="spark-fill" style="width:' + pct + '%"></div></div></td>' +
      '<td class="num' + (slow?' slow':'') + '">' + fmtDur(avgNs) + '</td>' +
      '<td class="num">' + fmtDur(r.MaxTime) + '</td>' +
      '<td class="num">' + fmtDur(r.TotalTime) + '</td>' +
      '<td class="num err">' + (r.Errors || '—') + '</td>' +
      '<td class="num err">' + (r.Errors ? errPct.toFixed(0)+'%' : '—') + '</td>' +
      '</tr>';
  }).join('');
}

function updateMeta(rows) {
  const total  = rows.reduce((s,r) => s + r.Count, 0);
  const errors = rows.reduce((s,r) => s + r.Errors, 0);
  document.getElementById('m-total').textContent  = fmtN(total);
  document.getElementById('m-errors').textContent = fmtN(errors);
  document.getElementById('m-unique').textContent = rows.length;
}

function updateCatBar(rows) {
  const total = rows.reduce((s,r) => s + r.Count, 0);
  const cats  = {};
  rows.forEach(r => { cats[r.Category] = (cats[r.Category]||0) + r.Count; });
  document.getElementById('cat-bar').innerHTML = CAT_ORDER
    .filter(c => cats[c])
    .map(c => {
      const pct = total ? (cats[c]/total*100).toFixed(0) : 0;
      return '<span class="cat-pill ' + catClass(c) + '">' + c + ' ' + pct + '%</span>';
    }).join('');
}

// sort header clicks
document.querySelector('thead').addEventListener('click', e => {
  const th = e.target.closest('th');
  if (!th || !th.dataset.col) return;
  if (sortCol === th.dataset.col) { sortDir *= -1; }
  else { sortCol = th.dataset.col; sortDir = -1; }
  document.querySelectorAll('thead th').forEach(t => t.classList.remove('asc','desc'));
  th.classList.add(sortDir === -1 ? 'desc' : 'asc');
  render(lastData);
});

// rate from successive snapshots
let prevTotal = 0, prevTs = Date.now();

function connect() {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws';
  const ws = new WebSocket(proto + '://' + location.host + '/stream');

  ws.onopen = () => {
    document.getElementById('status').textContent = 'Connected — live updates every second';
    document.getElementById('status').classList.remove('err');
  };

  ws.onmessage = e => {
    const rows = JSON.parse(e.data) || [];
    lastData = rows;
    const now = Date.now();
    const total = rows.reduce((s,r) => s + r.Count, 0);
    const rate  = prevTs !== now ? Math.round((total - prevTotal) / ((now - prevTs) / 1000)) : 0;
    prevTotal = total; prevTs = now;
    document.getElementById('m-rate').textContent = fmtN(Math.max(0, rate));
    updateMeta(rows);
    updateCatBar(rows);
    render(rows);
  };

  ws.onerror = () => {
    document.getElementById('status').textContent = 'WebSocket error — retrying…';
    document.getElementById('status').classList.add('err');
  };

  ws.onclose = () => {
    document.getElementById('status').textContent = 'Disconnected — reconnecting in 2 s…';
    document.getElementById('status').classList.add('err');
    setTimeout(connect, 2000);
  };
}

connect();
</script>
</body>
</html>`

