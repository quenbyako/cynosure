package gateway

import (
	"log/slog"
	"net/url"

	"github.com/quenbyako/core"
	"github.com/quenbyako/core/contrib/params/http"
	"github.com/quenbyako/core/contrib/params/secrets"
)

type Config struct {
	core.UnsafeActionConfig `env:"-"`

	CACerts      []string `env:"CA_CERTS"    default:""  envSeparator:","`
	VaultAddress *url.URL `env:"VAULT_ADDR"  default:""`

	LogLevel      slog.Level     `env:"CYNOSURE_LOG_LEVEL"    default:"info"`
	HttpPort      http.Server    `env:"CYNOSURE_HTTP_ADDR"    default:"http://0.0.0.0:5002"`
	MetricsPort   *url.URL       `env:"CYNOSURE_METRICS_ADDR" default:""`
	TelegramToken secrets.Secret `env:"CYNOSURE_TELEGRAM_TOKEN"`
	WebhookAddr   url.URL        `env:"CYNOSURE_TELEGRAM_WEBHOOK"`
	TLSKey        string         `env:"CYNOSURE_TLS_KEY"      default:""`
	TLSCert       string         `env:"CYNOSURE_TLS_CERT"     default:""`
	FileSecret    *url.URL       `env:"CYNOSURE_FILE_SECRETS" default:""`
	OtelHost      url.URL        `env:"CYNOSURE_OTEL_HOST"    default:"grpc://127.0.0.1:4317"`
}

var _ core.ActionConfig = (*Config)(nil)

func (f Config) GetLogLevel() slog.Level             { return f.LogLevel }
func (f Config) GetCertPaths() []string              { return f.CACerts }
func (f Config) ClientCertPaths() (cert, key string) { return f.TLSCert, f.TLSKey }
func (f Config) GetTraceEndpoint() *url.URL          { return &f.OtelHost }
func (f Config) GetMetricsAddr() *url.URL            { return f.MetricsPort }

func (f Config) GetSecretDSNs() map[string]*url.URL {
	return map[string]*url.URL{
		"file":  f.FileSecret,
		"vault": f.VaultAddress,
	}
}
