package app

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"git.sr.ht/~jakintosh/compass/internal/database"
	"git.sr.ht/~jakintosh/compass/internal/service"
	"git.sr.ht/~jakintosh/consent/pkg/client"
	consenttesting "git.sr.ht/~jakintosh/consent/pkg/testing"
)

type fakeProfileFetcher struct {
	userInfo *client.UserInfo
	err      error
}

func (f fakeProfileFetcher) FetchUserInfo(
	accessToken string,
) (
	*client.UserInfo,
	error,
) {
	return f.userInfo, f.err
}

func newAccountTestApp(
	t *testing.T,
	profileFetcher fakeProfileFetcher,
) (
	*App,
	*database.DB,
	*consenttesting.TestEnv,
) {
	t.Helper()

	db, err := database.Open(database.Options{Path: filepath.Join(t.TempDir(), "compass.db")})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close database: %v", err)
		}
	})

	svc, err := service.New(service.Options{Store: db, Clock: time.Now})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	env := consenttesting.NewTestEnv("localhost", "compass-dev")
	app, err := New(Options{
		Service: svc,
		Auth: AuthConfig{
			Verifier:       consenttesting.NewTestVerifierWithEnv(env),
			ProfileFetcher: profileFetcher,
			LoginURL:       "/login",
			LogoutURL:      "/logout",
		},
	})
	if err != nil {
		t.Fatalf("new app: %v", err)
	}

	return app, db, env
}

func TestAccountForTokenCreatesAccountFromConsentProfile(t *testing.T) {
	app, db, env := newAccountTestApp(t, fakeProfileFetcher{
		userInfo: &client.UserInfo{
			Sub: "subject-alice",
			Profile: &client.UserInfoProfile{
				Handle: " alice ",
			},
		},
	})

	token, err := env.IssueAccessToken("subject-alice", time.Hour)
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}

	account, err := app.accountForToken(token)
	if err != nil {
		t.Fatalf("accountForToken: %v", err)
	}
	if account.ConsentSubject != "subject-alice" {
		t.Fatalf("subject = %q, want subject-alice", account.ConsentSubject)
	}
	if account.Handle != "alice" {
		t.Fatalf("handle = %q, want alice", account.Handle)
	}

	bySubject, err := db.GetAccountBySubject("subject-alice")
	if err != nil {
		t.Fatalf("get account by subject: %v", err)
	}
	if bySubject.ID != account.ID {
		t.Fatalf("stored account ID = %q, want %q", bySubject.ID, account.ID)
	}
}

func TestAccountForTokenRejectsMismatchedConsentProfileSubject(t *testing.T) {
	app, db, env := newAccountTestApp(t, fakeProfileFetcher{
		userInfo: &client.UserInfo{
			Sub: "subject-bob",
			Profile: &client.UserInfoProfile{
				Handle: "alice",
			},
		},
	})

	token, err := env.IssueAccessToken("subject-alice", time.Hour)
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}

	if _, err := app.accountForToken(token); !isSetupFailureKind(err, accountSetupFailureUserAction) {
		t.Fatalf("accountForToken error = %v, want user-action setup failure", err)
	}
	if _, err := db.GetAccountBySubject("subject-alice"); err == nil {
		t.Fatal("account was created after mismatched profile subject; want no account")
	}
}

func TestAccountForTokenClassifiesFetchFailureAsTransient(t *testing.T) {
	app, _, env := newAccountTestApp(t, fakeProfileFetcher{err: errors.New("consent unavailable")})

	token, err := env.IssueAccessToken("subject-alice", time.Hour)
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}

	if _, err := app.accountForToken(token); !isSetupFailureKind(err, accountSetupFailureTransient) {
		t.Fatalf("accountForToken error = %v, want transient setup failure", err)
	}
}

func TestAccountForTokenClassifiesMissingProfileAsUserAction(t *testing.T) {
	app, _, env := newAccountTestApp(t, fakeProfileFetcher{
		userInfo: &client.UserInfo{Sub: "subject-alice"},
	})

	token, err := env.IssueAccessToken("subject-alice", time.Hour)
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}

	if _, err := app.accountForToken(token); !isSetupFailureKind(err, accountSetupFailureUserAction) {
		t.Fatalf("accountForToken error = %v, want user-action setup failure", err)
	}
}

func TestAccountForTokenClassifiesBlankHandleAsUserAction(t *testing.T) {
	app, _, env := newAccountTestApp(t, fakeProfileFetcher{
		userInfo: &client.UserInfo{
			Sub:     "subject-alice",
			Profile: &client.UserInfoProfile{Handle: "   "},
		},
	})

	token, err := env.IssueAccessToken("subject-alice", time.Hour)
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}

	if _, err := app.accountForToken(token); !isSetupFailureKind(err, accountSetupFailureUserAction) {
		t.Fatalf("accountForToken error = %v, want user-action setup failure", err)
	}
}

