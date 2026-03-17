package chat

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/tools"
)

// buildToolbox constructs an immutable Toolbox from semantically relevant tools.
//
// Workflow (Three-Stage RAG):
//  1. Generate embedding from conversation history (ToolSemanticIndex)
//  2. Find top-K relevant tools using vector similarity search (ToolStorage)
//  3. Load account metadata and build Toolbox (AccountStorage + Toolbox.Merge)
//
// This method is called:
//   - During aggregate initialization (newChatAggregate)
//   - After each user message (AcceptUserMessage) to update context
//
// The toolbox is immutable once created and contains all information needed
// for the LLM to make tool calls (schemas, account selectors, etc.)
func (c *Chat) buildToolbox(ctx context.Context) (tools.Toolbox, error) {
	embedding, err := c.indexer.BuildToolEmbedding(ctx, c.thread.Messages())
	if err != nil {
		return tools.Toolbox{}, fmt.Errorf("building embedding: %w", err)
	}

	const topK = 10 // TODO: make configurable. Through agent config maybe?

	relevantTools, err := c.toolStorage.LookupTools(ctx, c.thread.ID().User(), embedding, topK)
	if err != nil {
		return tools.Toolbox{}, fmt.Errorf("looking up tools: %w", err)
	}

	if len(relevantTools) == 0 {
		return tools.NewToolbox(), nil
	}

	// TODO: could be a great idea to cache this, since we ONLY need a
	// description of the account.
	descriptions, err := c.accountsDescriptions(ctx, relevantTools)
	if err != nil {
		return tools.Toolbox{}, fmt.Errorf("loading account descriptions: %w", err)
	}

	// 5. Build RawToolInfo for each tool with account metadata
	rawTools := make([]tools.RawTool, 0, len(relevantTools))
	for _, tool := range relevantTools {
		desc, ok := descriptions[tool.ID().Account()]
		if !ok {
			// TODO: log warning about stale data
			continue
		}

		rawTool, toolErr := tools.NewRawTool(
			tool.Name(),
			tool.Description(),
			tool.InputSchema(),
			tool.OutputSchema(),
			tool.ID(), desc.slug, desc.desc,
		)
		if toolErr != nil {
			return tools.Toolbox{}, fmt.Errorf("creating raw tool %q: %w", tool.Name(), toolErr)
		}

		rawTools = append(rawTools, rawTool)
	}

	// 6. Build toolbox by merging all tools
	// Toolbox.Merge handles collision detection (same tool name from different accounts)
	toolbox, err := tools.NewToolbox().Merge(rawTools...)
	if err != nil {
		return tools.Toolbox{}, fmt.Errorf("merging tools into toolbox: %w", err)
	}

	// 7. Update internal tool cache for rapid execution lookup
	c.tools = make(map[ids.ToolID]*entities.Tool, len(relevantTools))
	for _, tool := range relevantTools {
		c.tools[tool.ID()] = tool
	}

	return toolbox, nil
}

type accountDesc struct{ slug, desc string }

func (c *Chat) accountsDescriptions(
	ctx context.Context, toolList []*entities.Tool,
) (map[ids.AccountID]accountDesc, error) {
	accountIDs := extractUniqueAccounts(toolList)

	accounts, err := c.accounts.GetAccountsBatch(ctx, accountIDs)
	if err != nil {
		return nil, fmt.Errorf("loading accounts: %w", err)
	}

	descriptions := make(map[ids.AccountID]accountDesc, len(accounts))
	for _, account := range accounts {
		descriptions[account.ID()] = accountDesc{
			slug: account.Name(),
			desc: account.Description(),
		}
	}

	return descriptions, nil
}

// extractUniqueAccounts returns unique account IDs from a list of tools.
func extractUniqueAccounts(toolList []*entities.Tool) []ids.AccountID {
	accountSet := make(map[ids.AccountID]struct{}, len(toolList))
	for _, tool := range toolList {
		accountSet[tool.ID().Account()] = struct{}{}
	}

	return slices.Collect(maps.Keys(accountSet))
}
