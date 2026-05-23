package testsuite

import (
	"errors"
)

var (
	errExpectedSuccess = errors.New("expected success")
	errExpectedError   = errors.New("expected error")
	errInvalidParam    = errors.New("invalid parameter")
	errNoActiveChat    = errors.New("no active chat request to reserve")
	errRetryMismatch   = errors.New("retry time mismatch")
)
