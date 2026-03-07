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
	srv := server.New(":0", agg)

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
	srv := server.New(":0", agg)

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
}

func TestDashboard_UnknownPath(t *testing.T) {
	agg := aggregator.New()
	srv := server.New(":0", agg)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/no/such/path", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}



func TestStats(t *testing.T) {
	agg := newPopulatedAgg()
	srv := server.New(":0", agg)

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
	srv := server.New(":0", agg)

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
	srv := server.New(":0", agg)

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
	srv := server.New(":0", agg)

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
	srv := server.New("127.0.0.1:0", agg) // port 0 = random free port

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Start should return nil (context cancelled cleanly)
	if err := srv.Start(ctx); err != nil {
		t.Fatalf("unexpected start error: %v", err)
	}
}
