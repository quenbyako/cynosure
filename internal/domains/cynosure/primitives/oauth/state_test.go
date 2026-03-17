package oauth_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"

	. "github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/oauth"
)

func TestState(t *testing.T) {
	userID := ids.RandomUserID()
	serverID := ids.RandomServerID()
	accountID, err := ids.RandomAccountID(userID, serverID)
	require.NoError(t, err, "generating account id")

	state, err := NewState(
		accountID,
		"test_account",
		"some description just to be sure that it's not too huge for token",
		[]byte{16, 32, 64, 128},
		time.Now(),
	)
	require.NoError(t, err, "generating state")

	key := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

	token, err := state.State("", key)
	require.NoError(t, err, "generating state token")

	t.Log("size of token is:", len(token))

	reversed, err := StateFromToken(token, key)
	require.NoError(t, err, "reversing state from token")
	require.Equal(t, state, reversed)
}
