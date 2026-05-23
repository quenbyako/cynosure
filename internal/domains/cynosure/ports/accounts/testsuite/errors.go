package testsuite

import (
	"errors"
)

var (
	errExpectedSuccess          = errors.New("expected success")
	errExpectedNotFound         = errors.New("expected not found error")
	errExpectedInvalidAccountID = errors.New("expected invalid account id error")
	errVerificationFailed       = errors.New("verification failed")
	errNotFoundInState          = errors.New("not found in test state")
	errRandomAccountIDFailed    = errors.New("failed to generate random account id")
	errUnexpectedState          = errors.New("unexpected state")
)
