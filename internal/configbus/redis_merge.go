package configbus

import (
	"context"
	"encoding/json"
	"fmt"

	"ascentia-core/internal/configenv"

	"github.com/redis/go-redis/v9"
)

// MergeFromRedis 读取 Redis 中的 JSON 对象（string→string），按白名单合并到环境变量。
func MergeFromRedis(ctx context.Context, rdb *redis.Client) (applied int, err error) {
	if rdb == nil {
		return 0, nil
	}
	val, err := rdb.Get(ctx, RedisKeyCoreConfig).Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(val), &m); err != nil {
		return 0, fmt.Errorf("configbus: json: %w", err)
	}
	return configenv.ApplyMap(m), nil
}
