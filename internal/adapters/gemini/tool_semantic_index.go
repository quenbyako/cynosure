package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"google.golang.org/genai"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
)

const (
	embeddingModel = "gemini-embedding-001"
	embeddingSize  = 1536
)

// BuildToolEmbedding implements ports.ToolSemanticIndex.
func (g *GeminiModel) BuildToolEmbedding(ctx context.Context, msgs []messages.Message) ([embeddingSize]float32, error) {
	// Construct query from messages
	var sb strings.Builder

	for _, msg := range msgs {
		// Extract text content from different message types
		switch m := msg.(type) {
		case messages.MessageUser:
			fmt.Fprintf(&sb, "User: %s\n\n", m.Content())
		case messages.MessageAssistant:
			fmt.Fprintf(&sb, "Model: %s\n\n", m.Content())
		case messages.MessageToolRequest:
			fmt.Fprintf(&sb, "Tool Request: %s\n\n", m.ToolName())
		case messages.MessageToolResponse:
			fmt.Fprintf(&sb, "Tool Response: %s\n\n", string(m.Content()))
		case messages.MessageToolError:
			fmt.Fprintf(&sb, "Tool Error: %s\n\n", string(m.Content()))
		}
	}

	// Handle empty messages: provide a default query
	content := sb.String()
	if content == "" {
		content = "No conversation context"
	}

	return g.embed(ctx, content, "RETRIEVAL_QUERY")
}

// IndexTool implements ports.ToolSemanticIndex.
func (g *GeminiModel) IndexTool(ctx context.Context, tool entities.ToolReadOnly) ([embeddingSize]float32, error) {
	// Serialize tool definition
	schema := tool.InputSchema()
	schemaBytes, _ := json.Marshal(schema)

	content := fmt.Sprintf("Tool Name: %s\nAccount: %s\nDescription: %s\nArguments: %s",
		tool.Name(),
		tool.AccountName(),
		tool.Description(),
		string(schemaBytes),
	)

	return g.embed(ctx, content, "RETRIEVAL_DOCUMENT")
}

func (g *GeminiModel) embed(ctx context.Context, content, taskType string) ([embeddingSize]float32, error) {
	ctx, span := g.trace.Start(ctx, "Gemini.embed")
	defer span.End()

	res, err := g.client.Models.EmbedContent(ctx, embeddingModel, []*genai.Content{
		{Parts: []*genai.Part{genai.NewPartFromText(content)}},
	}, &genai.EmbedContentConfig{
		TaskType:             taskType,
		OutputDimensionality: ptr(int32(embeddingSize)),
		HTTPOptions:          nil,
		Title:                "",
		MIMEType:             "",
		AutoTruncate:         false,
	})
	if err != nil {
		return [embeddingSize]float32{}, fmt.Errorf("embedding generation failed: %w", err)
	}

	if len(res.Embeddings) == 0 {
		return [embeddingSize]float32{}, errors.New("no embeddings returned")
	}

	values := res.Embeddings[0].Values
	if len(values) != embeddingSize {
		return [embeddingSize]float32{}, fmt.Errorf("unexpected embedding dimension: got %d, want %d", len(values), embeddingSize)
	}

	var embedding [embeddingSize]float32
	copy(embedding[:], values)

	return embedding, nil
}
