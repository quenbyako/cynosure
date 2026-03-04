package datatransfer

import (
	"fmt"

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

	agent, err := entities.NewModelSettings(
		id,
		row.Model,
		entities.WithSystemMessage(row.SystemMessage),
		entities.WithTemperature(row.Temperature),
		entities.WithTopP(row.TopP),
		entities.WithStopWords(row.StopWords),
	)
	if err != nil {
		return nil, fmt.Errorf("new model settings: %w", err)
	}

	return agent, nil
}

// ToDBAgentParams converts domain entity to DB insert parameters
func ToDBAgentParams(agent entities.AgentReadOnly) (db.UpsertAgentSettingsParams, error) {
	dbID := agent.ID().ID()
	userID := agent.ID().UserID().ID()

	model := agent.Model()
	sysMsg := agent.SystemMessage()
	temp, _ := agent.Temperature()

	if temp < 0 {
		temp = 0
	}

	topP, _ := agent.TopP()

	if topP < 0 {
		topP = 0
	}

	stopWords := agent.StopWords()
	if stopWords == nil {
		stopWords = []string{}
	}

	return db.UpsertAgentSettingsParams{
		ID:            dbID,
		UserID:        userID,
		Model:         model,
		SystemMessage: sysMsg,
		Temperature:   temp,
		TopP:          topP,
		StopWords:     stopWords,
	}, nil
}
