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
	"os"

	"git.sr.ht/~jakintosh/consent/pkg/client"
	contesting "git.sr.ht/~jakintosh/consent/pkg/testing"
	"git.sr.ht/~jakintosh/consent/pkg/tokens"
	"git.sr.ht/~jakintosh/compass/internal/store"
	"git.sr.ht/~jakintosh/compass/internal/web"
)

// getConfigValue returns the CLI flag value if set, otherwise falls back to env var.
func getConfigValue(flagVal, envKey string) string {
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
	appID := flag.String("app-id", "", "Application identifier/audience (env: APP_ID)")
	flag.Parse()

	// Resolve config with CLI > env fallback
	resolvedConsentURL := getConfigValue(*consentURL, "CONSENT_URL")
	resolvedConsentPubkey := getConfigValue(*consentPubkey, "CONSENT_PUBKEY")
	resolvedAppID := getConfigValue(*appID, "APP_ID")

	// Initialize Store
	store, err := store.NewSQLiteStore("compass.db", true)
	if err != nil {
		log.Fatalf("Failed to initialize store: %v", err)
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
		if resolvedConsentURL == "" || resolvedConsentPubkey == "" || resolvedAppID == "" {
			log.Fatalf("Production mode requires --consent-url, --consent-pubkey, and --app-id (or use --dev for development)")
		}

		pubKey, err := parsePublicKey(resolvedConsentPubkey)
		if err != nil {
			log.Fatalf("Failed to parse consent public key: %v", err)
		}

		validator := tokens.InitClient(pubKey, resolvedConsentURL, resolvedAppID)
		authClient := client.Init(validator, resolvedConsentURL)

		// TODO: Construct proper authorize URL with client_id, redirect_uri params
		loginURL := resolvedConsentURL + "/authorize"
		logoutURL := resolvedConsentURL + "/logout"

		authConfig = web.AuthConfig{
			Verifier:  authClient,
			LoginURL:  loginURL,
			LogoutURL: logoutURL,
			Routes: map[string]http.HandlerFunc{
				"/auth/callback": authClient.HandleAuthorizationCode(),
			},
		}
	}

	opts := web.ServerOptions{Auth: authConfig}
	srv, err := web.NewServer(store, opts)
	if err != nil {
		log.Fatalf("Failed to initialize server: %v", err)
	}

	// Start Server
	if *devMode {
		log.Println("Starting server in DEV mode on :8080...")
		log.Println("  → Visit /dev/login to authenticate as 'alice'")
	} else {
		log.Println("Starting server in PRODUCTION mode on :8080...")
	}
	if err := http.ListenAndServe(":8080", srv); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

// parsePublicKey parses a PEM-encoded ECDSA public key.
func parsePublicKey(pemData string) (*ecdsa.PublicKey, error) {
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
func getOrGenerateDevKey(filename string) (*ecdsa.PrivateKey, error) {
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
