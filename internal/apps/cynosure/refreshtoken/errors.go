package refreshtoken

import (
	"errors"
)

var ErrRefreshTokenNotSet = errors.New("token expired and refresh token is not set")
