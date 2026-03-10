package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/fabianoflorentino/stracectl/internal/aggregator"
	"github.com/fabianoflorentino/stracectl/internal/models"
	"github.com/fabianoflorentino/stracectl/internal/server"
)

func TestStream_WebSocket_WithValidToken(t *testing.T) {
	agg := newPopulatedAgg()
	token := "supersecret"
	srv := server.New(":0", agg, token)

	ts := httptest.NewServer(srv)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/stream?token=" + token
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect WebSocket with valid token: %v", err)
	}
	if resp != nil {
		defer resp.Body.Close()
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	var stats []aggregator.SyscallStat
	if err := conn.ReadJSON(&stats); err != nil {
		t.Fatalf("failed to read WebSocket message: %v", err)
	}
	if len(stats) == 0 {
		t.Fatal("expected at least one stat from WebSocket")
	}

	// Test via Authorization header
	header := http.Header{}
	header.Set("Authorization", "Bearer "+token)
	conn2, resp2, err2 := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(ts.URL, "http")+"/stream", header)
	if err2 != nil {
		t.Fatalf("failed to connect WebSocket with valid Authorization header: %v", err2)
	}
	if resp2 != nil {
		defer resp2.Body.Close()
	}
	defer conn2.Close()
	conn2.SetReadDeadline(time.Now().Add(3 * time.Second))
	var stats2 []aggregator.SyscallStat
	if err := conn2.ReadJSON(&stats2); err != nil {
		t.Fatalf("failed to read WebSocket message: %v", err)
	}
	if len(stats2) == 0 {
		t.Fatal("expected at least one stat from WebSocket (header)")
	}
}

func TestStream_WebSocket_WithInvalidToken(t *testing.T) {
	agg := newPopulatedAgg()
	token := "supersecret"
	srv := server.New(":0", agg, token)

	ts := httptest.NewServer(srv)
	defer ts.Close()

	// Wrong token via query
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/stream?token=wrongtoken"
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err == nil {
		t.Fatal("expected WebSocket connection to fail with invalid token (query)")
	}
	if resp != nil && resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d", resp.StatusCode)
	}

	// Wrong token via header
	header := http.Header{}
	header.Set("Authorization", "Bearer wrongtoken")
	_, resp2, err2 := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(ts.URL, "http")+"/stream", header)
	if resp2 != nil {
		defer resp2.Body.Close()
	}
	if err2 == nil {
		t.Fatal("expected WebSocket connection to fail with invalid token (header)")
	}
	if resp2 != nil && resp2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d", resp2.StatusCode)
	}
}

func newPopulatedAgg() *aggregator.Aggregator {
	agg := aggregator.New()
	agg.Add(models.SyscallEvent{Name: "read", Latency: 100 * time.Microsecond})
	agg.Add(models.SyscallEvent{Name: "read", Latency: 200 * time.Microsecond})
	agg.Add(models.SyscallEvent{Name: "write", Latency: 50 * time.Microsecond, Error: "EBADF"})
	agg.Add(models.SyscallEvent{Name: "openat", Latency: 300 * time.Microsecond})
	return agg
}

func TestHealthz(t *testing.T) {
	agg := aggregator.New()
	srv := server.New(":0", agg, "")

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if rr.Body.String() != "ok" {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}

func TestDashboard(t *testing.T) {
	agg := aggregator.New()
	srv := server.New(":0", agg, "")

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Fatalf("expected HTML content-type, got %q", ct)
	}
	if !strings.Contains(rr.Body.String(), "stracectl") {
		t.Fatal("expected dashboard HTML to contain 'stracectl'")
	}
	if !strings.Contains(rr.Body.String(), "/static/dashboard.js") {
		t.Fatal("expected dashboard HTML to load external dashboard.js")
	}
}

func TestDashboardJS(t *testing.T) {
	agg := aggregator.New()
	srv := server.New(":0", agg, "")

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/static/dashboard.js", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/javascript; charset=utf-8" {
		t.Fatalf("expected JavaScript content-type, got %q", ct)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "function switchTab(name)") {
		t.Fatal("expected dashboard JS to contain switchTab")
	}
	if !strings.Contains(body, "connect();") {
		t.Fatal("expected dashboard JS to bootstrap connection")
	}
}

func TestDashboard_UnknownPath(t *testing.T) {
	agg := aggregator.New()
	srv := server.New(":0", agg, "")

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/no/such/path", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestStats(t *testing.T) {
	agg := newPopulatedAgg()
	srv := server.New(":0", agg, "")

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/stats", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("expected JSON content-type, got %q", ct)
	}

	var stats []aggregator.SyscallStat
	if err := json.NewDecoder(rr.Body).Decode(&stats); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(stats) == 0 {
		t.Fatal("expected at least one stat")
	}
	// first result should be "read" (2 calls)
	if stats[0].Name != "read" {
		t.Fatalf("expected read first, got %s", stats[0].Name)
	}
}

func TestCategories(t *testing.T) {
	agg := newPopulatedAgg()
	srv := server.New(":0", agg, "")

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/categories", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var cats map[string]aggregator.CategoryStats
	if err := json.NewDecoder(rr.Body).Decode(&cats); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := cats["I/O"]; !ok {
		t.Fatalf("expected I/O category, got: %v", cats)
	}
}

