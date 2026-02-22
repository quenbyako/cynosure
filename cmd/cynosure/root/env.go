package root

import (
	"log/slog"
	"net/url"

	"github.com/quenbyako/core"
	"github.com/quenbyako/core/contrib/params/grpc"
	"github.com/quenbyako/core/contrib/params/http"
	"github.com/quenbyako/core/contrib/params/secrets"
)

type Config struct {
	core.UnsafeActionConfig `env:"-"`

	CACerts      []string `env:"CA_CERTS"    default:""  envSeparator:","`
	VaultAddress *url.URL `env:"VAULT_ADDR"  default:""`

	LogLevel           slog.Level     `env:"CYNOSURE_LOG_LEVEL"     default:"info"`
	Port               grpc.Server    `env:"CYNOSURE_GRPC_ADDR"     default:"grpc://0.0.0.0:5001"`
	HttpPort           http.Server    `env:"CYNOSURE_HTTP_ADDR"     default:"http://0.0.0.0:5002"`
	TelegramPort       http.Server    `env:"CYNOSURE_TELEGRAM_ADDR" default:"http://0.0.0.0:5003"`
	MCPPort            http.Server    `env:"CYNOSURE_MCP_ADDR"      default:"http://0.0.0.0:5004"`
	DatabaseURL        *url.URL       `env:"CYNOSURE_DATABASE_URL"`
	GeminiKey          secrets.Secret `env:"CYNOSURE_GEMINI_KEY"`
	TelegramKey        secrets.Secret `env:"CYNOSURE_TELEGRAM_KEY"`
	TelegramPublicAddr *url.URL       `env:"CYNOSURE_TELEGRAM_PUBLIC_ADDR"`
	TLSKey             string         `env:"CYNOSURE_TLS_KEY"      default:""`
	TLSCert            string         `env:"CYNOSURE_TLS_CERT"     default:""`
	FileSecret         *url.URL       `env:"CYNOSURE_FILE_SECRETS" default:""`
	OryAdminKey        secrets.Secret `env:"CYNOSURE_ORY_ADMIN_API_KEY"`
	OryEndpoint        *url.URL       `env:"CYNOSURE_ORY_ISSUER_URL"`
	OryJwksUrl         *url.URL       `env:"CYNOSURE_ORY_JWKS_URL"`

	MetricsPort  *url.URL          `env:"CYNOSURE_METRICS_ADDR"  default:""`
	OtlpHost     *url.URL          `env:"CYNOSURE_OTLP_HOST"     default:""`
	OtlpMetadata map[string]string `env:"CYNOSURE_OTLP_METADATA" default:"" envSeparator:","`
}

var _ core.ActionConfig = (*Config)(nil)

func (f Config) GetLogLevel() slog.Level             { return f.LogLevel }
func (f Config) GetCertPaths() []string              { return f.CACerts }
func (f Config) ClientCertPaths() (cert, key string) { return f.TLSCert, f.TLSKey }
func (f Config) GetObservabilityConfig() core.ObservabilityConfig {
	var metricsPort *url.URL
	// TODO: какой-то баг с портом: если не указывать, то он пихает нулевое НЕ NIL значение
	if f.MetricsPort != nil && f.MetricsPort.Host != "" {
		metricsPort = f.MetricsPort
	}

	return core.ObservabilityConfig{
		MetricsEndpoint: metricsPort,
		TraceEndpoint:   f.OtlpHost,
		OtlpMetadata:    f.OtlpMetadata,
	}
}

func (f Config) GetSecretDSNs() map[string]*url.URL {
	return map[string]*url.URL{
		"file":  f.FileSecret,
		"vault": f.VaultAddress,
	}
}
