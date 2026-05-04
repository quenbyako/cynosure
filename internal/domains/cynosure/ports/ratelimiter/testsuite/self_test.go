package testsuite

import (
	"context"
	"testing"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/ratelimiter"
)

// This test validates, that all features written correctly, can be compiled,
// and executed.
func TestSelfCheckup(t *testing.T) {
	Run(func(_ context.Context, params SetupParams) (ratelimiter.Port, error) {
		return nil, &selfTestError{}
	})(t)
}
