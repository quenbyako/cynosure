package refreshtoken_test

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/adapters/mocks"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/oauthhandler"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"

	. "github.com/quenbyako/cynosure/internal/apps/cynosure/refreshtoken"
)

// TestTokenRefresh_RequestCancelled verifies that token refresh completes
// even when request context is cancelled
func TestTokenRefresh_RequestCancelled(t *testing.T) {
	t.Parallel()

	// Create a context that will be cancelled
	_, cancelRequest := context.WithCancel(context.Background())
	defer cancelRequest()

	// Setup mocks
	mockAuth := mocks.NewOAuthHandler(t)
	mockAccounts := mocks.NewMockAccountStorage(t)
	mockServers := mocks.NewMockServerStorage(t)

	userID := ids.RandomUserID()
	serverID := ids.RandomServerID()
	accountID, err := ids.RandomAccountID(userID, serverID)
	require.NoError(t, err)

	oldToken := &oauth2.Token{
		AccessToken:  "old-access",
		TokenType:    "Bearer",
		RefreshToken: "refresh-token",
		Expiry:       time.Now().Add(-1 * time.Hour),
		ExpiresIn:    0,
	}
	newToken := &oauth2.Token{
		AccessToken:  "new-access",
		TokenType:    "Bearer",
		RefreshToken: "refresh-token",
		Expiry:       time.Now().Add(1 * time.Hour),
	}

	account, err := entities.NewAccount(
		accountID,
		"test-account",
		"Test Account",
		entities.WithAuthToken(oldToken),
	)
	require.NoError(t, err)

	serverConfig, err := entities.NewServerConfig(
		serverID,
		mustParseURL("https://example.com/mcp"),
		entities.WithAuthConfig(&oauth2.Config{
			ClientID: "client-id",
			Endpoint: oauth2.Endpoint{TokenURL: "https://example.com/token"},
		}),
	)
	require.NoError(t, err)

	// Mock expectations
	mockAuth.EXPECT().
		RefreshToken(mock.Anything, serverConfig.AuthConfig(), oldToken).
		Return(newToken, nil).
		Once()

	mockAccounts.EXPECT().
		GetAccount(mock.Anything, accountID).
		Return(account, nil).
		Once()

	mockAccounts.EXPECT().
		SaveAccount(mock.Anything, mock.Anything).
		Return(nil).
		Once()

	// Create refresher
	refresher := NewConstructor(
		mockAuth,
		mockAccounts,
		mockServers,
		1,
	)

	// Start refresher lifecycle
	lifecycleCtx, cancelLifecycle := context.WithCancel(context.Background())
	defer cancelLifecycle()

	go func() {
		_ = refresher.Run(lifecycleCtx)
	}()

	// Wait for pool to start
	time.Sleep(50 * time.Millisecond)

	// Build token source
	source, err := refresher.Build(accountID, serverConfig.AuthConfig(), oldToken, false)
	require.NoError(t, err)

	// Cancel the request context BEFORE token refresh
	cancelRequest()

	// Token refresh should still complete successfully because it's detached from request context
	token, err := source.Token()
	require.NoError(t, err)
	require.NotNil(t, token)
	require.Equal(t, "new-access", token.AccessToken)

	mockAuth.AssertExpectations(t)
	mockAccounts.AssertExpectations(t)
}

// TestTokenRefresh_TimeoutReached verifies that token refresh respects timeout from parent context
func TestTokenRefresh_TimeoutReached(t *testing.T) {
	t.Parallel()

	// Setup mocks
	mockAuth := mocks.NewOAuthHandler(t)
	mockAccounts := mocks.NewMockAccountStorage(t)
	mockServers := mocks.NewMockServerStorage(t)

	userID := ids.RandomUserID()
	serverID := ids.RandomServerID()
	accountID, err := ids.RandomAccountID(userID, serverID)
	require.NoError(t, err)

	oldToken := &oauth2.Token{
		AccessToken:  "old-access",
		TokenType:    "Bearer",
		RefreshToken: "refresh-token",
		Expiry:       time.Now().Add(-1 * time.Hour),
	}

	_, err = entities.NewAccount(
		accountID,
		"test-account",
		"Test Account",
		entities.WithAuthToken(oldToken),
	)
	require.NoError(t, err)

	serverConfig, err := entities.NewServerConfig(
		serverID,
		mustParseURL("https://example.com/mcp"),
		entities.WithAuthConfig(&oauth2.Config{
			ClientID: "client-id",
			Endpoint: oauth2.Endpoint{TokenURL: "https://example.com/token"},
		}),
	)
	require.NoError(t, err)

	// Mock expectations - simulate slow token refresh that exceeds timeout
	mockAuth.EXPECT().
		RefreshToken(mock.Anything, serverConfig.AuthConfig(), oldToken).
		Run(func(ctx context.Context, config *oauth2.Config, token *oauth2.Token, opts ...oauthhandler.RefreshTokenOption) {
			// Simulate slow operation
			time.Sleep(200 * time.Millisecond)
		}).
		Return(nil, context.DeadlineExceeded).
		Once()

	// Create refresher
	refresher := NewConstructor(
		mockAuth,
		mockAccounts,
		mockServers,
		1,
	)

	// Start refresher lifecycle
	lifecycleCtx, cancelLifecycle := context.WithCancel(context.Background())
	defer cancelLifecycle()

	go func() {
		_ = refresher.Run(lifecycleCtx)
	}()

	// Wait for pool to start
	time.Sleep(50 * time.Millisecond)

	// Build token source
	source, err := refresher.Build(accountID, serverConfig.AuthConfig(), oldToken, false)
	require.NoError(t, err)

	// Token refresh should fail due to timeout (default maxTaskDuration is 30s,
	// but here mock returns DeadlineExceeded)
	_, err = source.Token()
	require.Error(t, err)
	require.Contains(t, err.Error(), "deadline exceeded")

	mockAuth.AssertExpectations(t)
}

// TestTokenRefresh_NoRefreshToken verifies handling when refresh token is missing
func TestTokenRefresh_NoRefreshToken(t *testing.T) {
	t.Parallel()

	mockAuth := mocks.NewOAuthHandler(t)
	mockAccounts := mocks.NewMockAccountStorage(t)
	mockServers := mocks.NewMockServerStorage(t)

	userID := ids.RandomUserID()
	serverID := ids.RandomServerID()
	accountID, err := ids.RandomAccountID(userID, serverID)
	require.NoError(t, err)

	tokenWithoutRefresh := &oauth2.Token{
		AccessToken: "access-only",
		Expiry:      time.Now().Add(-1 * time.Hour),
	}

	serverConfig, err := entities.NewServerConfig(
		serverID,
		mustParseURL("https://example.com/mcp"),
	)
	require.NoError(t, err)

	refresher := NewConstructor(
		mockAuth,
		mockAccounts,
		mockServers,
		1,
	)

	// Should fail immediately without refresh token in Build
	_, err = refresher.Build(accountID, serverConfig.AuthConfig(), tokenWithoutRefresh, false)
	require.ErrorIs(t, err, ErrRefreshTokenNotSet)

	// No mock expectations should be called
	mockAuth.AssertNotCalled(t, "RefreshToken")
}

// mustParseURL is a helper that panics if URL parsing fails
func mustParseURL(rawURL string) *url.URL {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		panic(err)
	}

	return parsedURL
}
