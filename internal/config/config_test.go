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
