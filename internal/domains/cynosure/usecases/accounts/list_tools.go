package accounts

import (
	"context"
	"fmt"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/tools"
)

func (s *Usecase) ListTools(ctx context.Context, accountID ids.AccountID) ([]tools.RawTool, error) {
	toolsList, err := s.tools.ListTools(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("listing tools: %w", err)
	}

	res := make([]tools.RawTool, len(toolsList))
	for i, tool := range toolsList {
		res[i], err = tools.NewRawTool(
			tool.Name(),
			tool.Description(),
			tool.InputSchema(),
			tool.OutputSchema(),
			tool.ID(), tool.AccountName(), "",
		)
		if err != nil {
			return nil, fmt.Errorf("converting %q: %w", tool.Name(), err)
		}
	}

	return res, nil
}

func (s *Usecase) SearchMcpTools(
	ctx context.Context,
	user ids.UserID,
	query string,
	limit int,
) ([]tools.RawTool, error) {
	if query == "" {
		return nil, nil
	}

	emb, err := s.getQueryEmbedding(ctx, query)
	if err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 10
	}

	matched, err := s.tools.LookupTools(ctx, user, emb, limit)
	if err != nil {
		return nil, fmt.Errorf("lookup tools: %w", err)
	}

	return s.mapToRawTools(matched)
}

func (s *Usecase) getQueryEmbedding(ctx context.Context, query string) ([1536]float32, error) {
	msg, err := messages.NewMessageUser(query)
	if err != nil {
		return [1536]float32{}, fmt.Errorf("creating message: %w", err)
	}

	emb, err := s.index.BuildToolEmbedding(ctx, []messages.Message{msg})
	if err != nil {
		return [1536]float32{}, fmt.Errorf("embedding query: %w", err)
	}

	return emb, nil
}

func (s *Usecase) mapToRawTools(matched []*entities.Tool) ([]tools.RawTool, error) {
	res := make([]tools.RawTool, len(matched))
	for i, tool := range matched {
		var err error

		res[i], err = tools.NewRawTool(
			tool.Name(), tool.Description(), tool.InputSchema(),
			tool.OutputSchema(), tool.ID(), tool.AccountName(), "",
		)
		if err != nil {
			return nil, fmt.Errorf("converting tool %q: %w", tool.Name(), err)
		}
	}

	return res, nil
}
