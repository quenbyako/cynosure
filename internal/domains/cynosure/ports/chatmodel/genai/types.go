// Package genai provides semconv schema for genai input messages.
package genai

import (
	"encoding/json"
)

// Role of the entity that created the message.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Modality of the data if it is known.
type Modality string

const (
	ModalityImage Modality = "image"
	ModalityVideo Modality = "video"
	ModalityAudio Modality = "audio"
)

type FinishReason string

const (
	FinishReasonStop          FinishReason = "stop"
	FinishReasonLength        FinishReason = "length"
	FinishReasonContentFilter FinishReason = "content_filter"
	FinishReasonToolCall      FinishReason = "tool_call"
	FinishReasonError         FinishReason = "error"
)

// ChatMessages represents the list of input messages sent to the model.
type ChatMessages []ChatMessage

// ChatMessage represents a single message in the conversation.
type ChatMessage struct {
	Name         *string       `json:"name,omitempty"`
	Role         Role          `json:"role"`
	FinishReason FinishReason  `json:"finish_reason,omitempty"`
	Parts        []MessagePart `json:"parts"`
}

// MessagePart is a strictly typed interface for message parts.
type MessagePart interface{ _MessagePart() }

var (
	_ MessagePart = (*TextPart)(nil)
	_ MessagePart = (*BlobPart)(nil)
	_ MessagePart = (*FilePart)(nil)
	_ MessagePart = (*URIPart)(nil)
	_ MessagePart = (*ReasoningPart)(nil)
	_ MessagePart = (*ToolCallRequestPart)(nil)
	_ MessagePart = (*ToolCallResponsePart)(nil)
	_ MessagePart = (*ServerToolCallPart)(nil)
	_ MessagePart = (*ServerToolCallResponsePart)(nil)
	_ MessagePart = (*GenericPart)(nil)
)

// TextPart represents text content sent to or received from the model.
type TextPart struct {
	Type    string `json:"type"` // const: text
	Content string `json:"content"`
}

func (TextPart) _MessagePart() {}

// BlobPart represents blob binary data sent inline to the model.
type BlobPart struct {
	Type     string   `json:"type"` // const: blob
	MimeType *string  `json:"mime_type,omitempty"`
	Modality Modality `json:"modality"`
	Content  []byte   `json:"content"` // Marshals to base64 string
}

func (BlobPart) _MessagePart() {}

// FilePart represents an external referenced file sent to the model by file id.
type FilePart struct {
	Type     string   `json:"type"` // const: file
	MimeType *string  `json:"mime_type,omitempty"`
	Modality Modality `json:"modality"`
	FileID   string   `json:"file_id"`
}

func (FilePart) _MessagePart() {}

// URIPart represents an external referenced file sent to the model by URI.
type URIPart struct {
	Type     string   `json:"type"` // const: uri
	MimeType *string  `json:"mime_type,omitempty"`
	Modality Modality `json:"modality"`
	URI      string   `json:"uri"`
}

func (URIPart) _MessagePart() {}

// ReasoningPart represents reasoning/thinking content received from the model.
type ReasoningPart struct {
	Type    string `json:"type"` // const: reasoning
	Content string `json:"content"`
}

func (ReasoningPart) _MessagePart() {}

// ToolCallRequestPart represents a tool call requested by the model.
type ToolCallRequestPart struct {
	Type      string          `json:"type"` // const: tool_call
	ID        *string         `json:"id,omitempty"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

func (ToolCallRequestPart) _MessagePart() {}

// ToolCallResponsePart represents a tool call result sent to the model.
type ToolCallResponsePart struct {
	Type     string          `json:"type"` // const: tool_call_response
	ID       *string         `json:"id,omitempty"`
	Response json.RawMessage `json:"response"`
}

func (ToolCallResponsePart) _MessagePart() {}

// ServerToolCall is a strictly typed interface for server-side tool calls.
type ServerToolCall interface {
	isServerToolCall()
}

// ServerToolCallPart represents a server-side tool call invocation.
type ServerToolCallPart struct {
	ServerToolCall ServerToolCall `json:"server_tool_call"`
	ID             *string        `json:"id,omitempty"`
	Type           string         `json:"type"`
	Name           string         `json:"name"`
}

func (ServerToolCallPart) _MessagePart() {}

// ServerToolCallResponse is a strictly typed interface for server-side tool call responses.
type ServerToolCallResponse interface {
	isServerToolCallResponse()
}

// ServerToolCallResponsePart represents a server-side tool call response.
type ServerToolCallResponsePart struct {
	ServerToolCallResponse ServerToolCallResponse `json:"server_tool_call_response"`
	ID                     *string                `json:"id,omitempty"`
	Type                   string                 `json:"type"`
}

func (ServerToolCallResponsePart) _MessagePart() {}

// GenericPart represents an arbitrary message part.
type GenericPart struct {
	Type string `json:"type"`
}

func (GenericPart) _MessagePart() {}

// GenericServerToolCall represents an arbitrary server tool call.
type GenericServerToolCall struct {
	Type string `json:"type"`
}

func (GenericServerToolCall) isServerToolCall() {}

// GenericServerToolCallResponse represents an arbitrary server tool call response.
type GenericServerToolCallResponse struct {
	Type string `json:"type"`
}

func (GenericServerToolCallResponse) isServerToolCallResponse() {}
