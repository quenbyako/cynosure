package root

import (
	"log/slog"
	"net/url"
	"time"

	"github.com/google/uuid"
	"github.com/quenbyako/core"
	"github.com/quenbyako/core/contrib/params/grpc"
	"github.com/quenbyako/core/contrib/params/http"
	"github.com/quenbyako/core/contrib/params/secrets"
	"github.com/quenbyako/cynosure/contrib/core-params/httpclient"
	"github.com/quenbyako/cynosure/contrib/core-params/ratelimit"
)

// Note: reason for ignoring most of linters in config is that tag order is that
// it's easier to read and self-explanatory

//nolint:tagalign,lll // see above
type Config struct {
	core.UnsafeActionConfig `env:"-"`
	TelegramPort            http.Server       `env:"CYNOSURE_TELEGRAM_ADDR"             default:"http://0.0.0.0:5003"`
	GeminiKey               secrets.Secret    `env:"CYNOSURE_GEMINI_KEY"                default:""`
	OryClient               httpclient.Client `env:"CYNOSURE_ORY_API"                   default:"#timeout=30s"`
	OryClientSecret         secrets.Secret    `env:"CYNOSURE_ORY_CLIENT_SECRET"`
	OryAdminKey             secrets.Secret    `env:"CYNOSURE_ORY_ADMIN_API_KEY"`
	Port                    grpc.Server       `env:"CYNOSURE_GRPC_ADDR"                 default:"grpc://0.0.0.0:5001"`
	InternalMCPClient       httpclient.Client `env:"CYNOSURE_MCP_API_INTERNAL"          default:"#timeout=30s"`
	MCPPort                 http.Server       `env:"CYNOSURE_MCP_ADDR"                  default:"http://0.0.0.0:5004"`
	HTTPPort                http.Server       `env:"CYNOSURE_HTTP_ADDR"                 default:"http://0.0.0.0:5002"`
	ExternalMCPClient       httpclient.Client `env:"CYNOSURE_MCP_API_EXTERNAL"          default:"#timeout=30s&ssrf=true"`
	TelegramKey             secrets.Secret    `env:"CYNOSURE_TELEGRAM_KEY"`
	GeminiClient            httpclient.Client `env:"CYNOSURE_GEMINI_API"                default:"https://generativelanguage.googleapis.com#timeout=30s"`
	OpenRouterKey           secrets.Secret    `env:"CYNOSURE_OPENROUTER_KEY"            default:""`
	OpenRouterClient        httpclient.Client `env:"CYNOSURE_OPENROUTER_API"            default:"https://openrouter.ai/api/v1#timeout=30s"`
	TelegramClient          httpclient.Client `env:"CYNOSURE_TELEGRAM_API"              default:"https://api.telegram.org#rate=30/1s"`
	DatabaseURL             *url.URL          `env:"CYNOSURE_DATABASE_URL"`
	TelegramPublicAddr      *url.URL          `env:"CYNOSURE_TELEGRAM_PUBLIC_ADDR"`
	TokenizerCache          *url.URL          `env:"CYNOSURE_TOKENIZER_CACHE"           default:"file:///tmp/tokenizers"`
	FileSecret              *url.URL          `env:"CYNOSURE_FILE_SECRETS"              default:""`
	OtlpMetadata            map[string]string `env:"CYNOSURE_OTLP_METADATA"             default:""                                                      envSeparator:","`
	OryEndpoint             *url.URL          `env:"CYNOSURE_ORY_ISSUER_URL"`
	OtlpHost                *url.URL          `env:"CYNOSURE_OTLP_HOST"                 default:""`
	MetricsPort             *url.URL          `env:"CYNOSURE_METRICS_ADDR"              default:""`
	OAuthRedirectURL        *url.URL          `env:"CYNOSURE_OAUTH_REDIRECT_URL"        default:"http://localhost:5002/oauth/callback"`
	VaultAddress            *url.URL          `env:"VAULT_ADDR"                         default:""`
	AdminMCPServerID        string            `env:"CYNOSURE_ADMIN_MCP_SERVER_ID"`
	TLSKey                  string            `env:"CYNOSURE_TLS_KEY"                   default:""`
	OryClientID             string            `env:"CYNOSURE_ORY_CLIENT_ID"`
	TLSCert                 string            `env:"CYNOSURE_TLS_CERT"                  default:""`
	CACerts                 []string          `env:"CA_CERTS"                           default:""                                                      envSeparator:","`
	GlobEmbeddingRate       ratelimit.Policy  `env:"CYNOSURE_RATELIMIT_GLOB_EMBEDDING"  default:"1000000/24h"`
	GlobChatInputRate       ratelimit.Policy  `env:"CYNOSURE_RATELIMIT_GLOB_CHAT_INPUT" default:"1000000/24h"`
	EmbeddingRate           ratelimit.Policy  `env:"CYNOSURE_RATELIMIT_EMBEDDING"       default:"10000/12h"`
	ChatOutputRate          ratelimit.Policy  `env:"CYNOSURE_RATELIMIT_CHAT_OUTPUT"     default:"10000/12h"`
	ChatInputRate           ratelimit.Policy  `env:"CYNOSURE_RATELIMIT_CHAT_INPUT"      default:"10000/12h"`
	MaxWaitTimeLimit        time.Duration     `env:"CYNOSURE_RATELIMIT_MAX_WAIT"        default:"6h"`
	ChatHistoryLimit        uint              `env:"CYNOSURE_CHAT_HISTORY_LIMIT"        default:"50"`
	LogLevel                slog.Level        `env:"CYNOSURE_LOG_LEVEL"                 default:"info"`
	DefaultPlanID           uuid.UUID         `env:"CYNOSURE_DEFAULT_PLAN_ID"`
}

var _ core.ActionConfig = Config{}

//nolint:gocritic // calls once
func (f Config) GetLogLevel() slog.Level { return f.LogLevel }

//nolint:gocritic // calls once
func (f Config) GetCertPaths() []string { return f.CACerts }

//nolint:gocritic // calls once
func (f Config) ClientCertPaths() (cert, key string) { return f.TLSCert, f.TLSKey }

//nolint:gocritic // calls once
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

//nolint:gocritic // calls once
func (f Config) GetSecretDSNs() map[string]*url.URL {
	return map[string]*url.URL{
		"file":  f.FileSecret,
		"vault": f.VaultAddress,
	}
}
