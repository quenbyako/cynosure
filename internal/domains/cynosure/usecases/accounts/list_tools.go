package accounts

import (
	"context"
	"fmt"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/tools"
)

func (s *Usecase) ListTools(ctx context.Context, accountID ids.AccountID) ([]tools.RawToolInfo, error) {
	t, err := s.tools.ListTools(ctx, accountID)
	if err != nil {
		return nil, err
	}

	res := make([]tools.RawToolInfo, len(t))
	for i, tool := range t {
		res[i], err = tools.NewRawToolInfo(
			tool.Name(),
			tool.Description(),
			tool.InputSchema(),
			tool.OutputSchema(),
			tools.WithMergedTool(tool.ID(), tool.AccountName(), ""),
		)

		if err != nil {
			return nil, fmt.Errorf("converting %q: %w", tool.Name(), err)
		}
	}

	return res, nil
}
