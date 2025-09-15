package chat_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/k0kubun/pp/v3"
	"github.com/stretchr/testify/require"

	"tg-helper/internal/adapters/account-storage/file"
	modelStorage "tg-helper/internal/adapters/model-settings/file"
	"tg-helper/internal/adapters/gemini"
	oauth "tg-helper/internal/adapters/oauth/classic"
	srv "tg-helper/internal/adapters/server-storage/file"
	"tg-helper/internal/adapters/zep"
	primitive "tg-helper/internal/adapters/tool-handler"
	"tg-helper/internal/domains/components/ids"
	"tg-helper/internal/domains/components/messages"
	"tg-helper/internal/domains/components/tools"

	. "tg-helper/internal/domains/services/chat"
)

var userID = must(ids.NewUserIDFromString("620ccadd-dcb4-4007-a316-6fed71487cfd"))

var auth = oauth.New([]string{"mcp.read", "mcp.write"})
var models = modelStorage.New("/Users/rcooper/repositories/tg-helper/models.yaml")
var tokens = file.NewFileTokenStorage("/Users/rcooper/repositories/tg-helper/tokens.yaml")
var servers = srv.New("/Users/rcooper/repositories/tg-helper/servers.yaml")
var toolHandler = primitive.NewHandler(auth, servers, tokens)

var zepStorage = must(zep.NewZepStorage(
	zep.WithAPIKey("<REDACTED>"),
))

var geminiModel = must(gemini.NewGeminiModel(
	context.TODO(),
	"gemini-2.5-flash",
	&gemini.ClientConfig{
		APIKey: "<REDACTED>",
	},
))

var modelID = must(ids.NewModelConfigIDFromString("e0689c78-4fd0-4eca-a907-8e00515bc88d"))

func TestServiceReAct(t *testing.T) {
	chat := New(zepStorage, geminiModel, toolHandler, servers, tokens, models, modelID)

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
