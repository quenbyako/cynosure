package ory

import (
	"errors"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/identitymanager"
)

var (
	ErrBaseURLRequired     = errors.New("base url is required")
	ErrAdminKeyRequired    = errors.New("admin key is required")
	ErrClientIDRequired    = errors.New("client id is required")
	ErrAuthURLRequired     = errors.New("auth url is required")
	ErrTokenURLRequired    = errors.New("token url is required")
	ErrRedirectURLRequired = errors.New("redirect url is required")
	ErrScopesRequired      = errors.New("scopes are required")
	ErrIdentityMissing     = errors.New("invalid response from ory: missing identity")
	ErrRedirectMissing     = errors.New("invalid response from ory: missing redirect_to")
	ErrTooManyRedirects    = errors.New("too many redirects")
	ErrRateLimited         = identitymanager.ErrRateLimited
	ErrInternal            = errors.New("internal ory adapter error")
	ErrUnexpectedResponse  = errors.New("unexpected response from ory")
)
