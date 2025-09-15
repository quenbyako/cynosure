package messages

import "errors"

var (
	ErrMessageTooLarge = errors.New("message is too large")
)
