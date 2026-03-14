package entities

import (
	"errors"
	"slices"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

type Agent struct {
	// Model is the model name.
	//
	// TODO: есть такая штука как model-card.json, и это буквально реестр
	// моделей, однако есть важный нюанс, что это реф для моделей, которые можно
	// скачать и запустить. Какого-то общего service discovery для любых
	// языковых моделей нет. По итогу:
	//
	// 1. создается отдельная мапа между провайдерами и названиями моделей 2. в
	// идеале, потом надо сделать карточки моделей 3. теоретически, можно
	// попробовать отыскать openai-совместимые апи для разных моделей чтоб можно
	// было их легко подключать.
	model string

	// system message for model
	systemMessage string
	// Stop is the stop words for the model, which controls the stopping
	// condition of the model.
	stopWords     []string
	pendingEvents []AgentEvent
	// Temperature is the temperature for the model, which controls the
	// randomness of the model.
	//
	// if temperature is <= 0 then it's not set.
	temperature float32

	// TopP is the top p for the model, which controls the diversity of the model.
	//
	// If topP is <= 0, then it's not set.
	topP   float32
	id     ids.AgentID
	_valid bool
}

var (
	_ AgentReadOnly            = (*Agent)(nil)
	_ EventsReader[AgentEvent] = (*Agent)(nil)
)

type NewModelSettingsOption func(*Agent)

func WithSystemMessage(message string) NewModelSettingsOption {
	return func(m *Agent) { m.systemMessage = message }
}

func WithTemperature(temperature float32) NewModelSettingsOption {
	return func(m *Agent) { m.temperature = temperature }
}

func WithTopP(topP float32) NewModelSettingsOption {
	return func(m *Agent) { m.topP = topP }
}

func WithStopWords(stopWords []string) NewModelSettingsOption {
	return func(m *Agent) { m.stopWords = stopWords }
}

func NewModelSettings(id ids.AgentID, model string, opts ...NewModelSettingsOption) (*Agent, error) {
	agent := &Agent{
		id:            id,
		model:         model,
		systemMessage: "",
		temperature:   -1,
		topP:          -1,
		stopWords:     nil,
		pendingEvents: nil,
		_valid:        false,
	}
	for _, opt := range opts {
		opt(agent)
	}

	if err := agent.Validate(); err != nil {
		return nil, err
	}

	agent._valid = true

	return agent, nil
}

func (m *Agent) Valid() bool { return m._valid || m.Validate() == nil }
func (m *Agent) Validate() error {
	if !m.id.Valid() {
		return errors.New("ID is invalid")
	}

	if m.model == "" {
		return errors.New("model is required")
	}

	return nil
}

// CHANGES

func (m *Agent) Synchronized() bool          { return len(m.pendingEvents) == 0 }
func (m *Agent) PendingEvents() []AgentEvent { return slices.Clone(m.pendingEvents) }
func (m *Agent) ClearEvents()                { m.pendingEvents = m.pendingEvents[:0] }

// READ

type AgentReadOnly interface {
	ID() ids.AgentID
	Model() string
	SystemMessage() string
	Temperature() (float32, bool)
	TopP() (float32, bool)
	StopWords() []string
}

func (c *Agent) ID() ids.AgentID              { return c.id }
func (c *Agent) Model() string                { return c.model }
func (c *Agent) SystemMessage() string        { return c.systemMessage }
func (c *Agent) Temperature() (float32, bool) { return c.temperature, c.temperature > 0 }
func (c *Agent) TopP() (float32, bool)        { return c.topP, c.topP > 0 }
func (c *Agent) StopWords() []string          { return slices.Clone(c.stopWords) }

// WRITE

func (c *Agent) SetSystemMessage(message string) error {
	c.systemMessage = message

	if err := c.Validate(); err != nil {
		return err
	}

	c.pendingEvents = append(c.pendingEvents, &AgentEventSystemMessageUpdated{
		msg: message,
	})

	return nil
}

// EVENTS

type AgentEvent interface {
	_AgentEvent()
}

var _ AgentEvent = (*AgentEventSystemMessageUpdated)(nil)

type AgentEventSystemMessageUpdated struct {
	msg string
}

func (e *AgentEventSystemMessageUpdated) _AgentEvent() {}

func (e *AgentEventSystemMessageUpdated) Message() string { return e.msg }
