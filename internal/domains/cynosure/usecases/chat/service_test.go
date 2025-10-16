package chat_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/k0kubun/pp/v3"
	"github.com/stretchr/testify/require"

	"github.com/quenbyako/cynosure/internal/adapters/file"

	"github.com/quenbyako/cynosure/internal/adapters/gemini"
	"github.com/quenbyako/cynosure/internal/adapters/oauth"
	primitive "github.com/quenbyako/cynosure/internal/adapters/tool-handler"
	"github.com/quenbyako/cynosure/internal/adapters/zep"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/messages"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/tools"

	. "github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/chat"
)

var userID = must(ids.NewUserIDFromString("620ccadd-dcb4-4007-a316-6fed71487cfd"))

var auth = oauth.New([]string{"mcp.read", "mcp.write"})
var storage = file.New("/Users/rcooper/repositories/cynosure/storage_test.yaml")
var toolHandler = primitive.NewHandler(auth, storage, storage)

var zepStorage = must(zep.NewZepStorage(
	zep.WithAPIKey("<REDACTED>"),
))

var geminiModel = must(gemini.NewGeminiModel(
	context.TODO(),
	&gemini.ClientConfig{
		APIKey: "<REDACTED>",
	},
))

var modelID = must(ids.NewModelConfigIDFromString("e0689c78-4fd0-4eca-a907-8e00515bc88d"))

func TestServiceReAct(t *testing.T) {
	chat := New(zepStorage, geminiModel, toolHandler, storage, storage, storage, modelID, nil)

	msgs, err := chat.GenerateResponse(
		t.Context(),
		userID,
		uuid.NewString(),
		must(messages.NewMessageUser("Посмотри список моих задач в Jira")),
		WithToolChoice(tools.ToolChoiceAllowed),
	)
	require.NoError(t, err, "Failed to generate response")

	pp.Println(msgs)
}

func must[T any](t T, err error) T {
	if err != nil {
		panic(err)
	}
	return t
}
