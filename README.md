# Cynosure

> **"Agents little home"** — the platform for creating personal AI agents with zero friction.

## What is Cynosure?

**Cynosure** is a platform for creating personal AI agents with zero friction. Users simply describe their task in Telegram (or any other messenger), and a meta-agent automatically assembles and configures an agent with the necessary tools.

### Problem

Ordinary people want to use AI to automate their tasks, but:

- 🚫 **LangChain** requires programming
- 🚫 **n8n** has complex UI
- 🚫 **Gemini Gems** limited by Gemini UI and tools by Google Workspace
- 🚫 **ChatGPT/Claude/Gemini web** do not support MCP or any other external tools

**Result:** 99% of people cannot create AI agents for their tasks.

### Solution

**Zero friction:**

1. The user writes in Telegram: "I want an agent to manage tasks"
2. The meta-agent asks clarifying questions
3. The agent is ready and working ✅

### Features

- 🤖 **Automatic agent creation** — the meta-agent configures everything for you
- 🔧 **MCP tools** — support for any MCP servers (Todoist, Gmail, Notion, and thousands more)
- 🧠 **RAG filtering** — smart selection from 1M+ tools
- 👥 **Agents for others** — create an agent for your grandma or client
- 💬 **Agent-to-Agent** — agents can communicate with each other
- 🔐 **OAuth support** — secure authorization in services

---

## 📚 Documentation

**Complete project documentation:** [docs/](docs/)

- **[Product Vision](docs/VISION.md)** - Product vision, architecture, use cases
- **[Architecture Diagrams](docs/architecture/)** - PlantUML diagrams

## Deployment

### Required dependencies

1. **PostgreSQL** database.

   You can use local or remote instance. Make sure to create a dedicated database and user for the application.

2. **Zep chat storage** (instance or managed).

   The application uses Zep for storing chat history. You can deploy your own instance or use a managed service.
3. **Gemini API**, currently it works only with Gemini models.

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
