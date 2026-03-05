// Package tools defines tool management primitives.
package tools

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"html/template"
	"maps"
	"slices"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

const (
	RawAccountInjectKey    = "_target_account"
	accountDescriptionTmpl = `The account that will be used to perform this action.
Different accounts may have different access rights or contexts.

Allowed values:
{{range $acc := . -}}
- ` + "`" + `{{$acc.Name}}` + "`" + ` — {{$acc.Desc}}
{{end}}`
)

var tmpl = template.Must(template.New("account_description").Parse(accountDescriptionTmpl))

type accountDesc struct {
	Name, Desc string
}

type toolAccounts = map[ids.ToolID]accountDesc

// RawTool is a set of different tools, combined into a single entity for
// correct config conversion.
//
// The core issue with language models is that they are not understanding
// multi-accounting scenarios, especially with  Adding account name to
// tool name, like "account_name.tool_name" for most of models didn't get any
// effect, models usually are going to  (including Gemini, Claude in playground, and ChatGPT in
// playground too)
//
// в отличии от ToolInfo, RawTool является сконвертированным форматом,
// удобным для моделей
type RawTool struct {
	name string
	desc string

	// encodedTools, value is a description of the account, not the tool
	// (description of tool located at [RawTool.desc])
	encodedTools toolAccounts

	params   json.RawMessage
	response json.RawMessage

	_valid bool
}

// NewRawTool constructs and validates a tool definition.
func NewRawTool(
	name, desc string,
	params, response json.RawMessage,
	accountID ids.ToolID,
	accountName, accountDec string,
) (RawTool, error) {
	tool := unsafeRawTool(name, desc, params, response, accountID, accountName, accountDec)

	if err := tool.Validate(); err != nil {
		return RawTool{}, err
	}

	tool._valid = true

	return tool, nil
}

// MergeTools combines this tool with another that has the same schema but
// different accounts. This is used when multiple sources provide the same tool
// but for different accounts. Returns error if tool schemas (name, description,
// params, response) don't match.
func MergeTools(items ...RawTool) (RawTool, error) {
	if err := checkMergeCompatibility(items...); err != nil {
		return RawTool{}, err
	}

	if len(items) == 1 {
		return items[0], nil
	}

	tools, err := items[0].mergeToolMap(items[1:]...)
	if err != nil {
		return RawTool{}, err
	}

	return RawTool{
		name:         items[0].name,
		desc:         items[0].desc,
		encodedTools: tools,
		params:       items[0].params,
		response:     items[0].response,
		_valid:       true,
	}, nil
}

func unsafeRawTool(
	name string,
	desc string,
	params json.RawMessage,
	response json.RawMessage,
	accountID ids.ToolID,
	accountName, accountDec string,
) RawTool {
	tool := RawTool{
		name: name,
		desc: desc,
		encodedTools: toolAccounts{
			accountID: {
				Name: accountName,
				Desc: accountDec,
			},
		},
		params:   params,
		response: response,
		_valid:   false,
	}

	return tool
}

// Valid reports whether this tool definition is properly constructed.
func (r RawTool) Valid() bool { return r._valid || r.Validate() == nil }

// Validate checks tool invariants and returns detailed errors if any are
// violated.
func (r RawTool) Validate() error {
	if err := r.validateFields(); err != nil {
		return err
	}

	if err := r.validateAccounts(); err != nil {
		return err
	}

	return r.validateMultiAccountSchema()
}

func (r RawTool) validateFields() error {
	if r.name == "" {
		return ErrToolNameEmpty
	}

	if r.desc == "" {
		return ErrToolDescriptionEmpty
	}

	return nil
}

func (r RawTool) validateAccounts() error {
	if len(r.encodedTools) < 1 {
		return ErrToolNoAccounts
	}

	for id, account := range r.encodedTools {
		if !id.Valid() {
			return fmt.Errorf("%w: %v", ErrInvalidToolID, id.ID())
		}

		if account.Name == "" {
			return fmt.Errorf("%w: tool %q", ErrAccountSlugEmpty, r.name)
		}
	}

	return nil
}

func (r RawTool) validateMultiAccountSchema() error {
	if len(r.encodedTools) <= 1 {
		return nil
	}

	var schema openapi3.Schema
	if err := json.Unmarshal(r.params, &schema); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidInputSchema, err)
	}

	if _, ok := schema.Properties[RawAccountInjectKey]; ok {
		return fmt.Errorf("%w: %s", ErrReservedPropertyUsed, RawAccountInjectKey)
	}

	return nil
}

func (r RawTool) Name() string { return r.name }
func (r RawTool) Desc() string { return r.desc }

// EncodedTools returns all tool-account associations.
// The map key is the ToolID, value is the account description.
func (r RawTool) EncodedTools() map[ids.ToolID]accountDesc {
	return maps.Clone(r.encodedTools)
}

func (r RawTool) Params() json.RawMessage   { return slices.Clone(r.params) }
func (r RawTool) Response() json.RawMessage { return slices.Clone(r.response) }

func (r RawTool) mergeToolMap(others ...RawTool) (toolAccounts, error) {
	tools := maps.Clone(r.encodedTools)
	for _, tool := range others {
		for id, acc := range tool.encodedTools {
			existing, exists := tools[id]
			if exists && existing != acc {
				return nil, fmt.Errorf("%w: tool %q: %v", ErrDuplicateToolID, r.name, id.ID())
			}

			tools[id] = acc
		}
	}

	return tools, nil
}

