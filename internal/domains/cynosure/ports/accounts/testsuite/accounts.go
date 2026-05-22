// Package testsuite provides a BDD test suite for the accounts port.
package testsuite

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/cucumber/godog"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/accounts"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

//go:embed features/*.feature
var features embed.FS

var (
	errExpectedSuccess          = errors.New("expected success")
	errExpectedNotFound         = errors.New("expected not found error")
	errExpectedInvalidAccountID = errors.New("expected invalid account id error")
	errVerificationFailed       = errors.New("verification failed")
	errNotFoundInState          = errors.New("not found in test state")
	errRandomAccountIDFailed    = errors.New("failed to generate random account id")
	errUnexpectedState          = errors.New("unexpected state")
)

type SaveAccountFixture struct {
	Name        string
	Description string
	AccountID   ids.AccountID
}

type FixtureBuilder = func(context.Context, SaveAccountFixture) error

type runParams struct {
	saveAccountFixture FixtureBuilder
	cleanup            func(context.Context) error
}

type AccountStorageTestSuiteOption func(*runParams)

func WithSaveAccountSeeder(f FixtureBuilder) AccountStorageTestSuiteOption {
	return func(s *runParams) { s.saveAccountFixture = f }
}

func WithAccountStorageCleanup(f func(context.Context) error) AccountStorageTestSuiteOption {
	return func(s *runParams) { s.cleanup = f }
}

type SelfTestError struct{}

func (e *SelfTestError) Error() string { return "self test" }

type setupFunc func(context.Context) (accounts.Port, error)

// Run test suite for the Accounts port.
func Run(
	setup setupFunc,
	opts ...AccountStorageTestSuiteOption,
) func(t *testing.T) {
	params := runParams{
		saveAccountFixture: nil,
		cleanup:            func(ctx context.Context) error { return nil },
	}

	for _, opt := range opts {
		opt(&params)
	}

	return func(t *testing.T) {
		t.Helper()

		runTestSuite(t, setup, params)
	}
}

