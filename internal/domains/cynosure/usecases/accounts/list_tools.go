package accounts

import (
	"context"
	"fmt"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/tools"
)

func (s *Usecase) ListTools(ctx context.Context, accountID ids.AccountID) ([]tools.RawTool, error) {
	t, err := s.tools.ListTools(ctx, accountID)
	if err != nil {
		return nil, err
	}

	res := make([]tools.RawTool, len(t))
	for i, tool := range t {
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
