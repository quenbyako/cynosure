// Package testsuite provides a BDD test suite for the ratelimiter port.
package testsuite

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/cucumber/godog"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/ratelimiter"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

//go:embed features/*.feature
var features embed.FS

var (
	errUserNotFound    = errors.New("user not found")
	errExpectedSuccess = errors.New("expected success")
	errExpectedError   = errors.New("expected error")
)

type SetupParams struct {
	Now   func() time.Time
	Limit int
	Burst int
}

type setupFunc func(context.Context, SetupParams) (ratelimiter.Port, error)

// Run test suite for the Rate Limiter port.
func Run(setup setupFunc) func(t *testing.T) {
	return func(t *testing.T) {
		t.Helper()

		var state godogState

		suite := godog.TestSuite{
			Name:                 "ratelimiter",
			ScenarioInitializer:  state.InitializeScenario(setup),
			TestSuiteInitializer: nil,
			Options:              createOptions(t),
		}

		if exit := suite.Run(); exit != 0 {
			t.Fatal("non-zero status returned, failed to run feature tests")
		}
	}
}

func createOptions(t *testing.T) *godog.Options {
	t.Helper()

	return &godog.Options{
		Format:              "pretty",
		TestingT:            t,
		FS:                  features,
		DefaultContext:      t.Context(),
		ShowStepDefinitions: false,
		Randomize:           0,
		StopOnFailure:       false,
		Strict:              false,
		NoColors:            false,
		Tags:                "",
		Dialect:             "",
		Concurrency:         0,
		Paths:               nil,
		Output:              nil,
		FeatureContents:     nil,
		ShowHelp:            false,
	}
}

type godogState struct {
	setup setupFunc

	adapter     ratelimiter.Port
	currentTime time.Time

	users   map[string]ids.UserID
	lastErr error
}

func (s *godogState) InitializeScenario(setup setupFunc) func(*godog.ScenarioContext) {
	s.setup = setup

	return func(ctx *godog.ScenarioContext) {
		ctx.Before(func(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
			s.reset()

			return ctx, nil
		})

		ctx.Given(`^rate limit is set to (\d+) message per second with burst (\d+)$`,
			s.setupLimiter)
		ctx.Given(`^there is a random user "([^"]*)"$`,
			s.createUser)
		ctx.When(`^user "([^"]*)" consumes (\d+) message$`,
			s.consumeMessages)
		ctx.When(`^time passes for ([-+]?(?:[0-9]*(?:\.[0-9]*)?[a-z]+)+)$`,
			s.timePasses)
		ctx.Then(`^operation is successful$`,
			s.assertSuccess)
		ctx.Then(`^rate limit exceeded error is returned$`,
			s.assertLimitError)
	}
}

func (s *godogState) reset() {
	*s = godogState{
		setup:       s.setup,
		adapter:     nil,
		currentTime: time.Unix(0, 0),
		users:       make(map[string]ids.UserID),
		lastErr:     nil,
	}
}

func (s *godogState) buildAdapter(ctx context.Context, params SetupParams) (err error) {
	s.adapter, err = s.setup(ctx, params)
	return err
}

func (s *godogState) setupLimiter(ctx context.Context, limit, burst int) (err error) {
	return s.buildAdapter(ctx, SetupParams{
		Now:   func() time.Time { return s.currentTime },
		Limit: limit,
		Burst: burst,
	})
}

func (s *godogState) createUser(name string) error {
	s.users[name] = ids.RandomUserID()
	return nil
}

func (s *godogState) timePasses(durStr string) error {
	dur, err := time.ParseDuration(durStr)
	if err != nil {
		return fmt.Errorf("%w: parse duration: %w", godog.ErrAmbiguous, err)
	}

	s.currentTime = s.currentTime.Add(dur)

	return nil
}

func (s *godogState) consumeMessages(name string, count int) error {
	user, ok := s.users[name]
	if !ok {
		return fmt.Errorf("%w: %q", errUserNotFound, name)
	}

	s.lastErr = s.adapter.Consume(context.Background(), user, count)

	return nil
}

func (s *godogState) assertSuccess() error {
	if s.lastErr != nil {
		return fmt.Errorf("%w, got: %w", errExpectedSuccess, s.lastErr)
	}

	return nil
}

func (s *godogState) assertLimitError() error {
	if s.lastErr == nil {
		return fmt.Errorf("%w, got success", errExpectedError)
	}

	if !errors.Is(s.lastErr, ratelimiter.ErrRateLimitExceeded) {
		return fmt.Errorf("expected ErrRateLimitExceeded, got: %w", s.lastErr)
	}

	return nil
}
