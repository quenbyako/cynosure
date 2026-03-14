package mcp

import (
	"errors"
)

var (
	ErrUnauthorized        = errors.New("unauthorized")
	ErrUnimplemented       = errors.New("unimplemented")
	ErrIssuerNotAllowed    = errors.New("issuer is not allowed")
	ErrInvalidIssuerDomain = errors.New("invalid issuer domain")
	ErrMissingSubClaim     = errors.New("missing sub claim")
	ErrJwksFetchStatus     = errors.New("failed to fetch jwks")

	// validation errors

	ErrNameRequired              = errors.New("name is required")
	ErrDescriptionRequired       = errors.New("description is required")
	ErrUnexpectedAccountResponse = errors.New("unexpected account response")
)
