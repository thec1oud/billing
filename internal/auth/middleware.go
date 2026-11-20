package auth

import (
	"net/http"

	"github.com/thec1oud/billing/internal/infra/api/response"
)

func Middleware(next http.Handler, authenticator Authenticator) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" || IsWebhookPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		token, ok := BearerToken(r.Header.Get("Authorization"))
		if !ok || authenticator == nil {
			unauthorized(w)
			return
		}
		principal, err := authenticator.Authenticate(token)
		if err != nil {
			unauthorized(w)
			return
		}
		next.ServeHTTP(w, r.WithContext(WithPrincipal(r.Context(), principal)))
	})
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	response.Write(w, http.StatusUnauthorized, &response.ErrorResponse{
		Code:    "UNAUTHENTICATED",
		Message: "A valid bearer access token is required",
	})
}
