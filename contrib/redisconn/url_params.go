package redisconn

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/schema"
	"github.com/redis/go-redis/v9"
)

type SSLMode uint8

const (
	SSLModeDefault SSLMode = iota
	SSLModeDisable
	SSLModeRequired
)

type RedisConnParams struct {
	ClientName      string        `schema:"client_name"`
	Host            string        `schema:"-"`
	KeyPrefix       string        `schema:"-"`
	Scheme          string        `schema:"-"`
	User            string        `schema:"-"`
	Password        string        `schema:"-"`
	Addrs           []string      `schema:"addr"`
	MaxActiveConns  int           `schema:"max_active_conns"`
	MaxIdleConns    int           `schema:"max_idle_conns"`
	MinRetryBackoff time.Duration `schema:"min_retry_backoff"`
	MaxRetryBackoff time.Duration `schema:"max_retry_backoff"`
	DialTimeout     time.Duration `schema:"dial_timeout"`
	ReadTimeout     time.Duration `schema:"read_timeout"`
	WriteTimeout    time.Duration `schema:"write_timeout"`
	Protocol        int           `schema:"protocol"`
	PoolSize        int           `schema:"pool_size"`
	PoolTimeout     time.Duration `schema:"pool_timeout"`
	MinIdleConns    int           `schema:"min_idle_conns"`
	MaxRetries      int           `schema:"max_retries"`
	ConnMaxIdleTime time.Duration `schema:"conn_max_idle_time"`
	ConnMaxLifetime time.Duration `schema:"conn_max_lifetime"`
	DB              int           `schema:"-"`
	MaxRedirects    int           `schema:"max_redirects"`
	Port            uint16        `schema:"-"`
	ReadOnly        bool          `schema:"read_only"`
	RouteByLatency  bool          `schema:"route_by_latency"`
	RouteRandomly   bool          `schema:"route_randomly"`
	PoolFIFO        bool          `schema:"pool_fifo"`
	SSLMode         SSLMode       `schema:"sslmode"`
	EnableTracing   bool          `schema:"enable_tracing"`
	Cluster         bool          `schema:"cluster"`
}

const (
	defaultHost    = "localhost"
	defaultPortInt = 6379
)

func parseRedisURL(u *url.URL) (_ *RedisConnParams, err error) {
	password, _ := u.User.Password()

	res := RedisConnParams{
		Scheme:   u.Scheme,
		User:     u.User.Username(),
		Password: password,
	}

	switch res.Scheme {
	case "unix", "redis+unix":
		// exception: unix has socket path, not host and port.
		res.Host = u.Path

	default:
		res.Host, res.Port, err = getHostPortWithDefaults(u.Host, defaultHost, defaultPortInt)
		if err != nil {
			return nil, fmt.Errorf("failed to parse host: %w", err)
		}

		f := strings.FieldsFunc(u.Path, func(r rune) bool {
			return r == '/'
		})
		switch len(f) {
		case 0:
			res.DB = 0
		case 1:
			if res.DB, err = strconv.Atoi(f[0]); err != nil {
				return nil, fmt.Errorf("invalid database number: %q", f[0])
			}
		default:
			if res.DB, err = strconv.Atoi(f[0]); err != nil {
				return nil, fmt.Errorf("invalid database number: %q", f[0])
			}

			// key prefix is going after database number. Important thing is
			// that redis splits prefixes by semicolon (:), not by slash (/).

			for _, part := range f[1:] {
				// additional filter to avoid keyspace injection
				if strings.Contains(part, ":") {
					return nil, fmt.Errorf("key prefix contains semicolon: %q", part)
				}
			}

			res.KeyPrefix = strings.Join(f[1:], ":")
		}
	}

	d := schema.NewDecoder()
	d.RegisterConverter(time.Duration(0), func(s string) reflect.Value {
		if d, err := time.ParseDuration(s); err == nil {
			return reflect.ValueOf(d)
		}

		// returning invalid value to trigger error
		return reflect.Value{}
	})
	d.RegisterConverter(SSLModeDefault, func(s string) reflect.Value {
		switch strings.ToLower(s) {
		case "disable":
			return reflect.ValueOf(SSLModeDisable)
		case "required":
			return reflect.ValueOf(SSLModeRequired)
		default:
			// returning invalid value to trigger error
			return reflect.Value{}
		}
	})

	if err := d.Decode(&res, u.Query()); err != nil {
		return nil, fmt.Errorf("failed to parse url: %w", err)
	}

	// normalization of cluster addresses, if any:
	for i, addr := range res.Addrs {
		host, port, err := getHostPortWithDefaults(addr, defaultHost, defaultPortInt)
		if err != nil {
			return nil, fmt.Errorf("failed to parse address %q: %w", addr, err)
		}

		res.Addrs[i] = net.JoinHostPort(host, strconv.Itoa(int(port)))
	}

	return &res, nil
}

