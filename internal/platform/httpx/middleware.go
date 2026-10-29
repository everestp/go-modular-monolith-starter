package httpx

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"time"
)

type contextKey uint8

const (
	requestIDKey contextKey = iota
	userIDKey
)

type responseRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

// Metrics holds bounded-cardinality HTTP request metrics.
type Metrics struct {
	mu       sync.Mutex
	requests map[string]uint64
	buckets  map[string][6]uint64
	inflight int64
}

// NewMetrics creates an empty request metrics collector.
func NewMetrics() *Metrics {
	return &Metrics{requests: make(map[string]uint64), buckets: make(map[string][6]uint64)}
}

// Instrument records request count, duration buckets, and in-flight requests.
func Instrument(metrics *Metrics, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		metrics.mu.Lock()
		metrics.inflight++
		metrics.mu.Unlock()
		recorder := &responseRecorder{ResponseWriter: w}
		next.ServeHTTP(recorder, r)
		status := recorder.status
		if status == 0 {
			status = http.StatusOK
		}
		class := strconv.Itoa(status/100) + "xx"
		route := r.Pattern
		if route == "" {
			route = "unmatched"
		}
		key := r.Method + "|" + route + "|" + class
		elapsed := time.Since(started).Seconds()
		limits := [...]float64{0.005, 0.01, 0.025, 0.1, 0.5, 1}
		metrics.mu.Lock()
		metrics.inflight--
		metrics.requests[key]++
		counts := metrics.buckets[key]
		for index, limit := range limits {
			if elapsed <= limit {
				counts[index]++
			}
		}
		metrics.buckets[key] = counts
		metrics.mu.Unlock()
	})
}

// ServeHTTP exposes Prometheus text exposition for HTTP request metrics.
func (m *Metrics) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	m.mu.Lock()
	defer m.mu.Unlock()
	_, _ = fmt.Fprintln(w, "# HELP http_requests_total Total HTTP requests.")
	_, _ = fmt.Fprintln(w, "# TYPE http_requests_total counter")
	_, _ = fmt.Fprintln(w, "# HELP http_request_duration_seconds HTTP request duration.")
	_, _ = fmt.Fprintln(w, "# TYPE http_request_duration_seconds histogram")
	_, _ = fmt.Fprintln(w, "# HELP http_in_flight_requests Current in-flight HTTP requests.")
	_, _ = fmt.Fprintln(w, "# TYPE http_in_flight_requests gauge")
	_, _ = fmt.Fprintf(w, "http_in_flight_requests %d\n", m.inflight)
	limits := [...]string{"0.005", "0.01", "0.025", "0.1", "0.5", "1"}
	for key, count := range m.requests {
		parts := strings.Split(key, "|")
		if len(parts) != 3 {
			continue
		}
		labels := fmt.Sprintf(`method="%s",route="%s",status_class="%s"`, parts[0], escapeLabel(parts[1]), parts[2])
		_, _ = fmt.Fprintf(w, "http_requests_total{%s} %d\n", labels, count)
		bucketCounts := m.buckets[key]
		var cumulative uint64
		for index, limit := range limits {
			cumulative += bucketCounts[index]
			_, _ = fmt.Fprintf(w, "http_request_duration_seconds_bucket{%s,le=\"%s\"} %d\n", labels, limit, cumulative)
		}
		_, _ = fmt.Fprintf(w, "http_request_duration_seconds_bucket{%s,le=\"+Inf\"} %d\n", labels, count)
		_, _ = fmt.Fprintf(w, "http_request_duration_seconds_count{%s} %d\n", labels, count)
	}
}

func (w *responseRecorder) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseRecorder) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	count, err := w.ResponseWriter.Write(body)
	w.bytes += count
	return count, err
}

