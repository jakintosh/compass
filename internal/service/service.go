package service

import (
	"errors"
	"time"
)

type Options struct {
	Store Store
	Clock func() time.Time
}

type Service struct {
	store Store
	clock func() time.Time
}

func New(
	opts Options,
) (
	*Service,
	error,
) {
	if opts.Store == nil {
		return nil, errors.New("service: store required")
	}
	if opts.Clock == nil {
		return nil, errors.New("service: clock required")
	}

	return &Service{
		store: opts.Store,
		clock: opts.Clock,
	}, nil
}
