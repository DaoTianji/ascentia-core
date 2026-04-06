# WebSocket 安全与信任模型

ascentia-core 定位为**多租户智能体运行时（可嵌入的中台能力）**：与具体前端栈或某一后台框架无强制绑定。调用方只需实现 JSON 协议与鉴权约定。

## 信任边界

- **`WS_AUTH_MODE=none`**（默认）：不校验握手身份；任何能连上服务的客户端可声明任意 `user_id` / `agent_id`。仅适用于内网、已前置网关鉴权、或纯本地开发。
- **生产对外**建议启用 **`bearer`** 或 **`jwt`**，或由反向代理注入可信身份后再访问 Core。

## 鉴权模式

| 模式 | 行为 |
|------|------|
| `none` | 不校验。 |
| `bearer` | 要求 `Authorization: Bearer <WS_BEARER_TOKEN>` 与配置一致。 |
| `jwt` | 校验 HS256 JWT；**`user_id` 以 token 内数值型 `ID`（JSON 字段 `ID`）为准**，忽略查询串中的 `user_id`。若你的 IdP 使用 `sub`、字符串 ID 等，需在网关做映射或扩展 Core 的解析逻辑。 |

**密钥环境变量（按优先级）**

1. `WS_JWT_HS256_SECRET`（推荐）
2. `GVA_JWT_SIGNING_KEY`（**已弃用别名**，仍可读，启动时会打一行迁移提示）

浏览器 WebSocket 往往无法设置自定义 Header，可选用（须显式打开、且 URL 可能进日志）：

- `WS_ALLOW_QUERY_BEARER=1`：接受 `?bearer_token=`
- `WS_ALLOW_QUERY_JWT=1`：接受 `?access_token=`（务必配合 HTTPS）

## Origin

- `WS_ALLOWED_ORIGINS`：逗号分隔的完整 Origin，或 `*`
- `WS_STRICT_ORIGIN=1`：未配置允许列表时，对携带 `Origin` 的请求更严格（无 `Origin` 的非浏览器客户端通常仍可通过）

## 其它

- **限流**：`WS_MAX_MESSAGES_PER_MINUTE` 按源 IP（`X-Forwarded-For` 首段）限制每分钟 `user_message` 条数。
- **工具**：`DISABLE_SPAWN_PET=1` 可从内置工具集中移除 SpawnPet。
- **L2 配置**：Redis 白名单可覆盖含 `WS_JWT_HS256_SECRET` / `WS_BEARER_TOKEN` 等敏感项，须限制 Redis 访问与审计。

**其它语言：** [English version](../en/WS_SECURITY.md)
