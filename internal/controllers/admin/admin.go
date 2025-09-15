package admin

import (
	"context"
	"fmt"
	"net/url"

	"google.golang.org/grpc"

	admin "tg-helper/contrib/agent-proto/pkg/xelaj/agent/v1alpha1"

	"tg-helper/internal/domains/components/ids"
	"tg-helper/internal/domains/services/accounts"
	"tg-helper/internal/domains/services/servers"
)

type Handler struct {
	admin.UnsafeAdminServiceServer

	servers  *servers.Service
	accounts *accounts.Service
}

var _ admin.AdminServiceServer = (*Handler)(nil)

func Register(accounts *accounts.Service, servers *servers.Service) func(server grpc.ServiceRegistrar) {
	handler := &Handler{
		accounts: accounts,
		servers:  servers,
	}

	return func(server grpc.ServiceRegistrar) {
		admin.RegisterAdminServiceServer(server, handler)
	}
}

// AddServer implements admin.AdminServiceServer.
func (h *Handler) AddServer(ctx context.Context, req *admin.AddServerRequest) (*admin.AddServerResponse, error) {
	serverID, err := ids.NewServerIDFromString(req.GetId())
	if err != nil {
		return nil, fmt.Errorf("invalid server ID: %w", err)
	}
	serverURL, err := url.Parse(req.GetUrl())
	if err != nil {
		return nil, fmt.Errorf("invalid server URL: %w", err)
	}

	if err := h.servers.AddServer(ctx, serverID, serverURL); err != nil {
		return nil, fmt.Errorf("failed to register server: %w", err)
	}

	return &admin.AddServerResponse{}, nil
}

// Authorize implements admin.AdminServiceServer.
func (h *Handler) Authorize(ctx context.Context, req *admin.AuthorizeRequest) (*admin.AuthorizeResponse, error) {
	serverID, err := ids.NewServerIDFromString(req.GetServerId())
	if err != nil {
		return nil, fmt.Errorf("invalid server ID: %w", err)
	}
	userID, err := ids.NewUserIDFromString(req.GetUserId())
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	link, err := h.accounts.SetupAuthLink(ctx, serverID, userID, req.GetAccountName(), req.GetAccountDesc())
	if err != nil {
		return nil, fmt.Errorf("failed to setup auth link: %w", err)
	}

	return &admin.AuthorizeResponse{Link: link.String()}, nil
}
