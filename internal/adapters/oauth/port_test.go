package oauth_test

import (
	"testing"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/oauthhandler/testsuite"

	. "github.com/quenbyako/cynosure/internal/adapters/oauth"
)

func TestOauthHandlerSuite(t *testing.T) {
	h := New([]string{"test:scope", "for:fallback"})

	testsuite.RunOAuthHandlerTests(h)(t)
}
