package configbus

// 与控制面 ascentia_admin/configpush 保持一致，供 L2 / 广播路径使用。
const (
	RedisKeyCoreConfig           = "ascentia:core:config"
	NATSSubjectCoreConfigUpdated = "ascentia.config.core_updated"
)
