package goose

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"slices"

	"github.com/caarlos0/env/v11"
	"github.com/open-feature/go-sdk/openfeature"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/quenbyako/cynosure/contrib/mongoose/metrics"
	"github.com/quenbyako/cynosure/contrib/mongoose/secrets"
	"github.com/quenbyako/cynosure/contrib/onelog"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

type AppCtx[T any] struct {
	Stdin          io.Reader
	Stdout         io.Writer
	Log            slog.Handler
	Metric         metric.MeterProvider
	Trace          trace.TracerProvider
	Features       openfeature.IClient
	CACertificates *x509.CertPool
	Config         T
	Version        Version
	IsPipeline     bool
}

type ActionFunc[T any] func(ctx context.Context, appCtx AppCtx[T]) int

type FlagDef interface {
	GetLogLevel() slog.Level                      // log level
	GetCertPaths() []string                       // paths to CA certificates
	ClientCertPaths() (cert, key string, ok bool) // path to client certificate
	GetSecretDSNs() map[string]*url.URL           // secret DSNs // TODO: two engines with one protocol? like vault-1:// and vault-2://?
	GetTraceEndpoint() *url.URL                   // OTEL trace endpoint
}

type ListenerPreset struct {
	Network string // tcp, tcp4, tcp6, unix, unixpacket
}

// Run is a helper function for cobra.Command.PreRun that loads flags
// from environment and merges them with flags from command line.
func Run[T FlagDef](action ActionFunc[T]) func(context.Context, []string) int {
	return func(ctx context.Context, _ []string) int {
		var flags T

		var configurations []configureType

		mappers := map[reflect.Type]env.ParserFunc{
			reflect.TypeFor[slog.Level]():       func(v string) (any, error) { return parseLogLevel(v) },
			reflect.TypeFor[*url.URL]():         func(v string) (any, error) { return url.Parse(v) },
			reflect.TypeFor[url.URL]():          func(v string) (any, error) { return valueErr(url.Parse(v)) },
			reflect.TypeFor[secrets.Secret]():   parseSecret(&configurations),
			reflect.TypeFor[net.Listener]():     parseListener(&configurations),
			reflect.TypeFor[GRPCServer]():       parseGRPCServer(&configurations),
			reflect.TypeFor[HTTPServer]():       parseHTTPServer(&configurations),
			reflect.TypeFor[PromhttpExporter](): parsePromhttpExporter(&configurations),
		}

		environ := env.ToMap(os.Environ())

		err := env.ParseWithOptions(&flags, envParams(environ, mappers))
		// warn: aggregate error is not returned by value, not by pointer
		if e := new(env.AggregateError); errors.As(err, e) {
			var missedFields []string

			for _, err := range e.Errors {
				if e := new(env.VarIsNotSetError); errors.As(err, e) {
					missedFields = append(missedFields, e.Key)
				} else {
					panic(err)
				}
			}

			slices.Sort(missedFields)

			if len(missedFields) > 0 {
				fmt.Fprintf(os.Stderr, "missing required environment variables: %v\n", missedFields)
			} else {
				panic("internal error: env.AggregateError without env.VarIsNotSetError")
			}

			return 1
		} else if err != nil {
			panic(err)
		}

		logHandler := defaultLogger(os.Stderr, flags.GetLogLevel())
		var log LogCallbacks = &logger{log: onelog.Wrap(logHandler)}

		log.EffectiveEnvironment(getEffectiveEnvironment(&flags, environ))

		var clientCert tls.Certificate
		if certPath, keyPath, ok := flags.ClientCertPaths(); ok {
			var err error
			if clientCert, err = tls.LoadX509KeyPair(certPath, keyPath); err != nil {
				panic(fmt.Errorf("loading client certificate: %w", err))
			}
		}

		secretEngine, err := secrets.BuildSecretEngine(ctx, flags.GetSecretDSNs())
		if err != nil {
			panic(fmt.Errorf("building secret engine: %w", err))
		}
		caCerts := loadCertificates(flags.GetCertPaths())
		version, _ := VersionFromContext(ctx)
		pipes := pipelineFromContext(ctx)

		var opts []metrics.RegisterOption
		if u := flags.GetTraceEndpoint(); u != nil {
			opts = append(opts, metrics.WithTraceURL(u))
		}

		m := metrics.RegisterPushMetrics(ctx, "some-service", version.Version, opts...)

		cfgData := configureData{
			appCert:      clientCert,
			pool:         caCerts,
			logger:       logHandler,
			secretEngine: secretEngine,
			version:      version,
			metric:       m,
			trace:        m,
			gatherer:     m.Gatherer(),
		}

		// configuring
		var errs []error

		for _, v := range configurations {
			if err := v.configure(ctx, &cfgData); err != nil {
				errs = append(errs, err)
			}
		}

		if len(errs) > 0 {
			for _, err := range errs {
				fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
			}
			return 1
		}

		acquireData := acquireData{
			configureData: cfgData,
		}

		for _, v := range configurations {
			if err := v.acquire(ctx, &acquireData); err != nil {
				panic(fmt.Errorf("configuring %T: %w", v, err))
			}
		}

		code := action(ctx, AppCtx[T]{
			IsPipeline: pipes.isPipeline,
			Stdin:      pipes.stdin,
			Stdout:     pipes.stdout,
			Log:        logHandler,
			Metric:     m,
			Trace:      m,
			Config:     flags,
			Version:    version,
		})

		shutdownData := shutdownData{
			acquireData: acquireData,
		}

		for _, v := range configurations {
			if err := v.shutdown(ctx, &shutdownData); err != nil {
				panic(fmt.Errorf("shutting down %T: %w", v, err))
			}
		}

		return code
	}
}