func (p *RedisConnParams) setupClusterOpts() (*redis.ClusterOptions, error) {
	if p.SSLMode == SSLModeRequired && p.Scheme != "rediss" {
		return nil, errors.New("SSL is required but scheme is not rediss")
	}

	addrs, isCluster := p.addrs()
	if !isCluster {
		return nil, errors.New("cluster mode is not enabled")
	}

	o := &redis.ClusterOptions{
		Addrs:    addrs,
		Username: p.User,
		Password: p.Password,

		Protocol:        p.Protocol,
		ClientName:      p.ClientName,
		MaxRedirects:    p.MaxRedirects,
		ReadOnly:        p.ReadOnly,
		RouteByLatency:  p.RouteByLatency,
		RouteRandomly:   p.RouteRandomly,
		MaxRetries:      p.MaxRetries,
		MinRetryBackoff: p.MinRetryBackoff,
		MaxRetryBackoff: p.MaxRetryBackoff,
		DialTimeout:     p.DialTimeout,
		ReadTimeout:     p.ReadTimeout,
		WriteTimeout:    p.WriteTimeout,
		PoolFIFO:        p.PoolFIFO,
		PoolSize:        p.PoolSize,
		MinIdleConns:    p.MinIdleConns,
		MaxIdleConns:    p.MaxIdleConns,
		MaxActiveConns:  p.MaxActiveConns,
		PoolTimeout:     p.PoolTimeout,
		ConnMaxLifetime: p.ConnMaxLifetime,
		ConnMaxIdleTime: p.ConnMaxIdleTime,
	}

	switch p.Scheme {
	case "rediss":
		o.TLSConfig = &tls.Config{ServerName: p.Host}
		if p.SSLMode == SSLModeDisable {
			o.TLSConfig.InsecureSkipVerify = true
		}

	case "redis":
		// do nothing

	default:
		return nil, fmt.Errorf("invalid URL scheme: %s", p.Scheme)
	}

	return o, nil
}

func (p *RedisConnParams) setupOpts() (*redis.Options, error) {
	switch p.Scheme {
	case "redis", "rediss":
		return p.setupTCPConn()
	case "unix":
		return p.setupUnixConn()
	default:
		return nil, fmt.Errorf("unsupported scheme %q", p.Scheme)
	}
}

func (p *RedisConnParams) setupTCPConn() (o *redis.Options, err error) {
	if p.SSLMode == SSLModeRequired && p.Scheme != "rediss" {
		return nil, errors.New("SSL is required but scheme is not rediss")
	}

	o = &redis.Options{
		Network:  "tcp",
		Addr:     net.JoinHostPort(p.Host, strconv.Itoa(int(p.Port))),
		Username: p.User,
		Password: p.Password,
		DB:       p.DB,

		Protocol:        p.Protocol,
		ClientName:      p.ClientName,
		MaxRetries:      p.MaxRetries,
		MinRetryBackoff: p.MinRetryBackoff,
		MaxRetryBackoff: p.MaxRetryBackoff,
		DialTimeout:     p.DialTimeout,
		ReadTimeout:     p.ReadTimeout,
		WriteTimeout:    p.WriteTimeout,
		PoolFIFO:        p.PoolFIFO,
		PoolSize:        p.PoolSize,
		PoolTimeout:     p.PoolTimeout,
		MinIdleConns:    p.MinIdleConns,
		MaxIdleConns:    p.MaxIdleConns,
		ConnMaxIdleTime: p.ConnMaxIdleTime,
		ConnMaxLifetime: p.ConnMaxLifetime,
	}

	if p.Scheme == "rediss" {
		o.TLSConfig = &tls.Config{
			ServerName: p.Host,
			MinVersion: tls.VersionTLS12,
		}

		if p.SSLMode == SSLModeDisable {
			o.TLSConfig.InsecureSkipVerify = true
		}
	}

	return o, nil
}

