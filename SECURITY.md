# Security

**Language:** **English (this page)** | [简体中文](SECURITY.zh-CN.md)

## Supported versions

Security fixes are applied to the default branch (`main` / `master`). Tags may be used for releases when the project adopts semver releases.

## Reporting a vulnerability

Please **do not** open a public GitHub issue for security vulnerabilities.

Instead, report privately by one of these methods:

- Prefer **GitHub Security Advisories** (Repository → Security → Advisories → Report a vulnerability), if the feature is enabled for your repo, or
- Email the maintainer **DaoTianji** at **goal2277858503@gmail.com** (please use a clear subject line, e.g. `[security] ascentia-core`).

Include:

- A short description of the issue and its impact
- Steps to reproduce (if possible)
- Affected component (e.g. WebSocket gateway, JWT auth, LLM client)

We will aim to acknowledge receipt within a few business days.

## Hardening reminders

- Default `WS_AUTH_MODE=none` is unsafe on public networks; use `bearer` or `jwt` in production.
- WebSocket trust boundaries: [English](docs/en/WS_SECURITY.md) · [简体中文](docs/zh-CN/WS_SECURITY.md) (or the [hub](WS_SECURITY.md)).
