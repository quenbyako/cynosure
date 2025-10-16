package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"

	goose "github.com/quenbyako/cynosure/contrib/mongoose"
	"github.com/quenbyako/cynosure/contrib/mongoose/secrets"

	"github.com/quenbyako/cynosure/internal/apps/gateway"
)

type Flags struct {
	CACerts      []string `env:"CA_CERTS"    default:""  envSeparator:","`
	VaultAddress *url.URL `env:"VAULT_ADDR"  default:""`

	LogLevel    slog.Level             `env:"CYNOSURE_LOG_LEVEL"    default:"info"`
	Port        goose.GRPCServer       `env:"CYNOSURE_GRPC_ADDR"    default:"grpc://0.0.0.0:5001"`
	HttpPort    goose.HTTPServer       `env:"CYNOSURE_HTTP_ADDR"    default:"http://0.0.0.0:5002"`
	MetricsPort goose.PromhttpExporter `env:"CYNOSURE_METRICS_ADDR" default:""`
	ZepKey      secrets.Secret         `env:"CYNOSURE_ZEP_KEY"`
	GeminiKey   secrets.Secret         `env:"CYNOSURE_GEMINI_KEY"`
	TLSKey      string                 `env:"CYNOSURE_TLS_KEY"      default:""`
	TLSCert     string                 `env:"CYNOSURE_TLS_CERT"     default:""`
	FileSecret  *url.URL               `env:"CYNOSURE_FILE_SECRETS" default:""`
	OtelHost    url.URL                `env:"CYNOSURE_OTEL_HOST"    default:"grpc://127.0.0.1:4317"`
}

func (f Flags) GetLogLevel() slog.Level { return f.LogLevel }

func (f Flags) GetCertPaths() []string { return f.CACerts }

func (f Flags) ClientCertPaths() (cert, key string, ok bool) {
	return f.TLSCert, f.TLSKey, f.TLSCert != "" && f.TLSKey != ""
}

func (f Flags) GetSecretDSNs() map[string]*url.URL {
	return map[string]*url.URL{
		"file":  f.FileSecret,
		"vault": f.VaultAddress,
	}
}

func (f Flags) GetTraceEndpoint() *url.URL { return &f.OtelHost }

var _ goose.FlagDef = (*Flags)(nil)

func Cmd(ctx context.Context, appCtx goose.AppCtx[Flags]) int {
	_ = gateway.NewApp(ctx)

	jobs := []func(context.Context) error{
		appCtx.Config.Port.Serve,
		appCtx.Config.HttpPort.Serve,
	}
	if appCtx.Config.MetricsPort != nil {
		jobs = append(jobs, appCtx.Config.MetricsPort.Serve)
	}

	if err := goose.RunJobs(ctx, jobs...); err != nil {
		fmt.Println("Oopsie: ", err)

		return 1
	}

	return 0
}
