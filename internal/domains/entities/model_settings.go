package entities

import (
	"errors"
	"slices"
	"tg-helper/internal/domains/components/ids"
)

type ModelSettings struct {
	id ids.ModelConfigID

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
	// Temperature is the temperature for the model, which controls the
	// randomness of the model.
	//
	// if temperature is negative, then it's not set.
	temperature float32

	// TopP is the top p for the model, which controls the diversity of the model.
	//
	// If topP is negative, then it's not set.
	topP float32
	// Stop is the stop words for the model, which controls the stopping
	// condition of the model.
	stopWords []string

	pendingEvents []ModelSettingsEvent
	valid         bool
}

var _ ModelSettingsReadOnly = (*ModelSettings)(nil)
var _ EventsReader[ModelSettingsEvent] = (*ModelSettings)(nil)

type NewModelSettingsOption func(*ModelSettings)

func WithSystemMessage(message string) NewModelSettingsOption {
	return func(m *ModelSettings) { m.systemMessage = message }
}

func WithTemperature(temperature float32) NewModelSettingsOption {
	return func(m *ModelSettings) { m.temperature = temperature }
}

func WithTopP(topP float32) NewModelSettingsOption {
	return func(m *ModelSettings) { m.topP = topP }
}

func WithStopWords(stopWords []string) NewModelSettingsOption {
	return func(m *ModelSettings) { m.stopWords = stopWords }
}

func NewModelSettings(id ids.ModelConfigID, model string, opts ...NewModelSettingsOption) (*ModelSettings, error) {
	m := &ModelSettings{
		id:            id,
		model:         model,
		systemMessage: "",
		temperature:   -1,
		topP:          -1,
		stopWords:     []string{},
	}
	for _, opt := range opts {
		opt(m)
	}

	if err := m.Validate(); err != nil {
		return nil, err
	}
	m.valid = true

	return m, nil
}

func (m *ModelSettings) Valid() bool { return m.valid || m.Validate() == nil }
func (m *ModelSettings) Validate() error {
	if m.id.Valid() == false {
		return errors.New("ID is invalid")
	}
	if m.model == "" {
		return errors.New("model is required")
	}

	return nil
}

// CHANGES

func (m *ModelSettings) Synchronized() bool                  { return len(m.pendingEvents) == 0 }
func (m *ModelSettings) PendingEvents() []ModelSettingsEvent { return slices.Clone(m.pendingEvents) }
func (m *ModelSettings) ClearEvents()                        { m.pendingEvents = m.pendingEvents[:0] }

// READ

type ModelSettingsReadOnly interface {
	ID() ids.ModelConfigID
	Model() string
	SystemMessage() string
	Temperature() float32
	TopP() float32
	StopWords() []string
}

func (c *ModelSettings) ID() ids.ModelConfigID { return c.id }
func (c *ModelSettings) Model() string         { return c.model }
func (c *ModelSettings) SystemMessage() string { return c.systemMessage }
func (c *ModelSettings) Temperature() float32  { return c.temperature }
func (c *ModelSettings) TopP() float32         { return c.topP }
func (c *ModelSettings) StopWords() []string   { return slices.Clone(c.stopWords) }

// WRITE

func (c *ModelSettings) SetSystemMessage(message string) error {
	c.systemMessage = message

	if err := c.Validate(); err != nil {
		return err
	}

	c.pendingEvents = append(c.pendingEvents, &ModelSettingsEventSystemMessageUpdated{
		msg: message,
	})
	return nil
}

// EVENTS

type ModelSettingsEvent interface {
	_ModelSettingsEvent()
}

var _ ModelSettingsEvent = (*ModelSettingsEventSystemMessageUpdated)(nil)

type ModelSettingsEventSystemMessageUpdated struct {
	msg string
}

func (e *ModelSettingsEventSystemMessageUpdated) _ModelSettingsEvent() {}

func (e *ModelSettingsEventSystemMessageUpdated) Message() string { return e.msg }