func (p *RedisConnParams) setupUnixConn() (*redis.Options, error) {
	if _, isCluster := p.addrs(); isCluster {
		return nil, errors.New("cluster mode is not supported for unix socket")
	}

	if p.SSLMode != SSLModeDefault {
		return nil, errors.New("SSL is not supported for unix socket")
	}

	if p.Cluster {
		return nil, errors.New("cluster mode is not supported for unix socket")
	}

	o := &redis.Options{
		Network:  "unix",
		Addr:     p.Host, // for unix host is a path to socket
		Username: p.User,
		Password: p.Password,

		Protocol:        p.Protocol,
		ClientName:      p.ClientName,
		MaxRetries:      p.MaxRetries,
		MinRetryBackoff: p.MinRetryBackoff,
		MaxRetryBackoff: p.MaxRetryBackoff,
		DialTimeout:     p.DialTimeout,
		ReadTimeout:     p.ReadTimeout,
		WriteTimeout:    p.WriteTimeout,
		PoolFIFO:        p.PoolFIFO,
		PoolSize:        p.PoolSize,
		PoolTimeout:     p.PoolTimeout,
		MinIdleConns:    p.MinIdleConns,
		MaxIdleConns:    p.MaxIdleConns,
		ConnMaxIdleTime: p.ConnMaxIdleTime,
		ConnMaxLifetime: p.ConnMaxLifetime,
	}

	return o, nil
}

func (p *RedisConnParams) addrs() (allAddrs []string, isCluster bool) {
	primary := net.JoinHostPort(p.Host, strconv.Itoa(int(p.Port)))

	if len(p.Addrs) == 0 {
		return []string{primary}, p.Cluster
	}

	return append([]string{primary}, p.Addrs...), true
}

const (
	netErrMissingPort = "missing port in address"
	netErrInvalidPort = "invalid port in address"
)

func getHostPortWithDefaults(hostRaw, defaultHost string, defaultPort uint16) (host string, port uint16, err error) {
	// possible cases:
	//  1 ✅ "example.com" — net.AddrError, should be just appended with default
	//    port
	//  2 ❌ ":" — no error, host=="", port=="", should throw an error
	//  3 ❌ "example.com:" — no error, host=="example.com", port=="", should
	//    throw an error (port semicolon means that there must be port)
	//  4 ✅ ":1234" — no error, host=="", port=="1234", should be appended with
	//    default host
	//  * ❌ "example.com:eight" — no error, host=="example.com", port=="eight",
	//    should throw invalid port (covered by [strconv.ParseUint])
	//  5 ❌ anything else throwed from [net.SplitHostPort] — throwing as is.
	//  * ✅ "example.com:1234" — no error, host=="example.com", port=="1234",
	//    best scenario (will be splitted with no errors)
	host, portRaw, err := net.SplitHostPort(hostRaw)
	switch e := new(net.AddrError); {
	case errors.As(err, &e) && e.Err == netErrMissingPort: // case 1
		portRaw = strconv.Itoa(int(defaultPort))

	case err == nil && host == "" && portRaw == "", // case 2
		err == nil && host != "" && portRaw == "": // case 3
		return "", 0, &net.AddrError{Err: netErrMissingPort, Addr: hostRaw}

	case err == nil && host == "" && portRaw != "": // case 4
		host = defaultHost

	case err != nil: // case 5
		return "", 0, err
	}

	if port64, err := strconv.ParseUint(portRaw, 10, 16); err != nil && portRaw == "" {
		return "", 0, &net.AddrError{Err: netErrInvalidPort, Addr: hostRaw}
	} else {
		port = uint16(port64)
	}

	return host, port, nil
}
