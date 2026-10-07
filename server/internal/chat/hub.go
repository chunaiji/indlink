package chat

import (
	"encoding/json"
	"sync"

	"github.com/gorilla/websocket"
)

// Hub 维护在线连接(userID -> 连接集合),单实例内存版。
// 多实例部署时可在此层接 Redis Pub/Sub 做跨实例广播。
type Hub struct {
	mu    sync.RWMutex
	conns map[int64]map[*Conn]struct{}
}

type Conn struct {
	userID int64
	ws     *websocket.Conn
	send   chan []byte
	hub    *Hub
}

func NewHub() *Hub {
	return &Hub{conns: make(map[int64]map[*Conn]struct{})}
}

func (h *Hub) add(c *Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.conns[c.userID] == nil {
		h.conns[c.userID] = make(map[*Conn]struct{})
	}
	h.conns[c.userID][c] = struct{}{}
}

func (h *Hub) remove(c *Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if set, ok := h.conns[c.userID]; ok {
		delete(set, c)
		if len(set) == 0 {
			delete(h.conns, c.userID)
		}
	}
}

// IsOnline 用户是否在线。
func (h *Hub) IsOnline(userID int64) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.conns[userID]) > 0
}

// OnlineUserIDs 返回当前所有在线用户 ID。
func (h *Hub) OnlineUserIDs() []int64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	ids := make([]int64, 0, len(h.conns))
	for id := range h.conns {
		ids = append(ids, id)
	}
	return ids
}

// PushTo 向某用户的所有连接推送消息(非阻塞)。
func (h *Hub) PushTo(userID int64, payload interface{}) {
	b, err := json.Marshal(payload)
	if err != nil {
		return
	}
	h.mu.RLock()
	set := h.conns[userID]
	conns := make([]*Conn, 0, len(set))
	for c := range set {
		conns = append(conns, c)
	}
	h.mu.RUnlock()
	for _, c := range conns {
		select {
		case c.send <- b:
		default:
			// 发送缓冲满,丢弃(客户端会通过历史接口补齐)
		}
	}
}
