package gemini //nolint:testpackage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockRoundTripper struct {
	mock.Mock
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	res := args.Get(0)
	err := args.Error(1)

	if res == nil {
		return nil, err //nolint:wrapcheck
	}

	resp, ok := res.(*http.Response)
	if !ok {
		return nil, fmt.Errorf("mock: unexpected response type: %T", res)
	}

	return resp, err //nolint:wrapcheck
}

type mockSecretGetter struct {
	mock.Mock
}

func (m *mockSecretGetter) Get(ctx context.Context) ([]byte, error) {
	args := m.Called(ctx)
	res := args.Get(0)
	err := args.Error(1)

	if res == nil {
		return nil, err //nolint:wrapcheck
	}

	secret, ok := res.([]byte)
	if !ok {
		return nil, fmt.Errorf("mock: unexpected secret type: %T", res)
	}

	return secret, err //nolint:wrapcheck
}

func TestRetryTransport(t *testing.T) {
	t.Run("successful request", testSuccessfulRequest)
	t.Run("retry on 503", testRetryOn503)
	t.Run("body re-read on retry", testBodyReReadOnRetry)
	t.Run("no retry on 400", testNoRetryOn400)
}

func testSuccessfulRequest(t *testing.T) {
	ctx := context.Background()
	apiKey := []byte("test-api-key")
	mockBase := new(mockRoundTripper)
	mockSecrets := new(mockSecretGetter)

	mockSecrets.On("Get", mock.Anything).Return(apiKey, nil)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader([]byte("ok"))),
	}

	mockBase.On("RoundTrip", mock.MatchedBy(func(req *http.Request) bool {
		return req.Header.Get(apiKeyHeader) == string(apiKey)
	})).Return(resp, nil)

	transport := newRetryTransport(mockBase, mockSecrets, []int{503})
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.com", http.NoBody)
	require.NoError(t, err)

	gotResp, err := transport.RoundTrip(req)
	require.NoError(t, err)

	defer func() { _ = gotResp.Body.Close() }() //nolint:errcheck

	assert.Equal(t, http.StatusOK, gotResp.StatusCode)
	mockBase.AssertExpectations(t)
	mockSecrets.AssertExpectations(t)
}

func testRetryOn503(t *testing.T) {
	ctx := context.Background()
	apiKey := []byte("test-api-key")
	mockBase := new(mockRoundTripper)
	mockSecrets := new(mockSecretGetter)

	mockSecrets.On("Get", mock.Anything).Return(apiKey, nil)

	// First call returns 503
	resp503 := &http.Response{
		StatusCode: http.StatusServiceUnavailable,
		Body:       io.NopCloser(bytes.NewReader([]byte("service unavailable"))),
	}
	// Second call returns 200
	resp200 := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader([]byte("ok"))),
	}

	mockBase.On("RoundTrip", mock.Anything).Return(resp503, nil).Once()
	mockBase.On("RoundTrip", mock.Anything).Return(resp200, nil).Once()

	transport := newRetryTransport(mockBase, mockSecrets, []int{503})
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.com", http.NoBody)
	require.NoError(t, err)

	gotResp, err := transport.RoundTrip(req)
	require.NoError(t, err)

	defer func() { _ = gotResp.Body.Close() }() //nolint:errcheck

	assert.Equal(t, http.StatusOK, gotResp.StatusCode)
	mockBase.AssertExpectations(t)
}

func testBodyReReadOnRetry(t *testing.T) {
	ctx := context.Background()
	apiKey := []byte("test-api-key")
	mockBase := new(mockRoundTripper)
	mockSecrets := new(mockSecretGetter)

	mockSecrets.On("Get", mock.Anything).Return(apiKey, nil)

	body := []byte("request body")

	mockBase.On("RoundTrip", mock.MatchedBy(func(req *http.Request) bool {
		return checkRequestBody(t, req, body)
	})).Return(&http.Response{
		StatusCode: http.StatusServiceUnavailable,
		Body:       io.NopCloser(bytes.NewReader([]byte("fail"))),
	}, nil).Once()

	mockBase.On("RoundTrip", mock.MatchedBy(func(req *http.Request) bool {
		return checkRequestBody(t, req, body)
	})).Return(&http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader([]byte("ok"))),
	}, nil).Once()

	transport := newRetryTransport(mockBase, mockSecrets, []int{503})
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		"http://example.com",
		bytes.NewReader(body),
	)
	require.NoError(t, err)

	gotResp, err := transport.RoundTrip(req)
	require.NoError(t, err)

	defer func() { _ = gotResp.Body.Close() }() //nolint:errcheck

	assert.Equal(t, http.StatusOK, gotResp.StatusCode)
	mockBase.AssertExpectations(t)
}

func testNoRetryOn400(t *testing.T) {
	ctx := context.Background()
	apiKey := []byte("test-api-key")
	mockBase := new(mockRoundTripper)
	mockSecrets := new(mockSecretGetter)

	mockSecrets.On("Get", mock.Anything).Return(apiKey, nil)

	resp400 := &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(bytes.NewReader([]byte("bad request"))),
	}

	mockBase.On("RoundTrip", mock.Anything).Return(resp400, nil).Once()

	transport := newRetryTransport(mockBase, mockSecrets, []int{503})
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.com", http.NoBody)
	require.NoError(t, err)

	gotResp, err := transport.RoundTrip(req)
	require.NoError(t, err)

	defer func() { _ = gotResp.Body.Close() }() //nolint:errcheck

	assert.Equal(t, http.StatusBadRequest, gotResp.StatusCode)
	mockBase.AssertExpectations(t)
}

func checkRequestBody(t *testing.T, req *http.Request, expectedBody []byte) bool {
	t.Helper()

	if req.GetBody == nil {
		return false
	}

	bodyReader, err := req.GetBody()
	if err != nil {
		return false
	}

	content, err := io.ReadAll(bodyReader)
	if err != nil {
		return false
	}

	err = bodyReader.Close()
	require.NoError(t, err)

	return bytes.Equal(content, expectedBody)
}
