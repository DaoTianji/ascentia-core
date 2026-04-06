# Capabilities & design rationale

This document explains **what ascentia-core does**, **why it is structured as an LLM/agent harness**, and **where the “memory / thinking / reflection” story shows up in code**.

---

## Harness engineering (why this repo exists)

In production AI systems, the **model weights and a single HTTP call are not the product**. What you actually ship is a **harness**: the scaffolding that **constrains, schedules, and observes** the model—sessions, tools, policies, memory, retries, streaming to clients, and attribution.

**ascentia-core is an agent runtime harness** in that sense:

| Concern | Handled here |
|--------|----------------|
| Who is speaking, as which agent? | `TenantScope`, WS handshake / auth |
| What does the model see this turn? | Prompt assembly, compaction, side memory recall, Todo focus |
| How does it use tools safely? | ReAct loop, `MAX_TURNS`, tool-failure circuit breaker |
| Where is conversation state? | STM (Redis or in-process), bounded length + TTL |
| What persists across sessions? | Optional LTM in PostgreSQL + read/write tools + post-turn extraction |
| How do we avoid runaway cost/latency? | Context budget, tool-result pruning, HTTP timeouts |
| How do clients consume output? | WebSocket JSON framing, streaming assistant deltas |

The **LLM** is plugged in as an **OpenAI-compatible Chat Completions** client; the harness owns the **control plane around the call**.

---

## What the system can do (feature map)

### 1. Conversational agent loop (ReAct)

- Multi-step **tool use** with **streaming** assistant text per model round.
- **Configurable max tool rounds** (`MAX_TURNS`) and **consecutive tool failure cap** (`MAX_TOOL_FAILURE_STREAK`) to stop runaway loops.
- Built-in **cognitive tools** for explicit task lists (`update_todo`, `mark_task_done`) so the model can externalize plan state into a `planner` store.

### 2. Prompt & context management

- **Composable system prompt**: core rules, persona, time context, optional **memory recall block**, optional **Todo focus** block.
- **Context compaction** (`CONTEXT_TOKEN_BUDGET`, tail preservation) to keep prompts within budget.
- **Tool-result pruning** (`TOOL_RESULT_MAX_RUNES`) to limit oversized tool payloads in history.

### 3. Memory (short-term and long-term)

| Layer | Mechanism | When it helps |
|-------|-----------|----------------|
| **STM** | Redis list (or in-memory fallback) per scoped chat key | Multi-turn dialogue without re-sending full history from the client |
| **LTM (optional)** | PostgreSQL `overseer_memories` scoped by session + tenant | Facts and preferences that should survive across conversations |
| **Side query** | Before each turn, retrieve relevant LTM snippets and inject into the prompt | “Remember what we agreed” without manual RAG wiring in the client |
| **Post-turn extraction** | Async LLM pass (`LLMExtractor`) after a successful turn writes durable rows | Automatic distillation of stable facts from chat |
| **Dream consolidation** | Periodic LLM pass (`DreamConsolidator`) merges/dedupes memories per session/tenant | Reduces noise and contradictions in LTM over time |

All memory paths respect **tenant scope** (`user_id` + `agent_id` + session namespace).

### 4. Gateway & operations

- **WebSocket JSON protocol** for browsers, mobile, or backend clients.
- **Auth modes**: none (dev / trusted network), static bearer, HS256 JWT with numeric `ID` claim.
- **Origin policy** and optional **per-IP message rate limit**.
- **Usage attribution** hooks for token accounting (operator role, session, request id).

### 5. Thinking stream (engine capability)

The `pkg/agent_core/thinking` package can **demux** `<thinking>…</thinking>` blocks from streamed text so UIs or logs can treat “chain-of-thought” separately. The default HTTP LLM adapter may not expose this end-to-end to WebSocket clients yet; the **hook point exists** for custom `ModelClient` / `StreamSink` implementations.

### 6. Optional demo integrations

- **NATS** for a sample **SpawnPet** tool (can be disabled with `DISABLE_SPAWN_PET=1`).
- **Redis / NATS** optional **config bus** for hot-reloading whitelisted environment variables (advanced ops).

---

## Strengths (when to choose this harness)

- **Clear separation**: `pkg/agent_core` stays free of HTTP/DB imports; adapters live under `internal/`.
- **Multi-tenant by design**: scope is validated and threaded through memory and tools.
- **Operational guardrails**: turn limits, breaker, compaction, rate limits—sane defaults for a **service**, not a notebook.
- **Memory story is first-class**: STM + LTM + side query + post-turn + consolidation—not only “chat history in a string”.
- **Deployable artifact**: single Go binary, env-driven configuration, CI-friendly.

---

## Non-goals (honest boundaries)

- Not a hosted SaaS, billing UI, or visual workflow designer.
- Not a full **eval harness** (golden datasets, automated scoring) out of the box.
- Not a replacement for LangChain-style composition libraries; focus is **runtime + gateway**.
- JWT parsing is opinionated (`ID` claim); other IdPs should map at the edge or extend parsing.

---

## See also

- [Architecture (layers & diagram)](ARCHITECTURE.md)
- [WebSocket security](WS_SECURITY.md)
- [Repository README](../../README.md)