// AccessLog records request metadata without logging bodies or credentials.
func AccessLog(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		recorder := &responseRecorder{ResponseWriter: w}
		next.ServeHTTP(recorder, r)
		status := recorder.status
		if status == 0 {
			status = http.StatusOK
		}
		level := slog.LevelInfo
		if status >= http.StatusInternalServerError {
			level = slog.LevelError
		} else if status >= http.StatusBadRequest {
			level = slog.LevelWarn
		}
		log.Log(r.Context(), level, "http request",
			"request_id", RequestID(r.Context()),
			"method", r.Method,
			"path", r.Pattern,
			"status", status,
			"latency_ms", time.Since(started).Milliseconds(),
			"bytes", recorder.bytes,
			"remote_ip", r.RemoteAddr,
			"user_agent", r.UserAgent(),
		)
	})
}

// RequestID returns the request identifier associated with a request.
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

// AuthenticatedUserID returns the verified user ID from request context.
func AuthenticatedUserID(ctx context.Context) string {
	id, _ := ctx.Value(userIDKey).(string)
	return id
}

// Authenticate validates a bearer access token and stores its subject in context.
func Authenticate(validate func(string) (string, error), next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const prefix = "Bearer "
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, prefix) {
			WriteProblemFor(w, r, http.StatusUnauthorized, "Unauthorized", "A valid bearer token is required.")
			return
		}
		userID, err := validate(strings.TrimPrefix(header, prefix))
		if err != nil || userID == "" {
			WriteProblemFor(w, r, http.StatusUnauthorized, "Unauthorized", "A valid bearer token is required.")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userIDKey, userID)))
	})
}

// WithRequestID accepts a validated inbound identifier or generates one.
func WithRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if !validRequestID(id) {
			id = newRequestID()
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, id)))
	})
}

// WriteProblemFor writes an RFC 9457 response enriched with request context.
func WriteProblemFor(w http.ResponseWriter, r *http.Request, status int, title, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"type":       "about:blank",
		"title":      title,
		"status":     status,
		"detail":     detail,
		"instance":   r.URL.Path,
		"request_id": RequestID(r.Context()),
	})
}

// Recover logs panics and returns a generic problem response.
func Recover(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer recoverRequest(log, w, r)
		next.ServeHTTP(w, r)
	})
}

func recoverRequest(log *slog.Logger, w http.ResponseWriter, r *http.Request) {
	if recovered := recover(); recovered != nil {
		log.ErrorContext(r.Context(), "request panic", "request_id", RequestID(r.Context()), "panic", fmt.Sprint(recovered))
		WriteProblemFor(w, r, http.StatusInternalServerError, "Internal Server Error", "An unexpected error occurred.")
	}
}

// SecurityHeaders adds baseline browser security headers to responses.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := w.Header()
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("X-Frame-Options", "DENY")
		header.Set("Content-Security-Policy", "frame-ancestors 'none'")
		header.Set("Referrer-Policy", "no-referrer")
		if strings.HasPrefix(r.URL.Path, "/api/v1/auth/") {
			header.Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

// Deprecation adds RFC 9745 deprecation and RFC 8594 sunset headers.
func Deprecation(deprecatedAt, sunset string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Deprecation", deprecatedAt)
		w.Header().Set("Sunset", sunset)
		next.ServeHTTP(w, r)
	})
}

// ProblemMethodErrors replaces ServeMux's plain-text 405 response with problem JSON.
func ProblemMethodErrors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buffered := httptest.NewRecorder()
		next.ServeHTTP(buffered, r)
		if buffered.Code == http.StatusMethodNotAllowed {
			for key, values := range buffered.Header() {
				for _, value := range values {
					w.Header().Add(key, value)
				}
			}
			WriteProblemFor(w, r, http.StatusMethodNotAllowed, "Method Not Allowed", "The HTTP method is not supported for this resource.")
			return
		}
		for key, values := range buffered.Header() {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		w.WriteHeader(buffered.Code)
		_, _ = w.Write(buffered.Body.Bytes())
	})
}

// CORS allows only configured origins and handles browser preflight requests.
func CORS(origins []string, next http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(origins))
	for _, origin := range origins {
		if trimmed := strings.TrimSpace(origin); trimmed != "" {
			allowed[trimmed] = struct{}{}
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if _, ok := allowed[origin]; ok && origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		}
		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			if _, ok := allowed[origin]; !ok {
				WriteProblemFor(w, r, http.StatusForbidden, "Forbidden", "Origin is not allowed.")
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// LimitBody caps the size of request bodies before handlers decode them.
func LimitBody(maxBytes int64, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
		next.ServeHTTP(w, r)
	})
}

