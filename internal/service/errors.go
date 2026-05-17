package service

import (
	"database/sql"
	"errors"
	"strings"
)

var (
	ErrForbidden    = errors.New("forbidden")
	ErrInvalidInput = errors.New("invalid input")
	ErrNotFound     = errors.New("not found")
)

func mapStoreError(
	err error,
) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if strings.Contains(strings.ToLower(err.Error()), "not found") {
		return ErrNotFound
	}
	if strings.Contains(strings.ToLower(err.Error()), "not active") {
		return ErrInvalidInput
	}
	return err
}
