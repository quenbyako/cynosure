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

type accountDesc struct {
	slug, desc string
}

func (r accountDesc) Slug() string { return r.slug }
func (r accountDesc) Desc() string { return r.desc }

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

	// encodedTools, value is a description of the account, not the tool
	// (description of tool located at [RawToolInfo.desc])
	encodedTools map[ids.ToolID]accountDesc

	params   json.RawMessage
	response json.RawMessage

	_valid bool
}

type RawToolInfoOption func(*RawToolInfo)

// WithMergedTool associates a tool with a specific account. Use this to define
// which accounts can execute this tool and provide a description of each
// account's purpose or context.
func WithMergedTool(id ids.ToolID, accountSlug, accountDescription string) RawToolInfoOption {
	return func(r *RawToolInfo) { r.encodedTools[id] = accountDesc{slug: accountSlug, desc: accountDescription} }
}

// NewRawToolInfo constructs and validates a tool definition.
// Use functional options to associate the tool with one or more accounts.
func NewRawToolInfo(name, desc string, params, response json.RawMessage, opts ...RawToolInfoOption) (RawToolInfo, error) {
	r := RawToolInfo{
		name:         name,
		desc:         desc,
		encodedTools: make(map[ids.ToolID]accountDesc),
		params:       params,
		response:     response,
	}
	for _, opt := range opts {
		opt(&r)
	}

	if err := r.Validate(); err != nil {
		return RawToolInfo{}, err
	}
	r._valid = true

	return r, nil
}

// Valid reports whether this tool definition is properly constructed.
func (r RawToolInfo) Valid() bool { return r._valid || r.Validate() == nil }

// Validate checks tool invariants and returns detailed errors if any are
// violated.
func (r RawToolInfo) Validate() error {
	if r.name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}
	if r.desc == "" {
		return fmt.Errorf("tool description cannot be empty")
	}
	if len(r.encodedTools) < 1 {
		return fmt.Errorf("tool must be associated with at least one account")
	}
	for id, account := range r.encodedTools {
		if !id.Valid() {
			return fmt.Errorf("invalid tool id: %v", id.ID())
		}

		// account slug must not be empty for proper conversion
		if account.slug == "" {
			return fmt.Errorf("account slug cannot be empty for tool %q", r.name)
		}
	}
	if len(r.encodedTools) > 1 {
		var schema openapi3.Schema
		if err := json.Unmarshal(r.params, &schema); err != nil {
			return fmt.Errorf("invalid input schema: %w", err)
		}

		if _, ok := schema.Properties[RawAccountInjectKey]; ok {
			return fmt.Errorf("input schema must not contain '%s' property", RawAccountInjectKey)
		}
	}

	return nil
}

func (r RawToolInfo) Name() string { return r.name }
func (r RawToolInfo) Desc() string { return r.desc }

// EncodedTools returns all tool-account associations.
// The map key is the ToolID, value is the account description.
func (r RawToolInfo) EncodedTools() map[ids.ToolID]accountDesc { return maps.Clone(r.encodedTools) }

func (r RawToolInfo) Params() json.RawMessage   { return slices.Clone(r.params) }
func (r RawToolInfo) Response() json.RawMessage { return slices.Clone(r.response) }

// Merge combines this tool with another that has the same schema but different
// accounts. This is used when multiple sources provide the same tool but for
// different accounts. Returns error if tool schemas (name, description, params,
// response) don't match.
func (r RawToolInfo) Merge(other RawToolInfo) (RawToolInfo, error) {
	if r.name != other.name {
		return RawToolInfo{}, fmt.Errorf("cannot merge tools with different names")
	}
	if r.desc != other.desc {
		return RawToolInfo{}, fmt.Errorf("cannot merge tools with different descriptions")
	}
	if !bytes.Equal(r.params, other.params) {
		return RawToolInfo{}, fmt.Errorf("cannot merge tools with different params")
	}
	if !bytes.Equal(r.response, other.response) {
		return RawToolInfo{}, fmt.Errorf("cannot merge tools with different response")
	}

	var opts []RawToolInfoOption
	for id, desc := range r.encodedTools {
		opts = append(opts, WithMergedTool(id, desc.slug, desc.desc))
	}

	for id, desc := range other.encodedTools {
		// Check for conflicting account descriptions. Extremely rare case, but
		// still possible. Just to be sure at 100%.
		if existingDesc, exists := r.encodedTools[id]; exists && (existingDesc.slug != desc.slug || existingDesc.desc != desc.desc) {
			return RawToolInfo{}, fmt.Errorf("cannot merge: duplicate tool id with different descriptions")
		}
		opts = append(opts, WithMergedTool(id, desc.slug, desc.desc))
	}

	return NewRawToolInfo(r.name, r.desc, r.params, r.response, opts...)
}