func TestMetrics(t *testing.T) {
	agg := newPopulatedAgg()
	srv := server.New(":0", agg, "")

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	for _, want := range []string{
		"stracectl_syscall_calls_total",
		"stracectl_syscall_errors_total",
		"stracectl_syscalls_per_second",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("metrics output missing %q", want)
		}
	}
}

func TestStream_WebSocket(t *testing.T) {
	agg := newPopulatedAgg()
	srv := server.New(":0", agg, "")

	ts := httptest.NewServer(srv)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/stream"
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect WebSocket: %v", err)
	}
	if resp != nil {
		defer resp.Body.Close()
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	var stats []aggregator.SyscallStat
	if err := conn.ReadJSON(&stats); err != nil {
		t.Fatalf("failed to read WebSocket message: %v", err)
	}
	if len(stats) == 0 {
		t.Fatal("expected at least one stat from WebSocket")
	}
}

func TestStart_Shutdown(t *testing.T) {
	agg := aggregator.New()
	srv := server.New("127.0.0.1:0", agg, "") // port 0 = random free port

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Start should return nil (context cancelled cleanly)
	if err := srv.Start(ctx); err != nil {
		t.Fatalf("unexpected start error: %v", err)
	}
}

func TestSyscallStat_Found(t *testing.T) {
	agg := newPopulatedAgg()
	srv := server.New(":0", agg, "")

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/syscall/read", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("expected JSON content-type, got %q", ct)
	}

	var stat aggregator.SyscallStat
	if err := json.NewDecoder(rr.Body).Decode(&stat); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if stat.Name != "read" {
		t.Fatalf("expected stat.Name=read, got %q", stat.Name)
	}
	if stat.Count != 2 {
		t.Fatalf("expected Count=2, got %d", stat.Count)
	}
}

func TestSyscallStat_NotFound(t *testing.T) {
	agg := aggregator.New()
	srv := server.New(":0", agg, "")

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/syscall/nonexistent", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestSyscallDetail(t *testing.T) {
	agg := newPopulatedAgg()
	srv := server.New(":0", agg, "")

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/syscall/read", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Fatalf("expected HTML content-type, got %q", ct)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "stracectl") {
		t.Fatal("expected detail HTML to contain 'stracectl'")
	}
	if !strings.Contains(body, "/stream") {
		t.Fatal("expected detail HTML to reference /stream WebSocket endpoint")
	}
	if !strings.Contains(body, "SYSCALL REFERENCE") {
		t.Fatal("expected detail HTML to contain syscall reference section")
	}
}

func TestStatus_DefaultEmpty(t *testing.T) {
	agg := aggregator.New()
	srv := server.New(":0", agg, "")

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/status", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]json.RawMessage
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if _, ok := resp["Elapsed"]; !ok {
		t.Error("response should contain Elapsed field")
	}
}

func TestStatus_WithProcInfo(t *testing.T) {
	agg := aggregator.New()
	agg.SetProcInfo(aggregator.ProcInfo{PID: 42, Comm: "nginx", Exe: "/usr/sbin/nginx"})
	srv := server.New(":0", agg, "")

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/status", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp struct {
		Proc struct {
			PID  int    `json:"PID"`
			Comm string `json:"Comm"`
		} `json:"Proc"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if resp.Proc.PID != 42 {
		t.Errorf("PID: want 42, got %d", resp.Proc.PID)
	}
	if resp.Proc.Comm != "nginx" {
		t.Errorf("Comm: want nginx, got %q", resp.Proc.Comm)
	}
}

func TestLog_Empty(t *testing.T) {
	agg := aggregator.New()
	srv := server.New(":0", agg, "")

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/log", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	// Should return an empty JSON array, not null.
	body := strings.TrimSpace(rr.Body.String())
	if body != "[]" && body != "null" {
		t.Errorf("expected [] or null, got %q", body)
	}
}

func TestLog_ContainsEvents(t *testing.T) {
	agg := newPopulatedAgg()
	srv := server.New(":0", agg, "")

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/log", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var entries []map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&entries); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if len(entries) == 0 {
		t.Error("expected at least one log entry")
	}
}

func TestDashboard_ContainsSearchInput(t *testing.T) {
	agg := aggregator.New()
	srv := server.New(":0", agg, "")

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "search-input") {
		t.Error("dashboard HTML should contain search input element (#search-input)")
	}
	if !strings.Contains(body, "search-clear") {
		t.Error("dashboard HTML should contain search clear button (#search-clear)")
	}
}

func TestStatus_DoneFalseByDefault(t *testing.T) {
	agg := aggregator.New()
	srv := server.New(":0", agg, "")

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/status", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if done, ok := resp["Done"]; !ok || done.(bool) {
		t.Errorf("expected Done=false in status response, got %v", resp["Done"])
	}
}

func TestStatus_DoneTrueAfterSetDone(t *testing.T) {
	agg := aggregator.New()
	agg.SetDone()
	srv := server.New(":0", agg, "")

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/status", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if done, ok := resp["Done"]; !ok || !done.(bool) {
		t.Errorf("expected Done=true in status response, got %v", resp["Done"])
	}
}

func TestDashboard_ContainsDoneBanner(t *testing.T) {
	agg := aggregator.New()
	srv := server.New(":0", agg, "")

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "done-banner") {
		t.Error("dashboard HTML should contain the process-exited banner element (#done-banner)")
	}
}
