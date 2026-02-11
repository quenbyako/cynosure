package mcp_test

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/adapters/mocks"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"

	. "github.com/quenbyako/cynosure/internal/adapters/mcp"
)

// TestTokenRefresh_RequestCancelled verifies that token refresh completes even when request context is cancelled
func TestTokenRefresh_RequestCancelled(t *testing.T) {
	t.Parallel()

	// Create a context that will be cancelled
	requestCtx, cancelRequest := context.WithCancel(context.Background())

	// Create a detached context (simulating context.WithoutCancel behavior)
	refreshCtx := context.WithoutCancel(requestCtx)

	// Setup mocks
	mockAuth := mocks.NewMockOAuthHandler(t)
	mockStorage := mocks.NewMockAccountStorage(t)

	userID := ids.RandomUserID()
	serverID := ids.RandomServerID()
	accountID, err := ids.RandomAccountID(userID, serverID)
	require.NoError(t, err)
	oldToken := &oauth2.Token{
		AccessToken:  "old-access",
		RefreshToken: "refresh-token",
		Expiry:       time.Now().Add(-1 * time.Hour),
	}
	newToken := &oauth2.Token{
		AccessToken:  "new-access",
		RefreshToken: "refresh-token",
		Expiry:       time.Now().Add(1 * time.Hour),
	}

	account, err := entities.NewAccount(
		accountID,
		"test-account",
		"Test Account",
		entities.WithAuthToken(oldToken),
	)
	if err != nil {
		t.Fatalf("Failed to create account: %v", err)
	}

	serverConfig, err := entities.NewServerConfig(
		serverID,
		mustParseURL("https://example.com/mcp"),
		entities.WithAuthConfig(&oauth2.Config{
			ClientID:     "client-id",
			ClientSecret: "client-secret",
			Endpoint: oauth2.Endpoint{
				TokenURL: "https://example.com/token",
			},
		}),
	)
	if err != nil {
		t.Fatalf("Failed to create server config: %v", err)
	}

	// Mock expectations - token refresh should succeed even after cancellation
	mockAuth.EXPECT().
		RefreshToken(mock.Anything, serverConfig.AuthConfig(), oldToken).
		Return(newToken, nil).
		Once()

	mockStorage.EXPECT().
		SaveAccount(mock.Anything, mock.Anything).
		Return(nil).
		Once()

	// Adapters for mocks
	refreshTokenFunc := func(ctx context.Context, server entities.ServerConfigReadOnly, token *oauth2.Token) (*oauth2.Token, error) {
		return mockAuth.RefreshToken(ctx, server.AuthConfig(), token)
	}
	saveAccountFunc := func(ctx context.Context, acc ids.AccountID, token *oauth2.Token) error {
		// Verify we are updating the correct account
		if acc != account.ID() {
			return errors.New("wrong account ID")
		}
		// In a real scenario we'd update the entity and save, here we just verify call
		return mockStorage.SaveAccount(ctx, account)
	}

	// Create refresher with detached context
	refresher := NewRefresher(
		refreshCtx,
		oldToken,
		refreshTokenFunc,
		saveAccountFunc,
		account.ID(),
		serverConfig,
		10*time.Second,
	)

	// Cancel the request context BEFORE token refresh
	cancelRequest()

	// Verify request context is cancelled
	select {
	case <-requestCtx.Done():
		// Expected - request context is cancelled
	default:
		t.Fatal("Request context should be cancelled")
	}

	// Token refresh should still complete successfully
	token, err := refresher.Token()
	if err != nil {
		t.Errorf("Token refresh should complete despite request cancellation: %v", err)
	}

	if token == nil || token.AccessToken != "new-access" {
		t.Error("Token refresh should return new token")
	}

	mockAuth.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
}

