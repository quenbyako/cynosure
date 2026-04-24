package threads

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/quenbyako/cynosure/contrib/db/gen/go"

	errors1 "github.com/quenbyako/cynosure/internal/adapters/sql/errors"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
)

var emptyTxOptions pgx.TxOptions

func (t *Threads) UpdateThread(ctx context.Context, thread entities.ThreadReadOnly) error {
	pending := thread.PendingEvents()
	if len(pending) == 0 {
		return nil
	}

	transaction, err := t.tx.BeginTx(ctx, emptyTxOptions)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	//nolint:errcheck // it makes no sense to check the error in defer
	defer transaction.Rollback(ctx)

	qtx := t.q.WithTx(transaction)
	if err := t.processPendingEvents(ctx, qtx, thread); err != nil {
		return err
	}

	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

func (t *Threads) processPendingEvents(
	ctx context.Context, qtx *db.Queries, thread entities.ThreadReadOnly,
) error {
	pending := thread.PendingEvents()
	totalMessages := len(thread.Messages(0))
	startPos := int64(totalMessages - len(pending))
	currentPos := startPos

	for _, event := range pending {
		evt, ok := event.(entities.ThreadEventMessageAdded)
		if !ok {
			continue
		}

		currentPos++

		threadID := thread.ID().String()
		if err := t.insertMessage(ctx, qtx, threadID, currentPos, evt.Message()); err != nil {
			return fmt.Errorf("insert message at pos %d: %w", currentPos, err)
		}
	}

	return nil
}

var emptyUUID = pgtype.UUID{Valid: false, Bytes: [16]byte{}}

func (t *Threads) insertMessage(
	ctx context.Context, qtx *db.Queries, threadID string, pos int64, msg messages.Message,
) error {
	occPos := pos - 1
	//nolint:gosec // mergeTag is assumed to fit in int64
	mergeTag := int64(msg.MergeTag())

	switch msg := msg.(type) {
	case messages.MessageUser:
		return t.insertMessageUser(ctx, qtx, threadID, pos, occPos, mergeTag, msg)
	case messages.MessageAssistant:
		return t.insertMessageAssistant(ctx, qtx, threadID, pos, occPos, mergeTag, msg)
	case messages.MessageToolRequest:
		return t.insertMessageToolRequest(ctx, qtx, threadID, pos, occPos, mergeTag, msg)
	case messages.MessageToolResponse:
		return t.insertMessageToolResult(
			ctx, qtx, threadID, pos, occPos, mergeTag, msg, false,
		)
	case messages.MessageToolError:
		return t.insertMessageToolResult(
			ctx, qtx, threadID, pos, occPos, mergeTag, msg, true,
		)
	default:
		return errors1.ErrMessageTypeUnknown
	}
}

func (t *Threads) insertMessageUser(
	ctx context.Context, qtx *db.Queries, threadID string,
	pos, occPos, mergeTag int64, msg messages.MessageUser,
) error {
	_, err := qtx.InsertMessageUser(ctx, db.InsertMessageUserParams{
		Content: msg.Content(), Position: pos, ThreadID: threadID,
		CurrentLastMessagePos: occPos, MergeTag: mergeTag,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors1.ErrConcurrentModification
		}

		return fmt.Errorf("insert user msg: %w", err)
	}

	return nil
}

func (t *Threads) insertMessageAssistant(
	ctx context.Context, qtx *db.Queries, threadID string,
	pos, occPos, mergeTag int64, msg messages.MessageAssistant,
) error {
	_, err := qtx.InsertMessageAssistant(ctx, db.InsertMessageAssistantParams{
		Text: msg.Content(), Reasoning: msg.Reasoning(), AgentID: msg.AgentID().ID(),
		Position: pos, ThreadID: threadID, CurrentLastMessagePos: occPos, MergeTag: mergeTag,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors1.ErrConcurrentModification
		}

		return fmt.Errorf("insert assistant msg: %w", err)
	}

	return nil
}

func (t *Threads) insertMessageToolRequest(
	ctx context.Context, qtx *db.Queries, threadID string,
	pos, occPos, mergeTag int64, msg messages.MessageToolRequest,
) error {
	args, err := json.Marshal(msg.Arguments())
	if err != nil {
		return fmt.Errorf("marshal tool args: %w", err)
	}

	_, err = qtx.InsertMessageToolRequest(ctx, db.InsertMessageToolRequestParams{
		ToolID: emptyUUID, ToolName: msg.ToolName(), ToolCallID: msg.ToolCallID(),
		Reasoning: msg.Reasoning(), Arguments: args, Position: pos, ThreadID: threadID,
		CurrentLastMessagePos: occPos, MergeTag: mergeTag,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors1.ErrConcurrentModification
		}

		return fmt.Errorf("insert tool request: %w", err)
	}

	return nil
}

func (t *Threads) insertMessageToolResult(
	ctx context.Context, qtx *db.Queries, threadID string,
	pos, occPos, mergeTag int64, msg messages.MessageTool, isError bool,
) error {
	reqPos, err := qtx.GetToolRequestPosition(ctx, db.GetToolRequestPositionParams{
		ThreadID: threadID, ToolCallID: msg.ToolCallID(),
	})
	if err != nil {
		return fmt.Errorf("find tool request pos: %w", err)
	}

	_, err = qtx.InsertMessageToolResult(ctx, db.InsertMessageToolResultParams{
		RequestPosition: reqPos, ToolCallID: msg.ToolCallID(), IsError: isError,
		Content: []byte(msg.Content()), Position: pos, ThreadID: threadID,
		CurrentLastMessagePos: occPos, MergeTag: mergeTag,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors1.ErrConcurrentModification
		}

		return fmt.Errorf("insert tool result: %w", err)
	}

	return nil
}
