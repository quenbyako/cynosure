package tokencounter

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/pkoukk/tiktoken-go"
	"github.com/quenbyako/ext/container/lru"
	"github.com/quenbyako/ext/fs"
	"google.golang.org/genai"
	"google.golang.org/genai/tokenizer"
)

var (
	ErrDownloadFailed = errors.New("download failed")
	ErrModelNotFound  = errors.New("model not found")
)

type Message struct {
	Role    string
	Content string
}

type TokenCounter struct {
	bpeCache    *lru.Cache[string, *BPE]
	o200kEnc    *tiktoken.Tiktoken
	cl100kEnc   *tiktoken.Tiktoken
	geminiCache *lru.Cache[string, *tokenizer.LocalTokenizer]
	fsys        fs.WFS
}

func initEncodings() (o200k, cl100k *tiktoken.Tiktoken, err error) {
	o200k, err = tiktoken.GetEncoding("o200k_base")
	if err != nil {
		return nil, nil, fmt.Errorf("load o200k: %w", err)
	}

	cl100k, err = tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		return nil, nil, fmt.Errorf("load cl100k: %w", err)
	}

	return o200k, cl100k, nil
}

func NewTokenCounter(fsys fs.WFS, httpClient *http.Client) (*TokenCounter, error) {
	o200k, cl100k, err := initEncodings()
	if err != nil {
		return nil, err
	}

	bpeCache, geminiCache := initCaches(fsys, httpClient)

	tc := &TokenCounter{
		bpeCache:    bpeCache,
		o200kEnc:    o200k,
		cl100kEnc:   cl100k,
		geminiCache: geminiCache,
		fsys:        fsys,
	}

	return tc, nil
}

func initCaches(fsys fs.WFS, client *http.Client) (
	bpe *lru.Cache[string, *BPE],
	google *lru.Cache[string, *tokenizer.LocalTokenizer],
) {
	return lru.New(loadBPE(fsys, client), nil, nil, cacheCapacity, cacheTTL),
		lru.New(loadGemini, nil, nil, cacheCapacity, cacheTTL)
}

func (tc *TokenCounter) CountTokens(
	ctx context.Context,
	modelID string,
	msgs []Message,
) (int, error) {
	if strings.HasPrefix(modelID, "openai/") {
		return tc.countOpenAITokens(modelID, msgs), nil
	}

	if strings.HasPrefix(modelID, "google/") || strings.Contains(modelID, "gemini") {
		return tc.countGeminiTokens(ctx, modelID, msgs)
	}

	return tc.countBPETokens(ctx, modelID, msgs)
}

func (tc *TokenCounter) countOpenAITokens(modelID string, msgs []Message) int {
	encoder := tc.o200kEnc
	if strings.Contains(modelID, "gpt-4") && !strings.Contains(modelID, "gpt-4o") {
		encoder = tc.cl100kEnc
	}

	count := 0
	for _, msg := range msgs {
		count += 3 // chat completion framing overhead
		count += len(encoder.Encode(msg.Content, nil, nil))
	}

	count += 3 // assistant reply overhead

	return count
}

func (tc *TokenCounter) countGeminiTokens(
	ctx context.Context,
	modelID string,
	msgs []Message,
) (int, error) {
	localTok, release, err := tc.geminiCache.Get(ctx, modelID)
	if err != nil {
		return 0, fmt.Errorf("resolve gemini local: %w", err)
	}
	defer release()

	contents, systemInstruction := prepareGeminiContents(msgs)

	config := &genai.CountTokensConfig{
		SystemInstruction: systemInstruction,
		HTTPOptions:       nil,
		Tools:             nil,
		GenerationConfig:  nil,
	}

	res, err := localTok.CountTokens(contents, config)
	if err != nil {
		return 0, fmt.Errorf("gemini local token count failed: %w", err)
	}

	return int(res.TotalTokens), nil
}

func prepareGeminiContents(
	msgs []Message,
) (contents []*genai.Content, systemInstruction *genai.Content) {
	for _, msg := range msgs {
		if msg.Role == "system" {
			systemInstruction = &genai.Content{
				Role:  "system",
				Parts: []*genai.Part{{Text: msg.Content}},
			}

			continue
		}

		role := msg.Role
		if role == "assistant" {
			role = "model"
		}

		contents = append(contents, &genai.Content{
			Role:  role,
			Parts: []*genai.Part{{Text: msg.Content}},
		})
	}

	return contents, systemInstruction
}

func (tc *TokenCounter) countBPETokens(
	ctx context.Context,
	modelID string,
	msgs []Message,
) (int, error) {
	bpe, release, err := tc.bpeCache.Get(ctx, modelID)
	if err != nil {
		return 0, fmt.Errorf("resolve HF BPE: %w", err)
	}
	defer release()

	formatted := bpe.ApplyTemplate(msgs)
	tokens := bpe.Encode(formatted)

	return len(tokens), nil
}

