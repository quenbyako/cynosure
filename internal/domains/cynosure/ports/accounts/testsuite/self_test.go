package testsuite_test

import (
	"context"
	"testing"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/accounts"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/accounts/testsuite"
)

// This test validates, that all features written correctly, can be compiled,
// and executed.
func TestSelfCheckup(t *testing.T) {
	testsuite.Run(func(_ context.Context) (accounts.Port, error) {
		return nil, &testsuite.SelfTestError{}
	})(t)
}
