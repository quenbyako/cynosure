package botapi

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

const (
	secretTokenLen = 128

	_ uint = 256 - secretTokenLen // max token len
	_ uint = secretTokenLen - 1   // min token len
)

// MakeSecretToken creates a cryptographically secure secret token for Telegram webhooks.
// It uses optimized base64 encoding to map random bytes to secure characters.
func MakeSecretToken() (string, error) {
	// We need 6 bits of entropy per character.
	// Total bytes needed = ceil(secretTokenLen * 6 / 8).
	const rawLen = (secretTokenLen*6 + 7) / 8 //nolint:mnd // bits to bytes math

	var raw [rawLen]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("reading random bytes: %w", err)
	}

	// Base64 encoding maps 3 bytes to 4 characters (6 bits each).
	const encodedMaxLen = (rawLen + 2) / 3 * 4 //nolint:mnd // base64 math

	var encoded [encodedMaxLen]byte

	base64.RawURLEncoding.Encode(encoded[:], raw[:])

	var res [secretTokenLen]byte
	copy(res[:], encoded[:secretTokenLen])

	return string(res[:]), nil
}
