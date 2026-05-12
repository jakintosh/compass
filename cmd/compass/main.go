package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"git.sr.ht/~jakintosh/compass/internal/database"
	"git.sr.ht/~jakintosh/compass/internal/service"
	"git.sr.ht/~jakintosh/compass/internal/web"
	"git.sr.ht/~jakintosh/consent/pkg/client"
	contesting "git.sr.ht/~jakintosh/consent/pkg/testing"
	"git.sr.ht/~jakintosh/consent/pkg/tokens"
)

var consentScopes = []string{"identity", "profile"}

// getConfigValue returns the CLI flag value if set, otherwise falls back to env var.
func getConfigValue(
	flagVal,
	envKey string,
) string {
	if flagVal != "" {
		return flagVal
	}
	return os.Getenv(envKey)
}

func main() {
	// Parse CLI flags
	devMode := flag.Bool("dev", false, "Run in dev mode (no consent server needed)")
	consentURL := flag.String("consent-url", "", "Consent server URL (env: CONSENT_URL)")
	consentPubkey := flag.String("consent-pubkey", "", "Consent server public key PEM (env: CONSENT_PUBKEY)")
	integrationName := flag.String("integration-name", "", "Consent integration name (env: CONSENT_INTEGRATION, default: compass)")
	publicURL := flag.String("public-url", "", "Public base URL for this Compass instance (env: PUBLIC_URL)")
	appID := flag.String("app-id", "", "Deprecated alias for --integration-name when --integration-name is unset (env: APP_ID)")
	flag.Parse()

	// Resolve config with CLI > env fallback
	resolvedConsentURL := getConfigValue(*consentURL, "CONSENT_URL")
	resolvedConsentPubkey := getConfigValue(*consentPubkey, "CONSENT_PUBKEY")
	resolvedIntegrationName := getConfigValue(*integrationName, "CONSENT_INTEGRATION")
	if resolvedIntegrationName == "" {
		resolvedIntegrationName = getConfigValue(*appID, "APP_ID")
	}
	if resolvedIntegrationName == "" {
		resolvedIntegrationName = "compass"
	}
	resolvedPublicURL := getConfigValue(*publicURL, "PUBLIC_URL")

	// Initialize database
	databaseOpts := database.Options{
		Path: "compass.db",
		WAL:  true,
	}
	db, err := database.Open(databaseOpts)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	serviceOpts := service.Options{
		Store: db,
		Clock: time.Now,
	}
	svc, err := service.New(serviceOpts)
	if err != nil {
		log.Fatalf("Failed to initialize service: %v", err)
	}

	// Configure authentication based on mode
	var authConfig web.AuthConfig

	if *devMode {
		// Dev mode: use TestVerifier from consent/pkg/testing with persistent key
		key, err := getOrGenerateDevKey("dev.key")
		if err != nil {
			log.Fatalf("Failed to get/generate dev key: %v", err)
		}

		env := contesting.NewTestEnvWithKey(key, "localhost", "compass-dev")
		env.Scopes = consentScopes
		tv := contesting.NewTestVerifierWithEnv(env)

		authConfig = web.AuthConfig{
			Verifier:  tv,
			LoginURL:  "/dev/login",
			LogoutURL: "/dev/logout",
			Routes: map[string]http.HandlerFunc{
				"/dev/login":  tv.HandleDevLogin(),
				"/dev/logout": tv.HandleDevLogout(),
			},
		}
	} else {
		// Production mode: real consent server
		if resolvedConsentURL == "" || resolvedConsentPubkey == "" || resolvedPublicURL == "" {
			log.Fatalf("Production mode requires --consent-url, --consent-pubkey, and --public-url (or use --dev for development)")
		}

		pubKey, err := parsePublicKey(resolvedConsentPubkey)
		if err != nil {
			log.Fatalf("Failed to parse consent public key: %v", err)
		}

		normalizedConsentURL, consentIssuer, err := normalizeConsentURL(resolvedConsentURL)
		if err != nil {
			log.Fatalf("Invalid consent URL: %v", err)
		}

		appConfig, err := buildProductionAppConfig(resolvedPublicURL)
		if err != nil {
			log.Fatalf("Invalid public URL: %v", err)
		}

		validator := tokens.InitClient(tokens.ClientOptions{
			VerificationKey: pubKey,
			IssuerDomain:    consentIssuer,
			ValidAudience:   appConfig.audience,
		})
		authClient := client.Init(validator, normalizedConsentURL)

		loginURL := buildAuthorizeURL(normalizedConsentURL, resolvedIntegrationName)
		logoutURL := "/auth/logout"
		manifest := client.IntegrationManifest{
			Name:           resolvedIntegrationName,
			Display:        "Compass",
			Audience:       appConfig.audience,
			Redirect:       appConfig.callbackURL,
			Homepage:       appConfig.homepage,
			Logo:           appConfig.logoURL,
			ConsentIssuer:  consentIssuer,
			ConsentBaseURL: normalizedConsentURL,
		}

		authConfig = web.AuthConfig{
			Verifier:       authClient,
			ProfileFetcher: authClient,
			LoginURL:       loginURL,
			LogoutURL:      logoutURL,
			Routes: map[string]http.HandlerFunc{
				"/auth/callback":               authClient.HandleAuthorizationCode(),
				"/auth/logout":                 authClient.HandleLogout(),
				client.IntegrationManifestPath: client.HandleIntegrationManifest(manifest),
			},
		}
	}

	opts := web.ServerOptions{Auth: authConfig}
	srv, err := web.NewServer(svc, opts)
	if err != nil {
		log.Fatalf("Failed to initialize server: %v", err)
	}

	// Start Server
	if *devMode {
		log.Println("Starting server in DEV mode on :8080...")
		log.Println("  → Visit /dev/login to authenticate as 'alice'")
	} else {
		log.Println("Starting server in PRODUCTION mode on :8080...")
		log.Printf("  → Consent integration manifest: %s", client.IntegrationManifestPath)
	}
	if err := http.ListenAndServe(":8080", srv); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

type productionAppConfig struct {
	audience    string
	callbackURL string
	homepage    string
	logoURL     string
}

func normalizeConsentURL(
	raw string,
) (
	baseURL string,
	issuerDomain string,
	err error,
) {
	parsed, err := parseAbsoluteURL(raw)
	if err != nil {
		return "", "", err
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return "", "", fmt.Errorf("path is not allowed")
	}
	return (&url.URL{Scheme: parsed.Scheme, Host: parsed.Host}).String(), parsed.Host, nil
}

func buildProductionAppConfig(
	rawPublicURL string,
) (
	productionAppConfig,
	error,
) {
	parsed, err := parseAbsoluteURL(rawPublicURL)
	if err != nil {
		return productionAppConfig{}, err
	}

	basePath := strings.TrimRight(parsed.Path, "/")
	homepage := (&url.URL{Scheme: parsed.Scheme, Host: parsed.Host, Path: basePath}).String()
	callbackURL := (&url.URL{Scheme: parsed.Scheme, Host: parsed.Host, Path: basePath + "/auth/callback"}).String()
	logoURL := (&url.URL{Scheme: parsed.Scheme, Host: parsed.Host, Path: basePath + "/static/consent-logo.svg"}).String()

	return productionAppConfig{
		audience:    parsed.Host,
		callbackURL: callbackURL,
		homepage:    homepage,
		logoURL:     logoURL,
	}, nil
}

func parseAbsoluteURL(
	raw string,
) (
	*url.URL,
	error,
) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed == nil {
		return nil, fmt.Errorf("expected absolute URL with scheme and host")
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("expected absolute URL with scheme and host")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("query and fragment are not allowed")
	}
	return parsed, nil
}

