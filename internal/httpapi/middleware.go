package httpapi

import (
	"context"
	"crypto/rand"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

const requestIDHeader = "X-Request-ID"

type requestIDKey struct{}

func requestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

func observeRequests(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			id := rand.Text()
			r = r.WithContext(context.WithValue(r.Context(), requestIDKey{}, id))
			w.Header().Set(requestIDHeader, id)
			path := r.URL.EscapedPath()
			if path == livePath || path == readyPath {
				w.Header().Set("Cache-Control", "no-store")
			}
			response := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			defer func() {
				cause := recover()
				status := response.Status()
				if cause != nil && cause != http.ErrAbortHandler {
					logger.ErrorContext(r.Context(), "Pânico durante requisição HTTP", "request_id", id)
					if status == 0 {
						writeProblem(response, r, internalError)
						status = response.Status()
						cause = nil
					}
				}
				if status == 0 && cause == nil {
					status = http.StatusOK
				}
				logger.InfoContext(r.Context(), "Requisição HTTP",
					"request_id", id, "method", r.Method, "path", path,
					"status", status, "duration_ms", time.Since(start).Milliseconds())
				if cause != nil {
					panic(http.ErrAbortHandler)
				}
			}()
			next.ServeHTTP(response, r)
		})
	}
}

func cors(origins []string, allowsGet func(*http.Request) bool) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(origins))
	for _, origin := range origins {
		allowed[origin] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := w.Header()
			header.Add("Vary", "Origin")
			requestedMethod := r.Header.Get("Access-Control-Request-Method")
			preflight := r.Method == http.MethodOptions && requestedMethod != ""
			if r.Method == http.MethodOptions {
				header.Add("Vary", "Access-Control-Request-Method, Access-Control-Request-Headers")
			}
			origin := r.Header.Get("Origin")
			if origin != "" && allowed[origin] {
				if !preflight {
					header.Set("Access-Control-Allow-Origin", origin)
					header.Set("Access-Control-Expose-Headers", requestIDHeader)
				} else if requestedMethod == http.MethodGet && allowsGet(r) &&
					allowedPreflightHeaders(r.Header.Values("Access-Control-Request-Headers")) {
					header.Set("Access-Control-Allow-Origin", origin)
					header.Set("Access-Control-Allow-Methods", http.MethodGet)
					header.Set("Access-Control-Allow-Headers", "Content-Type")
					w.WriteHeader(http.StatusNoContent)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func allowedPreflightHeaders(values []string) bool {
	for _, value := range values {
		for name := range strings.SplitSeq(value, ",") {
			if !strings.EqualFold(strings.TrimSpace(name), "Content-Type") {
				return false
			}
		}
	}
	return true
}
