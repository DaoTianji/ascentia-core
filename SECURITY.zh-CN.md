# 安全政策

**语言：** [English](SECURITY.md) | **简体中文（本页）**

## 受支持的版本

安全修复会合入默认分支（`main` / `master`）。若项目采用语义化版本，可通过 tag 标记发布。

## 报告漏洞

请**不要**就安全问题公开发布 GitHub Issue。

请通过以下**非公开**渠道之一反馈：

- 优先使用 **GitHub Security Advisories**（仓库 → Security → Advisories → Report a vulnerability），若仓库已开启该功能；或
- 发邮件给维护者 **DaoTianji**：**[goal2277858503@gmail.com](mailto:goal2277858503@gmail.com)**（主题建议包含 `[security] ascentia-core`）。

请尽量包含：

- 问题与影响的简要说明
- 复现步骤（如可提供）
- 涉及组件（例如 WebSocket 网关、JWT 鉴权、LLM 客户端）

我们会在若干工作日内尽量确认收到。

## 加固提示

- 公网环境下默认 `WS_AUTH_MODE=none` **不安全**；生产请使用 `bearer` 或 `jwt`。
- WebSocket 信任边界与配置见：[简体中文](docs/zh-CN/WS_SECURITY.md) · [English](docs/en/WS_SECURITY.md)