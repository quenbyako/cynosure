package mcp

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/quenbyako/cynosure/contrib/core-params/httpclient/ssrf"
)

const (
	ssrfVerificationLocalhost = "http://127.0.0.1.nip.io"
	ssrfVerificationNetwork   = "https://169.254.169.254.nip.io"
)

func verifySSRF(ctx context.Context, client http.RoundTripper) error {
	for _, addr := range []string{
		ssrfVerificationLocalhost,
		ssrfVerificationNetwork,
	} {
		// Probe localhost to verify SSRF protection is active.
		// We expect the transport to block this and return ssrf.ErrProhibitedIP.
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, addr, http.NoBody)
		if err != nil {
			return fmt.Errorf("create SSRF probe for %s: %w", addr, err)
		}

		resp, err := client.RoundTrip(req)
		if err == nil {
			resp.Body.Close() //nolint:errcheck,gosec // safe to ignore error here

			return fmt.Errorf(
				"%w: connection to %s was not blocked",
				ErrSSRFVerificationFailed,
				addr,
			)
		}

		if !errors.Is(err, ssrf.ErrProhibitedIP) {
			return fmt.Errorf(
				"%w: connection to %s failed with unexpected error: %w",
				ErrSSRFVerificationFailed,
				addr,
				err,
			)
		}
	}

	return nil
}
