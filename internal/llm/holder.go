package llm

import "sync/atomic"

// Holder 持有可原子替换的 *Client，供 WebSocket 主链路与 memorywork 共享同一热更新目标。
type Holder struct {
	p atomic.Pointer[Client]
}

// NewHolder 包装非 nil 客户端；若 c 为 nil 则 Holder.Load() 返回 nil。
func NewHolder(c *Client) *Holder {
	h := &Holder{}
	if c != nil {
		h.p.Store(c)
	}
	return h
}

// Load 返回当前客户端（可能为 nil）。
func (h *Holder) Load() *Client {
	if h == nil {
		return nil
	}
	return h.p.Load()
}

// Store 原子替换客户端；c 为 nil 则忽略。
func (h *Holder) Store(c *Client) {
	if h == nil || c == nil {
		return
	}
	h.p.Store(c)
}
