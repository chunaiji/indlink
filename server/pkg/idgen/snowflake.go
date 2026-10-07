package idgen

import (
	"sync"
	"time"
)

// 轻量雪花 ID 生成器(单机足够 V1 使用)。
// 结构:41 位毫秒时间戳 + 10 位机器位 + 12 位序列。
const (
	epoch        int64 = 1704067200000 // 2024-01-01 00:00:00 UTC
	machineBits        = 10
	sequenceBits       = 12
	maxSequence  int64 = -1 ^ (-1 << sequenceBits)
)

type Node struct {
	mu        sync.Mutex
	machineID int64
	lastTs    int64
	sequence  int64
}

var defaultNode = &Node{machineID: 1}

// Next 返回下一个全局唯一 ID。
func Next() int64 {
	return defaultNode.generate()
}

func (n *Node) generate() int64 {
	n.mu.Lock()
	defer n.mu.Unlock()

	now := time.Now().UnixMilli()
	if now == n.lastTs {
		n.sequence = (n.sequence + 1) & maxSequence
		if n.sequence == 0 {
			// 同一毫秒序列用尽,等待下一毫秒
			for now <= n.lastTs {
				now = time.Now().UnixMilli()
			}
		}
	} else {
		n.sequence = 0
	}
	n.lastTs = now

	return ((now - epoch) << (machineBits + sequenceBits)) |
		(n.machineID << sequenceBits) |
		n.sequence
}
