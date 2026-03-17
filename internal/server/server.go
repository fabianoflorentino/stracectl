// Package server exposes aggregator data over HTTP for the sidecar mode.
//
// Endpoints: / (dashboard), /healthz (healthcheck), /api/stats (JSON),
// /api/categories (JSON), /stream (WebSocket), /metrics (Prometheus).
package server

import (
	_ "embed"

	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/fabianoflorentino/stracectl/internal/aggregator"
	"github.com/fabianoflorentino/stracectl/internal/procinfo"
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
	wsToken  string
	// routes keeps a list of registered HTTP paths for discovery.
	routes []routeInfo
}

type routeInfo struct {
	Path        string `json:"path"`
	Method      string `json:"method"`
	Description string `json:"description,omitempty"`
}

// New creates a Server listening on addr (e.g. ":8080").
func New(addr string, agg *aggregator.Aggregator, wsToken string) *Server {
	reg := prometheus.NewRegistry()
	s := &Server{agg: agg, registry: reg, mux: http.NewServeMux(), wsToken: wsToken, routes: []routeInfo{}}

	s.registerMetrics(reg)
	s.registerRoute("/", "GET", s.handleDashboard, "Web dashboard")
	s.registerRoute("/static/dashboard.js", "GET", s.handleDashboardJS, "Dashboard JavaScript")
	s.registerRoute("/healthz", "GET", s.handleHealthz, "Health check")
	s.registerRoute("/api", "GET", s.handleAPI, "List available API endpoints")
	s.registerRoute("/api/", "GET", s.handleAPI, "List available API endpoints (index)")
	s.registerRoute("/api/status", "GET", s.handleStatus, "Current trace/status information")
	s.registerRoute("/api/stats", "GET", s.handleStats, "Aggregated syscall statistics")
	s.registerRoute("/api/log", "GET", s.handleLog, "Recent events log")
	s.registerRoute("/api/categories", "GET", s.handleCategories, "Category breakdown of syscalls")
	s.registerRoute("/api/syscall/{name}", "GET", s.handleSyscallStat, "Stats for a single syscall (by name)")
	s.registerRoute("/syscall/{name}", "GET", s.handleSyscallDetail, "Per-syscall detail page (SPA)")
	s.registerRoute("/stream", "GET", s.handleStream, "WebSocket stream of live stats")
	s.registerHandler("/metrics", "GET", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}), "Prometheus metrics endpoint")

	// Debug: print registered routes on startup to help troubleshooting.
	for _, rt := range s.routes {
		fmt.Printf("registered route: %s %s\n", rt.Method, rt.Path)
	}

	s.httpSrv = &http.Server{
		Addr:              addr,
		Handler:           s.mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
	}
	return s
}

// registerRoute registers a handler and records the path for discovery.
func (s *Server) registerRoute(path, method string, handler func(http.ResponseWriter, *http.Request), desc string) {
	s.mux.HandleFunc(path, handler)
	s.routes = append(s.routes, routeInfo{Path: path, Method: method, Description: desc})
}

// registerHandler registers an http.Handler and records the path for discovery.
func (s *Server) registerHandler(path, method string, h http.Handler, desc string) {
	s.mux.Handle(path, h)
	s.routes = append(s.routes, routeInfo{Path: path, Method: method, Description: desc})
}

// handleAPI returns a JSON list of registered API endpoints with pagination.
func (s *Server) handleAPI(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api" && r.URL.Path != "/api/" {
		http.NotFound(w, r)
		return
	}
	// parse pagination params
	page := 1
	per := 20
	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	if pp := r.URL.Query().Get("per_page"); pp != "" {
		if v, err := strconv.Atoi(pp); err == nil && v > 0 {
			per = v
		}
	}

	total := len(s.routes)
	start := (page - 1) * per
	if start > total {
		start = total
	}
	end := start + per
	if end > total {
		end = total
	}
	var items []routeInfo
	if start < end {
		items = s.routes[start:end]
	} else {
		items = []routeInfo{}
	}

	resp := struct {
		Total   int         `json:"total"`
		Page    int         `json:"page"`
		PerPage int         `json:"per_page"`
		Items   []routeInfo `json:"items"`
	}{
		Total:   total,
		Page:    page,
		PerPage: per,
		Items:   items,
	}
	writeJSON(w, resp)
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
	_, _ = w.Write(dashboardHTML)
}

func (s *Server) handleDashboardJS(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/static/dashboard.js" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	_, _ = w.Write(dashboardJS)
}

func (s *Server) handleStats(w http.ResponseWriter, _ *http.Request) {
	stats := s.agg.Sorted(aggregator.SortByCount)
	writeJSON(w, stats)
}

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	type statusResp struct {
		Proc    procinfo.ProcInfo `json:"Proc"`
		Total   int64             `json:"Total"`
		Errors  int64             `json:"Errors"`
		Rate    float64           `json:"Rate"`
		Unique  int               `json:"Unique"`
		Elapsed string            `json:"Elapsed"`
		Done    bool              `json:"Done"`
	}
	resp := statusResp{
		Proc:    s.agg.GetProcInfo(),
		Total:   s.agg.Total(),
		Errors:  s.agg.Errors(),
		Rate:    s.agg.Rate(),
		Unique:  s.agg.UniqueCount(),
		Elapsed: time.Since(s.agg.StartTime()).Round(time.Second).String(),
		Done:    s.agg.IsDone(),
	}
	writeJSON(w, resp)
}

func (s *Server) handleLog(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, s.agg.RecentLog())
}

func (s *Server) handleCategories(w http.ResponseWriter, _ *http.Request) {
	bd := s.agg.CategoryBreakdown()
	out := make(map[string]aggregator.CategoryStats, len(bd))
	for cat, cs := range bd {
		out[cat.String()] = cs
	}
	writeJSON(w, out)
}

// handleSyscallStat returns JSON stats for a single syscall by name.
func (s *Server) handleSyscallStat(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	stat, ok := s.agg.Get(name)
	if !ok {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, stat)
}

// handleSyscallDetail serves the per-syscall detail SPA.
func (s *Server) handleSyscallDetail(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(syscallDetailHTML)
}

// handleStream upgrades to WebSocket and pushes a JSON stats snapshot every second.
func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	// Token validation if wsToken is set
	if s.wsToken != "" {
		token := ""
		// Check Authorization header
		authHeader := r.Header.Get("Authorization")
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			token = authHeader[7:]
		} else {
			// Fallback to query param
			token = r.URL.Query().Get("token")
		}
		// Secure compare
		if len(token) != len(s.wsToken) ||
			subtle.ConstantTimeCompare([]byte(token), []byte(s.wsToken)) != 1 {
			w.WriteHeader(http.StatusUnauthorized)
			w.Header().Set("Content-Type", "text/plain")

			if _, err := w.Write([]byte("unauthorized: invalid or missing token")); err != nil {
				return
			}
		}
	}

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
			// After sending the snapshot, notify the client if the process exited.
			if s.agg.IsDone() {
				type doneMsg struct {
					Done bool `json:"done"`
				}
				_ = conn.WriteJSON(doneMsg{Done: true})
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

//go:embed static/dashboard.html
var dashboardHTML []byte

//go:embed static/dashboard.js
var dashboardJS []byte

//go:embed static/syscall_detail.html
var syscallDetailHTML []byte