// RateLimiter decides whether a client key may make another request.
type RateLimiter interface {
	Allow(key string) bool
}

// TokenBucket provides an in-memory token bucket limiter.
type TokenBucket struct {
	mu      sync.Mutex
	clients map[string]bucket
	rate    float64
	burst   float64
	now     func() time.Time
}

type bucket struct {
	tokens   float64
	updated  time.Time
	lastSeen time.Time
}

// NewTokenBucket creates a limiter with a refill rate and burst capacity.
func NewTokenBucket(rate int, burst int) *TokenBucket {
	if rate < 1 {
		rate = 1
	}
	if burst < 1 {
		burst = 1
	}
	return &TokenBucket{clients: make(map[string]bucket), rate: float64(rate), burst: float64(burst), now: time.Now}
}

// Allow consumes one token for a key when the bucket has capacity.
func (l *TokenBucket) Allow(key string) bool {
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	for client, state := range l.clients {
		if now.Sub(state.lastSeen) > 10*time.Minute {
			delete(l.clients, client)
		}
	}
	state, exists := l.clients[key]
	if !exists {
		state = bucket{tokens: l.burst, updated: now}
	}
	state.tokens += now.Sub(state.updated).Seconds() * l.rate
	if state.tokens > l.burst {
		state.tokens = l.burst
	}
	state.updated = now
	state.lastSeen = now
	if state.tokens < 1 {
		l.clients[key] = state
		return false
	}
	state.tokens--
	l.clients[key] = state
	return true
}

// RateLimit rejects requests when the supplied limiter denies the client key.
func RateLimit(limiter RateLimiter, clientKey func(*http.Request) string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow(clientKey(r)) {
			w.Header().Set("Retry-After", strconv.Itoa(1))
			WriteProblemFor(w, r, http.StatusTooManyRequests, "Too Many Requests", "Request limit exceeded.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RateLimitByPath applies a stricter limiter to requests matching a path prefix.
func RateLimitByPath(defaultLimiter, strictLimiter RateLimiter, prefix string, clientKey func(*http.Request) string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		limiter := defaultLimiter
		if strings.HasPrefix(r.URL.Path, prefix) {
			limiter = strictLimiter
		}
		if !limiter.Allow(clientKey(r)) {
			w.Header().Set("Retry-After", strconv.Itoa(1))
			WriteProblemFor(w, r, http.StatusTooManyRequests, "Too Many Requests", "Request limit exceeded.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ClientIP resolves the peer IP and trusts forwarded addresses only from configured proxies.
func ClientIP(r *http.Request, trusted []string) string {
	peer, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		peer = r.RemoteAddr
	}
	peerIP := net.ParseIP(peer)
	if peerIP == nil || !isTrusted(peerIP, trusted) {
		return peer
	}
	forwarded := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for index := len(forwarded) - 1; index >= 0; index-- {
		candidate := strings.TrimSpace(forwarded[index])
		ip := net.ParseIP(candidate)
		if ip == nil {
			continue
		}
		if !isTrusted(ip, trusted) {
			return ip.String()
		}
	}
	return peerIP.String()
}

func isTrusted(ip net.IP, trusted []string) bool {
	for _, entry := range trusted {
		if parsed := net.ParseIP(entry); parsed != nil && parsed.Equal(ip) {
			return true
		}
		_, network, err := net.ParseCIDR(entry)
		if err == nil && network.Contains(ip) {
			return true
		}
	}
	return false
}

func escapeLabel(value string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`).Replace(value)
}

func validRequestID(id string) bool {
	if len(id) == 0 || len(id) > 128 {
		return false
	}
	for _, char := range id {
		if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || strings.ContainsRune("-_.:", char)) {
			return false
		}
	}
	return true
}

func newRequestID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "request-id-unavailable"
	}
	return hex.EncodeToString(bytes[:])
}
