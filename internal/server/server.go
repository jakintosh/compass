package server

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"git.sr.ht/~jakintosh/command-go/pkg/wire"
	"git.sr.ht/~jakintosh/compass/internal/app"
	"git.sr.ht/~jakintosh/compass/internal/database"
	"git.sr.ht/~jakintosh/compass/internal/service"
	"git.sr.ht/~jakintosh/consent/pkg/client"
	"git.sr.ht/~jakintosh/consent/pkg/testing"
	"git.sr.ht/~jakintosh/consent/pkg/tokens"
)

type Options struct {
	Addr              string
	DataDir           string
	Dev               bool
	ConsentURL        string
	ConsentPubkeyFile string
	IntegrationName   string
	PublicURL         string
}

const (
	databaseFilename = "compass.db"
)

type productionAppConfig struct {
	audience    string
	callbackURL string
	homepage    string
	logoURL     string
}

type authConfig struct {
	app.AuthConfig
	Routes map[string]http.HandlerFunc
}

var consentScopes = []string{"identity", "profile"}

func Serve(
	opts Options,
) error {
	handler, cleanup, err := BuildHandler(opts)
	if err != nil {
		return err
	}
	defer cleanup()

	if opts.Dev {
		log.Printf("Starting server in DEV mode on %s...", opts.Addr)
		log.Println("  -> Visit /dev/login to authenticate as 'alice'")
	} else {
		log.Printf("Starting server in PRODUCTION mode on %s...", opts.Addr)
		log.Printf("  -> Consent integration manifest: %s", client.IntegrationManifestPath)
	}

	if err := http.ListenAndServe(opts.Addr, handler); err != nil {
		return fmt.Errorf("server failed: %w", err)
	}
	return nil
}

func BuildHandler(
	opts Options,
) (
	http.Handler,
	func(),
	error,
) {
	dataDir := opts.DataDir
	if dataDir == "" {
		dataDir = "."
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	dbOpts := database.Options{
		Path: filepath.Join(dataDir, databaseFilename),
		WAL:  true,
	}
	db, err := database.Open(dbOpts)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	cleanup := func() {
		if err := db.Close(); err != nil {
			log.Printf("failed to close database: %v", err)
		}
	}

	svcOpts := service.Options{
		Store: db,
		Clock: time.Now,
	}
	svc, err := service.New(svcOpts)
	if err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("failed to initialize service: %w", err)
	}

	authConfig, err := buildAuthConfig(opts)
	if err != nil {
		cleanup()
		return nil, nil, err
	}

	appOpts := app.Options{
		Service: svc,
		Auth:    authConfig.AuthConfig,
	}
	renderedApp, err := app.New(appOpts)
	if err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("failed to initialize app: %w", err)
	}

	root := http.NewServeMux()
	for path, handler := range authConfig.Routes {
		root.HandleFunc(path, handler)
	}
	wire.Subrouter(root, "/", renderedApp.Handler())

	return root, cleanup, nil
}

func buildAuthConfig(
	opts Options,
) (
	authConfig,
	error,
) {
	if opts.Dev {
		return buildDevAuthConfig(opts)
	} else {
		return buildProductionAuthConfig(opts)
	}
}

func buildDevAuthConfig(
	opts Options,
) (
	authConfig,
	error,
) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return authConfig{}, fmt.Errorf("failed to generate dev key: %w", err)
	}

	env := testing.NewTestEnvWithKey(key, "localhost", "compass-dev")
	env.Scopes = consentScopes
	tv := testing.NewTestVerifierWithEnv(env)

	authConfig := authConfig{
		AuthConfig: app.AuthConfig{
			Verifier:  tv,
			LoginURL:  "/dev/login",
			LogoutURL: "/dev/logout",
		},
		Routes: map[string]http.HandlerFunc{
			"/dev/login":  tv.HandleDevLogin(),
			"/dev/logout": tv.HandleDevLogout(),
		},
	}
	return authConfig, nil
}

