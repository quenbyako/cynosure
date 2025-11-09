package usecases

import (
	"context"
	"errors"
	"fmt"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/quenbyako/cynosure/internal/domains/gateway/components"
	"github.com/quenbyako/cynosure/internal/domains/gateway/components/ids"
	"github.com/quenbyako/cynosure/internal/domains/gateway/entities"
	"github.com/quenbyako/cynosure/internal/domains/gateway/ports"
)

// Usecase orchestrates the message flow between Telegram messenger and A2A
// agent. It implements streaming response handling with time-based batching for
// optimal UX.
type Usecase struct {
	client ports.Messenger
	a2a    ports.Agent
}

// NewUsecase creates a new gateway usecase with the provided messenger client
// and A2A agent. Both parameters are required and will panic if nil.
func NewUsecase(
	client ports.Messenger,
	a2a ports.Agent,
) *Usecase {
	if client == nil {
		panic("messenger client is required")
	}
	if a2a == nil {
		panic("agent-to-agent component is required")
	}

	return &Usecase{
		client: client,
		a2a:    a2a,
	}
}

// ReceiveNewMessageEvent processes an incoming message from a messenger
// platform. It forwards the message to the A2A agent and streams the response
// back to the user with time-based batching (updates every 3 seconds) to avoid
// excessive API calls. Error handling: Sends user-friendly error messages for
// common failures (timeout, unavailable, etc.)
func (u *Usecase) ReceiveNewMessageEvent(ctx context.Context, msg *entities.Message) error {
	text, ok := msg.Text()
	if !ok {
		return nil // ignoring non-text messages
	}

	if err := u.client.NotifyProcessingStarted(ctx, msg.ID().ChannelID()); err != nil {
		return fmt.Errorf("notify processing started: %w", err)
	}

	resp, err := u.a2a.SendMessage(ctx, msg.ID(), text)
	if err != nil {
		friendlyMsg := userFriendlyError(err)
		if friendlyText, textErr := components.NewMessageText(friendlyMsg); textErr == nil {
			_, _ = u.client.SendMessage(ctx, msg.ID().ChannelID(), friendlyText)
		}
		return fmt.Errorf("failed to process message via a2a: %w", err)
	}

	// Streaming Response Handling with Time-Based Batching
	//
	// This implementation accumulates text chunks from the A2A agent and sends
	// updates to Telegram with intelligent throttling to avoid:
	// - Excessive Telegram API calls (rate limiting)
	// - Poor UX with constant "message edited" notifications
	// - Unnecessary network overhead
	//
	// Strategy:
	// 1. Send first chunk immediately (low latency for user feedback)
	// 2. Batch subsequent updates every 3 seconds
	// 3. Send final update when stream completes
	// 4. Handle errors gracefully with user notifications

	var sentMessageID ids.MessageID
	var accumulated string
	lastUpdateTime := time.Now()

	const updateInterval = 3 * time.Second

	// Track if we need to send a final update
	needsFinalUpdate := false

	// Consume streaming response with batched updates
	for part, err := range resp {
		if err != nil {
			friendlyMsg := userFriendlyError(err)
			if friendlyText, textErr := components.NewMessageText(friendlyMsg); textErr == nil {
				if sentMessageID.Valid() {
					// Update existing message with error
					_ = u.client.UpdateMessage(ctx, sentMessageID, friendlyText)
				} else {
					// Send new message with error
					_, _ = u.client.SendMessage(ctx, msg.ID().ChannelID(), friendlyText)
				}
			}
			return fmt.Errorf("streaming response error: %w", err)
		}

		// Accumulate the text
		accumulated += part.Text()
		needsFinalUpdate = true

		// Send initial message immediately with first chunk
		if !sentMessageID.Valid() {
			fullText, err := components.NewMessageText(accumulated)
			if err != nil {
				return fmt.Errorf("creating initial message text: %w", err)
			}
			sentMessageID, err = u.client.SendMessage(ctx, msg.ID().ChannelID(), fullText)
			if err != nil {
				return fmt.Errorf("sending initial message via messenger: %w", err)
			}
			lastUpdateTime = time.Now()
			needsFinalUpdate = false
			continue
		}

		// Check if enough time has passed for an update
		now := time.Now()
		if now.Sub(lastUpdateTime) >= updateInterval {
			fullText, err := components.NewMessageText(accumulated)
			if err != nil {
				return fmt.Errorf("creating accumulated message text: %w", err)
			}
			if err := u.client.UpdateMessage(ctx, sentMessageID, fullText); err != nil {
				return fmt.Errorf("updating message via messenger: %w", err)
			}
			lastUpdateTime = now
			needsFinalUpdate = false
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Continue processing
		}
	}

	if needsFinalUpdate && sentMessageID.Valid() {
		fullText, err := components.NewMessageText(accumulated)
		if err != nil {
			return fmt.Errorf("creating final message text: %w", err)
		}
		if err := u.client.UpdateMessage(ctx, sentMessageID, fullText); err != nil {
			return fmt.Errorf("updating final message via messenger: %w", err)
		}
	}

	return nil
}

// TODO: STRICTLY NECESSARY to move this part into localization package later
//
// userFriendlyError converts technical errors into user-friendly messages with emojis.
// This function categorizes common failure scenarios and provides helpful guidance to users.
// Used by the gateway usecase to communicate errors clearly to end users via Telegram.
//
// Error categories handled:
// - Context deadline exceeded: Agent timeout
// - codes.Unavailable: Service temporarily down
// - codes.ResourceExhausted: Service overloaded
// - codes.Unauthenticated/PermissionDenied: Access issues
// - codes.InvalidArgument: Bad input format
// - Default: Generic error with details
func userFriendlyError(err error) string {
	if err == nil {
		return ""
	}

	// Check for context deadline exceeded
	if errors.Is(err, context.DeadlineExceeded) {
		return "⏱ The agent is taking too long to respond. Please try again later."
	}

	// Check for gRPC status errors
	if st, ok := status.FromError(err); ok {
		switch st.Code() {
		case codes.Unavailable:
			return "🔌 The agent service is temporarily unavailable. Please try again in a few moments."
		case codes.DeadlineExceeded:
			return "⏱ The agent is taking too long to respond. Please try again later."
		case codes.Canceled:
			return "🚫 The request was canceled. Please try again."
		case codes.ResourceExhausted:
			return "⚠️ The service is currently overloaded. Please try again in a few moments."
		case codes.Unauthenticated:
			return "🔐 Authentication failed. Please check your credentials."
		case codes.PermissionDenied:
			return "🚫 You don't have permission to perform this action."
		case codes.InvalidArgument:
			return "❌ Invalid message format. Please check your input."
		}
	}

	// Default error message
	return fmt.Sprintf("❌ An unexpected error occurred: %v", err)
}
