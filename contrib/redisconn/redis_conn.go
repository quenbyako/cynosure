package redisconn

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	defaultPort   = "6379"
	traceRequests = false
)

type newRedisClusterConnectionParams struct {
	tempDNS map[string]string
}

type NewRedisUniversalConnectionOption func(*newRedisClusterConnectionParams)

func WithDNSMapping(mapping map[string]string) NewRedisUniversalConnectionOption {
	return func(params *newRedisClusterConnectionParams) { params.tempDNS = mapping }
}

func NewRedisUniversalConnection(ctx context.Context, addr *url.URL, log *slog.Logger, opts ...NewRedisUniversalConnectionOption) (conn redis.UniversalClient, prefix string, err error) {
	params, err := parseRedisURL(addr)
	if err != nil {
		return nil, "", err
	}

	if _, isCluster := params.addrs(); isCluster {
		conn, err := newRedisClusterConnection(ctx, params, log, opts...)
		if err != nil {
			return nil, "", err
		}

		return conn, params.KeyPrefix, nil
	}

	conn, err = newRedisConnection(ctx, params, log)
	if err != nil {
		return nil, "", err
	}

	return conn, params.KeyPrefix, nil
}

func newRedisClusterConnection(ctx context.Context, options *RedisConnParams, log *slog.Logger, opts ...NewRedisUniversalConnectionOption) (redis.UniversalClient, error) {
	p := newRedisClusterConnectionParams{
		tempDNS: nil,
	}
	for _, opt := range opts {
		opt(&p)
	}

	clusterOpts, err := options.setupClusterOpts()
	if err != nil {
		return nil, err
	}

	if len(p.tempDNS) > 0 {
		dialer := clusterOpts.Dialer
		if dialer == nil {
			dialer = redisNewDialer(clusterOpts.DialTimeout, clusterOpts.TLSConfig)
		}

		clusterOpts.Dialer = wrapDNSRemap(p.tempDNS, log, dialer)
	}

	rdb := redis.NewClusterClient(clusterOpts)
	patchClusterScan(rdb)

	if err := rdb.Ping(ctx).Err(); err != nil {
		rdb.Close() // close connection, because we can't use it

		return nil, fmt.Errorf("ping: %w", err)
	}

	// tracing of https://github.com/releaseband/popcorn/issues/36 issue
	rdb.OnNewNode(func(rdb *redis.Client) {
		const pingTimeout = 2 * time.Second

		ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
		defer cancel()

		if err := rdb.Ping(ctx).Err(); err != nil {
			// we are not sure, does the node MUST pong, or not, so just warning.
			//
			// Moreover, we can't handle this error, here, LMAO, AROLF, KEKW 😂
			log.Warn("new node added to connection, but can't ping", slog.String("addr", rdb.Options().Addr), slog.Any("err", err))
		} else {
			log.Debug("new node added to connection", slog.String("addr", rdb.Options().Addr))
		}
	})

	if r, err := rdb.ClusterShards(ctx).Result(); err == nil {
		for _, shard := range r {
			var slots string
			var slotsSb104 strings.Builder
			for _, slot := range shard.Slots {
				slotsSb104.WriteString(fmt.Sprintf("[%v:%v]", slot.Start, slot.End))
			}
			slots += slotsSb104.String()

			for _, node := range shard.Nodes {
				log.Debug("foud node in a cluster, but can't check connection", slog.String("addr", node.Endpoint), slog.String("role", node.Role), slog.String("shard", slots))
			}
		}
	} else {
		log.Warn("couldn't get info about current cluster shards", slog.Any("err", err))
	}

	return rdb, nil
}

func NewRedisConnection(addr *url.URL, log *slog.Logger) (redis.UniversalClient, error) {
	options, err := newRedisConnOptions(prepareURL(addr, log))
	if err != nil {
		return nil, err
	}

	rdb := redis.NewClient(options)

	const pingTimeout = 2 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}

	return rdb, nil
}

func newRedisConnection(ctx context.Context, options *RedisConnParams, log *slog.Logger) (redis.UniversalClient, error) {
	opts, err := options.setupOpts()
	if err != nil {
		return nil, err
	}

	rdb := redis.NewClient(opts)

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}

	if err := rdb.ClusterInfo(ctx).Err(); err == nil {
		log.Warn("redis server is a cluster node, are you sure that it's a single node? recreating connection.")

		// auto recreate cluster connection, it's too risky to just live with single connection.
		rdb.Close()

		options.Cluster = true

		return newRedisClusterConnection(ctx, options, log)
	}

	return rdb, nil
}

func LinkValidator(u *url.URL) error {
	// sslmode is our custom query parameter, so we need to remove it before
	// checking from official lib
	copied := new(url.URL)
	*copied = *u

	query := copied.Query()
	delete(query, "sslmode")
	copied.RawQuery = query.Encode()

	_, err := redis.ParseURL(copied.String())

	return err
}

func LinkValidatorUniversal(keyPrefixRequired bool) func(*url.URL) error {
	return func(u *url.URL) (err error) {
		params, err := parseRedisURL(u)
		if err != nil {
			return err
		}

		if keyPrefixRequired && params.KeyPrefix == "" {
			return errors.New("keyprefix is required")
		}

		if _, isCluster := params.addrs(); isCluster {
			_, err = params.setupClusterOpts()
		} else {
			_, err = params.setupOpts()
		}

		return err
	}
}

