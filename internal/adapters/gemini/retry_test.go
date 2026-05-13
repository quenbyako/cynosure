package gemini

import (
	"bytes"
	"context"
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
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*http.Response), args.Error(1)
}

type mockSecretGetter struct {
	mock.Mock
}

func (m *mockSecretGetter) Get(ctx context.Context) ([]byte, error) {
	args := m.Called(ctx)
	return args.Get(0).([]byte), args.Error(1)
}

func TestRetryTransport(t *testing.T) {
	ctx := context.Background()
	apiKey := []byte("test-api-key")

	t.Run("successful request", func(t *testing.T) {
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
		req, _ := http.NewRequestWithContext(ctx, "GET", "http://example.com", nil)

		gotResp, err := transport.RoundTrip(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, gotResp.StatusCode)
		mockBase.AssertExpectations(t)
		mockSecrets.AssertExpectations(t)
	})

	t.Run("retry on 503", func(t *testing.T) {
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
		req, _ := http.NewRequestWithContext(ctx, "GET", "http://example.com", nil)

		gotResp, err := transport.RoundTrip(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, gotResp.StatusCode)
		mockBase.AssertExpectations(t)
	})

	t.Run("body re-read on retry", func(t *testing.T) {
		mockBase := new(mockRoundTripper)
		mockSecrets := new(mockSecretGetter)

		mockSecrets.On("Get", mock.Anything).Return(apiKey, nil)

		body := []byte("request body")

		mockBase.On("RoundTrip", mock.MatchedBy(func(req *http.Request) bool {
			if req.GetBody == nil {
				return false
			}
			rc, _ := req.GetBody()
			b, _ := io.ReadAll(rc)
			rc.Close()
			return bytes.Equal(b, body)
		})).Return(&http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Body:       io.NopCloser(bytes.NewReader([]byte("fail"))),
		}, nil).Once()

		mockBase.On("RoundTrip", mock.MatchedBy(func(req *http.Request) bool {
			if req.GetBody == nil {
				return false
			}
			rc, _ := req.GetBody()
			b, _ := io.ReadAll(rc)
			rc.Close()
			return bytes.Equal(b, body)
		})).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader([]byte("ok"))),
		}, nil).Once()

		transport := newRetryTransport(mockBase, mockSecrets, []int{503})
		req, _ := http.NewRequestWithContext(ctx, "POST", "http://example.com", bytes.NewReader(body))

		gotResp, err := transport.RoundTrip(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, gotResp.StatusCode)
		mockBase.AssertExpectations(t)
	})

	t.Run("no retry on 400", func(t *testing.T) {
		mockBase := new(mockRoundTripper)
		mockSecrets := new(mockSecretGetter)

		mockSecrets.On("Get", mock.Anything).Return(apiKey, nil)

		resp400 := &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       io.NopCloser(bytes.NewReader([]byte("bad request"))),
		}

		mockBase.On("RoundTrip", mock.Anything).Return(resp400, nil).Once()

		transport := newRetryTransport(mockBase, mockSecrets, []int{503})
		req, _ := http.NewRequestWithContext(ctx, "GET", "http://example.com", nil)

		gotResp, err := transport.RoundTrip(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, gotResp.StatusCode)
		mockBase.AssertExpectations(t)
	})
}