func checkMergeCompatibility(items ...RawTool) error {
	switch len(items) {
	case 0:
		return ErrNoToolsToMerge
	case 1:
		return nil
	default:
		first := items[0]
		if !first._valid {
			return first.Validate()
		}

		for i := 1; i < len(items); i++ {
			if err := checkMergeAccountsCompatibilityBetween(first, items[i]); err != nil {
				return err
			}
		}

		return nil
	}
}

func checkMergeAccountsCompatibilityBetween(first, other RawTool) error {
	if !other._valid {
		return other.Validate()
	}

	if first.name != other.name {
		return ErrMergeDifferentNames
	}

	if first.desc != other.desc {
		return ErrMergeDifferentDescs
	}

	if !bytes.Equal(first.params, other.params) {
		return ErrMergeDifferentParams
	}

	if !bytes.Equal(first.response, other.response) {
		return ErrMergeDifferentResps
	}

	return nil
}

// ConvertedSchema injects into original schema list of accounts, to provide
// aviability for model to choose between accounts.
func (r RawTool) ConvertedSchema() json.RawMessage {
	if !r.Valid() {
		return []byte("{}")
	}

	if len(r.encodedTools) == 1 {
		return slices.Clone(r.params)
	}

	return buildMultiAccountSchema([]byte(r.params), r.encodedTools)
}

func buildMultiAccountSchema(params []byte, accounts toolAccounts) json.RawMessage {
	var schema openapi3.Schema

	if err := json.Unmarshal(params, &schema); err != nil {
		// panic by intention: invariants must be detected on primitive
		// creation, not here.
		//
		//nolint:forbidigo // see above.
		panic(fmt.Errorf("unreachable: %w", err))
	}

	if err := injectMultiAccountProperty(&schema, accounts); err != nil {
		// panic by intention: invariants must be detected on primitive
		// creation, not here.
		//
		//nolint:forbidigo // see above.
		panic(fmt.Errorf("unreachable: %w", err))
	}

	res, err := json.Marshal(schema)
	if err != nil {
		// panic by intention: invariants must be detected on primitive
		// creation, not here.
		//
		//nolint:forbidigo // see above.
		panic(fmt.Errorf("unreachable: %w", err))
	}

	return res
}

func injectMultiAccountProperty(schema *openapi3.Schema, accounts toolAccounts) error {
	if schema.Properties == nil {
		schema.Properties = make(openapi3.Schemas)
	}

	if _, ok := schema.Properties[RawAccountInjectKey]; ok {
		return fmt.Errorf("%w: %q", ErrReservedPropertyCollision, RawAccountInjectKey)
	}

	schRef := accountNamesAsSchema(accounts)

	schema.Properties[RawAccountInjectKey] = schRef
	schema.Required = append(schema.Required, RawAccountInjectKey)

	return nil
}

func (r RawTool) ConvertRequest(
	req map[string]json.RawMessage,
) (ids.ToolID, map[string]json.RawMessage, error) {
	if len(r.encodedTools) == 1 {
		toolID := r.getSingleAccountID()

		delete(req, RawAccountInjectKey)

		return toolID, req, nil
	}

	value, ok := req[RawAccountInjectKey]
	if !ok {
		return ids.ToolID{}, nil, fmt.Errorf("%w: %s",
			ErrAccountInjectKeyMissing, RawAccountInjectKey)
	}

	delete(req, RawAccountInjectKey)

	return r.finishRequestConversion(value, req)
}

func (r RawTool) finishRequestConversion(
	val json.RawMessage,
	req map[string]json.RawMessage,
) (ids.ToolID, map[string]json.RawMessage, error) {
	var name string

	if err := json.Unmarshal(val, &name); err != nil {
		return ids.ToolID{}, nil, fmt.Errorf("%w: %w",
			ErrInvalidEnumFormat, err)
	}

	for tid, acc := range r.encodedTools {
		if acc.Name == name {
			return tid, req, nil
		}
	}

	return ids.ToolID{}, nil, fmt.Errorf("%w: %q", ErrAccountNotFound, name)
}

func accountNamesAsSchema(accounts map[ids.ToolID]accountDesc) *openapi3.SchemaRef {
	// Multiple accounts case - inject chooser enum
	sorted := slices.SortedFunc(maps.Values(accounts), func(a, b accountDesc) int {
		return cmp.Compare(a.Name, b.Name)
	})
	desc := getAccountDescription(sorted)

	return buildSchemaRef(sorted, desc)
}

func buildSchemaRef(names []accountDesc, desc string) *openapi3.SchemaRef {
	namesRaw := make([]any, len(names))
	for i, name := range names {
		namesRaw[i] = name.Name
	}

	sch := openapi3.NewSchema()
	sch.Type = &openapi3.Types{openapi3.TypeString}
	sch.Enum = namesRaw
	sch.Description = desc

	return openapi3.NewSchemaRef("", sch)
}

func getAccountDescription(data []accountDesc) string {
	var desc strings.Builder

	if err := tmpl.Execute(&desc, data); err != nil {
		// panicing by intention. Template is static, and we checked al
		// invariants above
		//
		//nolint:forbidigo // see above
		panic(fmt.Sprintf("unreachable: %v", err))
	}

	return desc.String()
}

func (r RawTool) getSingleAccountID() ids.ToolID {
	var toolID ids.ToolID

	for tid := range r.encodedTools {
		toolID = tid

		break
	}

	return toolID
}
