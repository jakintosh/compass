package main

import "testing"

func TestBuildProductionAppConfig(t *testing.T) {
	cfg, err := buildProductionAppConfig("https://compass.example.com")
	if err != nil {
		t.Fatalf("buildProductionAppConfig returned error: %v", err)
	}

	if cfg.audience != "compass.example.com" {
		t.Fatalf("audience = %q, want %q", cfg.audience, "compass.example.com")
	}
	if cfg.callbackURL != "https://compass.example.com/auth/callback" {
		t.Fatalf("callbackURL = %q", cfg.callbackURL)
	}
	if cfg.homepage != "https://compass.example.com" {
		t.Fatalf("homepage = %q", cfg.homepage)
	}
	if cfg.logoURL != "https://compass.example.com/static/consent-logo.svg" {
		t.Fatalf("logoURL = %q", cfg.logoURL)
	}
}

func TestBuildAuthorizeURL(t *testing.T) {
	got := buildAuthorizeURL("https://consent.example.com", "compass")
	want := "https://consent.example.com/authorize?integration=compass&scope=identity"
	if got != want {
		t.Fatalf("buildAuthorizeURL = %q, want %q", got, want)
	}
}
