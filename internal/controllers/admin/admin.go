// Package admin implements admin controller.
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

func Register(usecase *accounts.Usecase) func(server grpc.ServiceRegistrar) {
	handler := &Handler{
		UnsafeAdminServiceServer: nil,
		accounts:                 usecase,
	}

	return func(server grpc.ServiceRegistrar) {
		admin.RegisterAdminServiceServer(server, handler)
	}
}

func (h *Handler) AddServer(
	ctx context.Context, req *admin.AddServerRequest,
) (*admin.AddServerResponse, error) {
	//nolint:wrapcheck // yes, that's the point to use external error!
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (h *Handler) Authorize(
	ctx context.Context, req *admin.AuthorizeRequest,
) (*admin.AuthorizeResponse, error) {
	mcpURL, err := url.Parse(req.GetServerId())
	if err != nil {
		return nil, fmt.Errorf("invalid server URL: %w", err)
	}

	userID, err := ids.NewUserIDFromString(req.GetUserId())
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	link, err := h.accounts.AddAccount(
		ctx, userID, mcpURL, req.GetAccountName(), req.GetAccountDesc(),
	)
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
		return nil, status.Errorf(codes.Internal,
			"unexpected accounts.AddAccountResponse: %#v", link,
		)
	}
}