func buildAuthorizeURL(
	consentURL string,
	integrationName string,
) string {
	authorizeURL, err := url.Parse(strings.TrimRight(consentURL, "/") + "/authorize")
	if err != nil {
		return "/"
	}
	query := authorizeURL.Query()
	query.Set("integration", integrationName)
	for _, scope := range consentScopes {
		query.Add("scope", scope)
	}
	authorizeURL.RawQuery = query.Encode()
	return authorizeURL.String()
}

// parsePublicKey parses a PEM-encoded ECDSA public key.
func parsePublicKey(
	pemData string,
) (
	*ecdsa.PublicKey,
	error,
) {
	block, _ := pem.Decode([]byte(pemData))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	ecdsaPub, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an ECDSA public key")
	}

	return ecdsaPub, nil
}

// getOrGenerateDevKey attempts to load a private key from the given filename.
// If the file does not exist, it generates a new key and saves it.
func getOrGenerateDevKey(
	filename string,
) (
	*ecdsa.PrivateKey,
	error,
) {
	// Try to read existing key
	data, err := os.ReadFile(filename)
	if err == nil {
		block, _ := pem.Decode(data)
		if block == nil {
			return nil, fmt.Errorf("failed to decode PEM block from %s", filename)
		}
		key, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse EC private key: %w", err)
		}
		log.Printf("Loaded existing dev key from %s", filename)
		return key, nil
	}

	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}

	// Generate new key
	log.Printf("Generating new dev key to %s...", filename)
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}

	// Save key
	bytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal key: %w", err)
	}

	pemBlock := &pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: bytes,
	}

	f, err := os.Create(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create key file: %w", err)
	}
	defer f.Close()

	if err := pem.Encode(f, pemBlock); err != nil {
		return nil, fmt.Errorf("failed to write PEM block: %w", err)
	}

	return key, nil
}
