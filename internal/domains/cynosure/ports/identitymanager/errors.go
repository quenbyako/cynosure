package identitymanager

import (
	"errors"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrAlreadyExists = errors.New("already exists")
	ErrRateLimited   = errors.New("rate limited by identity provider")
)
