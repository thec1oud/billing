package logger

import (
	"log/slog"
	"net/http"
	"time"
)

type responseWriterInterceptor struct {
	http.ResponseWriter
	statusCode int
}

func (w *responseWriterInterceptor) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}


// RequestLogger is an HTTP middleware that comprehensively logs API requests and responses.
func RequestLogger(next http.Handler) http.Handler {
	log := ForComponent("http_api")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Prepare response interceptor to capture status code
		interceptor := &responseWriterInterceptor{
			ResponseWriter: w,
			statusCode:     http.StatusOK, // Default if WriteHeader is not explicitly called
		}

		// Pass execution to downstream handlers
		next.ServeHTTP(interceptor, r)

		duration := time.Since(start)

		// Log request details (omitting bodies for security and performance)
		log.Info("API Request Processed",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", interceptor.statusCode),
			slog.String("duration", duration.String()),
			slog.String("ip", r.RemoteAddr),
			slog.String("user_agent", r.UserAgent()),
		)
	})
}
