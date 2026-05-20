package openrouter

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"time"

	"github.com/cenkalti/backoff/v5"
)

// ErrRetryableStatus is returned when the server responds with a retryable HTTP status code.
var ErrRetryableStatus = errors.New("retryable status code")

type retryTransport struct {
	base                 http.RoundTripper
	apiKey               SecretGetter
	retriableStatusCodes []int // must be sorted
}

func newRetryTransport(
	base http.RoundTripper,
	apiKey SecretGetter,
	retriableStatusCodes []int,
) http.RoundTripper {
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

	req.Header.Set("Authorization", "Bearer "+string(key))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Http-Referer", "https://cynosure.bot")
	req.Header.Set("X-Title", "Cynosure Bot")

	if copyErr := t.copyRequestBody(req); copyErr != nil {
		return nil, copyErr
	}

	res, retryErr := backoff.Retry(ctx, t.performRequest(req), t.backoffOpts()...)
	if retryErr != nil {
		return nil, fmt.Errorf("retry request: %w", retryErr)
	}

	return res, nil
}

func (t *retryTransport) performRequest(req *http.Request) backoff.Operation[*http.Response] {
	var attempt int

	return func() (*http.Response, error) {
		attempt++

		if err := t.refreshBody(req, attempt); err != nil {
			return nil, fmt.Errorf("refresh body: %w", err)
		}

		resp, err := t.base.RoundTrip(req)
		if err != nil {
			return nil, fmt.Errorf("round trip: %w", err)
		}

		return t.handleResponse(resp)
	}
}

func (t *retryTransport) refreshBody(req *http.Request, attempt int) error {
	if attempt > 1 && req.GetBody != nil {
		newBody, err := req.GetBody()
		if err != nil {
			wrapped := fmt.Errorf("failed to refresh request body for retry: %w", err)

			//nolint:wrapcheck // backoff.Permanent is a retry-signaling sentinel
			return backoff.Permanent(wrapped)
		}

		req.Body = newBody
	}

	return nil
}

func (t *retryTransport) handleResponse(resp *http.Response) (*http.Response, error) {
	if slices.Contains(t.retriableStatusCodes, resp.StatusCode) {
		//nolint:errcheck // closing failing response body is best-effort
		_ = resp.Body.Close()

		return nil, fmt.Errorf("%w: %d", ErrRetryableStatus, resp.StatusCode)
	}

	return resp, nil
}

func (t *retryTransport) backoffOpts() []backoff.RetryOption {
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = 1 * time.Second

	return []backoff.RetryOption{
		backoff.WithBackOff(b),
		backoff.WithMaxElapsedTime(defaultMaxElapsedTime),
	}
}

func (t *retryTransport) copyRequestBody(req *http.Request) error {
	if req.Body == nil || req.GetBody != nil {
		return nil
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		return fmt.Errorf("reading body: %w", err)
	}

	if err := req.Body.Close(); err != nil {
		return fmt.Errorf("closing body: %w", err)
	}

	req.Body = io.NopCloser(bytes.NewReader(body))
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}

	return nil
}
