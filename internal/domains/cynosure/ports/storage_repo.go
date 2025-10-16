package ports

import (
	"context"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/ids"
)

type StorageRepository interface {
	CreateThread(ctx context.Context, thread entities.ChatHistoryReadOnly) error
	GetThread(ctx context.Context, user ids.UserID, threadID string) (*entities.ChatHistory, error)
	SaveThread(ctx context.Context, thread entities.ChatHistoryReadOnly) error
}

type StorageRepositoryFactory interface {
	StorageRepository() StorageRepository
}

func NewStorageRepository(factory StorageRepositoryFactory) StorageRepository {
	return factory.StorageRepository()
}
