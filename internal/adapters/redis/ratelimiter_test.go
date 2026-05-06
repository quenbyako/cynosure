package redis_test

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/quenbyako/cynosure/contrib/redisconn"
	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go/modules/compose"
	"github.com/testcontainers/testcontainers-go/wait"

	_ "embed"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/ratelimiter"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/ratelimiter/testsuite"

	. "github.com/quenbyako/cynosure/internal/adapters/redis"
)

//go:embed testdata/docker-compose.yaml
var dockerComposeYaml []byte

func TestRateLimiter(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "docker-compose.yaml")
	if err := os.WriteFile(tmpFile, dockerComposeYaml, 0o600); err != nil {
		t.Fatalf("failed to write tmp compose file: %s", err)
	}

	comp, err := compose.NewDockerCompose(tmpFile)
	if err != nil {
		t.Fatalf("failed to create compose: %s", err)
	}

	// Use testcontainers Fluent API to wait for the initialization service to log our ready signal.
	// This ensures that 'redis-cluster-init' has successfully created the cluster.
	comp.WaitForService("redis-cluster-init",
		wait.ForLog("CLUSTER_READY_SIGNAL").WithPollInterval(time.Second))

	t.Cleanup(func() {
		if err := comp.Down(t.Context(), compose.RemoveOrphans(true)); err != nil {
			t.Logf("failed to down compose: %s", err)
		}
	})

	if err := comp.Up(t.Context(), compose.Wait(true)); err != nil {
		t.Fatalf("failed to compose up: %s", err)
	}

	mapping := getDNSMapping(t.Context(), t, comp)
	seedAddr := mapping["redis-1:6379"]

	redisURL, err := url.Parse(fmt.Sprintf("redis://%s?cluster=true", seedAddr))
	if err != nil {
		t.Fatalf("failed to parse redis url: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	client, _, err := redisconn.NewRedisUniversalConnection(
		t.Context(),
		redisURL,
		logger,
		redisconn.WithDNSMapping(mapping),
	)
	if err != nil {
		t.Fatalf("failed to connect to redis cluster: %v", err)
	}

	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Logf("failed to close client: %v", err)
		}
	})

	// Wait for the cluster state to be officially 'ok' from the client's perspective.
	// This prevents 'CLUSTERDOWN' errors during the first few milliseconds of testing.
	ensureClusterReady(t.Context(), t, client)

	testsuite.Run(setupRedisLimiter(t, client))(t)
}

func ensureClusterReady(ctx context.Context, t *testing.T, client redis.UniversalClient) {
	t.Helper()

	for range 10 {
		info, err := client.ClusterInfo(ctx).Result()
		if err == nil && strings.Contains(info, "cluster_state:ok") {
			return
		}

		time.Sleep(time.Second)
	}

	t.Fatal("cluster did not reach 'ok' state in time")
}

func getDNSMapping(ctx context.Context, t *testing.T, comp compose.ComposeStack) map[string]string {
	t.Helper()

	mapping := make(map[string]string)

	for _, name := range []string{"redis-1", "redis-2", "redis-3"} {
		container, err := comp.ServiceContainer(ctx, name)
		if err != nil {
			t.Fatalf("failed to get container for %s: %v", name, err)
		}

		host, err := container.Host(ctx)
		if err != nil {
			t.Fatalf("failed to get host for %s: %v", name, err)
		}

		port, err := container.MappedPort(ctx, "6379")
		if err != nil {
			t.Fatalf("failed to get port for %s: %v", name, err)
		}

		mapping[net.JoinHostPort(name, "6379")] = net.JoinHostPort(host, port.Port())
	}

	return mapping
}

func setupRedisLimiter(
	t *testing.T,
	client redis.UniversalClient,
) func(context.Context, testsuite.SetupParams) (ratelimiter.Port, error) {
	t.Helper()

	return func(ctx context.Context, params testsuite.SetupParams) (ratelimiter.Port, error) {
		if err := client.FlushAll(ctx).Err(); err != nil {
			return nil, fmt.Errorf("flush all: %w", err)
		}

		inner := New(
			client,
			params.ChatInput.Period, params.ChatInput.Limit,
			params.ChatOutput.Period, params.ChatOutput.Limit,
			params.EmbeddingInput.Period, params.EmbeddingInput.Limit,
			params.MaxWait,
			nil,
		)

		return testsuite.NewWaitWrapper(t, inner, params), nil
	}
}
