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

func (t *Threads) UpdateThread(ctx context.Context, thread entities.ThreadReadOnly) error {
	pending := thread.PendingEvents()
	if len(pending) == 0 {
		return nil
	}

	// thread.Messages() contains all messages including pending ones.
	totalMessages := len(thread.Messages())
	pendingCount := len(pending)
	startPos := int64(totalMessages - pendingCount) // 0-based count of committed messages

	tx, err := t.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := t.q.WithTx(tx)

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

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

func (t *Threads) insertMessage(ctx context.Context, q *db.Queries, threadID string, pos int64, msg messages.Message) error {
	occPos := pos - 1
	//nolint:gosec // mergeTag is assumed to fit in int64
	mergeTag := int64(msg.MergeTag())

	switch m := msg.(type) {
	case messages.MessageUser:
		_, err := q.InsertMessageUser(ctx, db.InsertMessageUserParams{
			Content:               m.Content(),
			Position:              pos,
			ThreadID:              threadID,
			CurrentLastMessagePos: occPos,
			MergeTag:              mergeTag,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("concurrent modification")
			}
			return err
		}

	case messages.MessageAssistant:
		agentID := m.AgentID().ID()

		_, err := q.InsertMessageAssistant(ctx, db.InsertMessageAssistantParams{
			Text:                  m.Text(),
			Reasoning:             m.Reasoning(),
			AgentID:               agentID,
			Position:              pos,
			ThreadID:              threadID,
			CurrentLastMessagePos: occPos,
			MergeTag:              mergeTag,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("concurrent modification")
			}
			return err
		}

	case messages.MessageToolRequest:
		argsBytes, _ := json.Marshal(m.Arguments())

		_, err := q.InsertMessageToolRequest(ctx, db.InsertMessageToolRequestParams{
			ToolID:                pgtype.UUID{Valid: false}, // Always NULL now as we don't look up IDs
			ToolName:              m.ToolName(),
			ToolCallID:            m.ToolCallID(),
			Reasoning:             m.Reasoning(),
			Arguments:             argsBytes,
			Position:              pos,
			ThreadID:              threadID,
			CurrentLastMessagePos: occPos,
			MergeTag:              mergeTag,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("concurrent modification")
			}
			return err
		}

	case messages.MessageToolResponse:
		reqPos, err := q.GetToolRequestPosition(ctx, db.GetToolRequestPositionParams{
			ThreadID:   threadID,
			ToolCallID: m.ToolCallID(),
		})
		if err != nil {
			return fmt.Errorf("find tool request pos: %w", err)
		}

		_, err = q.InsertMessageToolResult(ctx, db.InsertMessageToolResultParams{
			RequestPosition:       reqPos,
			ToolCallID:            m.ToolCallID(),
			IsError:               false,
			Content:               []byte(m.Content()),
			Position:              pos,
			ThreadID:              threadID,
			CurrentLastMessagePos: occPos,
			MergeTag:              mergeTag,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("concurrent modification")
			}
			return err
		}

	case messages.MessageToolError:
		reqPos, err := q.GetToolRequestPosition(ctx, db.GetToolRequestPositionParams{
			ThreadID:   threadID,
			ToolCallID: m.ToolCallID(),
		})
		if err != nil {
			return fmt.Errorf("find tool request pos: %w", err)
		}

		_, err = q.InsertMessageToolResult(ctx, db.InsertMessageToolResultParams{
			RequestPosition:       reqPos,
			ToolCallID:            m.ToolCallID(),
			IsError:               true,
			Content:               []byte(m.Content()),
			Position:              pos,
			ThreadID:              threadID,
			CurrentLastMessagePos: occPos,
			MergeTag:              mergeTag,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("concurrent modification")
			}
			return err
		}
	}

	return nil
}
