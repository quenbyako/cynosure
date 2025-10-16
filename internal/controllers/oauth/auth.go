package oauth

import (
	"context"
	"net/http"
	"strings"

	httpapi "github.com/quenbyako/cynosure/contrib/oauth-openapi/v1/go"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/accounts"
)

type Handler struct {
	srv *accounts.Service
}

var _ httpapi.Handler = (*Handler)(nil)

func NewHandler(srv *accounts.Service) http.Handler {
	h := &Handler{
		srv: srv,
	}

	inner, err := httpapi.NewServer(h)
	if err != nil {
		panic(err)
	}

	return inner
}

// GetAgentCard implements httpapi.Handler.
func (h *Handler) GetAgentCard(ctx context.Context, params httpapi.GetAgentCardParams) (httpapi.GetAgentCardRes, error) {
	return &httpapi.GetAgentCardOK{
		ProtocolVersion:    "0.3.0",
		Name:               "TestAgent",
		Description:        "Some test agent, idk",
		Version:            "0.1.0",
		URL:                "https://af9f40da2e5e.ngrok-free.app/agent",
		PreferredTransport: httpapi.NewOptGetAgentCardOKPreferredTransport(httpapi.GetAgentCardOKPreferredTransportJSONRPC),
		DefaultInputModes:  []string{"text/plain"},
		DefaultOutputModes: []string{"text/plain"},
		Skills: []httpapi.AgentSkill{
			{
				Name:        "TestSkill",
				Description: "A skill for testing purposes",
			},
		},
		Capabilities: httpapi.AgentCapabilities{
			PushNotifications: httpapi.NewOptBool(false),
			Streaming:         httpapi.NewOptBool(true),
		},
	}, nil
}

// OAuthCallbackGet implements httpapi.Handler.
func (h *Handler) OAuthCallbackGet(ctx context.Context, params httpapi.OAuthCallbackGetParams) (httpapi.OAuthCallbackGetRes, error) {
	if !params.State.IsSet() {
		return &httpapi.OAuthCallbackGetBadRequest{
			Data: strings.NewReader(errorPage),
		}, nil
	}

	if err := h.srv.ExchangeToken(ctx, params.Code, params.State.Value); err != nil {
		return &httpapi.OAuthCallbackGetDefStatusCode{
			StatusCode: http.StatusInternalServerError,
			Response: httpapi.OAuthCallbackGetDef{
				Data: strings.NewReader(err.Error()),
			},
		}, nil
	}

	return &httpapi.OAuthCallbackGetOK{
		Data: strings.NewReader(successPage),
	}, nil
}

const errorPage = `<html>
				<body>
					<h1>Oopsie</h1>
					<p>The 'state' parameter is missing or invalid.</p>
				</body>
			</html>`

const successPage = `<html>
				<body>
					<h1>Authorization Successful</h1>
					<p>You can now close this window and return to the chat.</p>
				</body>
			</html>`
