package testsuite

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	suites "github.com/quenbyako/cynosure/contrib/bettersuites"
	"github.com/stretchr/testify/require"

	"github.com/quenbyako/cynosure/internal/domains/gateway/components"
	"github.com/quenbyako/cynosure/internal/domains/gateway/components/ids"
	"github.com/quenbyako/cynosure/internal/domains/gateway/ports"
)

// RunMessengerTests runs tests for the given adapter. These tests are predefined
// and recommended to be used for ANY adapter implementation.
//
// After constructing test function, just run it through `run(t)` call,
// everything else will be handled for you. Calling this function through
// `t.Run("general", run)` is not very recommended, cause test logs will be too
// hard to read cause of big nesting.
func RunMessengerTests(a ports.Messenger, opts ...MessengerSuiteOption) func(t *testing.T) {
	s := &MessengerTestSuite{
		adapter:        a,
		defaultTimeout: 5 * time.Second,
	}
	for _, opt := range opts {
		opt(s)
	}
	if err := s.validate(); err != nil {
		panic(err)
	}

	return suites.Run(s)
}

type MessengerTestSuite struct {
	adapter        ports.Messenger
	channel        ids.ChannelID
	defaultTimeout time.Duration
}

func (s *MessengerTestSuite) validate() error {
	if !s.channel.Valid() {
		return errors.New("empty channel id")
	}

	return nil
}

type MessengerSuiteOption func(*MessengerTestSuite)

// WithTimeout sets custom timeout for tests
func WithTimeout(timeout time.Duration) MessengerSuiteOption {
	return func(s *MessengerTestSuite) { s.defaultTimeout = timeout }
}

func WithChannel(provider, channel string) MessengerSuiteOption {
	return func(s *MessengerTestSuite) { must(ids.NewChannelID(provider, channel)) }
}

// ============================================================================
// Test Helpers & Mocks
// ============================================================================

func (s *MessengerTestSuite) invalidChannel() ids.ChannelID {
	return ids.ChannelID{} // empty/invalid
}

func (s *MessengerTestSuite) invalidMessageID() ids.MessageID {
	return ids.MessageID{} // empty/invalid
}

func (s *MessengerTestSuite) message(id string) ids.MessageID {
	return must(ids.NewMessageID(s.channel, id))
}

// Standard test messages
func (s *MessengerTestSuite) shortText() components.MessageText {
	return must(components.NewMessageText("Hello, World!"))
}

func (s *MessengerTestSuite) mediumText() components.MessageText {
	return must(components.NewMessageText(strings.Repeat("A", 2000)))
}

func (s *MessengerTestSuite) longText() components.MessageText {
	// 5000 chars - should trigger truncation at 4080
	return must(components.NewMessageText(strings.Repeat("B", 5000)))
}

// ============================================================================
// Category 1: Basic Functionality Tests
// ============================================================================

// TestSendMessage - P0 Critical
func (s *MessengerTestSuite) TestSendMessage(t *testing.T) {
	for _, tt := range []struct {
		name              string
		channelID         ids.ChannelID
		text              components.MessageText
		contextMiddleware func(context.Context) context.Context
		wantErr           require.ErrorAssertionFunc
	}{{
		name:      "BasicText",
		channelID: s.channel,
		text:      s.shortText(),
	}, {
		name:      "InvalidChannelID",
		channelID: s.invalidChannel(),
		text:      s.shortText(),
		wantErr:   require.Error,
	}, {
		name:      "ContextCancellation",
		channelID: s.channel,
		text:      s.shortText(),
		contextMiddleware: func(ctx context.Context) context.Context {
			ctx, cancel := context.WithCancel(ctx)
			cancel()
			return ctx
		},
		wantErr: requireErrorIs(context.Canceled),
	}, {
		name:      "LongMessage",
		channelID: s.channel,
		text:      s.longText(),
	}, {
		name:      "MediumMessage",
		channelID: s.channel,
		text:      s.mediumText(),
	}, {
		name:      "UnicodeCharacters",
		channelID: s.channel,
		text:      must(components.NewMessageText("Hello 👋 Мир 🌍 世界 🎉")),
	}, {
		name:      "SpecialCharacters",
		channelID: s.channel,
		text:      must(components.NewMessageText("Special chars: <>&\"'`*_[]()~#+-=|{}.!")),
	}, {
		name:      "Newlines",
		channelID: s.channel,
		text:      must(components.NewMessageText("Line 1\nLine 2\nLine 3")),
	}, {
		name:      "ContextDeadline",
		channelID: s.channel,
		text:      s.shortText(),
		contextMiddleware: func(ctx context.Context) context.Context {
			// negative deadline to trigger immediate timeout
			ctx, cancel := context.WithTimeout(ctx, -time.Hour)
			cancel()
			return ctx
		},
		wantErr: requireErrorIs(context.DeadlineExceeded),
	}} {
		tt.wantErr = noErrAsDefault(tt.wantErr)
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
			if tt.contextMiddleware != nil {
				ctx = tt.contextMiddleware(ctx)
			}

			messageID, err := s.adapter.SendMessage(ctx, tt.channelID, tt.text)
			if tt.wantErr(t, err); err != nil {
				return
			}

			require.NoError(t, err)
			require.True(t, messageID.Valid())
			require.Equal(t, tt.channelID.String(), messageID.ChannelID().String())
		})
	}
}