func buildProductionAuthConfig(
	opts Options,
) (
	authConfig,
	error,
) {
	if opts.ConsentURL == "" ||
		opts.ConsentPubkeyFile == "" ||
		opts.PublicURL == "" {
		return authConfig{}, fmt.Errorf("production mode requires --consent-url, --consent-pubkey-file, and --public-url")
	}

	pubKey, err := loadConsentPublicKey(opts)
	if err != nil {
		return authConfig{}, fmt.Errorf("failed to parse consent public key: %w", err)
	}

	baseUrl, issuer, err := processConsentURL(opts.ConsentURL)
	if err != nil {
		return authConfig{}, fmt.Errorf("invalid consent URL: %w", err)
	}

	appConfig, err := buildProductionAppConfig(opts.PublicURL)
	if err != nil {
		return authConfig{}, fmt.Errorf("invalid public URL: %w", err)
	}

	validatorOpts := tokens.ClientOptions{
		VerificationKey: pubKey,
		IssuerDomain:    issuer,
		ValidAudience:   appConfig.audience,
	}
	validator := tokens.InitClient(validatorOpts)
	authClient := client.Init(validator, baseUrl)

	manifest := client.IntegrationManifest{
		Name:           opts.IntegrationName,
		Display:        "Compass",
		Audience:       appConfig.audience,
		Redirect:       appConfig.callbackURL,
		Homepage:       appConfig.homepage,
		Logo:           appConfig.logoURL,
		ConsentIssuer:  issuer,
		ConsentBaseURL: baseUrl,
	}

	authConfig := authConfig{
		AuthConfig: app.AuthConfig{
			Verifier:       authClient,
			ProfileFetcher: authClient,
			LoginURL:       buildAuthorizeURL(baseUrl, opts.IntegrationName),
			LogoutURL:      "/auth/logout",
		},
		Routes: map[string]http.HandlerFunc{
			"/auth/callback":               authClient.HandleAuthorizationCode(),
			"/auth/logout":                 authClient.HandleLogout(),
			client.IntegrationManifestPath: client.HandleIntegrationManifest(manifest),
		},
	}
	return authConfig, nil
}

func buildProductionAppConfig(
	rawPublicURL string,
) (
	productionAppConfig,
	error,
) {
	homepage, audience, err := processPublicURL(rawPublicURL)
	if err != nil {
		return productionAppConfig{}, err
	}

	callbackURL, err := url.JoinPath(homepage, "auth", "callback")
	if err != nil {
		return productionAppConfig{}, err
	}

	logoURL, err := url.JoinPath(homepage, "static", "consent-logo.svg")
	if err != nil {
		return productionAppConfig{}, err
	}

	appConfig := productionAppConfig{
		audience:    audience,
		callbackURL: callbackURL,
		homepage:    homepage,
		logoURL:     logoURL,
	}
	return appConfig, nil
}

func processConsentURL(
	raw string,
) (
	baseURL string,
	issuerDomain string,
	err error,
) {
	raw = strings.TrimSpace(raw)
	parsed, err := url.Parse(raw)
	if err != nil || parsed == nil {
		return "", "", fmt.Errorf("expected absolute URL with scheme and host")
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", "", fmt.Errorf("expected absolute URL with scheme and host")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", "", fmt.Errorf("query and fragment are not allowed")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return "", "", fmt.Errorf("path is not allowed")
	}

	parsed.Path = ""
	baseURL = parsed.String()
	return baseURL, parsed.Host, nil
}

func processPublicURL(
	raw string,
) (
	homepage string,
	audience string,
	err error,
) {
	raw = strings.TrimSpace(raw)
	parsed, err := url.Parse(raw)
	if err != nil || parsed == nil {
		return "", "", fmt.Errorf("expected absolute URL with scheme and host")
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", "", fmt.Errorf("expected absolute URL with scheme and host")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", "", fmt.Errorf("query and fragment are not allowed")
	}

	parsed.Path = strings.TrimRight(parsed.Path, "/")
	homepage = parsed.String()
	return homepage, parsed.Host, nil
}

func buildAuthorizeURL(
	consentURL string,
	integrationName string,
) string {
	consentURL = strings.TrimRight(consentURL, "/")
	consentURL = consentURL + "/authorize"
	authorizeURL, err := url.Parse(consentURL)
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

func loadConsentPublicKey(
	opts Options,
) (
	*ecdsa.PublicKey,
	error,
) {
	data, err := os.ReadFile(opts.ConsentPubkeyFile)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", opts.ConsentPubkeyFile, err)
	}
	return parseDERPublicKey(data)
}

func parseDERPublicKey(
	derData []byte,
) (
	*ecdsa.PublicKey,
	error,
) {
	pub, err := x509.ParsePKIXPublicKey(derData)
	if err != nil {
		return nil, err
	}

	ecdsaPub, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an ECDSA public key")
	}

	return ecdsaPub, nil
}
