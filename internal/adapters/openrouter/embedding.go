package openrouter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
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

	res, err := o.callEmbeddingAPI(ctx, content)
	if err != nil {
		return vector{}, err
	}

	o.reportMismatch(ctx, totalTokens, res.tokens)
	o.recordMetrics(ctx, res.tokens, res.cost)

	return res.vec, nil
}

func (o *Adapter) reportMismatch(ctx context.Context, expected, got int) {
	if got > 0 && got != expected {
		if expected >= 0 && expected <= math.MaxUint32 && got >= 0 && got <= math.MaxUint32 {
			o.obs.tokenCountMismatch(ctx, embeddingModel, uint32(expected), uint32(got))
		}
	}
}

func (o *Adapter) recordMetrics(ctx context.Context, tokens int, cost decimal.Decimal) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.Int("prompt_tokens", tokens),
		attribute.String("cost_usd", cost.String()),
	)
}

func parseEmbeddingResponse(resp *operations.CreateEmbeddingsResponse) (vector, error) {
	if len(resp.CreateEmbeddingsResponseBody.Data) == 0 {
		return vector{}, ErrEmptyEmbeddingData
	}

	values := resp.CreateEmbeddingsResponseBody.Data[0].Embedding.ArrayOfNumber
	if len(values) != ports.EmbeddingSize {
		return vector{}, fmt.Errorf("%w: got %d, expected %d",
			ErrEmbeddingDimensionMismatch,
			len(values),
			ports.EmbeddingSize,
		)
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

type embeddingResult struct {
	cost   decimal.Decimal
	tokens int
	vec    vector
}

func (o *Adapter) callEmbeddingAPI(ctx context.Context, content string) (embeddingResult, error) {
	dimensions := int64(ports.EmbeddingSize)

	resp, err := o.sdkClient.Embeddings.Generate(ctx, operations.CreateEmbeddingsRequest{
		EncodingFormat: nil,
		InputType:      nil,
		Model:          embeddingModel,
		Provider:       nil,
		User:           nil,
		Input:          operations.CreateInputUnionStr(content),
		Dimensions:     &dimensions,
	})
	if err != nil {
		return embeddingResult{}, fmt.Errorf("embedding API call failed: %w", err)
	}

	vec, err := parseEmbeddingResponse(resp)
	if err != nil {
		return embeddingResult{}, err
	}

	tokens, cost := parseEmbeddingUsage(resp)

	return embeddingResult{vec: vec, tokens: tokens, cost: cost}, nil
}

func (o *Adapter) waitEmbeddingLimit(ctx context.Context, numTokens int) error {
	cause := ports.ErrHardQuotaExhausted

	limCtx, cancel := context.WithTimeoutCause(ctx, maxLimiterWait, cause)
	defer cancel()

	if err := o.embeddingLimiter.WaitN(limCtx, numTokens); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(context.Cause(limCtx), context.Canceled) {
			return fmt.Errorf("embedding rate limit exceeded: %w", err)
		}

		return ports.ErrHardQuotaExhausted
	}

	return nil
}
