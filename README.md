# Cynosure

## Deployment

### Secrets configuration

The application uses `github.com/quenbyako/core` framework for configuration.

- **Secrets are NOT plain strings.** They are resolved via DSNs.
- **Structure:**
  - each secret variable is a pointer (e.g., `CYNOSURE_GEMINI_KEY=file:GEMINI_KEY`), which shows, where to find the actual secret, including provider scheme and secret key, that will be resolved by provider itself.
  - `*_SECRETS` variables configure the storage backend (e.g., `CYNOSURE_FILE_SECRETS=file://.secrets`). You can easliy set multiple secret providers, e.g. setup file provider and vault provider at the same time.

To run locally, you MUST provide both the key pointer and the storage config.

For example:

```bash
CYNOSURE_GEMINI_KEY=file:GEMINI_KEY \
CYNOSURE_FILE_SECRETS=file://.secrets \
go run ./cmd/cynosure
```
