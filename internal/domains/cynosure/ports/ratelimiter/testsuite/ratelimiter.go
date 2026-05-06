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
	errExpectedSuccess = errors.New("expected success")
	errExpectedError   = errors.New("expected error")
	errInvalidParam    = errors.New("invalid parameter")
	errNoActiveChat    = errors.New("no active chat request to settle")
	errRetryMismatch   = errors.New("retry time mismatch")
)

const (
	tokenInput     = "input"
	tokenOutput    = "output"
	tokenEmbedding = "embedding"

	maxWaitTime = time.Minute
)

type Quota struct {
	Limit  int
	Period time.Duration
}

type SetupParams struct {
	StartedAt      time.Time
	Now            func() time.Time
	ChatInput      Quota
	ChatOutput     Quota
	EmbeddingInput Quota

	MaxWait time.Duration
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
	currentTime time.Time
	adapter     ratelimiter.Port
	lastErr     error
	setup       setupFunc
	users       map[string]ids.UserID
	lastSettle  ratelimiter.ConsumedTokensFunc
	setupParams SetupParams
	selfTest    bool
}

func (s *godogState) InitializeScenario(setup setupFunc) func(*godog.ScenarioContext) {
	s.setup = setup

	return func(ctx *godog.ScenarioContext) {
		ctx.Before(func(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
			s.reset()

			return ctx, nil
		})

		const (
			tokenExpr = `(` + tokenInput + `|` + tokenOutput + `|` + tokenEmbedding + `)`
		)

		// Setup steps
		ctx.Given(`^`+tokenExpr+` token limit is set to (\d+) token(?:s)? per ([smh0-9.+-]+)$`,
			s.setupTokenLimit)
		ctx.Given(`^maximum wait time is set to ([smh0-9.+-]+)$`, s.setupMaxWait)

		// State setup steps (pre-consumption)
		ctx.Given(`^user has already consumed (\d+) `+tokenExpr+` token(?:s)? for "([^"]*)" model$`,
			s.givenUserAlreadySpent)

		// Action steps
		ctx.When(`^user consumes (\d+) `+tokenExpr+` token(?:s)? for "([^"]*)" model$`,
			s.whenUserConsumes)
		ctx.When(`^time passes for ([smh0-9.+-]+)$`, s.timePasses)

		// Assertion steps
		ctx.Then(`^operation is successful$`, s.assertSuccess)
		ctx.Then(`^rate limit exceeded error is returned$`, s.assertLimitError)
		ctx.Then(`^retry is after ([smh0-9.+-]+)$`, s.assertRetryAfter)
	}
}

func (s *godogState) reset() {
	start := time.Unix(0, 0)
	*s = godogState{
		setup: s.setup,
		setupParams: SetupParams{
			StartedAt:      start,
			Now:            func() time.Time { return s.currentTime },
			ChatInput:      Quota{Limit: 0, Period: 0},
			ChatOutput:     Quota{Limit: 0, Period: 0},
			EmbeddingInput: Quota{Limit: 0, Period: 0},
			MaxWait:        0,
		},
		adapter:     nil,
		lastSettle:  nil,
		lastErr:     nil,
		users:       make(map[string]ids.UserID),
		currentTime: start,
		selfTest:    false,
	}
}

func (s *godogState) setupTokenLimit(typ string, limit int, periodStr string) error {
	dur, err := time.ParseDuration(periodStr)
	if err != nil {
		return fmt.Errorf("%w: parse duration: %w", errInvalidParam, err)
	}

	switch typ {
	case tokenInput:
		s.setupParams.ChatInput.Limit = limit
		s.setupParams.ChatInput.Period = dur

	case tokenOutput:
		s.setupParams.ChatOutput.Limit = limit
		s.setupParams.ChatOutput.Period = dur

	case tokenEmbedding:
		s.setupParams.EmbeddingInput.Limit = limit
		s.setupParams.EmbeddingInput.Period = dur
	}

	return nil
}

func (s *godogState) setupMaxWait(ctx context.Context, durStr string) error {
	dur, err := time.ParseDuration(durStr)
	if err != nil {
		return fmt.Errorf("%w: parse duration: %w", errInvalidParam, err)
	}

	s.setupParams.MaxWait = dur

	return nil
}

func (s *godogState) givenUserAlreadySpent(
	ctx context.Context, count int, typ, model string,
) error {
	if err := s.ensureAdapterCreated(ctx); err != nil {
		return err
	}

	if s.selfTest {
		return nil
	}

	userID := s.getUser("default")

	switch typ {
	case tokenInput:
		return s.spendInput(ctx, userID, model, count)
	case tokenOutput:
		return s.spendOutput(ctx, userID, model, count)
	case tokenEmbedding:
		return s.spendEmbedding(ctx, userID, model, count)
	default:
		return fmt.Errorf("%w: unknown token type: %s", errInvalidParam, typ)
	}
}