func runTestSuite(t *testing.T, setup setupFunc, params runParams) {
	t.Helper()

	var state godogState

	suite := godog.TestSuite{
		Name: "accounts",
		ScenarioInitializer: state.InitializeScenario(
			setup,
			params.saveAccountFixture,
			params.cleanup,
		),
		TestSuiteInitializer: nil,
		Options:              createOptions(t),
	}

	if suite.Run() != 0 {
		t.Fatal("non-zero status returned, failed to run feature tests")
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
	setup              setupFunc
	adapter            accounts.Port
	lastErr            error
	saveAccountFixture FixtureBuilder
	accounts           map[string]*entities.Account
	accountIDs         map[string]ids.AccountID
	lastRetrieved      *entities.Account
	searchResults      []accounts.SearchResult
	listedIDs          []ids.AccountID
	batchResults       []*entities.Account
	defaultUser        ids.UserID
	selfTest           bool
}

func (s *godogState) InitializeScenario(
	setup setupFunc,
	saveAccountFixture FixtureBuilder,
	cleanup func(context.Context) error,
) func(*godog.ScenarioContext) {
	s.setup = setup
	s.saveAccountFixture = saveAccountFixture

	return func(ctx *godog.ScenarioContext) {
		ctx.Before(func(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
			s.reset()

			if err := cleanup(ctx); err != nil {
				return ctx, fmt.Errorf("cleanup failed: %w", err)
			}

			return ctx, nil
		})

		s.registerSteps(ctx)
	}
}

func (s *godogState) registerSteps(ctx *godog.ScenarioContext) {
	s.registerGivenSteps(ctx)
	s.registerWhenSteps(ctx)
	s.registerThenSteps(ctx)
}

func (s *godogState) registerGivenSteps(ctx *godog.ScenarioContext) {
	ctx.Given(`^account "([^"]*)" is saved with name "([^"]*)" and description "([^"]*)"$`,
		s.givenAccountSavedWithNameAndDesc)
	ctx.Given(`^account "([^"]*)" is deleted$`,
		s.givenAccountDeleted)
}

func (s *godogState) registerWhenSteps(ctx *godog.ScenarioContext) {
	ctx.When(`^I save account "([^"]*)" with name "([^"]*)" and description "([^"]*)"$`,
		s.whenISaveAccount)
	ctx.When(`^I get account "([^"]*)"$`,
		s.whenIGetAccount)
	ctx.When(`^I get account with invalid account ID$`,
		s.whenIGetAccountWithInvalidID)
	ctx.When(`^I list accounts$`,
		s.whenIListAccounts)
	ctx.When(`^I delete account "([^"]*)"$`,
		s.whenIDeleteAccount)
	ctx.When(`^I get account "([^"]*)" including deleted$`,
		s.whenIGetAccountIncludingDeleted)
	ctx.When(`^I reactivate account "([^"]*)"$`,
		s.whenIReactivateAccount)
	ctx.When(`^I find accounts by name "([^"]*)"$`,
		s.whenIFindAccountsByName)
	ctx.When(`^I get accounts batch for "([^"]*)"$`,
		s.whenIGetAccountsBatch)
}

func (s *godogState) registerThenSteps(ctx *godog.ScenarioContext) {
	ctx.Then(`^the operation is successful$`,
		s.thenOperationSuccessful)
	ctx.Then(`^I can get account "([^"]*)" details$`,
		s.andICanGetAccountDetails)
	ctx.Then(`^the retrieved account has name "([^"]*)" and description "([^"]*)"$`,
		s.andRetrievedAccountHasNameAndDesc)
	ctx.Then(`^a not found error is returned$`,
		s.thenNotFoundError)
	ctx.Then(`^an invalid account ID error is returned$`,
		s.thenInvalidAccountIDError)
	ctx.Then(`^the list contains "([^"]*)"$`,
		s.thenListContains)
	ctx.Then(`^the list does not contain "([^"]*)"$`,
		s.thenListDoesNotContain)
	ctx.Then(`^the search results contain active account "([^"]*)"$`,
		s.thenSearchResultsContainActive)
	ctx.Then(`^the search results contain deleted account "([^"]*)"$`,
		s.thenSearchResultsContainDeleted)
	ctx.Then(`^the search results do not contain "([^"]*)"$`,
		s.thenSearchResultsDoNotContain)
	ctx.Then(`^the batch contains "([^"]*)"$`,
		s.thenBatchContains)
	ctx.Then(`^the batch does not contain "([^"]*)"$`,
		s.thenBatchDoesNotContain)
}

func (s *godogState) reset() {
	s.lastErr = nil
	s.accounts = make(map[string]*entities.Account)
	s.accountIDs = make(map[string]ids.AccountID)
	s.defaultUser = ids.RandomUserID()
	s.searchResults = nil
	s.listedIDs = nil
	s.batchResults = nil
	s.lastRetrieved = nil
	s.selfTest = false
	s.adapter = nil
}

func (s *godogState) ensureAdapterCreated(ctx context.Context) error {
	if s.adapter != nil {
		return nil
	}

	var err error

	s.adapter, err = s.setup(ctx)
	if e := new(SelfTestError); errors.As(err, &e) {
		s.selfTest = true

		return nil
	}

	if err != nil {
		return fmt.Errorf("setup adapter: %w", err)
	}

	return nil
}

func (s *godogState) getOrCreateAccountID(name string) (ids.AccountID, error) {
	accountID, ok := s.accountIDs[name]
	if ok {
		return accountID, nil
	}

	serverID := ids.RandomServerID()

	accountID, err := ids.RandomAccountID(s.defaultUser, serverID)
	if err != nil {
		return ids.AccountID{}, fmt.Errorf("%w: %w", errRandomAccountIDFailed, err)
	}

	s.accountIDs[name] = accountID

	return accountID, nil
}

func (s *godogState) runSaveAccountFixture(
	ctx context.Context,
	accountID ids.AccountID,
	accName, description string,
) error {
	if s.saveAccountFixture == nil {
		return nil
	}

	fixture := SaveAccountFixture{
		Name:        accName,
		Description: description,
		AccountID:   accountID,
	}

	if err := s.saveAccountFixture(ctx, fixture); err != nil {
		return fmt.Errorf("%w: failed to setup fixture: %w", errVerificationFailed, err)
	}

	return nil
}

func (s *godogState) whenISaveAccount(
	ctx context.Context, name, accName, description string,
) error {
	if err := s.ensureAdapterCreated(ctx); err != nil || s.selfTest {
		return err
	}

	accountID, err := s.getOrCreateAccountID(name)
	if err != nil {
		return err
	}

	if err = s.runSaveAccountFixture(ctx, accountID, accName, description); err != nil {
		return err
	}

	account, err := entities.NewAccount(accountID, accName, description)
	if err != nil {
		return fmt.Errorf("%w: new account: %w", errVerificationFailed, err)
	}

	s.accounts[name] = account
	s.lastErr = s.adapter.SaveAccount(ctx, account)

	return nil
}

func (s *godogState) thenOperationSuccessful() error {
	if s.selfTest {
		return nil
	}

	if s.lastErr != nil {
		return fmt.Errorf("%w: got: %w", errExpectedSuccess, s.lastErr)
	}

	return nil
}

func (s *godogState) andICanGetAccountDetails(ctx context.Context, name string) error {
	if err := s.ensureAdapterCreated(ctx); err != nil || s.selfTest {
		return err
	}

	accountID, ok := s.accountIDs[name]
	if !ok {
		return fmt.Errorf("%w: account %s not found in test state", errNotFoundInState, name)
	}

	got, err := s.adapter.GetAccount(ctx, accountID)
	if err != nil {
		return fmt.Errorf("%w: get account: %w", errVerificationFailed, err)
	}

	s.lastRetrieved = got

	return nil
}

func (s *godogState) andRetrievedAccountHasNameAndDesc(name, description string) error {
	if s.selfTest {
		return nil
	}

	if s.lastRetrieved == nil {
		return fmt.Errorf("%w: no retrieved account to verify", errUnexpectedState)
	}

	if s.lastRetrieved.Name() != name {
		return fmt.Errorf("%w: expected name %q, got %q",
			errVerificationFailed, name, s.lastRetrieved.Name())
	}

	if s.lastRetrieved.Description() != description {
		return fmt.Errorf("%w: expected description %q, got %q",
			errVerificationFailed, description, s.lastRetrieved.Description())
	}

	return nil
}

func (s *godogState) whenIGetAccount(ctx context.Context, name string) error {
	if err := s.ensureAdapterCreated(ctx); err != nil || s.selfTest {
		return err
	}

	accountID, ok := s.accountIDs[name]
	if !ok {
		serverID := ids.RandomServerID()

		var err error

		accountID, err = ids.RandomAccountID(s.defaultUser, serverID)
		if err != nil {
			return fmt.Errorf("%w: random account id: %w", errRandomAccountIDFailed, err)
		}
	}

	s.lastRetrieved, s.lastErr = s.adapter.GetAccount(ctx, accountID)

	return nil
}

func (s *godogState) thenNotFoundError() error {
	if s.selfTest {
		return nil
	}

	if !errors.Is(s.lastErr, accounts.ErrNotFound) {
		return fmt.Errorf("%w: expected ErrNotFound, got: %w", errExpectedNotFound, s.lastErr)
	}

	return nil
}

func (s *godogState) whenIGetAccountWithInvalidID(ctx context.Context) error {
	if err := s.ensureAdapterCreated(ctx); err != nil || s.selfTest {
		return err
	}

	_, s.lastErr = s.adapter.GetAccount(ctx, ids.AccountID{})

	return nil
}

func (s *godogState) thenInvalidAccountIDError() error {
	if s.selfTest {
		return nil
	}

	if !errors.Is(s.lastErr, accounts.ErrInvalidAccountID) {
		return fmt.Errorf("%w: expected ErrInvalidAccountID, got: %w",
			errExpectedInvalidAccountID, s.lastErr)
	}

	return nil
}

func (s *godogState) givenAccountSavedWithNameAndDesc(
	ctx context.Context,
	name, accName, description string,
) error {
	if err := s.whenISaveAccount(ctx, name, accName, description); err != nil {
		return err
	}

	return s.thenOperationSuccessful()
}

func (s *godogState) givenAccountDeleted(ctx context.Context, name string) error {
	if err := s.ensureAdapterCreated(ctx); err != nil || s.selfTest {
		return err
	}

	id, ok := s.accountIDs[name]
	if !ok {
		return fmt.Errorf("%w: account %s not found in test state", errNotFoundInState, name)
	}

	if err := s.adapter.DeleteAccount(ctx, id); err != nil {
		return fmt.Errorf("%w: delete account: %w", errVerificationFailed, err)
	}

	return nil
}

func (s *godogState) whenIListAccounts(ctx context.Context) error {
	if err := s.ensureAdapterCreated(ctx); err != nil || s.selfTest {
		return err
	}

	var err error

	s.listedIDs, err = s.adapter.ListAccounts(ctx, s.defaultUser)
	if err != nil {
		s.lastErr = err

		return fmt.Errorf("%w: list accounts: %w", errVerificationFailed, err)
	}

	return nil
}

func (s *godogState) containsID(list []ids.AccountID, id ids.AccountID) bool {
	for _, item := range list {
		if item == id {
			return true
		}
	}

	return false
}

func (s *godogState) thenListContains(name string) error {
	if s.selfTest {
		return nil
	}

	id, ok := s.accountIDs[name]
	if !ok {
		return fmt.Errorf("%w: %s not found in test state", errNotFoundInState, name)
	}

	if !s.containsID(s.listedIDs, id) {
		return fmt.Errorf("%w: list does not contain %s", errVerificationFailed, name)
	}

	return nil
}

func (s *godogState) thenListDoesNotContain(name string) error {
	if s.selfTest {
		return nil
	}

	id, ok := s.accountIDs[name]
	if !ok {
		return nil
	}

	if s.containsID(s.listedIDs, id) {
		return fmt.Errorf("%w: list unexpectedly contains %s", errVerificationFailed, name)
	}

	return nil
}

func (s *godogState) whenIDeleteAccount(ctx context.Context, name string) error {
	if err := s.ensureAdapterCreated(ctx); err != nil || s.selfTest {
		return err
	}

	id, ok := s.accountIDs[name]
	if !ok {
		return fmt.Errorf("%w: account %s not found in test state", errNotFoundInState, name)
	}

	s.lastErr = s.adapter.DeleteAccount(ctx, id)

	return nil
}

func (s *godogState) whenIGetAccountIncludingDeleted(ctx context.Context, name string) error {
	if err := s.ensureAdapterCreated(ctx); err != nil || s.selfTest {
		return err
	}

	id, ok := s.accountIDs[name]
	if !ok {
		return fmt.Errorf("%w: account %s not found in test state", errNotFoundInState, name)
	}

	s.lastRetrieved, s.lastErr = s.adapter.GetAccount(ctx, id, accounts.WithIncludeDeleted())

	return nil
}

func (s *godogState) whenIReactivateAccount(ctx context.Context, name string) error {
	if err := s.ensureAdapterCreated(ctx); err != nil || s.selfTest {
		return err
	}

	id, ok := s.accountIDs[name]
	if !ok {
		return fmt.Errorf("%w: account %s not found in test state", errNotFoundInState, name)
	}

	s.lastErr = s.adapter.ReactivateAccount(ctx, id)

	return nil
}

func (s *godogState) whenIFindAccountsByName(ctx context.Context, query string) error {
	if err := s.ensureAdapterCreated(ctx); err != nil || s.selfTest {
		return err
	}

	var err error

	s.searchResults, err = s.adapter.FindAccountsByName(ctx, s.defaultUser, query)
	if err != nil {
		s.lastErr = err

		return fmt.Errorf("%w: find accounts by name: %w", errVerificationFailed, err)
	}

	return nil
}

func containsSearchResult(
	results []accounts.SearchResult,
	target ids.AccountID,
	expectActive bool,
) bool {
	for _, res := range results {
		if res.ID() != target {
			continue
		}

		_, isActive := res.(accounts.SearchResultActive)
		if isActive == expectActive {
			return true
		}
	}

	return false
}

func (s *godogState) thenSearchResultsContainActive(name string) error {
	if s.selfTest {
		return nil
	}

	id, ok := s.accountIDs[name]
	if !ok {
		return fmt.Errorf("%w: %s not found in test state", errNotFoundInState, name)
	}

	if !containsSearchResult(s.searchResults, id, true) {
		return fmt.Errorf("%w: search results do not contain active account %s",
			errVerificationFailed, name)
	}

	return nil
}

func (s *godogState) thenSearchResultsContainDeleted(name string) error {
	if s.selfTest {
		return nil
	}

	id, ok := s.accountIDs[name]
	if !ok {
		return fmt.Errorf("%w: %s not found in test state", errNotFoundInState, name)
	}

	if !containsSearchResult(s.searchResults, id, false) {
		return fmt.Errorf("%w: search results do not contain deleted account %s",
			errVerificationFailed, name)
	}

	return nil
}

func (s *godogState) thenSearchResultsDoNotContain(name string) error {
	if s.selfTest {
		return nil
	}

	id, ok := s.accountIDs[name]
	if !ok {
		return nil
	}

	for _, res := range s.searchResults {
		if res.ID() == id {
			return fmt.Errorf(
				"%w: search results unexpectedly contain account %s",
				errVerificationFailed, name)
		}
	}

	return nil
}

func (s *godogState) resolveBatchIDs(names []string) ([]ids.AccountID, error) {
	var batchIDs []ids.AccountID

	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		id, err := s.getOrCreateAccountID(name)
		if err != nil {
			return nil, err
		}

		batchIDs = append(batchIDs, id)
	}

	return batchIDs, nil
}

