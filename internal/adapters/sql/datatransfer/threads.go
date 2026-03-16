package datatransfer

import (
	"encoding/json"
	"fmt"

	db "github.com/quenbyako/cynosure/contrib/db/gen/go"

	"github.com/quenbyako/cynosure/internal/adapters/sql/errors"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
)

// ThreadFromRows converts database rows into a domain aggregate.
// It assumes rows are ordered by position ascending.
func ThreadFromRows(rows []db.GetThreadWithMessagesRow) (*entities.Thread, error) {
	if len(rows) == 0 {
		return nil, errors.ErrEmptyResultSet
	}

	threadID, err := ids.NewThreadIDFromString(rows[0].ThreadID)
	if err != nil {
		return nil, fmt.Errorf("invalid thread id: %w", err)
	}

	msgs, err := mapThreadMessages(rows)
	if err != nil {
		return nil, err
	}

	thread, err := entities.NewThread(threadID, msgs)
	if err != nil {
		return nil, fmt.Errorf("new thread: %w", err)
	}

	return thread, nil
}

func mapThreadMessages(rows []db.GetThreadWithMessagesRow) ([]messages.Message, error) {
	msgs := make([]messages.Message, 0, len(rows))
	for i := range rows {
		row := &rows[i]
		if row.Position == nil {
			continue
		}

		m, err := messageFromRow(row)
		if err != nil {
			return nil, fmt.Errorf("mapping message index %d (pos %d): %w", i, *row.Position, err)
		}

		msgs = append(msgs, m)
	}

	if len(msgs) == 0 {
		return nil, errors.ErrThreadNoMessages
	}

	return msgs, nil
}

//nolint:ireturn // returning interface is intended for polymorphic messages
func messageFromRow(row *db.GetThreadWithMessagesRow) (messages.Message, error) {
	if row.MsgType == nil {
		return nil, errors.ErrMessageTypeNil
	}

	var mergeTag uint64
	if row.MergeTag != nil {
		//nolint:gosec // mergeTag is assumed to fit in uint64
		mergeTag = uint64(*row.MergeTag)
	}

	switch *row.MsgType {
	case "user":
		return mapUserMessage(row, mergeTag)
	case "assistant":
		return mapAssistantMessage(row, mergeTag)
	case "tool_request":
		return mapToolRequest(row, mergeTag)
	case "tool_result":
		return mapToolResult(row, mergeTag)
	default:
		return nil, fmt.Errorf("%w: %s", errors.ErrMessageTypeUnknown, *row.MsgType)
	}
}

//nolint:ireturn // returning interface is intended for polymorphic messages
func mapUserMessage(row *db.GetThreadWithMessagesRow, mergeTag uint64) (messages.Message, error) {
	if row.UserContent == nil {
		return nil, errors.ErrUserContentNil
	}

	msg, err := messages.NewMessageUser(*row.UserContent,
		messages.WithMessageUserMergeTag(mergeTag))
	if err != nil {
		return nil, fmt.Errorf("new user message: %w", err)
	}

	return msg, nil
}

//nolint:ireturn // returning interface is intended for polymorphic messages
func mapAssistantMessage(
	row *db.GetThreadWithMessagesRow,
	mergeTag uint64,
) (messages.Message, error) {
	userID, err := ids.NewUserID(row.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user id: %w", err)
	}

	agentID, err := ids.NewAgentID(userID, row.AssistantAgentID)
	if err != nil {
		return nil, fmt.Errorf("invalid agent id: %w", err)
	}

	opts := []messages.NewMessageAssistantOpt{
		messages.WithMessageAssistantReasoning(deref(row.AssistantReasoning)),
		messages.WithMessageAssistantAgentID(agentID),
		messages.WithMessageAssistantMergeTag(mergeTag),
	}

	msg, err := messages.NewMessageAssistant(deref(row.AssistantText), opts...)
	if err != nil {
		return nil, fmt.Errorf("new assistant message: %w", err)
	}

	return msg, nil
}

//nolint:ireturn // returning interface is intended for polymorphic messages
func mapToolRequest(row *db.GetThreadWithMessagesRow, mergeTag uint64) (messages.Message, error) {
	if row.ReqToolName == nil || row.ReqToolCallID == nil || row.ReqArguments == nil {
		return nil, errors.ErrToolRequestFieldsMissing
	}

	var args map[string]json.RawMessage
	if len(row.ReqArguments) > 0 {
		if err := json.Unmarshal(row.ReqArguments, &args); err != nil {
			return nil, fmt.Errorf("unmarshaling tool args: %w", err)
		}
	}

	opts := []messages.NewMessageToolRequestOpt{
		messages.WithMessageToolRequestReasoning(deref(row.ReqReasoning)),
		messages.WithMessageToolRequestMergeTag(mergeTag),
	}

	msg, err := messages.NewMessageToolRequest(args, *row.ReqToolName, *row.ReqToolCallID, opts...)
	if err != nil {
		return nil, fmt.Errorf("new tool request: %w", err)
	}

	return msg, nil
}

//nolint:ireturn // returning interface is intended for polymorphic messages
func mapToolResult(row *db.GetThreadWithMessagesRow, mergeTag uint64) (messages.Message, error) {
	if row.ResultContent == nil || row.ResultToolCallID == nil || row.ResultToolName == nil {
		return nil, errors.ErrToolResultContentMissing // combine checks to shorten
	}

	isError := deref(row.ResultIsError)
	content := json.RawMessage(row.ResultContent)

	if isError {
		msg, err := messages.NewMessageToolError(
			content, *row.ResultToolName, *row.ResultToolCallID,
			messages.WithMessageToolErrorMergeTag(mergeTag))
		if err != nil {
			return nil, fmt.Errorf("new tool error: %w", err)
		}

		return msg, nil
	}

	msg, err := messages.NewMessageToolResponse(
		content, *row.ResultToolName, *row.ResultToolCallID,
		messages.WithMessageToolResponseMergeTag(mergeTag),
	)
	if err != nil {
		return nil, fmt.Errorf("new tool response: %w", err)
	}

	return msg, nil
}