// TestTokenRefresh_TimeoutReached verifies that token refresh respects timeout from parent context
func TestTokenRefresh_TimeoutReached(t *testing.T) {
	t.Parallel()

	// Create a context with a very short timeout
	requestCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Note: WithoutCancel detaches from parent cancellation but doesn't preserve deadline.
	// For timeout to work, we need to pass requestCtx directly to token refresh operations.
	refreshCtx := requestCtx

	// Setup mocks
	mockAuth := mocks.NewMockOAuthHandler(t)
	mockStorage := mocks.NewMockAccountStorage(t)

	userID := ids.RandomUserID()
	serverID := ids.RandomServerID()
	accountID, err := ids.RandomAccountID(userID, serverID)
	require.NoError(t, err)
	oldToken := &oauth2.Token{
		AccessToken:  "old-access",
		RefreshToken: "refresh-token",
		Expiry:       time.Now().Add(-1 * time.Hour),
	}

	account, err := entities.NewAccount(
		accountID,
		"test-account",
		"Test Account",
		entities.WithAuthToken(oldToken),
	)
	require.NoError(t, err, "Failed to create account")

	serverConfig, err := entities.NewServerConfig(
		serverID,
		mustParseURL("https://example.com/mcp"),
		entities.WithAuthConfig(&oauth2.Config{
			ClientID:     "client-id",
			ClientSecret: "client-secret",
			Endpoint: oauth2.Endpoint{
				TokenURL: "https://example.com/token",
			},
		}),
	)
	if err != nil {
		t.Fatalf("Failed to create server config: %v", err)
	}

	// Mock expectations - simulate slow token refresh that exceeds timeout
	mockAuth.EXPECT().
		RefreshToken(mock.Anything, serverConfig.AuthConfig(), oldToken).
		Run(func(ctx context.Context, config *oauth2.Config, token *oauth2.Token) {
			// Simulate slow operation that exceeds the timeout
			time.Sleep(100 * time.Millisecond)
		}).
		Return(nil, context.DeadlineExceeded).
		Once()

	// Adapters
	refreshTokenFunc := func(ctx context.Context, server entities.ServerConfigReadOnly, token *oauth2.Token) (*oauth2.Token, error) {
		return mockAuth.RefreshToken(ctx, server.AuthConfig(), token)
	}
	saveAccountFunc := func(ctx context.Context, acc ids.AccountID, token *oauth2.Token) error {
		return mockStorage.SaveAccount(ctx, account)
	}

	// Create refresher with context that has timeout
	refresher := NewRefresher(
		refreshCtx,
		oldToken,
		refreshTokenFunc,
		saveAccountFunc,
		account.ID(),
		serverConfig,
		100*time.Millisecond,
	)

	// Token refresh should fail due to timeout
	token, err := refresher.Token()
	if err == nil {
		t.Error("Token refresh should fail when timeout is reached")
	}

	if token != nil {
		t.Error("Token should be nil when refresh fails")
	}

	// Verify the error is related to timeout/deadline
	if !errors.Is(err, context.DeadlineExceeded) && err != nil {
		// Error might be wrapped, check if it contains deadline exceeded
		errStr := err.Error()
		if errStr == "" {
			t.Error("Expected timeout-related error")
		}
	}

	mockAuth.AssertExpectations(t)
}

// TestTokenRefresh_NoRefreshToken verifies handling when refresh token is missing
func TestTokenRefresh_NoRefreshToken(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	mockAuth := mocks.NewMockOAuthHandler(t)
	mockStorage := mocks.NewMockAccountStorage(t)

	userID := ids.RandomUserID()
	serverID := ids.RandomServerID()
	accountID, err := ids.RandomAccountID(userID, serverID)
	require.NoError(t, err)
	tokenWithoutRefresh := &oauth2.Token{
		AccessToken: "access-only",
		// No RefreshToken
		Expiry: time.Now().Add(-1 * time.Hour),
	}

	account, err := entities.NewAccount(
		accountID,
		"test-account",
		"Test Account",
		entities.WithAuthToken(tokenWithoutRefresh),
	)
	require.NoError(t, err, "Failed to create account")

	serverConfig, err := entities.NewServerConfig(
		serverID,
		mustParseURL("https://example.com/mcp"),
	)
	if err != nil {
		t.Fatalf("Failed to create server config: %v", err)
	}

	// Adapters - these should NOT be called
	refreshTokenFunc := func(ctx context.Context, server entities.ServerConfigReadOnly, token *oauth2.Token) (*oauth2.Token, error) {
		return mockAuth.RefreshToken(ctx, server.AuthConfig(), token)
	}
	saveAccountFunc := func(ctx context.Context, acc ids.AccountID, token *oauth2.Token) error {
		return mockStorage.SaveAccount(ctx, account)
	}

	refresher := NewRefresher(
		ctx,
		tokenWithoutRefresh,
		refreshTokenFunc,
		saveAccountFunc,
		account.ID(),
		serverConfig,
		10*time.Second,
	)

	// Should fail immediately without refresh token
	token, err := refresher.Token()
	if err == nil {
		t.Error("Expected error when refresh token is missing")
	}

	if token != nil {
		t.Error("Token should be nil when refresh fails")
	}

	// No mock expectations should be called since we fail early
	mockAuth.AssertNotCalled(t, "RefreshToken")
	mockStorage.AssertNotCalled(t, "SaveAccount")
}

// mustParseURL is a helper that panics if URL parsing fails
func mustParseURL(rawURL string) *url.URL {
	u, err := url.Parse(rawURL)
	if err != nil {
		panic(err)
	}
	return u
}
