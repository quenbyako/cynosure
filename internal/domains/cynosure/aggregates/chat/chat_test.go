package chat_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/quenbyako/cynosure/internal/adapters/mocks"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/aggregates/chat"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
)

func TestChat_AcceptUserMessage_RAG_Orchestration(t *testing.T) {
	ctx, fixture := context.Background(), newChatFixture(t)

	// Arrange: Define tools and expect RAG to find them twice (on New and on Accept)
	tool1 := fixture.tool("get_weather")
	tool2 := fixture.tool("send_email")
	fixture.expectRAG(map[string][]*entities.Tool{
		"Personal": {tool1},
		"Work":     {tool2},
	})

	// Act: Add user message which triggers re-indexing
	err := fixture.instance(ctx).AcceptUserMessage(ctx, fixture.msg("What is the weather?"))

	// Assert: Check toolbox consistency and account enrichment
	require.NoError(t, err)
	fixture.assertToolbox(tool1, tool2)
}

// --- Fixture ---

type chatFixture struct {
	t              *testing.T
	user           ids.UserID
	server         ids.ServerID
	threadID       ids.ThreadID
	indexer        *mocks.MockToolSemanticIndex
	toolStorage    *mocks.MockToolStorage
	accountStorage *mocks.MockAccountStorage
	threadStorage  *mocks.MockThreadStorage
	agentStorage   *mocks.MockAgentStorage

	_chat *chat.Chat
}

func newChatFixture(t *testing.T) *chatFixture {
	t.Helper()

	user := ids.RandomUserID()
	tid, _ := ids.NewThreadID(user, "test-thread-id")

	return &chatFixture{
		t:              t,
		user:           user,
		server:         ids.RandomServerID(),
		threadID:       tid,
		indexer:        new(mocks.MockToolSemanticIndex),
		toolStorage:    new(mocks.MockToolStorage),
		accountStorage: new(mocks.MockAccountStorage),
		threadStorage:  new(mocks.MockThreadStorage),
		agentStorage:   new(mocks.MockAgentStorage),
	}
}

func (f *chatFixture) tool(name string) *entities.Tool {
	accID, err := ids.RandomAccountID(f.user, f.server)
	require.NoError(f.t, err)

	tID, err := ids.NewToolID(accID, uuid.New())
	require.NoError(f.t, err)

	schema := json.RawMessage(`{"type":"object","properties":{}}`)
	tool, err := entities.NewTool(tID, "test-account", name, "desc", schema, schema)
	require.NoError(f.t, err)

	return tool
}

func (f *chatFixture) expectRAG(tools map[string][]*entities.Tool) {
	allTools := make([]*entities.Tool, 0)
	for _, tools := range tools {
		allTools = append(allTools, tools...)
	}

	emb := [1536]float32{0.1}
	f.indexer.EXPECT().BuildToolEmbedding(mock.Anything, mock.Anything).Return(emb, nil).Twice()
	f.toolStorage.EXPECT().LookupTools(mock.Anything, f.user, emb, 10).Return(allTools, nil).Twice()

	var accounts []*entities.Account

	for accountSlug, tools := range tools {
		for _, t := range tools {
			acc, err := entities.NewAccount(t.ID().Account(), accountSlug, accountSlug+" account")
			require.NoError(f.t, err)

			accounts = append(accounts, acc)
		}
	}

	f.accountStorage.EXPECT().
		GetAccountsBatch(mock.Anything, mock.Anything).
		Return(accounts, nil).
		Twice()
}

func (f *chatFixture) instance(ctx context.Context) *chat.Chat {
	if f._chat != nil {
		return f._chat
	}

	msg1, err := messages.NewMessageUser("init")
	require.NoError(f.t, err)

	thread, err := entities.NewThread(f.threadID, []messages.Message{msg1})
	require.NoError(f.t, err)

	f.threadStorage.EXPECT().GetThread(mock.Anything, f.threadID).Return(thread, nil)
	f.threadStorage.EXPECT().UpdateThread(mock.Anything, mock.Anything).Return(nil)

	chatAggregate, err := chat.New(
		ctx, f.threadStorage, f.indexer, f.toolStorage,
		f.accountStorage, f.agentStorage, f.threadID,
	)
	require.NoError(f.t, err)
	f._chat = chatAggregate

	return chatAggregate
}

func (f *chatFixture) msg(content string) messages.MessageUser {
	m, err := messages.NewMessageUser(content)
	require.NoError(f.t, err)

	return m
}

func (f *chatFixture) assertToolbox(expected ...*entities.Tool) {
	toolbox := f._chat.RelevantTools()
	require.Len(f.t, toolbox.Tools(), len(expected))

	for _, exp := range expected {
		info, ok := toolbox.Tools()[exp.Name()]
		require.True(f.t, ok)

		found := false

		for _, acc := range info.EncodedTools() {
			if strings.Contains(strings.ToLower(acc.Desc), strings.ToLower(acc.Name)) {
				found = true
			}
		}

		require.True(f.t, found)
	}
}