func (tc *TokenCounter) EstimateConservativeFallback(msgs []Message) int {
	chars := 0
	for _, msg := range msgs {
		chars += len(msg.Content)
	}

	return (chars / fallbackDivisor) + fallbackOffset
}

func (tc *TokenCounter) CountEmbeddingTokens(ctx context.Context, model, text string) (int, error) {
	switch {
	case strings.HasPrefix(model, "openai/"):
		return len(tc.cl100kEnc.Encode(text, nil, nil)), nil
	case strings.HasPrefix(model, "google/"):
		localTok, release, err := tc.geminiCache.Get(ctx, model)
		if err != nil {
			return 0, fmt.Errorf("resolve gemini local: %w", err)
		}
		defer release()

		contents := []*genai.Content{{
			Parts: []*genai.Part{genai.NewPartFromText(text)},
		}}

		res, err := localTok.CountTokens(contents, nil)
		if err != nil {
			return 0, fmt.Errorf("gemini local token count failed: %w", err)
		}

		return int(res.TotalTokens), nil
	default:
		bpe, release, err := tc.bpeCache.Get(ctx, model)
		if err != nil {
			return 0, fmt.Errorf("counting for %q: %w", model, err)
		}
		defer release()

		return len(bpe.Encode(text)), nil
	}
}

func loadGemini(
	ctx context.Context,
	modelID string,
) (*tokenizer.LocalTokenizer, error) {
	resolvedModel := strings.TrimPrefix(modelID, "google/")
	// issue with embedding models: for some reason genai does not provide local
	// tokenizers for embedding models.
	if strings.Contains(resolvedModel, "gemini-embedding") {
		resolvedModel = "gemini-1.5-flash"
	}

	tok, err := tokenizer.NewLocalTokenizer(resolvedModel)
	if err != nil {
		return nil, fmt.Errorf("gemini local tokenizer creation: %w", err)
	}

	return tok, nil
}

func loadBPE(
	fsys fs.WFS,
	client *http.Client,
) func(ctx context.Context, modelID string) (*BPE, error) {
	return func(ctx context.Context, modelID string) (*BPE, error) {
		safeName := strings.ReplaceAll(strings.ReplaceAll(modelID, "/", "_"), "-", "_")
		vocabPath := safeName + "/tokenizer.json"
		configPath := safeName + "/tokenizer_config.json"

		if err := ensureHFFiles(ctx, fsys, client, modelID, vocabPath, configPath); err != nil {
			return nil, err
		}

		bpe, err := NewBPE(fsys, configPath, vocabPath, nil)
		if err != nil {
			return nil, fmt.Errorf("create bpe parser: %w", err)
		}

		return bpe, nil
	}
}

func ensureHFFiles(
	ctx context.Context,
	fsys fs.WFS,
	client *http.Client,
	modelID, vocabPath, configPath string,
) error {
	if _, err := fs.Stat(fsys, vocabPath); errors.Is(err, fs.ErrNotExist) {
		url := fmt.Sprintf("https://huggingface.co/%s/resolve/main/tokenizer.json", modelID)
		if err := downloadFile(ctx, client, fsys, url, vocabPath); err != nil {
			return fmt.Errorf("download tokenizer.json: %w", err)
		}
	}

	if _, err := fs.Stat(fsys, configPath); errors.Is(err, fs.ErrNotExist) {
		url := fmt.Sprintf("https://huggingface.co/%s/resolve/main/tokenizer_config.json", modelID)
		if err := downloadFile(ctx, client, fsys, url, configPath); err != nil {
			return fmt.Errorf("download tokenizer_config.json: %w", err)
		}
	}

	return nil
}

func downloadFile(
	ctx context.Context,
	client *http.Client,
	fsys fs.WFS,
	url,
	destPath string,
) error {
	resp, err := doDownloadRequest(ctx, client, url)
	if err != nil {
		return err
	}
	//nolint:errcheck // can't do anything here
	defer resp.Body.Close()

	out, err := fsys.OpenW(destPath)
	if err != nil {
		return fmt.Errorf("create destination file: %w", err)
	}

	if _, err = io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("write file payload: %w", err)
	}

	if errClose := out.Close(); errClose != nil {
		return fmt.Errorf("close file: %w", errClose)
	}

	return nil
}

func doDownloadRequest(
	ctx context.Context,
	client *http.Client,
	url string,
) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create download request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute download request: %w", err)
	}

	if resp.StatusCode == http.StatusOK {
		return resp, nil
	}

	if errClose := resp.Body.Close(); errClose != nil {
		return resp, fmt.Errorf("close download response body: %w", errClose)
	}

	switch resp.StatusCode {
	// for some reason, huggingface throws 401 for models, instead of 404,
	// not following AIP guidelines.
	case http.StatusUnauthorized, http.StatusNotFound:
		return nil, ErrModelNotFound
	default:
		return nil, fmt.Errorf("%w: status %d", ErrDownloadFailed, resp.StatusCode)
	}
}