// ConvertedSchema injects into original schema list of accounts, to provide
// aviability for model to choose between accounts.
func (r RawToolInfo) ConvertedSchema() json.RawMessage {
	if len(r.encodedTools) == 1 {
		return slices.Clone(r.params)
	}

	var schema openapi3.Schema
	if err := json.Unmarshal(r.params, &schema); err != nil {
		// exception: validated that schema is correct jsonschema on creation.
		panic(fmt.Errorf("failed to unmarshal params: %w", err))
	}

	if schema.Properties == nil {
		schema.Properties = make(openapi3.Schemas)
	}

	// Prevent overriding existing properties if tool ironically has a param
	// named the same as our injection key
	if _, ok := schema.Properties[RawAccountInjectKey]; ok {
		// exception: validated that schema is correct jsonschema on creation,
		// when we got 2 or more accounts.
		panic(fmt.Errorf("schema already has property %q, collision with reserved key", RawAccountInjectKey))
	}

	schema.Properties[RawAccountInjectKey] = accountNamesAsSchema(r.encodedTools)
	schema.Required = append(schema.Required, RawAccountInjectKey)

	return must(json.Marshal(schema))
}

// ConvertRequest selects the appropriate account for executing this tool. For
// single-account tools: automatically selects the only available account. For
// multi-account tools: reads the _target_account field from the request to
// determine which account to use. Returns the selected ToolID and the request
// parameters (with _target_account removed).
func (r RawToolInfo) ConvertRequest(req map[string]json.RawMessage) (ids.ToolID, map[string]json.RawMessage, error) {
	if len(r.encodedTools) == 1 {
		var id ids.ToolID
		for k := range r.encodedTools {
			id = k
			break
		}
		// Remove _target_account field if present (even for single account)
		delete(req, RawAccountInjectKey)
		return id, req, nil
	}

	value, ok := req[RawAccountInjectKey]
	if !ok {
		return ids.ToolID{}, nil, fmt.Errorf("field %q is empty, expected to be required", RawAccountInjectKey)
	}
	delete(req, RawAccountInjectKey)

	var slug string
	if err := json.Unmarshal(value, &slug); err != nil {
		return ids.ToolID{}, nil, fmt.Errorf("field %q expected to be a enum string, got %v: %w", RawAccountInjectKey, value, err)
	}

	if slug == "" {
		return ids.ToolID{}, nil, fmt.Errorf("field %q cannot be empty", RawAccountInjectKey)
	}

	for id, account := range r.encodedTools {
		if account.slug == slug {
			return id, req, nil
		}
	}

	return ids.ToolID{}, nil, fmt.Errorf("field %q has value %q, which is invalid", RawAccountInjectKey, slug)
}

const RawAccountInjectKey = "_target_account"

type accountData struct {
	Name string
	Desc string
}

func accountNamesAsSchema(accounts map[ids.ToolID]accountDesc) *openapi3.SchemaRef {
	// Multiple accounts case - inject chooser enum
	sorted := slices.SortedFunc(maps.Values(accounts), func(a, b accountDesc) int {
		return cmp.Compare(a.slug, b.slug)
	})

	accountSlugsRaw := make([]any, len(sorted))
	templateData := make([]accountData, len(sorted))

	for i, account := range sorted {
		accountSlugsRaw[i] = account.slug
		templateData[i] = accountData{
			Name: account.slug,
			Desc: account.desc,
		}
	}

	var description strings.Builder
	if err := tmpl.Execute(&description, templateData); err != nil {
		panic(err)
	}

	return &openapi3.SchemaRef{
		Value: &openapi3.Schema{
			Type:        &openapi3.Types{openapi3.TypeString},
			Enum:        accountSlugsRaw,
			Description: description.String(),
		},
	}
}

const accountDescriptionTemplate = `The account that will be used to perform this action.
Different accounts may have different access rights or contexts.

Allowed values:
{{range $acc := . -}}
- ` + "`" + `{{$acc.Name}}` + "`" + ` — {{$acc.Desc}}
{{end}}`

var tmpl = template.Must(template.New("account_description").Parse(accountDescriptionTemplate))
