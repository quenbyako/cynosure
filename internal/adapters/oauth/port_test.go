package oauth_test

import (
	"testing"

	"github.com/quenbyako/cynosure/internal/adapters/oauth"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/oauthhandler/testsuite"
)

func TestOauthHandlerSuite(t *testing.T) {
	h := oauth.New([]string{"test:scope", "for:fallback"})

	testsuite.RunOAuthHandlerTests(h)(t)
}
