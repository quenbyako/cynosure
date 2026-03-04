package telegram_test

import (
	"net/http"

	"testing"

	"github.com/henvic/httpretty"
	"github.com/quenbyako/cynosure/internal/domains/gateway/ports/testsuite"
	"github.com/stretchr/testify/require"

	. "github.com/quenbyako/cynosure/internal/adapters/telegram"
)

func TestMessengerSuite(t *testing.T) {
	logger := &httpretty.Logger{
		Time:           true,
		TLS:            true,
		RequestHeader:  true,
		RequestBody:    true,
		ResponseHeader: true,
		ResponseBody:   true,
		Colors:         true,
		Formatters:     []httpretty.Formatter{&httpretty.JSONFormatter{}},
	}

	tg, err := NewMessenger(
		t.Context(),
		"<REDACTED>",
		WithRoundTripper(logger.RoundTripper(http.DefaultTransport)),
	)
	require.NoError(t, err, "Failed to create Telegram client")

	testsuite.RunMessengerTests(
		tg,
		testsuite.WithChannel("telegram", "412884386"),
	)(t)
}