func (s *godogState) whenIGetAccountsBatch(ctx context.Context, namesStr string) error {
	if err := s.ensureAdapterCreated(ctx); err != nil || s.selfTest {
		return err
	}

	batchIDs, err := s.resolveBatchIDs(strings.Split(namesStr, ","))
	if err != nil {
		return err
	}

	res, err := s.adapter.GetAccountsBatch(ctx, batchIDs)
	if err != nil {
		s.lastErr = err

		return fmt.Errorf("%w: get accounts batch: %w", errVerificationFailed, err)
	}

	s.batchResults = res

	return nil
}

func containsAccountID(list []*entities.Account, id ids.AccountID) bool {
	for _, item := range list {
		if item != nil && item.ID() == id {
			return true
		}
	}

	return false
}

func (s *godogState) thenBatchContains(name string) error {
	if s.selfTest {
		return nil
	}

	id, ok := s.accountIDs[name]
	if !ok {
		return fmt.Errorf("%w: %s not found in test state", errNotFoundInState, name)
	}

	if !containsAccountID(s.batchResults, id) {
		return fmt.Errorf("%w: batch results do not contain %s", errVerificationFailed, name)
	}

	return nil
}

func (s *godogState) thenBatchDoesNotContain(name string) error {
	if s.selfTest {
		return nil
	}

	id, ok := s.accountIDs[name]
	if !ok {
		return nil
	}

	if containsAccountID(s.batchResults, id) {
		return fmt.Errorf("%w: batch results unexpectedly contain %s", errVerificationFailed, name)
	}

	return nil
}