func (s *godogState) spendInput(
	ctx context.Context, userID ids.UserID, model string, count int,
) error {
	if _, err := s.adapter.ConsumeChatRequests(ctx, userID, model, count); err != nil {
		return fmt.Errorf("spend input: %w", err)
	}

	return nil
}

func (s *godogState) spendOutput(
	ctx context.Context, userID ids.UserID, model string, count int,
) error {
	settle, err := s.adapter.ConsumeChatRequests(ctx, userID, model, 0)
	if err != nil {
		return fmt.Errorf("spend output (settle): %w", err)
	}

	if err := settle(ctx, count); err != nil {
		return fmt.Errorf("settle output: %w", err)
	}

	return nil
}

func (s *godogState) spendEmbedding(
	ctx context.Context, userID ids.UserID, model string, count int,
) error {
	if err := s.adapter.ConsumeEmbeddingRequests(ctx, userID, model, count); err != nil {
		return fmt.Errorf("spend embedding: %w", err)
	}

	return nil
}

func (s *godogState) whenUserConsumes(ctx context.Context, count int, typ, model string) error {
	if err := s.ensureAdapterCreated(ctx); err != nil {
		return err
	}

	if s.selfTest {
		return nil
	}

	userID := s.getUser("default")

	switch typ {
	case tokenInput:
		s.lastSettle, s.lastErr = s.adapter.ConsumeChatRequests(ctx, userID, model, count)
	case tokenOutput:
		if s.lastSettle == nil {
			return errNoActiveChat
		}

		s.lastErr = s.lastSettle(ctx, count)
		s.lastSettle = nil
	case tokenEmbedding:
		s.lastErr = s.adapter.ConsumeEmbeddingRequests(ctx, userID, model, count)
	}

	return nil
}

func (s *godogState) timePasses(ctx context.Context, durStr string) error {
	dur, err := time.ParseDuration(durStr)
	if err != nil {
		return fmt.Errorf("%w: parse duration: %w", errInvalidParam, err)
	}

	if dur > maxWaitTime {
		return fmt.Errorf("%w: duration %v is too long (max 1m)", errInvalidParam, dur)
	}

	s.currentTime = s.currentTime.Add(dur)

	return nil
}

func (s *godogState) assertSuccess() error {
	if s.selfTest {
		return nil
	}

	if s.lastErr != nil {
		return fmt.Errorf("%w, got: %w", errExpectedSuccess, s.lastErr)
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

	var errExceeded *ratelimiter.RateLimitExceededError
	if !errors.As(s.lastErr, &errExceeded) {
		return fmt.Errorf("expected RateLimitExceededError, got: %T (%w)",
			s.lastErr, s.lastErr)
	}

	return nil
}

func (s *godogState) assertRetryAfter(durStr string) error {
	expectedDur, err := time.ParseDuration(durStr)
	if err != nil {
		return fmt.Errorf("%w: parse duration: %w", errInvalidParam, err)
	}

	expectedAt := s.currentTime.Add(expectedDur)

	if s.selfTest {
		return nil
	}

	errExceeded := new(ratelimiter.RateLimitExceededError)
	if !errors.As(s.lastErr, &errExceeded) {
		return fmt.Errorf("expected RateLimitExceededError, got: %w", s.lastErr)
	}

	diff := errExceeded.RetryAt().Sub(expectedAt).Abs()
	if diff > 5*time.Second {
		return fmt.Errorf("%w: expected retry at %v, got %v (diff %v)",
			errRetryMismatch, expectedAt, errExceeded.RetryAt(), diff)
	}

	return nil
}

func (s *godogState) ensureAdapterCreated(ctx context.Context) (err error) {
	if s.adapter != nil {
		return nil
	}

	s.adapter, err = s.setup(ctx, s.setupParams)
	if e := new(SelfTestError); errors.As(err, &e) {
		s.selfTest = true
		return nil
	}

	if err != nil {
		return fmt.Errorf("setup adapter: %w", err)
	}

	return nil
}

func (s *godogState) getUser(name string) ids.UserID {
	user, ok := s.users[name]
	if !ok {
		user = ids.RandomUserID()
		s.users[name] = user
	}

	return user
}

type SelfTestError struct{}

func (e *SelfTestError) Error() string { return "self test" }
