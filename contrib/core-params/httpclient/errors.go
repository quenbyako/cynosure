package httpclient

import (
	"errors"
)

var errUnonfigured = errors.New("http client not configured")
