package ory_test

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	_ "embed"

	"github.com/quenbyako/cynosure/internal/adapters/ory"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/identitymanager/testsuite"
)

//go:embed ory.secret
var secretsRaw []byte

//nolint:tagliatelle // better to do in that way.
type secrets struct {
	Endpoint     string `yaml:"endpoint"`
	AdminKey     string `yaml:"admin_key"`
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
}

func TestOryIdentityManager(t *testing.T) {
	var sec secrets

	err := yaml.Unmarshal(secretsRaw, &sec)
	require.NoErrorf(t, err, "unmarshaling secrets")

	require.NotEmpty(t, sec.Endpoint, "endpoint is empty")
	require.NotEmpty(t, sec.AdminKey, "admin key is empty")
	require.NotEmpty(t, sec.ClientID, "client id is empty")
	require.NotEmpty(t, sec.ClientSecret, "client secret is empty")

	endpoint, err := url.Parse(sec.Endpoint)
	require.NoErrorf(t, err, "parsing endpoint")

	adapter, err := ory.New(endpoint, sec.AdminKey,
		ory.WithClientCredentials(sec.ClientID, sec.ClientSecret),
		ory.WithScopes("mcp:read", "mcp:write", "offline_access"),
		ory.WithRedirectURL("http://localhost:5001"),
	)
	require.NoError(t, err, "creating ory client")

	testsuite.RunIdentityManagerTests(adapter.IdentityManager())(t)
}
