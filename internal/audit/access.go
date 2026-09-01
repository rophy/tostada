package audit

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"gopkg.in/lumberjack.v2"
)

type accessEntry struct {
	Ts         string `json:"ts"`
	Method     string `json:"method"`
	Path       string `json:"path"`
	User       string `json:"user"`
	Status     int    `json:"status"`
	DurationMs int64  `json:"duration_ms"`
	IP         string `json:"ip"`
	UserAgent  string `json:"user_agent,omitempty"`
}

type AccessLogger struct {
	w  io.WriteCloser
	mu sync.Mutex
}

func NewAccessLogger(path string, maxSizeMB, maxBackups int) *AccessLogger {
	return &AccessLogger{
		w: &lumberjack.Logger{
			Filename:   path,
			MaxSize:    maxSizeMB,
			MaxBackups: maxBackups,
		},
	}
}

// UserFunc extracts the authenticated username from a request.
type UserFunc func(r *http.Request) string

// Middleware returns an HTTP middleware that logs each request as a JSONL line.
func (a *AccessLogger) Middleware(next http.Handler, userFn UserFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)

		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		if ip == "" {
			ip = r.RemoteAddr
		}

		entry := accessEntry{
			Ts:         start.UTC().Format("2006-01-02T15:04:05.000Z"),
			Method:     r.Method,
			Path:       r.URL.Path,
			User:       userFn(r),
			Status:     sw.status,
			DurationMs: time.Since(start).Milliseconds(),
			IP:         ip,
			UserAgent:  r.UserAgent(),
		}
		data, err := json.Marshal(entry)
		if err != nil {
			return
		}
		data = append(data, '\n')
		a.mu.Lock()
		defer a.mu.Unlock()
		a.w.Write(data)
	})
}

func (a *AccessLogger) Close() error {
	return a.w.Close()
}

type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *statusWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.status = code
		w.wroteHeader = true
	}
	w.ResponseWriter.WriteHeader(code)
}
