package testsuite

import (
	"errors"
	"testing"

	suites "github.com/quenbyako/cynosure/contrib/bettersuites"
	"github.com/stretchr/testify/require"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/ids"
)

// RunModelSettingsStorageTests runs tests for the given adapter. These tests are predefined
// and REQUIRED to be used for ANY adapter implementation.
func RunModelSettingsStorageTests(a ports.ModelSettingsStorage, opts ...ModelSettingsStorageTestSuiteOption) func(t *testing.T) {
	s := &ModelSettingsStorageTestSuite{
		adapter: a,
	}
	for _, opt := range opts {
		opt(s)
	}
	if err := s.validate(); err != nil {
		panic(err)
	}

	return suites.Run(s)
}

type ModelSettingsStorageTestSuite struct {
	adapter ports.ModelSettingsStorage

	cleanup func() error
}

var _ suites.AfterTest = (*ModelSettingsStorageTestSuite)(nil)

type ModelSettingsStorageTestSuiteOption func(*ModelSettingsStorageTestSuite)

func WithModelSettingsStorageCleanup(f func() error) ModelSettingsStorageTestSuiteOption {
	return func(s *ModelSettingsStorageTestSuite) { s.cleanup = f }
}

func (s *ModelSettingsStorageTestSuite) validate() error {
	if s.adapter == nil {
		return errors.New("adapter is nil")
	}

	return nil
}

func (s *ModelSettingsStorageTestSuite) AfterTest(t *testing.T) {
	if s.cleanup != nil {
		if err := s.cleanup(); err != nil {
			t.Fatalf("cleanup failed: %v", err)
		}
	}
}

// TODO: this test is poorly formatted, it's extremely necessary to refactor it
// TODO: need to verify that adapters understand filtering by user id, by
// creating two users with two ids and retrieving models for each of them.
func (s *ModelSettingsStorageTestSuite) TestSaveModel(t *testing.T) {
	modelID := ids.RandomModelConfigID()
	userID := ids.RandomUserID()

	model := must(entities.NewModelSettings(
		modelID,
		"oompa-loompa-6000",
		entities.WithSystemMessage("You are a helpful Oompa-Loompa... Wait, what?"),
		entities.WithTemperature(0.7),
		entities.WithTopP(0.9),
		entities.WithStopWords([]string{"STOP"}),
	))

	t.Run("saving_model", func(t *testing.T) {
		err := s.adapter.SaveModel(t.Context(), model)
		require.NoError(t, err, "failed to save model")
	})

	t.Run("retrieving_model", func(t *testing.T) {
		retrieved, err := s.adapter.GetModel(t.Context(), modelID)
		require.NoError(t, err, "failed to get model")
		require.NotNil(t, retrieved)
		require.Equal(t, model.Model(), retrieved.Model())
		require.Equal(t, model.SystemMessage(), retrieved.SystemMessage())
		require.Equal(t, asResult(model.Temperature()), asResult(retrieved.Temperature()))
		require.Equal(t, asResult(model.TopP()), asResult(retrieved.TopP()))
		require.Equal(t, model.StopWords(), retrieved.StopWords())
	})

	t.Run("listing_models", func(t *testing.T) {
		models, err := s.adapter.ListModels(t.Context(), userID)
		require.NoError(t, err, "failed to list models")
		require.GreaterOrEqual(t, len(models), 1)

		// Find our model in the list
		found := false
		for _, m := range models {
			if m.ID() == modelID {
				found = true
				require.Equal(t, model.Model(), m.Model())
				break
			}
		}
		require.True(t, found, "saved model not found in list")
	})

	t.Run("deleting_model", func(t *testing.T) {
		err := s.adapter.DeleteModel(t.Context(), modelID)
		require.NoError(t, err, "failed to delete model")

		// Verify deletion
		_, err = s.adapter.GetModel(t.Context(), modelID)
		require.Error(t, err, "model should not be found after deletion")
	})
}

type result[T any] struct {
	value T
	ok    bool
}

func asResult[T any](value T, ok bool) result[T] {
	return result[T]{value: value, ok: ok}
}
