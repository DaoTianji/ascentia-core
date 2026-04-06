# Contributing

Thank you for your interest in ascentia-core.

**Language:** **English (this page)** | [简体中文](CONTRIBUTING.zh-CN.md)

**Docs:** [Documentation index](docs/README.md) · [README](README.md) · [简体中文 README](README.zh-CN.md)

## How to contribute

1. **Issues first** — For non-trivial changes, open an issue to describe the problem or feature so maintainers can align on direction.
2. **Small PRs** — Prefer focused pull requests that are easy to review.
3. **Tests** — Add or update tests when you change behavior; run `go test ./...` locally.
4. **Style** — Run `go fmt ./...` and `go vet ./...` before submitting.

## Monorepo note

If `ascentia-core` is not the Git repository root, point CI at this directory (e.g. set `defaults.run.working-directory` in `.github/workflows/ci.yml` to the module path) or keep a separate repo for the module.

## Development setup

```bash
cp .env.example .env
# Edit .env: set ANTHROPIC_BASE_URL and model credentials at minimum

go run ./cmd/ascentia-core/
```

Optional: Redis (`REDIS_URL` or `REDIS_ADDR`) and PostgreSQL (`DATABASE_URL`) unlock session persistence and long-term memory.

## Code layout (short)

- `pkg/agent_core` — Pure agent engine (no HTTP/DB imports); keep it that way.
- `internal/gateway`, `internal/runtime` — WebSocket server and wiring.
- `internal/integration` — PostgreSQL, Redis, NATS adapters.

## License

By contributing, you agree that your contributions will be licensed under the same terms as the project (see [LICENSE](LICENSE)).
