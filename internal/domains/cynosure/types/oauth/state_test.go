package oauth_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/k0kubun/pp/v3"
	"github.com/stretchr/testify/require"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/ids"
	. "github.com/quenbyako/cynosure/internal/domains/cynosure/types/oauth"
)

func TestState(t *testing.T) {
	state := must(NewState(
		must(ids.RandomAccountID(
			must(ids.NewUserID(must(uuid.NewRandom()))),
			must(ids.NewServerID(must(uuid.NewRandom()))),
		)),
		"test_account",
		"some description just to be sure that it's not too huge for token",
		[]byte{16, 32, 64, 128},
		time.Now(),
	))

	var key = [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

	token := state.State("", key)

	t.Log("size of token is:", len(token))

	reversed, err := StateFromToken(token, key)
	require.NoError(t, err, "reversing state from token")

	pp.Println(reversed)
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}

	return v
}
