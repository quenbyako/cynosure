# Token Counter Contrib Module

## Purpose

This module provides offline, highly accurate, zero-latency token counting capabilities for multiple model families (OpenAI, Hugging Face/BPE, Gemini) without hitting network endpoints.

## Architectural Invariants

- **Dynamic pre-tokenization**: The BPE regex splitter must be parsed dynamically from the `tokenizer.json` configuration file, avoiding hardcoded patterns.
- **Configurable Cache**: The vocabulary and tokenizer cache directory is fully configurable by the client.
