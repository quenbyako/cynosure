package gemini

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"slices"
	"time"

	"github.com/cenkalti/backoff/v5"
)

const (
	apiKeyHeader = "X-Goog-Api-Key"
)

type retryTransport struct {
	base                 http.RoundTripper
	apiKey               SecretGetter
	retriableStatusCodes []int // must be sorted
}

func newRetryTransport(base http.RoundTripper, apiKey SecretGetter, retriableStatusCodes []int) http.RoundTripper {
	// must be sorted
	slices.Sort(retriableStatusCodes)

	return &retryTransport{
		base:                 base,
		apiKey:               apiKey,
		retriableStatusCodes: retriableStatusCodes,
	}
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()

	req = req.Clone(ctx)

	key, err := t.apiKey.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting api key: %w", err)
	}

	req.Header.Set(apiKeyHeader, string(key))

	// replacing body to copy reader, since for retry we might get it once more.
	// TODO: tee reader? bufio? idk
	var body []byte
	if req.Body != nil && req.GetBody == nil {
		if body, err = io.ReadAll(req.Body); err != nil {
			return nil, fmt.Errorf("reading body: %w", err)
		}
		req.Body.Close()
		req.Body = io.NopCloser(bytes.NewReader(body))
		req.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(body)), nil }
	}

	return backoff.Retry(ctx, t.performRequest(req), t.backoffOpts()...)
}

func (t *retryTransport) performRequest(req *http.Request) backoff.Operation[*http.Response] {
	var attempt int
	return func() (*http.Response, error) {
		attempt++

		// If it's not the first attempt, we need to refresh the body
		if attempt > 1 && req.GetBody != nil {
			newBody, err := req.GetBody()
			if err != nil {
				return nil, backoff.Permanent(
					fmt.Errorf("failed to refresh request body for retry: %w", err),
				)
			}
			req.Body = newBody
		}

		resp, err := t.base.RoundTrip(req)
		if err != nil {
			// Network error, worth retrying
			return nil, err
		}

		// Check for retryable status codes
		if slices.Contains(t.retriableStatusCodes, resp.StatusCode) {
			// Close the body of the failing response to avoid leak
			resp.Body.Close()

			// Return error to trigger retry
			return nil, fmt.Errorf("retryable status code: %d", resp.StatusCode)
		}

		return resp, nil
	}
}

func (t *retryTransport) backoffOpts() []backoff.RetryOption {
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = 1 * time.Second

	return []backoff.RetryOption{
		backoff.WithBackOff(b),
		backoff.WithMaxElapsedTime(30 * time.Second),
	}
}
