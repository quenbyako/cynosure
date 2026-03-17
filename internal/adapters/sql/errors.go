package sql

import (
	"github.com/quenbyako/cynosure/internal/adapters/sql/errors"
)

var (
	// ErrEmptyResultSet is returned when no rows are found.
	ErrEmptyResultSet = errors.ErrEmptyResultSet

	// ErrMessageTypeNil is returned when a message has no type.
	ErrMessageTypeNil = errors.ErrMessageTypeNil

	// ErrMessageTypeUnknown is returned when an unknown message type is encountered.
	ErrMessageTypeUnknown = errors.ErrMessageTypeUnknown

	// ErrUserContentNil is returned when a user message has no content.
	ErrUserContentNil = errors.ErrUserContentNil

	// ErrToolRequestFieldsMissing is returned when tool request fields are missing.
	ErrToolRequestFieldsMissing = errors.ErrToolRequestFieldsMissing

	// ErrToolResultContentMissing is returned when tool result content is missing.
	ErrToolResultContentMissing = errors.ErrToolResultContentMissing

	// ErrToolResultCallIDMissing is returned when tool result call ID is missing.
	ErrToolResultCallIDMissing = errors.ErrToolResultCallIDMissing

	// ErrToolResultOriginMissing is returned when the origin request for a tool result is missing.
	ErrToolResultOriginMissing = errors.ErrToolResultOriginMissing

	// ErrThreadNoMessages is returned when a thread has no messages in database.
	ErrThreadNoMessages = errors.ErrThreadNoMessages

	// ErrOAuthAuthURLEmpty is returned when the OAuth auth URL is empty.
	ErrOAuthAuthURLEmpty = errors.ErrOAuthAuthURLEmpty

	// ErrOAuthTokenURLEmpty is returned when the OAuth token URL is empty.
	ErrOAuthTokenURLEmpty = errors.ErrOAuthTokenURLEmpty

	// ErrConcurrentModification is returned when a concurrent modification is detected.
	ErrConcurrentModification = errors.ErrConcurrentModification
)
