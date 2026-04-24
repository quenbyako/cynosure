package datatransfer

import (
	"fmt"
	"math"

	db "github.com/quenbyako/cynosure/contrib/db/gen/go"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

// ToDomainAgent converts database model to domain entity
func ToDomainAgent(row db.AgentsAgentSetting) (*entities.Agent, error) {
	userID, err := ids.NewUserID(row.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user id: %w", err)
	}

	id, err := ids.NewAgentID(userID, row.ID)
	if err != nil {
		return nil, fmt.Errorf("invalid agent id: %w", err)
	}

	// Unlike throwing error on int32 overflow, we just cap it, cause it's more
	// likely to have negative values in database, rather than extremely large.
	maxContext := uint(max(0, row.MaxContextMessages))

	agent, err := entities.NewModelSettings(
		id,
		row.Model,
		entities.WithSystemMessage(row.SystemMessage),
		entities.WithTemperature(row.Temperature),
		entities.WithTopP(row.TopP),
		entities.WithStopWords(row.StopWords),
		entities.WithMaxContext(maxContext),
	)
	if err != nil {
		return nil, fmt.Errorf("new model settings: %w", err)
	}

	return agent, nil
}

// ToDBAgentParams converts domain entity to DB insert parameters
func ToDBAgentParams(agent entities.AgentReadOnly) (db.UpsertAgentSettingsParams, error) {
	temp, _ := agent.Temperature()
	topP, _ := agent.TopP()
	maxContext, _ := agent.MaxContext()

	if maxContext > math.MaxInt32 {
		return db.UpsertAgentSettingsParams{}, ErrMaxContextOverflow
	}

	stopWords := agent.StopWords()
	if stopWords == nil {
		stopWords = []string{}
	}

	return db.UpsertAgentSettingsParams{
		ID:                 agent.ID().ID(),
		UserID:             agent.ID().UserID().ID(),
		Model:              agent.Model(),
		SystemMessage:      agent.SystemMessage(),
		Temperature:        max(0, temp),
		TopP:               max(0, topP),
		MaxContextMessages: int32(maxContext),
		StopWords:          stopWords,
	}, nil
}
