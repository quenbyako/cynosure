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
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/embedding"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
)

const (
	embeddingModel = "gemini-embedding-001"
)

type vector = [embedding.EmbeddingSize]float32

// BuildToolEmbedding implements ports.ToolSemanticIndex.
func (g *GeminiModel) BuildToolEmbedding(
	ctx context.Context,
	msgs []messages.Message,
	opts ...embedding.BuildToolEmbeddingOption,
) (vector, error) {
	params, err := embedding.BuildToolEmbeddingParams(msgs, opts...)
	if err != nil {
		return vector{}, fmt.Errorf("building params: %w", err)
	}

	if uint(len(params.Messages())) > g.maxMsgsPerReq {
		return vector{}, embedding.ErrHistoryTooLong
	}

	content := g.formatMessages(params.Messages())

	return g.embed(ctx, content, "RETRIEVAL_QUERY", params.PreflightCheck())
}

func (g *GeminiModel) formatMessages(msgs []messages.Message) string {
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
		return "No conversation context"
	}

	return content
}

// IndexTool implements ports.ToolSemanticIndex.
func (g *GeminiModel) IndexTool(
	ctx context.Context,
	tool entities.ToolReadOnly,
	opts ...embedding.IndexToolOption,
) (vector, error) {
	params, err := embedding.IndexToolParams(tool, opts...)
	if err != nil {
		return vector{}, fmt.Errorf("indexing params: %w", err)
	}

	schema := params.Tool().InputSchema()

	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		return vector{}, fmt.Errorf("failed to marshal tool schema: %w", err)
	}

	content := fmt.Sprintf("Tool Name: %s\nAccount: %s\nDescription: %s\nArguments: %s",
		params.Tool().Name(),
		params.Tool().AccountName(),
		params.Tool().Description(),
		string(schemaBytes),
	)

	return g.embed(ctx, content, "RETRIEVAL_DOCUMENT", params.PreflightCheck())
}

func (g *GeminiModel) embed(
	ctx context.Context,
	content, taskType string,
	preflightCheck embedding.PreflightFunc,
) (vector, error) {
	input := []*genai.Content{{
		Parts: []*genai.Part{genai.NewPartFromText(content)},
		Role:  "",
	}}

	totalTokens, err := g.countAndCheckEmbeddingTokens(ctx, input, preflightCheck)
	if err != nil {
		return vector{}, err
	}

	res, err := g.client.Models.EmbedContent(ctx, embeddingModel, input, &genai.EmbedContentConfig{
		TaskType:             taskType,
		OutputDimensionality: ptr[int32](ports.EmbeddingSize),
		HTTPOptions:          nil,
		Title:                "",
		MIMEType:             "",
		AutoTruncate:         false,
		DocumentOcr:          nil,
		AudioTrackExtraction: nil,
	})
	if err != nil {
		return vector{}, fmt.Errorf("embedding generation failed: %w", err)
	}

	return g.getEmbeddingResponse(ctx, res, embeddingModel, uint32(max(0, totalTokens)))
}

// TODO: use local counting, since we may have a big latency due to http
// calls.
func (g *GeminiModel) countAndCheckEmbeddingTokens(
	ctx context.Context,
	input []*genai.Content,
	preflightCheck embedding.PreflightFunc,
) (int32, error) {
	tokens, err := g.client.Models.CountTokens(ctx, embeddingModel, input, nil)
	if err != nil {
		return 0, fmt.Errorf("token counting failed: %w", err)
	}

	if err = preflightCheck(ctx, embeddingModel, int(tokens.TotalTokens)); err != nil {
		return 0, fmt.Errorf("preflight check failed: %w", err)
	}

	if err = g.waitEmbeddingLimit(ctx, int(tokens.TotalTokens)); err != nil {
		return 0, err
	}

	return tokens.TotalTokens, nil
}

func (g *GeminiModel) waitEmbeddingLimit(ctx context.Context, numTokens int) error {
	cause := ports.ErrHardQuotaExhausted
	limCtx, cancel := context.WithTimeoutCause(ctx, maxLimiterWait, cause)

	defer cancel()

	if err := g.embeddingLimiter.WaitN(limCtx, numTokens); err != nil {
		isCanceled := errors.Is(err, context.Canceled) ||
			errors.Is(context.Cause(limCtx), context.Canceled)

		if isCanceled {
			return err //nolint:wrapcheck
		}

		return ports.ErrHardQuotaExhausted
	}

	return nil
}

func (g *GeminiModel) getEmbeddingResponse(
	ctx context.Context,
	response *genai.EmbedContentResponse,
	model string,
	expectedTokens uint32,
) (vector, error) {
	if len(response.Embeddings) == 0 {
		return vector{}, ErrNoEmbeddings
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
		return vector{}, ErrEmbeddingDimension(len(values), ports.EmbeddingSize)
	}

	var result vector
	copy(result[:], values)

	return result, nil
}
