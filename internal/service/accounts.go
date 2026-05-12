package service

import (
	"strings"
	"time"
)

type Account struct {
	ID                 string    `json:"id"`
	ConsentSubject     string    `json:"consent_subject"`
	Handle             string    `json:"handle"`
	ProfileRefreshedAt time.Time `json:"profile_refreshed_at"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type GetAccountByHandleInput struct {
	Handle string
}

func (in *GetAccountByHandleInput) Normalize() {
	in.Handle = strings.TrimSpace(in.Handle)
}

func (in *GetAccountByHandleInput) Validate() error {
	if in.Handle == "" {
		return ErrInvalidInput
	}
	return nil
}

func (s *Service) GetAccountByHandle(
	input GetAccountByHandleInput,
) (
	*Account,
	error,
) {
	input.Normalize()
	if err := input.Validate(); err != nil {
		return nil, err
	}

	account, err := s.store.GetAccountByHandle(input.Handle)
	if err != nil {
		return nil, mapStoreError(err)
	}

	return account, nil
}

type GetAccountBySubjectInput struct {
	Subject string
}

func (in *GetAccountBySubjectInput) Normalize() {
	in.Subject = strings.TrimSpace(in.Subject)
}

func (in *GetAccountBySubjectInput) Validate() error {
	if in.Subject == "" {
		return ErrInvalidInput
	}
	return nil
}

func (s *Service) GetAccountBySubject(
	input GetAccountBySubjectInput,
) (
	*Account,
	error,
) {
	input.Normalize()
	if err := input.Validate(); err != nil {
		return nil, err
	}

	account, err := s.store.GetAccountBySubject(input.Subject)
	if err != nil {
		return nil, mapStoreError(err)
	}

	return account, nil
}

type UpsertAccountInput struct {
	Subject     string
	Handle      string
	RefreshedAt time.Time
}

func (in *UpsertAccountInput) Normalize() {
	in.Subject = strings.TrimSpace(in.Subject)
	in.Handle = strings.TrimSpace(in.Handle)
}

func (in *UpsertAccountInput) Validate() error {
	if in.Subject == "" ||
		in.Handle == "" {
		return ErrInvalidInput
	}
	return nil
}

func (s *Service) UpsertAccount(
	input UpsertAccountInput,
) (
	*Account,
	error,
) {
	input.Normalize()
	if err := input.Validate(); err != nil {
		return nil, err
	}

	account, err := s.store.UpsertAccount(input.Subject, input.Handle, input.RefreshedAt)
	if err != nil {
		return nil, mapStoreError(err)
	}

	return account, nil
}
