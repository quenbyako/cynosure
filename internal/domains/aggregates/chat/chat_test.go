package chat_test

import (
	"testing"

	"github.com/k0kubun/pp/v3"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"tg-helper/internal/adapters/mocks"
	"tg-helper/internal/domains/components/ids"
	"tg-helper/internal/domains/components/messages"
	"tg-helper/internal/domains/components/tools"
	"tg-helper/internal/domains/entities"

	. "tg-helper/internal/domains/aggregates/chat"
)

func TestChatAggregate(t *testing.T) {
	user := must(ids.NewUserIDFromString("f997c386-9be3-4eab-9126-db95828a942d"))
	server := must(ids.NewServerIDFromString("f997c386-9be3-4eab-9126-db95828a942d"))
	account1 := must(ids.NewAccountIDFromString(user, server, "f997c386-9be3-4eab-1111-db95828a942d"))
	account2 := must(ids.NewAccountIDFromString(user, server, "f997c386-9be3-4eab-2222-db95828a942d"))
	threadID := "thread-1"
	messages := []messages.Message{
		must(messages.NewMessageUser("Hello")),
		must(messages.NewMessageUser("World")),
	}

	singleTool := must(tools.NewToolInfo("tool1", "description1", []byte(`{"type":"object","properties":{"a":{"type":"string"},"b":{"type":"string"}}}`), []byte(`{"type":"string"}`)))
	allAccounts := []*entities.Account{
		must(entities.NewAccount(account1, "account-1", "acc_description-1", []tools.ToolInfo{singleTool})),
		must(entities.NewAccount(account2, "account-2", "acc_description-2", []tools.ToolInfo{singleTool})),
	}

	relevantTools := map[ids.AccountID][]tools.ToolInfo{
		account1: {singleTool},
		account2: {singleTool},
	}

	threads := mocks.NewMockStorageRepository(t)
	threads.EXPECT().GetThread(mock.Anything, user, threadID).Return(must(entities.NewChatHistory(user, threadID, messages)), nil).Once()

	accounts := mocks.NewMockAccountStorage(t)
	accounts.EXPECT().GetAccountsBatch(mock.Anything, []ids.AccountID{account1, account2}).Return(allAccounts, nil).Once()

	tools := mocks.NewMockToolManager(t)
	tools.EXPECT().RetrieveRelevantTools(mock.Anything, user, messages).Return(relevantTools, nil).Once()

	chat, err := New(t.Context(), threads, tools, accounts, user, threadID)
	require.NoError(t, err)

	for _, tool := range chat.RelevantTools() {
		pp.Println(string(tool.Params()))
	}
}

func must[T any](t T, err error) T {
	if err != nil {
		panic(err)
	}
	return t
}
