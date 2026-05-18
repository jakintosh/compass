package app

import (
	"errors"
	"log"
	"net/http"
	"net/url"
)

type accountSetupFailureKind int

const (
	accountSetupFailureTransient accountSetupFailureKind = iota
	accountSetupFailureUserAction
)

type accountSetupError struct {
	kind    accountSetupFailureKind
	message string
	err     error
}

func (e *accountSetupError) Error() string {
	if e.err != nil {
		return e.err.Error()
	}
	return e.message
}

func (e *accountSetupError) Unwrap() error {
	return e.err
}

func transientAccountSetupError(
	message string,
	err error,
) error {
	return &accountSetupError{kind: accountSetupFailureTransient, message: message, err: err}
}

func userActionAccountSetupError(
	message string,
	err error,
) error {
	return &accountSetupError{kind: accountSetupFailureUserAction, message: message, err: err}
}

func normalizeAccountSetupError(
	err error,
) *accountSetupError {
	var setupErr *accountSetupError
	if errors.As(err, &setupErr) {
		return setupErr
	}
	return &accountSetupError{
		kind:    accountSetupFailureTransient,
		message: "Compass could not finish setting up your account. This is probably temporary; try signing in again in a few minutes.",
		err:     err,
	}
}

func (s *App) renderAccountSetupFailure(
	w http.ResponseWriter,
	r *http.Request,
	auth AuthContext,
	err error,
) {
	setupErr := normalizeAccountSetupError(err)
	status := http.StatusServiceUnavailable
	kicker := "Account setup paused"
	if setupErr.kind == accountSetupFailureUserAction {
		status = http.StatusConflict
		kicker = "Action needed"
	}

	w.WriteHeader(status)
	if renderErr := s.renderer.RenderMessagePage(w, auth, MessageView{
		Kicker:       kicker,
		Title:        "We could not finish setting up your Compass account.",
		Body:         setupErr.message,
		PrimaryURL:   s.loginURL(r),
		PrimaryLabel: "Try signing in again",
	}); renderErr != nil {
		log.Printf("failed to render account setup failure page: %v", renderErr)
	}
}

func (s *App) renderUserNotFound(
	w http.ResponseWriter,
	auth AuthContext,
	handle string,
) {
	primaryURL := "/"
	primaryLabel := "Go home"
	secondaryURL := ""
	secondaryLabel := ""
	if auth.Handle != "" {
		primaryURL = "/" + url.PathEscape(auth.Handle) + "/"
		primaryLabel = "Go to your workspace"
		secondaryURL = "/"
		secondaryLabel = "Home"
	}

	w.WriteHeader(http.StatusNotFound)
	if err := s.renderer.RenderMessagePage(w, auth, MessageView{
		Kicker:         "User not found",
		Title:          "No Compass workspace exists for " + handle + ".",
		Body:           "This link may be mistyped, or the person you are looking for has not created a Compass account yet.",
		PrimaryURL:     primaryURL,
		PrimaryLabel:   primaryLabel,
		SecondaryURL:   secondaryURL,
		SecondaryLabel: secondaryLabel,
	}); err != nil {
		log.Printf("failed to render user not found page: %v", err)
	}
}
