package ory_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/identitymanager/testsuite"
)

func TestOryIdentityManager(t *testing.T) {
	adapter, telegramUserID, stop := vcrAdapter(t, "testdata/test_identity_manager.yaml")
	defer func() {
		require.NoError(t, stop())
	}()

	testsuite.RunIdentityManagerTests(
		adapter.IdentityManager(),
		testsuite.WithIdentityManagerID(telegramUserID),
	)(t)
}