func TestAccountForTokenClassifiesDuplicateHandleAsUserAction(t *testing.T) {
	app, db, env := newAccountTestApp(t, fakeProfileFetcher{
		userInfo: &client.UserInfo{
			Sub:     "subject-alice",
			Profile: &client.UserInfoProfile{Handle: "shared"},
		},
	})
	if _, err := db.UpsertAccount("subject-bob", "shared", time.Now()); err != nil {
		t.Fatalf("seed account: %v", err)
	}

	token, err := env.IssueAccessToken("subject-alice", time.Hour)
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}

	if _, err := app.accountForToken(token); !isSetupFailureKind(err, accountSetupFailureUserAction) {
		t.Fatalf("accountForToken error = %v, want user-action setup failure", err)
	}
}

func TestAccountForTokenUsesCachedAccountWhenRefreshFails(t *testing.T) {
	app, db, env := newAccountTestApp(t, fakeProfileFetcher{err: errors.New("consent unavailable")})
	app.auth.ProfileRefreshInterval = time.Nanosecond
	seeded, err := db.UpsertAccount("subject-alice", "alice", time.Now().Add(-48*time.Hour))
	if err != nil {
		t.Fatalf("seed account: %v", err)
	}

	token, err := env.IssueAccessToken("subject-alice", time.Hour)
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}

	account, err := app.accountForToken(token)
	if err != nil {
		t.Fatalf("accountForToken: %v", err)
	}
	if account.ID != seeded.ID {
		t.Fatalf("account ID = %q, want cached %q", account.ID, seeded.ID)
	}
}

func TestHandleIndexLoggedOutRendersHomepage(t *testing.T) {
	app, _, _ := newAccountTestApp(t, fakeProfileFetcher{})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	app.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Track what is in progress.") || !strings.Contains(body, "Sign in with Consent") {
		t.Fatalf("homepage body missing expected login empty state: %s", body)
	}
}

func TestTenantIndexLoggedOutRendersPublicWorkspace(t *testing.T) {
	app, db, _ := newAccountTestApp(t, fakeProfileFetcher{})
	account, err := db.UpsertAccount("subject-alice", "alice", time.Now())
	if err != nil {
		t.Fatalf("seed account: %v", err)
	}
	if _, err := db.AddCategory(account.ID, "Public Category"); err != nil {
		t.Fatalf("seed category: %v", err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/alice/", nil)

	app.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, "Track what is in progress.") {
		t.Fatalf("tenant page rendered homepage splash: %s", body)
	}
	if !strings.Contains(body, "Public Category") || !strings.Contains(body, `id="categories-list"`) {
		t.Fatalf("tenant page missing public workspace content: %s", body)
	}
}

func TestHandleIndexAuthenticatedRedirectsToAccount(t *testing.T) {
	app, _, env := newAccountTestApp(t, fakeProfileFetcher{
		userInfo: &client.UserInfo{
			Sub:     "subject-alice",
			Profile: &client.UserInfoProfile{Handle: "alice"},
		},
	})
	req, err := env.AuthenticatedRequest(http.MethodGet, "/", "subject-alice")
	if err != nil {
		t.Fatalf("authenticated request: %v", err)
	}
	rec := httptest.NewRecorder()

	app.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	if got := rec.Header().Get("Location"); got != "/alice/" {
		t.Fatalf("Location = %q, want /alice/", got)
	}
}

func TestHandleIndexProvisioningFailureRendersSetupPage(t *testing.T) {
	app, _, env := newAccountTestApp(t, fakeProfileFetcher{err: errors.New("consent unavailable")})
	req, err := env.AuthenticatedRequest(http.MethodGet, "/", "subject-alice")
	if err != nil {
		t.Fatalf("authenticated request: %v", err)
	}
	rec := httptest.NewRecorder()

	app.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "We could not finish setting up your Compass account.") ||
		!strings.Contains(body, "Try signing in again") {
		t.Fatalf("setup failure body missing expected copy: %s", body)
	}
}

func TestUnknownTenantRendersUserNotFoundPage(t *testing.T) {
	app, _, _ := newAccountTestApp(t, fakeProfileFetcher{})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/notfound/", nil)

	app.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "User not found") || !strings.Contains(body, "No Compass workspace exists for notfound.") {
		t.Fatalf("not found body missing expected copy: %s", body)
	}
}

func isSetupFailureKind(
	err error,
	kind accountSetupFailureKind,
) bool {
	var setupErr *accountSetupError
	return errors.As(err, &setupErr) && setupErr.kind == kind
}
