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

type Quota struct {
	Limit  int
	Period time.Duration
}

type SetupParams struct {
	Now      func() time.Time
	Messages Quota
	Tokens   Quota
	MaxWait  time.Duration
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
		Strict:              true,
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
	setupParams SetupParams
	setup       setupFunc

	adapter     ratelimiter.Port
	currentTime time.Time

	users   map[string]ids.UserID
	lastErr error

	// This flag is responsible to verify gherkin configuration.
	selfTest bool
}

func (s *godogState) InitializeScenario(setup setupFunc) func(*godog.ScenarioContext) {
	s.setup = setup

	return func(ctx *godog.ScenarioContext) {
		ctx.Before(func(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
			s.reset()

			return ctx, nil
		})

		// Setup steps
		ctx.Given(`^rate limit is set to (\d+) message(?:s)? per ([smh0-9.+-]+)$`, s.setupMessageLimit)
		ctx.Given(`^token limit is set to (\d+) token(?:s)? per ([smh0-9.+-]+)$`, s.setupTokenLimit)
		ctx.Given(`^maximum wait time is set to ([smh0-9.+-]+)$`, s.setupMaxWait)

		// State setup steps (pre-consumption)
		ctx.Given(`^user has already sent (\d+) message(?:s)? for "([^"]*)" model$`, s.givenUserAlreadySent)
		ctx.Given(`^user has already spent (\d+) token(?:s)? for "([^"]*)" model$`, s.givenUserAlreadySpent)

		// Action steps
		ctx.When(`^user consumes (\d+) message(?:s)? for "([^"]*)" model$`, s.whenUserConsumes)
		ctx.When(`^user settles (\d+) token(?:s)? for "([^"]*)" model$`, s.whenUserSettles)
		ctx.Step(`^time passes for ([smh0-9.+-]+)$`, s.timePasses)

		// Assertion steps
		ctx.Then(`^operation is successful$`, s.assertSuccess)
		ctx.Then(`^rate limit exceeded error is returned$`, s.assertLimitError)
		ctx.Then(`^retry is after ([smh0-9.+-]+)$`, s.assertRetryAfter)
	}
}

func (s *godogState) reset() {
	*s = godogState{
		setupParams: SetupParams{
			Now:      func() time.Time { return s.currentTime },
			Messages: Quota{},
			Tokens:   Quota{},
			MaxWait:  0,
		},

		setup:       s.setup,
		adapter:     nil,
		currentTime: time.Unix(0, 0),
		users:       make(map[string]ids.UserID),
		lastErr:     nil,
	}
}

func (s *godogState) setupMessageLimit(limit int, periodStr string) error {
	dur, err := time.ParseDuration(periodStr)
	if err != nil {
		return err
	}
	s.setupParams.Messages.Limit = limit
	s.setupParams.Messages.Period = dur
	return nil
}

func (s *godogState) setupTokenLimit(limit int, periodStr string) error {
	dur, err := time.ParseDuration(periodStr)
	if err != nil {
		return err
	}
	s.setupParams.Tokens.Limit = limit
	s.setupParams.Tokens.Period = dur
	return nil
}

func (s *godogState) setupMaxWait(durStr string) error {
	dur, err := time.ParseDuration(durStr)
	if err != nil {
		return err
	}
	s.setupParams.MaxWait = dur
	return nil
}

func (s *godogState) givenUserAlreadySent(ctx context.Context, count int) error {
	return s.whenUserConsumes(ctx, count, "default")
}

func (s *godogState) givenUserAlreadySpent(ctx context.Context, tokens int, model string) error {
	if err := s.ensureAdapterCreated(ctx); err != nil {
		return err
	}
	if s.selfTest {
		return nil
	}
	userID := s.getUser("default")

	settle, err := s.adapter.ConsumeChatRequests(ctx, userID, "default", 0)
	if err != nil {
		return err
	}
	return settle(ctx, tokens)
}

func (s *godogState) whenUserConsumes(ctx context.Context, count int, model string) error {
	if err := s.ensureAdapterCreated(ctx); err != nil {
		return err
	}
	if s.selfTest {
		return nil
	}
	userID := s.getUser("default")
	_, s.lastErr = s.adapter.ConsumeChatRequests(ctx, userID, model, count)
	return nil
}

func (s *godogState) whenUserSettles(ctx context.Context, tokens int, model string) error {
	if err := s.ensureAdapterCreated(ctx); err != nil {
		return err
	}
	if s.selfTest {
		return nil
	}
	userID := s.getUser("default")
	settle, err := s.adapter.ConsumeChatRequests(ctx, userID, model, 1)
	if err != nil {
		return err
	}
	return settle(ctx, tokens)
}

func (s *godogState) timePasses(durStr string) error {
	dur, err := time.ParseDuration(durStr)
	if err != nil {
		return err
	}
	s.currentTime = s.currentTime.Add(dur)
	return nil
}

func (s *godogState) assertSuccess() error {
	if s.selfTest {
		return nil
	}
	if s.lastErr != nil {
		return fmt.Errorf("%w, got: %v", errExpectedSuccess, s.lastErr)
	}

	return nil
}

func (s *godogState) assertLimitError() error {
	if s.selfTest {
		return nil
	}
	if s.lastErr == nil {
		return errExpectedError
	}
	var e *ratelimiter.RateLimitExceededError
	if !errors.As(s.lastErr, &e) {
		return fmt.Errorf("expected RateLimitExceededError, got: %T (%v)", s.lastErr, s.lastErr)
	}
	return nil
}

func (s *godogState) assertRetryAfter(durStr string) error {
	expectedDur, err := time.ParseDuration(durStr)
	if err != nil {
		return err
	}
	expectedAt := s.currentTime.Add(expectedDur)

	if s.selfTest {
		return nil
	}

	var e *ratelimiter.RateLimitExceededError
	if !errors.As(s.lastErr, &e) {
		return fmt.Errorf("expected RateLimitExceededError, got: %v", s.lastErr)
	}

	diff := e.RetryAt().Sub(expectedAt).Abs()
	if diff > time.Second {
		return fmt.Errorf("expected retry at %v, got %v (diff %v)", expectedAt, e.RetryAt(), diff)
	}

	return nil
}

func (s *godogState) ensureAdapterCreated(ctx context.Context) (err error) {
	if s.adapter != nil {
		return nil
	}
	s.adapter, err = s.setup(ctx, s.setupParams)
	if e := new(selfTestError); errors.As(err, &e) {
		s.selfTest = true
		return nil
	}
	return err
}

func (s *godogState) getUser(name string) ids.UserID {
	user, ok := s.users[name]
	if !ok {
		user = ids.RandomUserID()
		s.users[name] = user
	}
	return user
}

type selfTestError struct{}

func (e *selfTestError) Error() string { return "self test" }
