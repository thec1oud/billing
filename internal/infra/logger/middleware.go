package logger

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"time"
)

type responseWriterInterceptor struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

func (w *responseWriterInterceptor) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *responseWriterInterceptor) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// RequestLogger is an HTTP middleware that comprehensively logs API requests and responses.
func RequestLogger(next http.Handler) http.Handler {
	log := ForComponent("http_api")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Intercept and buffer request body
		var reqBodyBytes []byte
		if r.Body != nil {
			reqBodyBytes, _ = io.ReadAll(r.Body)
			r.Body = io.NopCloser(bytes.NewBuffer(reqBodyBytes))
		}

		// Prepare response interceptor to capture status code and response payload
		interceptor := &responseWriterInterceptor{
			ResponseWriter: w,
			statusCode:     http.StatusOK, // Default if WriteHeader is not explicitly called
			body:           &bytes.Buffer{},
		}

		// Pass execution to downstream handlers
		next.ServeHTTP(interceptor, r)

		duration := time.Since(start)

		// Log comprehensive request/response details
		log.Info("API Request Processed",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", interceptor.statusCode),
			slog.String("duration", duration.String()),
			slog.String("ip", r.RemoteAddr),
			slog.String("user_agent", r.UserAgent()),
			slog.String("req_body", string(reqBodyBytes)),
			slog.String("res_body", interceptor.body.String()),
		)
	})
}
