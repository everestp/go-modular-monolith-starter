package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeJSONRejectsTrailingValuesAndUnknownFields(t *testing.T) {
	t.Parallel()
	for _, body := range []string{`{"name":"a"} {"name":"b"}`, `{"name":"a","other":true}`} {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		var payload struct {
			Name string `json:"name"`
		}
		if err := DecodeJSON(req, &payload); err == nil {
			t.Fatalf("DecodeJSON(%q) unexpectedly succeeded", body)
		}
	}
}

func TestDecodeJSONRejectsOversizedBody(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"value":"`+strings.Repeat("a", maxJSONBody)+`"}`))
	var payload map[string]string
	if err := DecodeJSON(req, &payload); err == nil {
		t.Fatal("DecodeJSON accepted an oversized body")
	}
}

func TestWithRequestID(t *testing.T) {
	t.Parallel()
	handler := WithRequestID(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		if got := RequestID(r.Context()); got == "" {
			t.Error("RequestID is empty")
		}
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "trace-123")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	if got := response.Header().Get("X-Request-ID"); got != "trace-123" {
		t.Fatalf("response request ID = %q, want trace-123", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "invalid id")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	if got := response.Header().Get("X-Request-ID"); got == "invalid id" || got == "" {
		t.Fatalf("invalid request ID was not replaced: %q", got)
	}
}

func TestWriteProblemForIncludesRequestID(t *testing.T) {
	t.Parallel()
	ctx := context.WithValue(context.Background(), requestIDKey, "request-1")
	req := httptest.NewRequest(http.MethodGet, "/private", nil).WithContext(ctx)
	response := httptest.NewRecorder()
	WriteProblemFor(response, req, http.StatusBadRequest, "Bad Request", `quote " safely`)
	var problem map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &problem); err != nil {
		t.Fatalf("problem response is not valid JSON: %v", err)
	}
	if problem["request_id"] != "request-1" || problem["instance"] != "/private" {
		t.Fatalf("unexpected problem fields: %#v", problem)
	}
}

func TestRecoverReturnsProblem(t *testing.T) {
	t.Parallel()
	handler := WithRequestID(Recover(slog.New(slog.NewTextHandler(io.Discard, nil)), http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic(errors.New("internal detail"))
	})))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusInternalServerError || !strings.Contains(response.Header().Get("Content-Type"), "application/problem+json") {
		t.Fatalf("recovery response = %d %q", response.Code, response.Header().Get("Content-Type"))
	}
	if strings.Contains(response.Body.String(), "internal detail") {
		t.Fatal("panic detail leaked to the response")
	}
}

func TestCORSAndSecurityHeaders(t *testing.T) {
	t.Parallel()
	handler := SecurityHeaders(CORS([]string{"https://client.example"}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/login", nil)
	req.Header.Set("Origin", "https://client.example")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	if response.Header().Get("Access-Control-Allow-Origin") != "https://client.example" || response.Header().Get("X-Content-Type-Options") != "nosniff" || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("missing security/CORS headers: %#v", response.Header())
	}
}

func TestTokenBucketAndTrustedClientIP(t *testing.T) {
	t.Parallel()
	limiter := NewTokenBucket(1, 1)
	if !limiter.Allow("client") || limiter.Allow("client") {
		t.Fatal("token bucket did not enforce burst capacity")
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.2:1234"
	req.Header.Set("X-Forwarded-For", "198.51.100.8, 10.0.0.1")
	if got := ClientIP(req, []string{"10.0.0.0/8"}); got != "198.51.100.8" {
		t.Fatalf("ClientIP = %q, want 198.51.100.8", got)
	}
	req.RemoteAddr = "192.0.2.3:1234"
	if got := ClientIP(req, []string{"10.0.0.0/8"}); got != "192.0.2.3" {
		t.Fatalf("untrusted peer forwarded spoofed IP: got %q", got)
	}
}

func TestMetricsExposeRouteCountersAndHistogram(t *testing.T) {
	t.Parallel()
	metrics := NewMetrics()
	handler := Instrument(metrics, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Pattern = "GET /items/{id}"
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/items/123", nil))
	response := httptest.NewRecorder()
	metrics.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	for _, expected := range []string{
		"http_requests_total{method=\"GET\",route=\"GET /items/{id}\",status_class=\"2xx\"} 1",
		"http_request_duration_seconds_bucket",
		"http_in_flight_requests 0",
	} {
		if !strings.Contains(response.Body.String(), expected) {
			t.Errorf("metrics output missing %q", expected)
		}
	}
}

func TestProblemMethodErrorsAndDeprecation(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /resource", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	handler := WithRequestID(ProblemMethodErrors(Deprecation("@1700000000", "Wed, 01 Jan 2030 00:00:00 GMT", mux)))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/resource", nil))
	if response.Code != http.StatusMethodNotAllowed || !strings.Contains(response.Header().Get("Content-Type"), "application/problem+json") {
		t.Fatalf("method response = %d, %q", response.Code, response.Header().Get("Content-Type"))
	}
	if response.Header().Get("Deprecation") != "@1700000000" || response.Header().Get("Sunset") == "" {
		t.Fatalf("deprecation headers missing: %#v", response.Header())
	}
}

func TestRateLimitByPathUsesStrictLimiter(t *testing.T) {
	t.Parallel()
	defaultLimiter := NewTokenBucket(1, 1)
	strictLimiter := NewTokenBucket(1, 1)
	handler := RateLimitByPath(defaultLimiter, strictLimiter, "/auth/", func(*http.Request) string { return "client" }, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	for index := 0; index < 2; index++ {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/auth/login", nil))
		if index == 0 && response.Code != http.StatusNoContent || index == 1 && response.Code != http.StatusTooManyRequests {
			t.Fatalf("request %d status = %d", index, response.Code)
		}
	}
}
