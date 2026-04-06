# 参与贡献

感谢你对 ascentia-core 的关注。

## 贡献方式

1. **先提 Issue** — 较大改动建议先开 Issue，与维护者对齐方向。
2. **小而专注的 PR** — 便于审查与合并。
3. **测试** — 修改行为时请补充或更新测试；本地执行 `go test ./...`。
4. **风格** — 提交前运行 `go fmt ./...` 与 `go vet ./...`。

## 开发环境

```bash
cp .env.example .env
# 编辑 .env：至少配置大模型网关与鉴权

go run ./cmd/ascentia-core/
```

可选：配置 Redis（`REDIS_URL` 或 `REDIS_ADDR`）与 PostgreSQL（`DATABASE_URL`）以启用会话持久化与长期记忆。

## 代码布局（简）

- `pkg/agent_core` — 纯引擎逻辑（不要在此引入 HTTP/DB 实现）。
- `internal/gateway`、`internal/runtime` — WebSocket 与编排。
- `internal/integration` — PostgreSQL、Redis、NATS 等适配。

## 单仓（monorepo）说明

若本模块不是 Git 仓库根目录，请在 CI 中把 `working-directory` 指到模块路径，或将 **ascentia-core** 单独拆成独立仓库（推荐）。

## 语言与文档

- 英文入口：[README.md](README.md)、[docs/en/](docs/en/README.md)
- 中文入口：[README.zh-CN.md](README.zh-CN.md)、[docs/zh-CN/](docs/zh-CN/README.md)

## 许可证

参与贡献即表示你同意将贡献内容以与本项目相同的条款授权（见 [LICENSE](LICENSE)）。
