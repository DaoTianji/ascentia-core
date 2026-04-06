package pgintegration

import (
	"context"

	"ascentia-core/internal/configenv"

	"github.com/jackc/pgx/v5/pgxpool"
)

// MergeCoreSettingsEnv 从表 sys_core_settings 读取未删除行，将非空 config_value 写入 os.Setenv（仅白名单键）。
// 表不存在或查询失败时返回 0, nil。
//
// 联调用途：仅当运行实例所连的 PostgreSQL 上已存在该表（例如控制面与引擎暂时共库）时使用。
// 生产上控制面与引擎分库时，推荐经 Redis/NATS（CORE_CONFIG_BUS）或你们自己的配置 API 下发，避免 Core 直连业务控制面库。
func MergeCoreSettingsEnv(ctx context.Context, pool *pgxpool.Pool) (applied int, err error) {
	if pool == nil {
		return 0, nil
	}
	rows, err := pool.Query(ctx, `
SELECT config_key, config_value
FROM sys_core_settings
WHERE deleted_at IS NULL
  AND config_key IS NOT NULL
  AND TRIM(config_key) <> ''
  AND config_value IS NOT NULL
  AND TRIM(config_value) <> ''`)
	if err != nil {
		return 0, nil
	}
	defer rows.Close()
	m := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			continue
		}
		m[k] = v
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	return configenv.ApplyMap(m), nil
}
