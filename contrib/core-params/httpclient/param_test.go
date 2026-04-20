package httpclient_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/quenbyako/core"
	"github.com/quenbyako/cynosure/contrib/core-params/httpclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func parseEnv[T any](ctx context.Context, rawValue string) (T, error) {
	typ := reflect.TypeFor[T]()
	parseFn, _, ok := core.GetParseFunc(typ)

	if !ok {
		return *new(T), fmt.Errorf("no parser for %v", typ)
	}

	val, err := parseFn(ctx, rawValue)
	if err != nil {
		return *new(T), err
	}

	res, ok := val.(T)
	if !ok {
		return *new(T), fmt.Errorf("invalid type %T, expected %v", val, typ)
	}

	return res, nil
}

func TestParseHTTPClient(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{{
		name:    "simple url",
		input:   "https://api.telegram.org",
		wantErr: false,
	}, {
		name:    "url with timeout",
		input:   "https://api.telegram.org#timeout=5s",
		wantErr: false,
	}, {
		name:    "url with rate and timeout",
		input:   "http://localhost:8080#timeout=100ms&rate=10/1s",
		wantErr: false,
	}, {
		name:    "invalid scheme",
		input:   "ftp://localhost",
		wantErr: true,
	}, {
		name:    "invalid timeout",
		input:   "http://localhost#timeout=invalid",
		wantErr: true,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := parseEnv[httpclient.Client](context.Background(), tt.input)

			if tt.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			require.NotNil(t, client)
		})
	}
}

func TestRoundTrip_BaseURLRewrite(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/test", r.URL.Path)

		w.WriteHeader(http.StatusOK)

		_, err := w.Write([]byte("ok"))
		assert.NoError(t, err)
	}))
	defer server.Close()

	client, err := parseEnv[httpclient.Client](context.Background(), server.URL+"/api")
	require.NoError(t, err)

	envParam, ok := client.(core.EnvParam)
	require.True(t, ok)

	err = envParam.Configure(context.Background(), &core.ConfigureData{
		AppCert: tls.Certificate{},
		Secrets: nil,
		Metric:  core.NoopMetrics(),
		Trace:   core.NoopMetrics(),
		Logger:  core.NoopMetrics(),
		Pool:    nil,
		Version: core.AppVersion{},
	})
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/v1/test",
		http.NoBody,
	)
	require.NoError(t, err)

	resp, err := client.RoundTrip(req)
	require.NoError(t, err)

	defer func() {
		_ = resp.Body.Close() //nolint:errcheck // tested
	}()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestRoundTrip_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := parseEnv[httpclient.Client](context.Background(), server.URL+"#timeout=50ms")
	require.NoError(t, err)

	envParam, ok := client.(core.EnvParam)
	require.True(t, ok)

	err = envParam.Configure(context.Background(), &core.ConfigureData{
		AppCert: tls.Certificate{},
		Secrets: nil,
		Metric:  core.NoopMetrics(),
		Trace:   core.NoopMetrics(),
		Logger:  core.NoopMetrics(),
		Pool:    nil,
		Version: core.AppVersion{},
	})
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		server.URL,
		http.NoBody,
	)
	require.NoError(t, err)

	resp, err := client.RoundTrip(req)
	if err == nil {
		_ = resp.Body.Close() //nolint:errcheck // tested
	}

	require.Error(t, err)
}

func TestRoundTrip_RateLimit(t *testing.T) {
	callCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := parseEnv[httpclient.Client](context.Background(), server.URL+"#rate=1/100ms")
	require.NoError(t, err)

	envParam, ok := client.(core.EnvParam)
	require.True(t, ok)

	err = envParam.Configure(context.Background(), &core.ConfigureData{
		AppCert: tls.Certificate{},
		Secrets: nil,
		Metric:  core.NoopMetrics(),
		Trace:   core.NoopMetrics(),
		Logger:  core.NoopMetrics(),
		Pool:    nil,
		Version: core.AppVersion{},
	})
	require.NoError(t, err)

	req1, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		server.URL,
		http.NoBody,
	)
	require.NoError(t, err)

	start := time.Now()

	resp1, err := client.RoundTrip(req1)
	require.NoError(t, err)

	_ = resp1.Body.Close() //nolint:errcheck // tested

	req2, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		server.URL,
		http.NoBody,
	)
	require.NoError(t, err)

	resp2, err := client.RoundTrip(req2)
	require.NoError(t, err)

	_ = resp2.Body.Close() //nolint:errcheck // tested

	duration := time.Since(start)
	assert.GreaterOrEqual(t, duration, 100*time.Millisecond)
}

func TestShutdown(t *testing.T) {
	client, err := parseEnv[httpclient.Client](context.Background(), "http://localhost")
	require.NoError(t, err)

	envParam, ok := client.(core.EnvParam)
	require.True(t, ok)

	err = envParam.Configure(context.Background(), &core.ConfigureData{
		AppCert: tls.Certificate{},
		Secrets: nil,
		Metric:  core.NoopMetrics(),
		Trace:   core.NoopMetrics(),
		Logger:  core.NoopMetrics(),
		Pool:    nil,
		Version: core.AppVersion{},
	})
	require.NoError(t, err)

	err = envParam.Shutdown(context.Background(), &core.ShutdownData{})
	assert.NoError(t, err)
}
