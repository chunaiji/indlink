// Package spark 主动匹配(火花):在线真人每隔随机 N–M 分钟被系统配一个人,
// App 弹窗提示,点进去免开聊费直接聊。
//
// 与 robot/outreach.go 的区别:那个是「挑近期活跃真人,让机器人静默发消息」;
// 这个要求对方**此刻在线**、会配两个真人、要弹窗、且开聊免费。
package spark

import (
	"fmt"
	"time"
)

type Kind int

const (
	KindRobot Kind = iota
	KindReal
)

// defaultInterval 间隔没配 / 配得不可用时的回落值。
// 不能回落到 0:那会让每一轮 tick 都给同一个人弹窗。
const defaultInterval = 30 * time.Minute

// pickKind 掷骰子决定配真人还是机器人。realRatio 是真人概率(%),越界夹到 [0,100]。
// roll 注入是为了测试能断言分支,生产传 rand.Intn。
func pickKind(realRatio int, roll func(n int) int) Kind {
	if realRatio < 0 {
		realRatio = 0
	}
	if realRatio > 100 {
		realRatio = 100
	}
	if realRatio == 0 {
		return KindRobot
	}
	if roll(100) < realRatio {
		return KindReal
	}
	return KindRobot
}

// nextInterval 下次匹配的间隔。填反了就交换;下界非正或两者都没配就回落 defaultInterval。
//
// ⚠️ 这里每一条兜底都是在挡 rand.Intn 收到非正数时的 panic——调度器是个常驻 goroutine,
// 一次 panic 就让整个功能静默消失,而后台配置页允许运营填任何数。
func nextInterval(minMin, maxMin int, roll func(n int) int) time.Duration {
	if minMin > maxMin {
		minMin, maxMin = maxMin, minMin
	}
	if minMin <= 0 || maxMin <= 0 {
		return defaultInterval
	}
	span := maxMin - minMin + 1
	return time.Duration(minMin+roll(span)) * time.Minute
}

// inWindow 当前小时是否在时段内(右端点不含)。start >= end 视为未配置,一律不在窗内。
// 与 robot/outreach.go 的 inWindow 同语义,刻意不跨包复用:那是 robot 的私有函数,
// 导出它只为这一个用途会把两个包绑在一起。
func inWindow(hour, start, end int) bool {
	if start >= end {
		return false
	}
	return hour >= start && hour < end
}

// pairKey 当日配对去重键,与两人先后顺序无关。
func pairKey(tenantID, a, b int64) string {
	if a > b {
		a, b = b, a
	}
	return fmt.Sprintf("spark:pair:%d:%d:%d", tenantID, a, b)
}
