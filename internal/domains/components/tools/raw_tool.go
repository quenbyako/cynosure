package tools

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"tg-helper/internal/domains/components/ids"
)

const RawToolInjectKey = "_target_account"

// ToolSet is a set of different tools, combined into a single entity for
// correct config conversion.
//
// The core issue with language models is that they are not understanding
// multi-accounting scenarios, especially with  Adding account name to
// tool name, like "account_name.tool_name" for most of models didn't get any
// effect, models usually are going to  (including Gemini, Claude in playground, and ChatGPT in
// playground too)
//
// в отличии от ToolInfo, RawToolInfo является сконвертированным форматом,
// удобным для моделей
type RawToolInfo struct {
	name string
	desc string

	// encodedAccounts are links to real accounts
	encodedAccounts map[string]ids.AccountID

	params   json.RawMessage
	response json.RawMessage

	valid bool
}

func NewRawToolInfo(name, desc string, accounts map[string]ids.AccountID, params, response json.RawMessage) (RawToolInfo, error) {
	r := RawToolInfo{
		name:            name,
		desc:            desc,
		encodedAccounts: accounts,
		params:          params,
		response:        response,
	}

	if err := r.Validate(); err != nil {
		return RawToolInfo{}, err
	}
	r.valid = true

	return r, nil
}

func (r RawToolInfo) Valid() bool { return r.valid || r.Validate() == nil }
func (r RawToolInfo) Validate() error {
	switch {
	case r.name == "":
		return fmt.Errorf("tool name cannot be empty")
	case r.desc == "":
		return fmt.Errorf("tool description cannot be empty")
	case len(r.encodedAccounts) < 1:
		return fmt.Errorf("tool must be associated with at least one account")
	// TODO: check validity of encoded schemas
	default:
		return nil
	}
}

func (r RawToolInfo) Name() string                              { return r.name }
func (r RawToolInfo) Desc() string                              { return r.desc }
func (r RawToolInfo) EncodedAccounts() map[string]ids.AccountID { return r.encodedAccounts }
func (r RawToolInfo) Params() json.RawMessage                   { return r.params }
func (r RawToolInfo) Response() json.RawMessage                 { return r.response }

func (r RawToolInfo) SelectToolFromCall(params map[string]json.RawMessage) (call ToolCall, err error) {
	if len(r.encodedAccounts) == 0 {
		return ToolCall{}, errors.New("invalid chat state: there are no accounts for any tool")
	}
	if len(r.encodedAccounts) == 1 {
		for _, id := range r.encodedAccounts {
			return NewToolCall(id, uuid.NewString(), r.name, params)
		}
	}

	accNameRaw, ok := params[RawToolInjectKey]
	if !ok {
		return ToolCall{}, fmt.Errorf("schema doesn't have property %q, unable to determine target account", RawToolInjectKey)
	}
	var accName string
	if json.Unmarshal(accNameRaw, &accName) != nil {
		return ToolCall{}, fmt.Errorf("invalid account name: %w", err)
	}
	id, ok := r.encodedAccounts[accName]
	if !ok {
		return ToolCall{}, fmt.Errorf("unknown account %q", accName)
	}
	delete(params, RawToolInjectKey)

	return NewToolCall(id, uuid.NewString(), r.name, params)
}
