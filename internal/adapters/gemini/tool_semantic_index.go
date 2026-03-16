package gemini

import (
	"context"
	"encoding/json"
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
func (g *GeminiModel) BuildToolEmbedding(
	ctx context.Context,
	msgs []messages.Message,
) (embedding, error) {
	var builder strings.Builder

	for _, msg := range msgs {
		switch typedMsg := msg.(type) {
		case messages.MessageUser:
			builder.WriteString("User: " + typedMsg.Content() + "\n\n")
		case messages.MessageAssistant:
			builder.WriteString("Model: " + typedMsg.Content() + "\n\n")
		case messages.MessageToolRequest:
			builder.WriteString("Tool Request: " + typedMsg.ToolName() + "\n\n")
		case messages.MessageToolResponse:
			builder.WriteString("Tool Response: " + string(typedMsg.Content()) + "\n\n")
		case messages.MessageToolError:
			builder.WriteString("Tool Error: " + string(typedMsg.Content()) + "\n\n")
		}
	}

	content := builder.String()
	if content == "" {
		content = "No conversation context"
	}

	return g.embed(ctx, content, "RETRIEVAL_QUERY")
}

// IndexTool implements ports.ToolSemanticIndex.
func (g *GeminiModel) IndexTool(
	ctx context.Context,
	tool entities.ToolReadOnly,
) ([embeddingSize]float32, error) {
	schema := tool.InputSchema()

	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		return [embeddingSize]float32{}, fmt.Errorf("failed to marshal tool schema: %w", err)
	}

	content := fmt.Sprintf("Tool Name: %s\nAccount: %s\nDescription: %s\nArguments: %s",
		tool.Name(),
		tool.AccountName(),
		tool.Description(),
		string(schemaBytes),
	)

	return g.embed(ctx, content, "RETRIEVAL_DOCUMENT")
}

type embedding = [embeddingSize]float32

func (g *GeminiModel) embed(ctx context.Context, content, taskType string) (embedding, error) {
	input := []*genai.Content{{
		Parts: []*genai.Part{
			genai.NewPartFromText(content),
		},
		Role: "",
	}}

	config := &genai.EmbedContentConfig{
		TaskType:             taskType,
		OutputDimensionality: ptr(int32(embeddingSize)),
		HTTPOptions:          nil,
		Title:                "",
		MIMEType:             "",
		AutoTruncate:         false,
	}

	res, err := g.client.Models.EmbedContent(ctx, embeddingModel, input, config)
	if err != nil {
		return embedding{}, fmt.Errorf("embedding generation failed: %w", err)
	}

	return getEmbeddingResponse(res)
}

func getEmbeddingResponse(res *genai.EmbedContentResponse) (embedding, error) {
	if len(res.Embeddings) == 0 {
		return embedding{}, ErrNoEmbeddings
	}

	values := res.Embeddings[0].Values
	if len(values) != embeddingSize {
		return embedding{}, ErrEmbeddingDimension(len(values), embeddingSize)
	}

	var result embedding
	copy(result[:], values)

	return result, nil
}
