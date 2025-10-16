package ports

import (
	"context"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/ids"
)

type ModelSettingsStorage interface {
	ModelSettingsStorageRead
	ModelSettingsStorageWrite
}

type ModelSettingsStorageFactory interface {
	ModelSettingsStorage() ModelSettingsStorage
}

func NewModelSettingsStorage(factory ModelSettingsStorageFactory) ModelSettingsStorage {
	return factory.ModelSettingsStorage()
}

type ModelSettingsStorageRead interface {
	ListModels(ctx context.Context, user ids.UserID) ([]*entities.ModelSettings, error)
	GetModel(ctx context.Context, model ids.ModelConfigID) (*entities.ModelSettings, error)
}

type ModelSettingsStorageWrite interface {
	SaveModel(ctx context.Context, model entities.ModelSettingsReadOnly) error
	DeleteModel(ctx context.Context, model ids.ModelConfigID) error
}
