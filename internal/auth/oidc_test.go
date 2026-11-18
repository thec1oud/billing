package auth_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/thec1oud/billing/internal/auth"
)

func TestOIDCVerifierChecksSignatureIssuerAudienceAndExpiry(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	jwks := fmt.Sprintf(
		`{"keys":[{"kty":"RSA","kid":"test-key","use":"sig","alg":"RS256","n":"%s","e":"AQAB"}]}`,
		base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(jwks))
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	verifier, err := auth.NewOIDCVerifier(ctx, server.URL, "https://issuer.example/", "billing-api")
	if err != nil {
		t.Fatal(err)
	}

	valid := signToken(t, key, jwt.RegisteredClaims{
		Issuer:    "https://issuer.example/",
		Subject:   "user-42",
		Audience:  []string{"billing-api"},
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
	})
	principal, err := verifier.Authenticate(valid)
	if err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}
	if principal.Subject != "user-42" {
		t.Fatalf("subject = %q, want user-42", principal.Subject)
	}

	for _, claims := range []jwt.RegisteredClaims{
		{
			Issuer:    "https://attacker.example/",
			Subject:   "user-42",
			Audience:  []string{"billing-api"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
		},
		{
			Issuer:    "https://issuer.example/",
			Subject:   "user-42",
			Audience:  []string{"other-api"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
		},
		{
			Issuer:    "https://issuer.example/",
			Subject:   "user-42",
			Audience:  []string{"billing-api"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
		},
	} {
		if _, err := verifier.Authenticate(signToken(t, key, claims)); err == nil {
			t.Fatal("expected invalid access token to be rejected")
		}
	}
}

func signToken(t *testing.T, key *rsa.PrivateKey, claims jwt.RegisteredClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = "test-key"
	signed, err := token.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}
