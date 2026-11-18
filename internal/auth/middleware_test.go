package auth_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thec1oud/billing/internal/auth"
)

type staticAuthenticator struct{}

func (staticAuthenticator) Authenticate(token string) (auth.Principal, error) {
	if token != "valid-token" {
		return auth.Principal{}, errors.New("invalid token")
	}
	return auth.Principal{Subject: "verified-user"}, nil
}

func TestMiddlewareRequiresValidBearerAndAddsVerifiedPrincipal(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			t.Error("verified principal missing from request context")
		}
		_, _ = w.Write([]byte(principal.Subject))
	})
	handler := auth.Middleware(next, staticAuthenticator{})

	for _, testCase := range []struct {
		name       string
		header     string
		wantStatus int
		wantBody   string
	}{
		{name: "missing", wantStatus: http.StatusUnauthorized},
		{name: "invalid", header: "Bearer invalid-token", wantStatus: http.StatusUnauthorized},
		{name: "valid", header: "Bearer valid-token", wantStatus: http.StatusOK, wantBody: "verified-user"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/1", nil)
			request.Header.Set("Authorization", testCase.header)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != testCase.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, testCase.wantStatus)
			}
			if testCase.wantBody != "" && response.Body.String() != testCase.wantBody {
				t.Fatalf("body = %q, want %q", response.Body.String(), testCase.wantBody)
			}
		})
	}
}

func TestMiddlewareLeavesHealthAndProviderWebhooksForTheirOwnVerification(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})
	handler := auth.Middleware(next, nil)
	for _, path := range []string{"/health", "/api/v1/webhooks/fake"} {
		request := httptest.NewRequest(http.MethodPost, path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusAccepted {
			t.Fatalf("%s returned %d, want %d", path, response.Code, http.StatusAccepted)
		}
	}
}
