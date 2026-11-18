package config

import "testing"

func TestDevEndpointsDefaultOff(t *testing.T) {
	enabled, err := parseEnableDevEndpoints("")
	if err != nil {
		t.Fatal(err)
	}
	if enabled {
		t.Fatal("development endpoints should be disabled by default")
	}
}

func TestDevEndpointsConfiguration(t *testing.T) {
	enabled, err := parseEnableDevEndpoints("true")
	if err != nil {
		t.Fatal(err)
	}
	if !enabled {
		t.Fatal("expected development endpoints to be enabled")
	}

	if _, err := parseEnableDevEndpoints("sometimes"); err == nil {
		t.Fatal("expected invalid boolean value to be rejected")
	}
}

func TestProductionRejectsDevEndpoints(t *testing.T) {
	if err := validateDevEndpoints("production", true); err == nil {
		t.Fatal("expected development endpoints to be rejected in production")
	}
	if err := validateDevEndpoints("development", true); err != nil {
		t.Fatalf("development environment should allow the opt-in: %v", err)
	}
}

func TestLoadAuthConfigRequiresCompleteOIDCSettings(t *testing.T) {
	t.Setenv("AUTH_JWKS_URL", "https://identity.example/keys")
	t.Setenv("AUTH_ISSUER", "https://identity.example/")
	t.Setenv("AUTH_AUDIENCE", "billing-api")
	jwksURL, issuer, audience, err := loadAuthConfig()
	if err != nil {
		t.Fatal(err)
	}
	if jwksURL == "" || issuer == "" || audience == "" {
		t.Fatal("expected OIDC settings to be returned")
	}

	t.Setenv("AUTH_AUDIENCE", "")
	if _, _, _, err := loadAuthConfig(); err == nil {
		t.Fatal("expected incomplete OIDC settings to be rejected")
	}

	t.Setenv("AUTH_AUDIENCE", "billing-api")
	t.Setenv("AUTH_JWKS_URL", "http://identity.example/keys")
	if _, _, _, err := loadAuthConfig(); err == nil {
		t.Fatal("expected non-loopback HTTP JWKS URL to be rejected")
	}
}
