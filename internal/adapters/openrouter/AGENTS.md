# OpenRouter Adapter Rules

## Architectural Invariants

- This package implements `chatmodel.PortFactory` and `embedding.PortFactory` to interface with the OpenRouter REST API.
- All dependencies on other adapters are strictly prohibited.
- Token counting must be done **100% offline** (0ms latency, zero API costs) using `tiktoken` for OpenAI models, BPE parser for Hugging Face models, and SentencePiece for Gemini models.
- **Anthropic is currently unsupported and disabled due to impossibility of offline token prediction.**

## API Contract

- Endpoint: `https://openrouter.ai/api/v1`
- Embeddings defaults to `openai/text-embedding-3-small` (dimension 1536).
- Chat completions use streaming `/chat/completions` with SSE (Server-Sent Events) chunk decoding.