type configureData struct {
	appCert      tls.Certificate
	pool         *x509.CertPool
	logger       slog.Handler
	secretEngine *secrets.SecretEngine
	version      Version
	metric       metric.MeterProvider
	trace        trace.TracerProvider
	gatherer     prometheus.Gatherer
}

type acquireData struct {
	configureData
}

type shutdownData struct {
	acquireData
}

type configureType interface {
	configure(ctx context.Context, data *configureData) error
	acquire(ctx context.Context, data *acquireData) error
	shutdown(ctx context.Context, data *shutdownData) error
}

func valueErr[T any](v *T, err error) (T, error) {
	if err != nil {
		return *new(T), err
	} else {
		return *v, nil
	}
}

func loadCertificates(additionalPaths []string) *x509.CertPool {
	certPool, err := x509.SystemCertPool()
	if err != nil {
		// On Windows, SystemCertPool() always returns nil, nil.
		// On other systems, a non-nil error means we couldn't get the system pool.
		// In either case, we create a new cert pool.
		fmt.Println("warning: failed to load system CA certificates, using empty cert pool") // TODO: use logger LATER
		certPool = x509.NewCertPool()
	}

	for _, globPath := range additionalPaths {
		// TODO: no os filesystem!!! only [fs.FS]!
		paths, err := filepath.Glob(globPath)
		if err != nil {
			// todo: for now panic, later return error
			panic(fmt.Errorf("parsing glob %q: %w", globPath, err))
		}

		for _, path := range paths {
			data, err := os.ReadFile(path)
			if err != nil {
				// todo: for now panic, later return error
				panic(fmt.Errorf("reading CA certificate %q: %w", path, err))
			}

			cert, err := x509.ParseCertificate(data)
			if err != nil {
				// todo: for now panic, later return error
				panic(fmt.Errorf("parsing CA certificate %q: %w", path, err))
			}

			certPool.AddCert(cert)
		}
	}

	return certPool
}

func getEffectiveEnvironment(config any, e map[string]string) map[string]string {
	fields, err := env.GetFieldParamsWithOptions(config, envParams(nil, nil))
	if err != nil {
		panic(err)
	}

	params := make(map[string]string)
	for _, field := range fields {
		params[field.Key] = field.DefaultValue
	}

	for k, v := range e {
		if _, ok := params[k]; ok {
			params[k] = v
		}
	}

	return params
}

func envParams(e map[string]string, mappers map[reflect.Type]env.ParserFunc) env.Options {
	return env.Options{
		TagName:             "env",
		PrefixTagName:       "prefix",
		DefaultValueTagName: "default",
		RequiredIfNoDef:     true,
		Environment:         e,
		FuncMap:             mappers,
	}
}
