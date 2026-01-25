package datatransfer

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	db "github.com/quenbyako/cynosure/contrib/db/gen/go"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
)

// ThreadFromRows converts database rows into a domain aggregate.
// It assumes rows are ordered by position ascending.
func ThreadFromRows(rows []db.GetThreadWithMessagesRow) (*entities.Thread, error) {
	if len(rows) == 0 {
		return nil, fmt.Errorf("empty result set")
	}

	first := rows[0]

	threadID, err := ids.NewThreadIDFromString(first.ThreadID)
	if err != nil {
		return nil, fmt.Errorf("invalid thread id: %w", err)
	}

	// Note: We don't restore UserID here as entities.Thread doesn't store it explicitly.
	// We construct messages and validate the sequence.

	var msgs []messages.Message
	for i, row := range rows {
		if row.Position == nil {
			// This row represents the thread itself but without messages.
			continue
		}

		var msg messages.Message

		if msg, err = messageFromRow(row); err != nil {
			return nil, fmt.Errorf("mapping message index %d (pos %d): %w", i, *row.Position, err)
		}

		msgs = append(msgs, msg)
	}

	thread, err := entities.NewThread(threadID, msgs)
	if err != nil {
		return nil, fmt.Errorf("new thread: %w", err)
	}

	return thread, nil
}

func messageFromRow(row db.GetThreadWithMessagesRow) (messages.Message, error) {
	if row.MsgType == nil {
		return nil, fmt.Errorf("message type is nil")
	}

	mergeTag := uint64(0)
	if row.MergeTag != nil {
		//nolint:gosec // mergeTag is assumed to fit in uint64
		mergeTag = uint64(*row.MergeTag)
	}

	switch *row.MsgType {
	case "user":
		if row.UserContent == nil {
			return nil, fmt.Errorf("user content is nil")
		}

		msg, err := messages.NewMessageUser(
			*row.UserContent,
			messages.WithMessageUserMergeTag(mergeTag),
		)
		if err != nil {
			return nil, fmt.Errorf("new user message: %w", err)
		}

		return msg, nil

	case "assistant":
		text := ""
		if row.AssistantText != nil {
			text = *row.AssistantText
		}

		reasoning := ""
		if row.AssistantReasoning != nil {
			reasoning = *row.AssistantReasoning
		}

		var agentID ids.AgentID
		if !row.AssistantAgentID.Valid {
			// If agent ID is missing in DB (unlikely due to NOT NULL), we fail?
			// Schema says NOT NULL, so it should be valid.
			return nil, fmt.Errorf("assistant agent id is missing")
		}

		var err error
		// Convert pgtype.UUID (Bytes [16]byte) to uuid.UUID
		uid := uuid.UUID(row.AssistantAgentID.Bytes)

		agentID, err = ids.NewModelConfigID(uid)
		if err != nil {
			return nil, fmt.Errorf("invalid agent id: %w", err)
		}

		// Attachments are not yet supported in DB schema?
		// Currently ignored.

		msg, err := messages.NewMessageAssistant(
			text,
			messages.WithMessageAssistantReasoning(reasoning),
			messages.WithMessageAssistantAgentID(agentID),
			messages.WithMessageAssistantMergeTag(mergeTag),
		)
		if err != nil {
			return nil, fmt.Errorf("new assistant message: %w", err)
		}

		return msg, nil

	case "tool_request":
		if row.ReqToolName == nil || row.ReqToolCallID == nil || row.ReqArguments == nil {
			return nil, fmt.Errorf("tool request fields missing")
		}

		var args map[string]json.RawMessage
		if len(row.ReqArguments) > 0 {
			if err := json.Unmarshal(row.ReqArguments, &args); err != nil {
				return nil, fmt.Errorf("unmarshaling tool args: %w", err)
			}
		}

		reasoning := ""
		if row.ReqReasoning != nil {
			reasoning = *row.ReqReasoning
		}

		msg, err := messages.NewMessageToolRequest(
			args,
			*row.ReqToolName,
			*row.ReqToolCallID,
			messages.WithMessageToolRequestReasoning(reasoning),
			messages.WithMessageToolRequestMergeTag(mergeTag),
		)
		if err != nil {
			return nil, fmt.Errorf("new tool request: %w", err)
		}

		return msg, nil

	case "tool_result":
		if row.ResultContent == nil {
			return nil, fmt.Errorf("tool result content missing")
		}
		if row.ResultToolCallID == nil {
			return nil, fmt.Errorf("tool result call id missing")
		}

		var toolName string
		if row.ResultToolName != nil {
			toolName = *row.ResultToolName
		} else {
			// This happens if the referenced request is missing (data inconsistency)
			return nil, fmt.Errorf("tool result origin request missing (tool name unknown)")
		}

		isError := false
		if row.ResultIsError != nil {
			isError = *row.ResultIsError
		}

		content := json.RawMessage(row.ResultContent)

		if isError {
			msg, err := messages.NewMessageToolError(
				content,
				toolName,
				*row.ResultToolCallID,
				messages.WithMessageToolErrorMergeTag(mergeTag),
			)
			if err != nil {
				return nil, fmt.Errorf("new tool error: %w", err)
			}
			return msg, nil
		}

		msg, err := messages.NewMessageToolResponse(
			content,
			toolName,
			*row.ResultToolCallID,
			messages.WithMessageToolResponseMergeTag(mergeTag),
		)
		if err != nil {
			return nil, fmt.Errorf("new tool response: %w", err)
		}
		return msg, nil

	default:
		return nil, fmt.Errorf("unknown message type: %s", *row.MsgType)
	}
}
