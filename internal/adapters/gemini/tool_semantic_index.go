package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"google.golang.org/genai"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
)

const (
	embeddingModel = "gemini-embedding-001"
)

// BuildToolEmbedding implements ports.ToolSemanticIndex.
func (g *GeminiModel) BuildToolEmbedding(
	ctx context.Context,
	msgs []messages.Message,
) (embedding, error) {
	var builder strings.Builder

	if uint(len(msgs)) > g.maxMsgsPerReq {
		return embedding{}, ports.ErrHistoryTooLong
	}

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
) ([ports.EmbeddingSize]float32, error) {
	schema := tool.InputSchema()

	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		return [ports.EmbeddingSize]float32{}, fmt.Errorf("failed to marshal tool schema: %w", err)
	}

	content := fmt.Sprintf("Tool Name: %s\nAccount: %s\nDescription: %s\nArguments: %s",
		tool.Name(),
		tool.AccountName(),
		tool.Description(),
		string(schemaBytes),
	)

	return g.embed(ctx, content, "RETRIEVAL_DOCUMENT")
}

type embedding = [ports.EmbeddingSize]float32

func (g *GeminiModel) embed(ctx context.Context, content, taskType string) (embedding, error) {
	input := []*genai.Content{{
		Parts: []*genai.Part{
			genai.NewPartFromText(content),
		},
		Role: "",
	}}

	config := &genai.EmbedContentConfig{
		TaskType:             taskType,
		OutputDimensionality: ptr[int32](ports.EmbeddingSize),
		HTTPOptions:          nil,
		Title:                "",
		MIMEType:             "",
		AutoTruncate:         false,
		DocumentOcr:          nil,
		AudioTrackExtraction: nil,
	}

	// custom context to prevent too long wait for rate limiter
	limiterCtx, cancel := context.WithTimeoutCause(ctx, maxLimiterWait, ports.ErrHardQuotaExhausted)
	defer cancel()

	// TODO: use local counting, since we may have a big latency due to http
	// calls.
	tokens, err := g.client.Models.CountTokens(ctx, embeddingModel, input, new(genai.CountTokensConfig))
	if err != nil {
		return embedding{}, fmt.Errorf("token counting failed: %w", err)
	}

	if err = g.embeddingLimiter.WaitN(limiterCtx, int(tokens.TotalTokens)); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
			return embedding{}, err //nolint:wrapcheck // returning context cancellation as is
		}

		return embedding{}, ports.ErrHardQuotaExhausted
	}

	res, err := g.client.Models.EmbedContent(ctx, embeddingModel, input, config)
	if err != nil {
		return embedding{}, fmt.Errorf("embedding generation failed: %w", err)
	}

	return g.getEmbeddingResponse(ctx, res, embeddingModel, uint32(max(0, tokens.TotalTokens)))
}

func (g *GeminiModel) getEmbeddingResponse(ctx context.Context, response *genai.EmbedContentResponse, model string, expectedTokens uint32) (embedding, error) {
	if len(response.Embeddings) == 0 {
		return embedding{}, ErrNoEmbeddings
	}

	res := response.Embeddings[0]
	if res.Statistics != nil {
		tokenCount := uint32(max(0, res.Statistics.TokenCount))
		if tokenCount != expectedTokens {
			g.log.TokenCountMismatch(ctx, model, expectedTokens, tokenCount)
		}
	}

	values := res.Values
	if len(values) != ports.EmbeddingSize {
		return embedding{}, ErrEmbeddingDimension(len(values), ports.EmbeddingSize)
	}

	var result embedding
	copy(result[:], values)

	return result, nil
}
