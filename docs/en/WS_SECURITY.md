# WebSocket security and trust model

ascentia-core is a **multi-tenant agent runtime**: it does not mandate a specific frontend or admin stack. Callers implement the JSON protocol and your chosen authentication.

## Trust boundary

- **`WS_AUTH_MODE=none` (default)** — The handshake is **not** authenticated; any client that can reach the service may claim arbitrary `user_id` / `agent_id`. Use only on private networks, behind an authenticated gateway, or for local development.
- **Production** — Prefer **`bearer`** or **`jwt`**, or terminate TLS and authenticate at a reverse proxy before Core.

## Auth modes

| Mode | Behavior |
|------|----------|
| `none` | No checks. |
| `bearer` | Requires `Authorization: Bearer <WS_BEARER_TOKEN>` matching configuration. |
| `jwt` | Validates an HS256 JWT; **`user_id` is taken from the numeric `ID` field** at the top level of claims; query `user_id` is ignored. If your IdP uses `sub`, string IDs, etc., map at the gateway or extend parsing in Core. |

**Signing secret (priority order)**

1. `WS_JWT_HS256_SECRET` (recommended)
2. `GVA_JWT_SIGNING_KEY` (**deprecated alias**; still read; logs a migration hint on startup)

Browsers often cannot set custom WebSocket headers. Optional (explicitly enabled; tokens in URLs may appear in logs):

- `WS_ALLOW_QUERY_BEARER=1` — also accept `?bearer_token=`
- `WS_ALLOW_QUERY_JWT=1` — also accept `?access_token=` (use with **HTTPS**)

## Origin

- `WS_ALLOWED_ORIGINS` — comma-separated full Origins, or `*`
- `WS_STRICT_ORIGIN=1` — stricter handling when an `Origin` header is present and no allowlist is configured (clients without `Origin` usually still connect)

## Other controls

- **Rate limit** — `WS_MAX_MESSAGES_PER_MINUTE` per client IP (first hop in `X-Forwarded-For` if present).
- **Tools** — `DISABLE_SPAWN_PET=1` removes the SpawnPet tool from the default bundle.
- **L2 config** — Redis merge allowlist may overwrite secrets such as `WS_JWT_HS256_SECRET` / `WS_BEARER_TOKEN`; lock down Redis and audit changes.

**Other languages:** [简体中文](../zh-CN/WS_SECURITY.md)
