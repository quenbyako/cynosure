package tgbot

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/k0kubun/pp/v3"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/quenbyako/cynosure/contrib/telegram-proto/pkg/telegram/botapi/v9"
	"github.com/quenbyako/cynosure/internal/domains/gateway/components"
	"github.com/quenbyako/cynosure/internal/domains/gateway/components/ids"
	"github.com/quenbyako/cynosure/internal/domains/gateway/entities"
	"github.com/quenbyako/cynosure/internal/domains/gateway/usecases"
)

type Handler struct {
	botapi.UnsafeWebhookServiceServer

	srv *usecases.Usecase
}

var _ botapi.WebhookServiceServer = (*Handler)(nil)

func NewHandler(srv *usecases.Usecase) http.Handler {
	h := &Handler{
		srv: srv,
	}

	mux := runtime.NewServeMux()

	// NOTE: context is unused here.
	if err := botapi.RegisterWebhookServiceHandlerServer(context.TODO(), mux, h); err != nil {
		panic(fmt.Sprintf("unreachable: %v", err))
	}

	return mux

}

func (h *Handler) SendUpdate(ctx context.Context, update *botapi.Update) (*emptypb.Empty, error) {
	pp.Println("Received update:", update.String())

	updateID := update.GetUpdateId()
	switch upd := update.GetUpdate().(type) {
	case *botapi.Update_Message:
		if res, err := h.processMessage(ctx, updateID, upd.Message); err != nil {
			pp.Println("OOPS!:", err.Error())
			return nil, err
		} else {
			return res, nil
		}
	default:
		pp.Println("WARNING! unknown event type!")
		return &emptypb.Empty{}, nil
	}

}

func (h *Handler) processMessage(ctx context.Context, _ int64, msg *botapi.Message) (*emptypb.Empty, error) {
	channelID, err := ids.NewChannelID("telegram", strconv.FormatInt(msg.GetChat().GetId(), 10))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Errorf("invalid channel id: %v", err).Error())
	}

	messageID, err := ids.NewMessageID(channelID, strconv.Itoa(int(msg.GetMessageId())))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Errorf("making message id: %w", err).Error())
	}

	userID, err := ids.NewUserID("telegram", strconv.FormatInt(msg.GetFrom().GetId(), 10))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Errorf("making user id: %w", err).Error())
	}

	var messageOptions []entities.NewMessageOption
	if msg.GetText() != "" {
		text, err := components.NewMessageText(msg.GetText())
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, fmt.Errorf("invalid message text: %v", err).Error())
		}
		messageOptions = append(messageOptions, entities.WithText(text))
	}

	message, err := entities.NewMessage(messageID, userID, messageOptions...)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Errorf("making message entity: %w", err).Error())
	}

	if err := h.srv.ReceiveNewMessageEvent(ctx, message); err != nil {
		return nil, status.Error(codes.Internal, fmt.Errorf("processing new message: %w", err).Error())
	}

	return &emptypb.Empty{}, nil
}
