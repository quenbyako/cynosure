package chat_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/quenbyako/cynosure/internal/adapters/mocks"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/aggregates/chat"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
)

func TestChat_AcceptUserMessage_RAG_Orchestration(t *testing.T) {
	ctx, f := context.Background(), newChatFixture(t)

	// Arrange: Define tools and expect RAG to find them twice (on New and on Accept)
	t1 := f.tool("get_weather", "Personal")
	t2 := f.tool("send_email", "Work")
	f.expectRAG(t1, t2)

	// Act: Add user message which triggers re-indexing
	err := f.instance(ctx).AcceptUserMessage(ctx, f.msg("What is the weather?"))

	// Assert: Check toolbox consistency and account enrichment
	assert.NoError(t, err)
	f.assertToolbox(t1, t2)
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

func (f *chatFixture) tool(name, accSlug string) *entities.Tool {
	accID, err := ids.RandomAccountID(f.user, f.server, ids.WithSlug(accSlug))
	require.NoError(f.t, err)

	tID, err := ids.NewToolID(accID, uuid.New(), ids.WithSlug(name))
	require.NoError(f.t, err)

	schema := json.RawMessage(`{"type":"object","properties":{}}`)
	tool, err := entities.NewTool(tID, name, "desc", schema, schema)
	require.NoError(f.t, err)
	return tool
}

func (f *chatFixture) expectRAG(tools ...*entities.Tool) {
	emb := [1536]float32{0.1}
	f.indexer.EXPECT().BuildToolEmbedding(mock.Anything, mock.Anything).Return(emb, nil).Twice()
	f.toolStorage.EXPECT().LookupTools(mock.Anything, f.user, emb, 10).Return(tools, nil).Twice()

	var accounts []*entities.Account
	for _, t := range tools {
		acc, err := entities.NewAccount(t.ID().Account(), t.ID().Account().Slug(), t.ID().Account().Slug()+" account")
		require.NoError(f.t, err)
		accounts = append(accounts, acc)
	}
	f.accountStorage.EXPECT().GetAccountsBatch(mock.Anything, mock.Anything).Return(accounts, nil).Twice()
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

	c, err := chat.New(ctx, f.threadStorage, f.indexer, f.toolStorage, f.accountStorage, f.agentStorage, f.threadID)
	require.NoError(f.t, err)
	f._chat = c
	return c
}

func (f *chatFixture) msg(content string) messages.MessageUser {
	m, err := messages.NewMessageUser(content)
	require.NoError(f.t, err)
	return m
}

func (f *chatFixture) assertToolbox(expected ...*entities.Tool) {
	toolbox := f._chat.RelevantTools()
	assert.Len(f.t, toolbox.Tools(), len(expected))
	for _, exp := range expected {
		info, ok := toolbox.Tools()[exp.Name()]
		assert.True(f.t, ok)
		found := false
		for _, desc := range info.EncodedTools() {
			if strings.Contains(strings.ToLower(desc), strings.ToLower(exp.ID().Account().Slug())) {
				found = true
			}
		}
		assert.True(f.t, found)
	}
}
