package zep_test

import (
	// "encoding/json"
	"testing"

	"github.com/k0kubun/pp/v3"
	"github.com/stretchr/testify/require"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/messages"

	. "github.com/quenbyako/cynosure/internal/adapters/zep"
)

func TestMessageFromZepContent(t *testing.T) {
	z, err := NewZepStorage(
		WithAPIKey("z_1dWlkIjoiZThmOTExMmQtNDkyYS00NjY3LWI0ZjEtNmE4NzBiMmRjYmE5In0.Qs6lrOeJUz3rILBL4EGbX9HRdWumrFBnqhkEgNxCp1Iq4LcSmd1H9VD_Bi5zUL66GZR-mgjbGJIkzUqLZySKVg"),
	)

	require.NoError(t, err, "Failed to create Zep storage")

	user := must(ids.NewUserIDFromString("265EC2F5-B1F5-4F44-B700-9C11EDDC9819"))

	thread, err := z.GetThread(t.Context(), user, "test_thread_id")
	require.NoError(t, err, "Failed to get thread from Zep storage")

	thread.AddMessage(must(messages.NewMessageUser("Ага, и что дальше?")))

	pp.Println(thread)

	pp.Println(z.SaveThread(t.Context(), thread))
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
