# ascentia-core

**🚀 A Pragmatic, Lightweight Agent Runtime in Go | The Scaffolding Around Your LLM**

**Language:** **English (this page)** | [简体中文](README.zh-CN.md)

[![CI](https://github.com/DaoTianji/ascentia-core/actions/workflows/ci.yml/badge.svg)](https://github.com/DaoTianji/ascentia-core/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

> GitHub does **not** auto-switch READMEs by locale. Use the links above or visit the **[Documentation Index](docs/README.md)**.

---

## 💡 What is Ascentia-Core?

In real-world applications, calling an LLM API is just the tip of the iceberg. The real challenge lies in the "dirty work": tenant isolation, prompt assembly, handling infinite tool-call loops, managing token budgets, and orchestrating layered memory.

**Ascentia-Core is the runtime engineered to handle that dirty work.** It is designed to sit behind your BFF (Backend-for-Frontend) or API Gateway. You plug in any OpenAI-compatible Chat Completions endpoint, and this repo takes over the **control plane**: managing the conversation lifecycle, tool execution, and memory flow across multiple turns. 

We don't just ship API calls; we ship the **engineering scaffolding** around the model.

---

## 🏗️ Architecture & Data Flow

Ascentia-Core is built with a strict separation of concerns. The pure logic (`pkg/agent_core`) is completely decoupled from HTTP/DB adapters (`internal/`), making it an excellent reference implementation for building your own robust agent systems.

```mermaid
flowchart TD
    classDef client fill:#e1f5fe,stroke:#01579b,stroke-width:2px;
    classDef core fill:#e8f5e9,stroke:#2e7d32,stroke-width:2px;
    classDef infra fill:#eceff1,stroke:#37474f,stroke-width:2px,stroke-dasharray: 5 5;
    classDef llm fill:#f3e5f5,stroke:#4a148c,stroke-width:2px;

    Client["📱 Client (App/Web)"]:::client

    subgraph "⚙️ Ascentia-Core (Control Plane)"
        Gateway["WebSocket & Auth"]:::core
        ReAct["🔄 ReAct Loop Engine"]:::core
        MemorySys["💭 Memory Pipeline"]:::core
        LocalCache["🧠 L1: In-Memory (LRU)"]:::core
    end

    subgraph "🗄️ Infrastructure (Optional)"
        Redis["⚡ L2: Redis Cache"]:::infra
        PG["🗄️ PostgreSQL (LTM)"]:::infra
        NATS["📡 NATS (Config Bus)"]:::infra
    end

    LLM["🧠 LLM API (DeepSeek/Claude/etc.)"]:::llm

    Client <-->|"WS Request & Stream Deltas"| Gateway
    Gateway --> ReAct
    ReAct <-->|"Tool Calling / Inference"| LLM
    ReAct <-->|"Token Stats / State"| MemorySys
    MemorySys -.->|"Async Extraction Write"| PG
    MemorySys <--> LocalCache
    LocalCache -.->|"Cache Miss Read"| Redis
    Redis -.->|"Cache Miss Read"| PG
    NATS -.->|"Broadcast Cache Eviction"| LocalCache
````

-----

## ✨ Core Capabilities

  - 🔄 **ReAct Loop Engine**: Handles streaming assistant text and multi-turn tool calling until the model converges or hits a safety limit (circuit breakers for max turns and consecutive tool failures).
  - 🧠 **Layered Memory System**:
      - **STM (Short-Term Memory)**: Redis or in-process memory for bounded chat history.
      - **LTM (Long-Term Memory)**: Optional PostgreSQL storage with pre-turn Side Query injection.
      - **Async Reflection**: Uses a smaller, cheaper LLM to extract durable knowledge asynchronously post-turn, plus a periodic "Dream" cron job to merge and deduplicate memories.
  - 🛡️ **Context Discipline**: Automatic token-budget compaction and tool-result pruning to ensure long conversations don't cause OOM errors or astronomical API bills.
  - 🛠️ **Native Thinking Stream Hooks**: The `thinking` package cleanly demuxes `<thinking>...</thinking>` blocks from streaming data, making it easy to build UIs for deep-reasoning models (like DeepSeek-R1).
  - 👥 **Built-in Tenancy**: Contexts and memories are strictly isolated via a `user_id` + `agent_id` + `session_id` namespace.

-----

## 🛑 Honest Scope: What this project is NOT

To maintain a clean architecture, we've set strict boundaries. Please be aware of the following before using Ascentia-Core:

1.  **Not a full SaaS "in a box"**: We provide the engine. We do not provide user sign-ups, billing dashboards, or a drag-and-drop workflow UI.
2.  **Not a LangChain / LlamaIndex substitute**: This is not a low-code composition DSL for data scientists. It is a compiled, deployable backend service for software engineers.
3.  **Not an enterprise API Gateway**: While we have basic JWT (HS256) and rate-limiting, we expect you to handle complex Identity Providers (IdP), OIDC, and advanced WAF routing at your edge gateway.

-----

## 🚀 Quick Start

### Prerequisites

  - **Go 1.24+**
  - **LLM**: Any OpenAI-compatible endpoint. *(Note: Our environment variables still use the `ANTHROPIC_*` prefix for historical/legacy reasons. See `.env.example`.)*
  - **Optional (for full features)**: Redis, PostgreSQL, NATS.

### Run Locally

```bash
git clone [https://github.com/DaoTianji/ascentia-core.git](https://github.com/DaoTianji/ascentia-core.git)
cd ascentia-core

# Copy the env template
cp .env.example .env
# Required: Set ANTHROPIC_BASE_URL, ANTHROPIC_API_KEY, and ANTHROPIC_MODEL in .env

# Start the runtime
go run ./cmd/ascentia-core/
```

By default, the WebSocket server listens at `ws://127.0.0.1:8080/ws`.

> **💡 Debugging tip:** \> With `WS_AUTH_MODE=none`, you can connect directly using query parameters like `?user_id=123&agent_id=456`. For production setups, please read our [Security Guide](https://www.google.com/search?q=docs/en/WS_SECURITY.md).

-----

## 📚 Documentation Index

| Resource | Description |
|------|------|
| [📖 Docs Root](https://www.google.com/search?q=docs/README.md) | Language index |
| [💡 Capabilities & Rationale](https://www.google.com/search?q=docs/en/CAPABILITIES.md) | **Must read.** Deep dive into the Agent narrative, memory flows, and reflection. |
| [🏗️ Architecture Overview](https://www.google.com/search?q=docs/en/ARCHITECTURE.md) | Module boundaries and dependency explanations. |
| [🔒 Security & WebSocket Auth](https://www.google.com/search?q=docs/en/WS_SECURITY.md) | JWT setups and gateway patterns. |

-----

## 🛣️ Roadmap

This is a growing project. Here's what we are planning to tackle next:

  - [ ] Implement pluggable JWT / OIDC claims mappings.
  - [ ] Expand entry points (HTTP SSE, gRPC) sharing the core `runtime.Service`.
  - [ ] Dynamic Tool Registry and MCP (Model Context Protocol) support.
  - [ ] Build a companion **Eval Harness** (golden datasets, automated regression scoring).
  - [ ] Expand test coverage and set up robust CI pipelines.

-----

## License & Community

  - **License:** [MIT License](https://www.google.com/search?q=LICENSE)
  - **Code of Conduct:** [Contributor Covenant](https://www.google.com/search?q=CODE_OF_CONDUCT.md)
  - **Security:** [SECURITY.md](SECURITY.md)

Maintained by **DaoTianji**. Contributions, issues, and PRs are welcome\!

```
```
