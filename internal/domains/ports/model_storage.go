package ports

import (
	"context"
	"tg-helper/internal/domains/components/ids"
	"tg-helper/internal/domains/entities"
)

type ModelSettingsStorage interface {
	ModelSettingsStorageRead
	ModelSettingsStorageWrite
}

type ModelSettingsStorageRead interface {
	ListModels(ctx context.Context, user ids.UserID) ([]*entities.ModelSettings, error)
	GetModel(ctx context.Context, model ids.ModelConfigID) (*entities.ModelSettings, error)
}

type ModelSettingsStorageWrite interface {
	SaveModel(ctx context.Context, model entities.ModelSettingsReadOnly) error
	DeleteModel(ctx context.Context, model ids.ModelConfigID) error
}
