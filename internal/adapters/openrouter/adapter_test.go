package openrouter_test

import (
	"context"
	"embed"
	"math"
	"testing"
	"time"

	"github.com/quenbyako/cynosure/contrib/core-params/ratelimit"
	"github.com/stretchr/testify/require"
	"gopkg.in/dnaeon/go-vcr.v4/pkg/cassette"
	"gopkg.in/dnaeon/go-vcr.v4/pkg/recorder"

	"github.com/quenbyako/cynosure/internal/adapters/openrouter"
	chatmodel "github.com/quenbyako/cynosure/internal/domains/cynosure/ports/chatmodel/testsuite"
	embedding "github.com/quenbyako/cynosure/internal/domains/cynosure/ports/embedding/testsuite"
	"github.com/quenbyako/cynosure/internal/vcrtest"
)

//go:embed testdata/*
var testdata embed.FS

const (
	maxDuration = time.Duration(math.MaxInt64)
)

var openrouterMaxTokenConsumptionPerTest = []openrouter.NewOption{
	openrouter.WithEmbeddingLimit(ratelimit.NewPolicy(10000, maxDuration)),
	openrouter.WithChatInputLimit(ratelimit.NewPolicy(10000, maxDuration)),
}

type staticSecretGetter []byte

func (s staticSecretGetter) Get(ctx context.Context) ([]byte, error) { return s, nil }

func vcrModel(t *testing.T, cassetteName string) (model *openrouter.Adapter, stop func() error) {
	t.Helper()

	rec := vcrtest.New(t, testdata, cassetteName,
		recorder.WithHook(func(i *cassette.Interaction) error {
			i.Request.Headers.Set("Authorization", "Bearer REDACTED")
			return nil
		}, recorder.BeforeSaveHook),
	)
	key := vcrtest.OpenRouterKey(t)

	// Create openrouter model with VCR transport
	adapter, err := openrouter.New(t.Context(), staticSecretGetter(key),
		append(openrouterMaxTokenConsumptionPerTest,
			openrouter.WithTransport(rec),
			openrouter.WithCacheDir(t.TempDir()),
		)...,
	)
	require.NoError(t, err, "Failed to create OpenRouter client")

	return adapter, rec.Stop
}

func TestAdapter(t *testing.T) {
	adapter, stop := vcrModel(t, "testdata/adapter_test")
	t.Cleanup(func() { require.NoError(t, stop()) })

	chatmodel.RunChatModelTests(adapter)(t)
	embedding.RunToolSemanticIndexTests(adapter)(t)
}
