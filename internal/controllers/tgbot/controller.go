package tgbot

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/k0kubun/pp/v3"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/quenbyako/cynosure/contrib/telegram-proto/pkg/telegram/botapi/v9"
	"github.com/quenbyako/cynosure/internal/domains/gateway/components"
	"github.com/quenbyako/cynosure/internal/domains/gateway/components/ids"
	"github.com/quenbyako/cynosure/internal/domains/gateway/entities"
	"github.com/quenbyako/cynosure/internal/domains/gateway/usecases"
)

type Handler struct {
	botapi.UnsafeWebhookServiceServer
	log LogCallbacks

	srv *usecases.Usecase
}

var _ botapi.WebhookServiceServer = (*Handler)(nil)

func NewHandler(logs LogCallbacks, srv *usecases.Usecase) http.Handler {
	h := &Handler{
		log: logs,
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
		pp.Println("Processing message")
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
		h.log.ProcessMessageIssue(ctx, msg.GetChat().GetId(), fmt.Errorf("making channel id: %w", err))

		return &emptypb.Empty{}, nil
	}

	messageID, err := ids.NewMessageID(channelID, strconv.Itoa(int(msg.GetMessageId())))
	if err != nil {
		h.log.ProcessMessageIssue(ctx, msg.GetChat().GetId(), fmt.Errorf("making message id: %w", err))

		return &emptypb.Empty{}, nil
	}

	userID, err := ids.NewUserID("telegram", strconv.FormatInt(msg.GetFrom().GetId(), 10))
	if err != nil {
		h.log.ProcessMessageIssue(ctx, msg.GetChat().GetId(), fmt.Errorf("making user id: %w", err))

		return &emptypb.Empty{}, nil
	}

	var messageOptions []entities.NewMessageOption
	if msg.GetText() != "" {
		text, err := components.NewMessageText(msg.GetText())
		if err != nil {
			h.log.ProcessMessageIssue(ctx, msg.GetChat().GetId(), fmt.Errorf("making message text: %w", err))

			return &emptypb.Empty{}, nil
		}
		messageOptions = append(messageOptions, entities.WithText(text))
	}

	message, err := entities.NewMessage(messageID, userID, messageOptions...)
	if err != nil {
		h.log.ProcessMessageIssue(ctx, msg.GetChat().GetId(), fmt.Errorf("making message entity: %w", err))
		return &emptypb.Empty{}, nil
	}

	pp.Println("going to process!")

	if err := h.srv.ReceiveNewMessageEvent(ctx, message); err != nil {
		h.log.ProcessMessageIssue(ctx, msg.GetChat().GetId(), fmt.Errorf("processing new message: %w", err))
		return &emptypb.Empty{}, nil
	}

	return &emptypb.Empty{}, nil
}
