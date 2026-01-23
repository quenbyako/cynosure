package sql

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	db "github.com/quenbyako/cynosure/contrib/db/gen/go"

	"github.com/quenbyako/cynosure/internal/adapters/sql/datatransfer"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/ids"
)

// ListModels implements ModelSettingsStorageRead.
// Note: Models now belong to everyone (no user filtering).
func (a *Adapter) ListModels(ctx context.Context, _ ids.UserID) ([]*entities.ModelSettings, error) {
	rows, err := a.q.ListModelSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list model settings: %w", err)
	}

	models := make([]*entities.ModelSettings, len(rows))
	for i, row := range rows {
		model, err := datatransfer.ModelSettingsFromDB(row)
		if err != nil {
			return nil, fmt.Errorf("failed to convert model settings row %d: %w", i, err)
		}
		models[i] = model
	}

	return models, nil
}

// GetModel implements ModelSettingsStorageRead.
func (a *Adapter) GetModel(ctx context.Context, model ids.ModelConfigID) (*entities.ModelSettings, error) {
	row, err := a.q.GetModelSettings(ctx, model.ID())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ports.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get model settings: %w", err)
	}

	return datatransfer.ModelSettingsFromDB(row)
}

// SaveModel implements ModelSettingsStorageWrite.
func (a *Adapter) SaveModel(ctx context.Context, model entities.ModelSettingsReadOnly) error {
	stopWords := model.StopWords()
	if stopWords == nil {
		stopWords = []string{}
	}

	var temp, topP float32
	if t, ok := model.Temperature(); ok {
		temp = t
	}
	if p, ok := model.TopP(); ok {
		topP = p
	}

	if err := a.q.UpsertModelSettings(ctx, db.UpsertModelSettingsParams{
		ID:            model.ID().ID(),
		Model:         model.Model(),
		SystemMessage: model.SystemMessage(),
		Temperature:   temp,
		TopP:          topP,
		StopWords:     stopWords,
	}); err != nil {
		return fmt.Errorf("failed to save model settings: %w", err)
	}

	return nil
}

// DeleteModel implements ModelSettingsStorageWrite.
func (a *Adapter) DeleteModel(ctx context.Context, model ids.ModelConfigID) error {
	if err := a.q.DeleteModelSettings(ctx, model.ID()); err != nil {
		return fmt.Errorf("failed to delete model settings: %w", err)
	}

	return nil
}
