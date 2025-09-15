package ports

import (
	"context"

	"tg-helper/internal/domains/components/ids"
	"tg-helper/internal/domains/entities"
)

type StorageRepository interface {
	CreateThread(ctx context.Context, thread entities.ChatHistoryReadOnly) error
	GetThread(ctx context.Context, user ids.UserID, threadID string) (*entities.ChatHistory, error)
	SaveThread(ctx context.Context, thread entities.ChatHistoryReadOnly) error
}
