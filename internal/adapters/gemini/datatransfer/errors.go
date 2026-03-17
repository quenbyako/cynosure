// Package datatransfer provides data transfer objects for Gemini adapter.
package datatransfer

import (
	"errors"
	"fmt"
)

var (
	ErrEmptyResponse       = errors.New("received empty response from model")
	ErrMultipleCandidates  = errors.New("multiple candidates are not supported")
	ErrCandidateContentNil = errors.New("candidate content is nil")
	ErrNoParts             = errors.New("candidate content has no parts")
	ErrFunctionCallNoName  = errors.New("function call has no name")
	ErrFileDataUnsupported = errors.New("file data is not yet supported")
	ErrContentUnsupported  = errors.New("content is not supported")
	ErrUnexpectedRole      = errors.New("unexpected role in candidate content")
	ErrOrphanedToolResp    = errors.New("tool response message is orphaned")
	ErrUnsupportedMsgType  = errors.New("unsupported message type")
	ErrUnexpectedPart      = errors.New("unexpected part type")
	ErrNilMsg              = errors.New("message is nil")
)

// UnexpectedPartError returns an error for unexpected part type with details.
func UnexpectedPartError(details string) error {
	return fmt.Errorf("%w: %s", ErrUnexpectedPart, details)
}
