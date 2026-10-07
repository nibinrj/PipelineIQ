package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRoutes(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	tests := []struct {
		name       string
		method     string
		path       string
		pingErr    error
		wantStatus int
		wantBody   string
	}{
		{name: "live", method: http.MethodGet, path: "/healthz", wantStatus: http.StatusOK, wantBody: "ok"},
		{name: "ready", method: http.MethodGet, path: "/readyz", wantStatus: http.StatusOK, wantBody: "ok"},
		{name: "ready down", method: http.MethodGet, path: "/readyz", pingErr: errors.New("dial tcp: connection refused"), wantStatus: http.StatusServiceUnavailable, wantBody: "unavailable"},
		{name: "post health is not allowed", method: http.MethodPost, path: "/healthz", wantStatus: http.StatusMethodNotAllowed},
		{name: "unknown", method: http.MethodGet, path: "/api/v1/builds", wantStatus: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := New(log, fakePinger{err: tt.pingErr}, time.Second)
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d, body %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantBody == "" {
				return
			}
			if rec.Code == http.StatusServiceUnavailable && strings.Contains(rec.Body.String(), "connection refused") {
				t.Fatalf("response leaked the driver error: %s", rec.Body.String())
			}
			var body map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("json: %v body %s", err, rec.Body.String())
			}
			if body["status"] != tt.wantBody {
				t.Fatalf("status field = %q, want %q", body["status"], tt.wantBody)
			}
		})
	}
}

func TestReadyzDoesNotCallPingForLiveness(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	pinger := &countingPinger{err: errors.New("down")}
	h := New(log, pinger, time.Second)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if pinger.calls != 0 {
		t.Fatalf("healthz called Ping %d times", pinger.calls)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if pinger.calls != 1 {
		t.Fatalf("readyz calls = %d, want 1", pinger.calls)
	}
}

func TestServeStopsOnCancel(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- Serve(ctx, ln, New(log, fakePinger{}, time.Second), time.Second)
	}()

	url := "http://" + ln.Addr().String() + "/healthz"
	var resp *http.Response
	deadline := time.Now().Add(2 * time.Second)
	for {
		resp, err = http.Get(url)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("server did not accept: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown did not return")
	}
}

type fakePinger struct {
	err error
}

func (f fakePinger) Ping(context.Context) error { return f.err }

type countingPinger struct {
	err   error
	calls int
}

func (p *countingPinger) Ping(context.Context) error {
	p.calls++
	return p.err
}
