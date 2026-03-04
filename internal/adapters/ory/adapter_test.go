package ory_test

import (
	_ "embed"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/quenbyako/cynosure/internal/adapters/ory"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/identitymanager/testsuite"
)

//go:embed ory.secret
var secretsRaw []byte

type secrets struct {
	Endpoint     string `yaml:"endpoint"`
	AdminKey     string `yaml:"admin_key"`
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
}

func TestOryIdentityManager(t *testing.T) {
	var s secrets
	err := yaml.Unmarshal(secretsRaw, &s)
	require.NoError(t, err)

	endpoint, err := url.Parse(s.Endpoint)
	require.NoError(t, err)

	adapter := ory.New(endpoint, s.AdminKey,
		ory.WithClientCredentials(s.ClientID, s.ClientSecret),
		ory.WithScopes("mcp:read", "mcp:write", "offline_access"),
		ory.WithRedirectURL("http://localhost:5001"),
	)

	testsuite.RunIdentityManagerTests(adapter.IdentityManager())(t)
}
