package server

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

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

func TestBuildProductionAppConfigWithBasePath(t *testing.T) {
	cfg, err := buildProductionAppConfig("https://compass.example.com/tools/compass/")
	if err != nil {
		t.Fatalf("buildProductionAppConfig returned error: %v", err)
	}

	if cfg.callbackURL != "https://compass.example.com/tools/compass/auth/callback" {
		t.Fatalf("callbackURL = %q", cfg.callbackURL)
	}
	if cfg.homepage != "https://compass.example.com/tools/compass" {
		t.Fatalf("homepage = %q", cfg.homepage)
	}
	if cfg.logoURL != "https://compass.example.com/tools/compass/static/consent-logo.svg" {
		t.Fatalf("logoURL = %q", cfg.logoURL)
	}
}

func TestBuildAuthorizeURL(t *testing.T) {
	got := buildAuthorizeURL("https://consent.example.com", "compass")
	want := "https://consent.example.com/authorize?integration=compass&scope=identity&scope=profile"
	if got != want {
		t.Fatalf("buildAuthorizeURL = %q, want %q", got, want)
	}
}

func TestLoadConsentPublicKeyFromDERFile(t *testing.T) {
	key := generateTestKey(t)
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}

	configDir := t.TempDir()
	filename := filepath.Join(configDir, "verification_key")
	if err := os.WriteFile(filename, der, 0o644); err != nil {
		t.Fatalf("write DER file: %v", err)
	}

	got, err := loadConsentPublicKey(Options{ConfigDir: configDir})
	if err != nil {
		t.Fatalf("loadConsentPublicKey returned error: %v", err)
	}

	assertSamePublicKey(t, got, &key.PublicKey)
}

func TestBuildHandlerMountsAuthAndAppRoutes(t *testing.T) {
	dir := t.TempDir()
	handler, cleanup, err := BuildHandler(Options{
		Dev:     true,
		DataDir: dir,
	})
	if err != nil {
		t.Fatalf("BuildHandler returned error: %v", err)
	}
	defer cleanup()

	if _, err := os.Stat(filepath.Join(dir, "compass.db")); err != nil {
		t.Fatalf("expected compass.db in data dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "dev.key")); !os.IsNotExist(err) {
		t.Fatalf("dev mode should not persist dev.key, got err: %v", err)
	}

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{
			name:   "app index",
			method: http.MethodGet,
			path:   "/",
		},
		{
			name:   "dev auth route",
			method: http.MethodGet,
			path:   "/dev/login",
		},
		{
			name:   "tenant category route",
			method: http.MethodPost,
			path:   "/alice/categories",
		},
		{
			name:   "tenant project route",
			method: http.MethodPost,
			path:   "/alice/projects/reorder",
		},
		{
			name:   "tenant task route",
			method: http.MethodPost,
			path:   "/alice/tasks/reorder",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code == http.StatusNotFound {
				t.Fatalf("%s returned 404", tt.path)
			}
		})
	}
}

func generateTestKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return key
}

func assertSamePublicKey(t *testing.T, got *ecdsa.PublicKey, want *ecdsa.PublicKey) {
	t.Helper()

	if got == nil {
		t.Fatal("got nil public key")
	}
	if got.Curve != want.Curve ||
		got.X.Cmp(want.X) != 0 ||
		got.Y.Cmp(want.Y) != 0 {
		t.Fatalf("public key mismatch")
	}
}