// TestUpdateMessage - P0 Critical
func (s *MessengerTestSuite) TestUpdateMessage(t *testing.T) {
	for _, tt := range []struct {
		name    string
		msgID   ids.MessageID
		text    components.MessageText
		wantErr require.ErrorAssertionFunc
	}{{
		name:  "BasicUpdate",
		msgID: s.message("960"),
		text:  must(components.NewMessageText("Updated message")),
	}, {
		name:    "InvalidMessageID",
		msgID:   s.invalidMessageID(),
		text:    must(components.NewMessageText("Some text")),
		wantErr: require.Error,
	}} {
		tt.wantErr = noErrAsDefault(tt.wantErr)

		t.Run(tt.name, func(t *testing.T) {
			text := tt.text
			err := s.adapter.UpdateMessage(t.Context(), tt.msgID, text)
			if tt.wantErr(t, err); err != nil {
				return
			}
		})
	}
}

// TestNotifyProcessingStarted - P0 Critical
func (s *MessengerTestSuite) TestNotifyProcessingStarted(t *testing.T) {
	for _, tt := range []struct {
		name      string
		channelID ids.ChannelID
		wantErr   bool
	}{{
		name:      "ValidChannel",
		channelID: s.channel,
		wantErr:   false,
	}, {
		name:      "InvalidChannel",
		channelID: s.invalidChannel(),
		wantErr:   true,
	}} {
		t.Run(tt.name, func(t *testing.T) {

			err := s.adapter.NotifyProcessingStarted(t.Context(), tt.channelID)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// ============================================================================
// Category 3: Provider Validation Tests
// ============================================================================

// TestSendMessage_Providers
func (s *MessengerTestSuite) TestSendMessage_Providers(t *testing.T) {
	for _, tt := range []struct {
		name    string
		channel ids.ChannelID
		wantErr require.ErrorAssertionFunc
	}{{
		name:    "Selected provider",
		channel: s.channel,
	}, {
		name: "Unsupported provider",
		channel: must(ids.NewChannelID(
			"some random provider, do whatever you want lmao",
			"C0123456",
		)),
		// throwing expected, cause must be selected from only available
		// providers
		wantErr: require.Error,
	}} {
		tt.wantErr = noErrAsDefault(tt.wantErr)

		t.Run(tt.name, func(t *testing.T) {
			messageID, err := s.adapter.SendMessage(t.Context(), tt.channel, s.shortText())
			if tt.wantErr(t, err); err != nil {
				return
			}

			require.Equal(t, tt.channel.ProviderID(), messageID.ChannelID().ProviderID())
			require.Equal(t, tt.channel.ChannelID(), messageID.ChannelID().ChannelID())
		})
	}
}

// ============================================================================
// Helper Functions
// ============================================================================

func requireErrorIs[T error](err T) require.ErrorAssertionFunc {
	return func(t require.TestingT, actual error, msgAndArgs ...interface{}) {
		if t, ok := t.(interface{ Helper() }); ok {
			t.Helper()
		}

		require.ErrorIs(t, actual, err, msgAndArgs...)
	}
}

func noErrAsDefault(f require.ErrorAssertionFunc) require.ErrorAssertionFunc {
	if f != nil {
		return f
	}

	return require.NoError
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