func newRedisClusterConnOptions(u *url.URL, optionalSSLVerify bool, logger *slog.Logger) (*redis.ClusterOptions, error) {
	options, err := redis.ParseClusterURL(u.String())
	if err != nil {
		return nil, fmt.Errorf("invalid connection url: %w", err)
	}

	options.OnConnect = redisAfterConnectHook

	if optionalSSLVerify {
		if options.TLSConfig == nil {
			options.TLSConfig = &tls.Config{InsecureSkipVerify: true}
		} else {
			options.TLSConfig.InsecureSkipVerify = true
		}
	}

	if traceRequests {
		dialer := options.Dialer
		if dialer == nil {
			dialer = redisNewDialer(options.DialTimeout, options.TLSConfig)
		}

		options.Dialer = wrapLogs(logger, dialer)
	}

	return options, nil
}

func newRedisConnOptions(u *url.URL, optionalSSLVerify bool, logger *slog.Logger) (*redis.Options, error) {
	options, err := redis.ParseURL(u.String())
	if err != nil {
		return nil, fmt.Errorf("invalid connection url: %w", err)
	}

	options.OnConnect = redisAfterConnectHook

	if optionalSSLVerify {
		if options.TLSConfig == nil {
			options.TLSConfig = &tls.Config{InsecureSkipVerify: true}
		} else {
			options.TLSConfig.InsecureSkipVerify = true
		}
	}

	if traceRequests {
		dialer := options.Dialer
		if dialer == nil {
			dialer = redis.NewDialer(options)
		}

		options.Dialer = wrapLogs(logger, dialer)
	}

	return options, nil
}

func prepareURL(u *url.URL, log *slog.Logger) (modified *url.URL, optionalVerify bool, _ *slog.Logger) {
	if u.Port() == "" {
		u.Host = net.JoinHostPort(u.Hostname(), defaultPort)
	}

	// sslmode is our custom query parameter, so we need to remove it before
	// checking from official lib
	mode := strings.ToLower(u.Query().Get("sslmode"))
	query := u.Query()
	delete(query, "sslmode")
	u.RawQuery = query.Encode()

	switch mode {
	case "disable":
		return u, u.Scheme == "rediss", log

	case "required":
		u.Scheme = "rediss"

		return u, false, log

	default:
		return u, false, log
	}
}

func redisAfterConnectHook(ctx context.Context, conn *redis.Conn) error {
	const pingTimeout = 2 * time.Second

	ctx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()

	if _, err := conn.Ping(ctx).Result(); err != nil {
		return fmt.Errorf("ping: %w", err)
	}

	return nil
}

// default dialer from [redis.NewDialer]
func redisNewDialer(dialTimeout time.Duration, tlsConfig *tls.Config) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		netDialer := &net.Dialer{
			Timeout:   dialTimeout,
			KeepAlive: 5 * time.Minute,
		}
		if tlsConfig == nil {
			return netDialer.DialContext(ctx, network, addr)
		}

		return tls.DialWithDialer(netDialer, network, addr, tlsConfig)
	}
}

type connLogger struct {
	net.Conn
	log *slog.Logger
}

var _ net.Conn = (*connLogger)(nil)

func wrapLogs(log *slog.Logger, inner redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		conn, err := inner(ctx, network, addr)
		if err != nil {
			return nil, err
		}

		return &connLogger{Conn: conn, log: log}, nil
	}
}

func wrapDNSRemap(mappings map[string]string, log *slog.Logger, inner redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		if newAddr, ok := mappings[addr]; ok {
			log.Debug("found mapping for address", slog.String("old", addr), slog.String("new", newAddr))
			addr = newAddr
		}

		return inner(ctx, network, addr)
	}
}

func (c *connLogger) Read(b []byte) (n int, err error) {
	if n, err = c.Conn.Read(b); err != nil {
		return n, err
	}

	trace(c.log, "READING ->", slog.String("DATA", string(b)))

	return n, err
}

func (c *connLogger) Write(b []byte) (n int, err error) {
	trace(c.log, "WRITING ->", slog.String("DATA", string(b)))

	return c.Conn.Write(b)
}

func trace(log *slog.Logger, msg string, args ...any) {
	log.Log(context.Background(), slog.LevelDebug-4, msg, args...)
}

// Issue with redis: by default, scan in cluster works differently: it calls
// random node and ask only that node for keys. This is wrong as hell, so in
// redis cluster client there is [redis.ClusterClient.ForEachMaster] method,
// that calls all masters and asks them for keys. This is a fix for that.
type clusterScanFixer struct {
	client *redis.ClusterClient
}

var _ redis.Hook = (*clusterScanFixer)(nil)

func patchClusterScan(client *redis.ClusterClient) {
	client.AddHook(&clusterScanFixer{client: client})
}

func (c *clusterScanFixer) DialHook(next redis.DialHook) redis.DialHook { return next }

func (c *clusterScanFixer) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		scanCmd, ok := cmd.(*redis.ScanCmd)
		if !ok {
			return next(ctx, cmd)
		}

		args := scanCmd.Args()
		values := make([]string, 0)

		// TODO: it's really important to find correct node by hash slot,
		// but pattern is stored "somewhere" inside args. It's not the best
		// idea to just find "match" string in elements and get next
		// element, cause you may catch "match" key, who knows. So it could
		// be great, to implement smart searching of pattern, then checking
		// its hash, and lookup ONLY on that shard.

		err := c.client.ForEachMaster(ctx, func(ctx context.Context, client *redis.Client) error {
			// important: if we won't scan ALL keys, it might panic on
			// iterator. So we need to scan all keys.
			innerCmd := redis.NewScanCmd(ctx, nil, args...)
			if err := client.Process(ctx, innerCmd); err != nil {
				return err
			}

			iter := innerCmd.Iterator()
			for iter.Next(ctx) {
				values = append(values, iter.Val())
			}

			return iter.Err()
		})
		if err != nil {
			return err
		}

		scanCmd.SetVal(values, 0)

		return nil
	}
}

func (c *clusterScanFixer) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}
