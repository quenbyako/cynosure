package botapi_test

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/quenbyako/cynosure/contrib/tg-openapi/gen/go/botapi"
)

func TestMakeSecretToken(t *testing.T) {
	t.Run("length and characters", func(t *testing.T) {
		token, err := MakeSecretToken()
		require.NoError(t, err)

		// Check for actual content beyond zeros
		var nonZero bool

		for _, b := range token {
			if b != 0 {
				nonZero = true
				break
			}
		}

		assert.True(t, nonZero, "token should not be all zeros")

		// Verify allowed characters for Telegram (A-Z, a-z, 0-9, _, -)
		// which matches RawURLEncoding
		validChars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_"
		for _, b := range token {
			assert.Contains(t, validChars, string(b), "token contains invalid character: %c", b)
		}

		// Try to decode it back to ensure it's valid base64 URL (raw)
		decoded := make([]byte, base64.RawURLEncoding.DecodedLen(len(token)))
		n, err := base64.RawURLEncoding.Decode(decoded, []byte(token))
		require.NoError(t, err, "token must be valid RawURLEncoding: %s", token)
		assert.Positive(t, n)
	})

	t.Run("uniqueness", func(t *testing.T) {
		token1, err := MakeSecretToken()
		require.NoError(t, err)
		token2, err := MakeSecretToken()
		require.NoError(t, err)

		assert.NotEqual(t, token1, token2, "tokens must be unique")
	})
}
