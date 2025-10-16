package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
	"text/template"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/tools"
)

type collisionStrategy uint8

const (
	_ collisionStrategy = iota
	collisionStrategyReplace
	collisionStrategyLeave
	collisionStrategyThrowError
)

const strategy = collisionStrategyLeave

type accountReversed struct {
	id   ids.AccountID
	desc string
}

type toolReversed struct {
	desc         string
	accounts     map[string]accountReversed
	inputSchema  json.RawMessage
	outputSchema json.RawMessage
}

func pullToolsAndAccounts(ctx context.Context, toolMgr ports.ToolManager, accounts ports.AccountStorage, thread *entities.ChatHistory) (schema map[string]tools.RawToolInfo, err error) {
	relevantTools, err := toolMgr.RetrieveRelevantTools(ctx, thread.User(), thread.Messages())
	if err != nil {
		return nil, fmt.Errorf("retrieving relevant tools: %w", err)
	}

	lookupIDs := make([]ids.AccountID, 0, len(relevantTools))
	toolFilter := make(map[ids.AccountID]map[string]struct{}, len(relevantTools))
	for id, tools := range relevantTools {
		lookupIDs = append(lookupIDs, id)
		toolFilter[id] = make(map[string]struct{}, len(tools))
		for _, tool := range tools {
			toolFilter[id][tool.Name()] = struct{}{}
		}
	}

	accs, err := accounts.GetAccountsBatch(ctx, lookupIDs)
	if err != nil {
		return nil, fmt.Errorf("getting accounts: %w", err)
	}

	// теперь ищем те инструменты, у которых есть коллизии: проходим по всем инструментам
	// и смотрим, есть ли у них несколько аккаунтов
	toolCollisions := make(map[string]toolReversed)
	for i, account := range accs {
		for _, tool := range account.Tools() {
			if _, ok := toolFilter[lookupIDs[i]][tool.Name()]; !ok {
				continue
			} else if _, ok := toolCollisions[tool.Name()]; !ok {
				toolCollisions[tool.Name()] = toolReversed{
					desc: tool.Desc(),
					accounts: map[string]accountReversed{
						account.Name(): {
							id:   account.ID(),
							desc: account.Description(),
						},
					},
					inputSchema:  tool.ParamsSchema(),
					outputSchema: tool.ResponseSchema(),
				}
				continue
			}

			// we got a collision!

			collidedTool := toolCollisions[tool.Name()]

			// must check schemas, if they differ — we should handle this issue.
			if bytes.Equal(tool.ParamsSchema(), collidedTool.inputSchema) &&
				bytes.Equal(tool.ResponseSchema(), collidedTool.outputSchema) &&
				tool.Desc() == collidedTool.desc {
				// schemas are identical, we can merge them safely
				toolCollisions[tool.Name()].accounts[account.Name()] = accountReversed{
					id:   account.ID(),
					desc: account.Description(),
				}
				continue
			}

			// Whoops — tools are not JUST have different accounts, but also
			// different schemas 😱

			if tool.Desc() != collidedTool.desc {
				// descriptions are not so painful: we MAY ignore them, since
				// usually are not critical for make decisions
				fmt.Println("ooops! descriptions are different")
			}

			switch strategy {
			case collisionStrategyReplace:
				toolCollisions[tool.Name()] = toolReversed{
					desc: tool.Desc(),
					accounts: map[string]accountReversed{
						account.Name(): {
							id:   account.ID(),
							desc: account.Description(),
						},
					},
					inputSchema:  tool.ParamsSchema(),
					outputSchema: tool.ResponseSchema(),
				}
			case collisionStrategyLeave:
				continue
			case collisionStrategyThrowError:
				return nil, fmt.Errorf("tool %q collided with differ schemas", tool.Name())
			default:
				panic("unknown collision strategy")
			}

		}
	}

	// теперь обрабатываем коллизии
	schema = make(map[string]tools.RawToolInfo, len(toolCollisions))
	for toolName, tool := range toolCollisions {
		if len(tool.accounts) <= 1 {
			encodedAccounts := make(map[string]ids.AccountID, len(tool.accounts))
			for name, acc := range tool.accounts {
				encodedAccounts[name] = acc.id
			}

			schema[toolName], err = tools.NewRawToolInfo(
				toolName,
				tool.desc,
				encodedAccounts,
				tool.inputSchema,
				tool.outputSchema,
			)
			continue
		}

		accountNamesStr := slices.Sorted(maps.Keys(tool.accounts))
		accountNamesRaw := make([]any, len(accountNamesStr))
		for i, str := range accountNamesStr {
			accountNamesRaw[i] = str
		}

		enumSchema := openapi3.NewStringSchema().WithEnum(accountNamesRaw...)
		enumSchema.Description = renderAccountDescription(tool.accounts)

		var parsedSchema openapi3.Schema
		if err := json.Unmarshal(tool.inputSchema, &parsedSchema); err != nil {
			return nil, fmt.Errorf("schema for tool %q: %w", toolName, err)
		}
		if parsedSchema.Properties == nil {
			parsedSchema.Properties = make(openapi3.Schemas)
		}

		if _, ok := parsedSchema.Properties[tools.RawToolInjectKey]; ok {
			return nil, fmt.Errorf("schema already has property %q", tools.RawToolInjectKey)
		}

		parsedSchema.Properties[tools.RawToolInjectKey] = openapi3.NewSchemaRef("", enumSchema)
		parsedSchema.Required = append(parsedSchema.Required, tools.RawToolInjectKey)

		resultedSchema, err := json.Marshal(&parsedSchema)
		if err != nil {
			panic("unreachable")
		}

		encodedAccounts := make(map[string]ids.AccountID, len(tool.accounts))
		for name, acc := range tool.accounts {
			encodedAccounts[name] = acc.id
		}

		schema[toolName], err = tools.NewRawToolInfo(
			toolName,
			tool.desc,
			encodedAccounts,
			resultedSchema,
			tool.outputSchema,
		)
		if err != nil {
			return nil, fmt.Errorf("creating raw tool info for %q: %w", toolName, err)
		}
	}

	return schema, nil
}

const accountDescriptionTemplate = `The account that will be used to perform this action.
Different accounts may have different access rights or contexts.

Allowed values:
{{range $acc := . -}}
- ` + "`" + `{{$acc.Name}}` + "`" + ` — {{$acc.Desc}}
{{end}}`

var tmpl = template.Must(template.New("account_description").Parse(accountDescriptionTemplate))

type accData struct {
	Name string
	Desc string
}

func renderAccountDescription(accounts map[string]accountReversed) string {
	data := make([]accData, 0, len(accounts))
	for _, name := range slices.Sorted(maps.Keys(accounts)) {
		data = append(data, accData{
			Name: name,
			Desc: accounts[name].desc,
		})
	}

	var builder strings.Builder
	if err := tmpl.Execute(&builder, data); err != nil {
		panic(err)
	}

	return builder.String()
}
