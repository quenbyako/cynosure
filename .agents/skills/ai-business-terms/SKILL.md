---
name: ai-business-terms
description: This skill defines SOTA terminology for AI-related terms. Use this skill when work with uncertain terminology, like, MCP, headless, harness, agent, AI, LLM, and others.
---

# Terms

## Critical rule

It is STRICTLY FORBIDDEN for you to confuse, substitute, or use generally accepted meanings for the terms listed below. When designing systems, writing Go code, or discussing architecture, rely solely on these definitions.

## Instructions of using (and mean) terms

- When generating a response, check that your use of terms does not contradict this dictionary.
- If an ambiguity arises (for example, the word "model" can be interpreted in two ways in the current context, like AI model and data model), you are REQUIRED to ask a clarifying question before completing the task.

## Terms definitions

### Agent

Agent is a combination of selected LLM model, assigned system prompt, model properties, and assigned tools + skills + memory. Agent is not tied to specific session, instead, it just a set of parameters, which can be used multiple times in different sessions.

### Session

Session is a temporary context of communication between user, autonomous system, or any other input entity, and agent. It contains conversation history, session properties, and session-specific tools + skills + memory. Session is created when user sends a message to agent and closed when user closes the session. While session exists, it can be shared between different agents. Each interaction with session MUST use agent, to continue the work. E.g. in session of finance advisor, session starts with asking recommendation of buying a new car, and "auto expert" agent starts the conversation. Then, when decision is made, user can use "legal expert" agent to help him with paperwork. Then, user can go to "finance planner" agent to plan financing of buying a car. And so on in a single session, related to specific context.

### Harness/Runtime

Harness is a special software, that allows to other systems (or users) work with different LLMs, AI, agents, mcp providers, in a unified way. E.g. Popular harness/runtime systems are Cynosure, OpenClaw, Hermes, etc. While apps like ChatGPT, Claude, Gemini, Perplexity are just specific services for chatting with ai (not harness/runtime).

### Router

Router is a special software, that allows to other systems (or users) work with different LLMs, AI, agents, mcp providers, in a unified way

### Model

Model (in context of AI) is a neural network, that is trained on a large dataset of text, images, or other data. It is used to generate text, images, or other data. Different models has different properties, but by default, MODEL IS ALWAYS STATELESS. Common callers for model are: Agents, Embedding retrievers, Search tools, etc.

Calling model is EXTREMELY EXPENSIVE operation, so you have to utilize most of the each model features, and minimize unnecessary calling (e.g. double model call to select best answer)
