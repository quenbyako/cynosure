// Package tokencounter provides fully offline token counting capabilities
// for multiple model families (OpenAI, Gemini, Hugging Face).
package tokencounter

const (
	cacheCapacity   = 20
	cacheTTL        = 0
	fallbackDivisor = 4
	fallbackOffset  = 15
)
