package threads

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/quenbyako/cynosure/contrib/db/gen/go"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
)

var emptyTxOptions pgx.TxOptions

func (t *Threads) UpdateThread(ctx context.Context, thread entities.ThreadReadOnly) error {
	pending := thread.PendingEvents()
	if len(pending) == 0 {
		return nil
	}

	// thread.Messages() contains all messages including pending ones.
	totalMessages := len(thread.Messages())
	pendingCount := len(pending)
	startPos := int64(totalMessages - pendingCount) // 0-based count of committed messages

	transaction, err := t.tx.BeginTx(ctx, emptyTxOptions)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer transaction.Rollback(ctx)

	qtx := t.q.WithTx(transaction)

	currentPos := startPos

	for _, event := range pending {
		evt, ok := event.(entities.ThreadEventMessageAdded)
		if !ok {
			continue
		}

		msg := evt.Message()
		currentPos++

		err := t.insertMessage(ctx, qtx, thread.ID().String(), currentPos, msg)
		if err != nil {
			return fmt.Errorf("insert message at pos %d: %w", currentPos, err)
		}
	}

	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

var emptyUUID = pgtype.UUID{Valid: false, Bytes: [16]byte{}}

func (t *Threads) insertMessage(ctx context.Context, aueries *db.Queries, threadID string, pos int64, msg messages.Message) error {
	occPos := pos - 1
	//nolint:gosec // mergeTag is assumed to fit in int64
	mergeTag := int64(msg.MergeTag())

	switch msg := msg.(type) {
	case messages.MessageUser:
		_, err := aueries.InsertMessageUser(ctx, db.InsertMessageUserParams{
			Content:               msg.Content(),
			Position:              pos,
			ThreadID:              threadID,
			CurrentLastMessagePos: occPos,
			MergeTag:              mergeTag,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return errors.New("concurrent modification")
			}

			return err
		}

	case messages.MessageAssistant:
		agentID := msg.AgentID().ID()

		_, err := aueries.InsertMessageAssistant(ctx, db.InsertMessageAssistantParams{
			Text:                  msg.Content(),
			Reasoning:             msg.Reasoning(),
			AgentID:               agentID,
			Position:              pos,
			ThreadID:              threadID,
			CurrentLastMessagePos: occPos,
			MergeTag:              mergeTag,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return errors.New("concurrent modification")
			}

			return err
		}

	case messages.MessageToolRequest:
		argsBytes, _ := json.Marshal(msg.Arguments())

		_, err := aueries.InsertMessageToolRequest(ctx, db.InsertMessageToolRequestParams{
			ToolID:                emptyUUID, // Always NULL now as we don't look up IDs
			ToolName:              msg.ToolName(),
			ToolCallID:            msg.ToolCallID(),
			Reasoning:             msg.Reasoning(),
			Arguments:             argsBytes,
			Position:              pos,
			ThreadID:              threadID,
			CurrentLastMessagePos: occPos,
			MergeTag:              mergeTag,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return errors.New("concurrent modification")
			}

			return err
		}

	case messages.MessageToolResponse:
		reqPos, err := aueries.GetToolRequestPosition(ctx, db.GetToolRequestPositionParams{
			ThreadID:   threadID,
			ToolCallID: msg.ToolCallID(),
		})
		if err != nil {
			return fmt.Errorf("find tool request pos: %w", err)
		}

		_, err = aueries.InsertMessageToolResult(ctx, db.InsertMessageToolResultParams{
			RequestPosition:       reqPos,
			ToolCallID:            msg.ToolCallID(),
			IsError:               false,
			Content:               []byte(msg.Content()),
			Position:              pos,
			ThreadID:              threadID,
			CurrentLastMessagePos: occPos,
			MergeTag:              mergeTag,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return errors.New("concurrent modification")
			}

			return err
		}

	case messages.MessageToolError:
		reqPos, err := aueries.GetToolRequestPosition(ctx, db.GetToolRequestPositionParams{
			ThreadID:   threadID,
			ToolCallID: msg.ToolCallID(),
		})
		if err != nil {
			return fmt.Errorf("find tool request pos: %w", err)
		}

		_, err = aueries.InsertMessageToolResult(ctx, db.InsertMessageToolResultParams{
			RequestPosition:       reqPos,
			ToolCallID:            msg.ToolCallID(),
			IsError:               true,
			Content:               []byte(msg.Content()),
			Position:              pos,
			ThreadID:              threadID,
			CurrentLastMessagePos: occPos,
			MergeTag:              mergeTag,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return errors.New("concurrent modification")
			}

			return err
		}
	}

	return nil
}
