package openrouter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/OpenRouterTeam/go-sdk/models/operations"
	"github.com/shopspring/decimal"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/embedding"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
)

const (
	embeddingModel = "google/gemini-embedding-001"
)

type vector = [embedding.EmbeddingSize]float32

// BuildToolEmbedding implements embedding.Port.
func (o *Adapter) BuildToolEmbedding(
	ctx context.Context,
	msgs []messages.Message,
	opts ...embedding.BuildToolEmbeddingOption,
) (vector, error) {
	params, err := embedding.BuildToolEmbeddingParams(msgs, opts...)
	if err != nil {
		return vector{}, fmt.Errorf("building params: %w", err)
	}

	if uint(len(params.Messages())) > o.maxMsgsPerReq {
		return vector{}, embedding.ErrHistoryTooLong
	}

	content := o.formatMessages(params.Messages())

	return o.embed(ctx, content, params.PreflightCheck())
}

func (o *Adapter) formatMessages(msgs []messages.Message) string {
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

// IndexTool implements embedding.Port.
func (o *Adapter) IndexTool(
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

	return o.embed(ctx, content, params.PreflightCheck())
}

func (o *Adapter) embed(
	ctx context.Context,
	content string,
	preflightCheck embedding.PreflightFunc,
) (vector, error) {
	totalTokens, err := o.tokenCounter.CountEmbeddingTokens(ctx, embeddingModel, content)
	if err != nil {
		return vector{}, fmt.Errorf("counting embedding tokens: %w", err)
	}

	if err = preflightCheck(ctx, embeddingModel, totalTokens); err != nil {
		return vector{}, embedding.ErrPreflightFailed(err)
	}

	if err = o.waitEmbeddingLimit(ctx, totalTokens); err != nil {
		return vector{}, err
	}

	vec, actualTokens, cost, err := o.callEmbeddingAPI(ctx, content)
	if err != nil {
		return vector{}, err
	}

	if actualTokens > 0 && actualTokens != totalTokens {
		o.obs.tokenCountMismatch(ctx, embeddingModel, uint32(totalTokens), uint32(actualTokens))
	}

	// todo: cut this shit, move to port layer
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.Int("prompt_tokens", actualTokens),
		attribute.String("cost_usd", cost.String()),
	)

	return vec, nil
}

func parseEmbeddingResponse(resp *operations.CreateEmbeddingsResponse) (vector, error) {
	if len(resp.CreateEmbeddingsResponseBody.Data) == 0 {
		return vector{}, errors.New("empty data in embedding response")
	}

	values := resp.CreateEmbeddingsResponseBody.Data[0].Embedding.ArrayOfNumber
	if len(values) != ports.EmbeddingSize {
		return vector{}, fmt.Errorf("embedding dimension mismatch: got %d, expected %d", len(values), ports.EmbeddingSize)
	}

	var result vector
	for i, v := range values {
		result[i] = float32(v)
	}

	return result, nil
}

func parseEmbeddingUsage(resp *operations.CreateEmbeddingsResponse) (int, decimal.Decimal) {
	if resp.CreateEmbeddingsResponseBody.Usage == nil {
		return 0, decimal.Zero
	}

	actualTokens := int(resp.CreateEmbeddingsResponseBody.Usage.PromptTokens)
	var cost decimal.Decimal
	if resp.CreateEmbeddingsResponseBody.Usage.Cost != nil {
		cost = decimal.NewFromFloat(*resp.CreateEmbeddingsResponseBody.Usage.Cost)
	}
	return actualTokens, cost
}

func (o *Adapter) callEmbeddingAPI(ctx context.Context, content string) (vector, int, decimal.Decimal, error) {
	dimensions := int64(ports.EmbeddingSize)
	resp, err := o.sdkClient.Embeddings.Generate(ctx, operations.CreateEmbeddingsRequest{
		Model:      embeddingModel,
		Input:      operations.CreateInputUnionStr(content),
		Dimensions: &dimensions,
	})
	if err != nil {
		return vector{}, 0, decimal.Zero, fmt.Errorf("embedding API call failed: %w", err)
	}

	vec, err := parseEmbeddingResponse(resp)
	if err != nil {
		return vector{}, 0, decimal.Zero, err
	}

	tokens, cost := parseEmbeddingUsage(resp)
	return vec, tokens, cost, nil
}

func (o *Adapter) waitEmbeddingLimit(ctx context.Context, numTokens int) error {
	cause := ports.ErrHardQuotaExhausted

	limCtx, cancel := context.WithTimeoutCause(ctx, maxLimiterWait, cause)
	defer cancel()

	if err := o.embeddingLimiter.WaitN(limCtx, numTokens); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(context.Cause(limCtx), context.Canceled) {
			return err
		}

		return ports.ErrHardQuotaExhausted
	}

	return nil
}
