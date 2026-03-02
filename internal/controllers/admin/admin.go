package admin

import (
	"context"
	"fmt"
	"net/url"

	admin "github.com/quenbyako/cynosure/contrib/agent-proto/pkg/xelaj/agent/v1alpha1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/accounts"
)

type Handler struct {
	admin.UnsafeAdminServiceServer

	accounts *accounts.Usecase
}

var _ admin.AdminServiceServer = (*Handler)(nil)

func Register(accounts *accounts.Usecase) func(server grpc.ServiceRegistrar) {
	handler := &Handler{
		accounts: accounts,
	}

	return func(server grpc.ServiceRegistrar) {
		admin.RegisterAdminServiceServer(server, handler)
	}
}

func (h *Handler) AddServer(ctx context.Context, req *admin.AddServerRequest) (*admin.AddServerResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (h *Handler) Authorize(ctx context.Context, req *admin.AuthorizeRequest) (*admin.AuthorizeResponse, error) {
	mcpUrl, err := url.Parse(req.GetServerId())
	if err != nil {
		return nil, fmt.Errorf("invalid server URL: %w", err)
	}

	userID, err := ids.NewUserIDFromString(req.GetUserId())
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	link, err := h.accounts.AddAccount(ctx, userID, mcpUrl, req.GetAccountName(), req.GetAccountDesc())
	if err != nil {
		return nil, fmt.Errorf("failed to setup auth link: %w", err)
	}

	switch link := link.(type) {
	case accounts.AddAccountResponseAuthRequired:
		return &admin.AuthorizeResponse{
			Link: link.AuthURL().String(),
		}, nil
	case accounts.AddAccountResponseOK:
		return &admin.AuthorizeResponse{
			Link: "",
		}, nil
	default:
		panic(fmt.Errorf("unexpected accounts.AddAccountResponse: %#v", link))
	}
}
