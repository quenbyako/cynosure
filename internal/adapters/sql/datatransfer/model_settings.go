package datatransfer

import (
	db "github.com/quenbyako/cynosure/contrib/db/gen/go"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/ids"
)

// ModelSettingsFromDB converts database row to domain entity.
// This is pure type conversion - NO database queries.
func ModelSettingsFromDB(row db.AgentsModelSetting) (*entities.ModelSettings, error) {
	modelID, err := ids.NewModelConfigID(row.ID)
	if err != nil {
		return nil, err
	}

	opts := []entities.NewModelSettingsOption{}

	if row.SystemMessage != "" {
		opts = append(opts, entities.WithSystemMessage(row.SystemMessage))
	}

	if row.Temperature > 0 {
		opts = append(opts, entities.WithTemperature(row.Temperature))
	}

	if row.TopP > 0 {
		opts = append(opts, entities.WithTopP(row.TopP))
	}

	if len(row.StopWords) > 0 {
		opts = append(opts, entities.WithStopWords(row.StopWords))
	}

	return entities.NewModelSettings(modelID, row.Model, opts...)
}
