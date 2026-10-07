package bottle

import (
	"testing"
	"time"
)

func TestExpireFrom(t *testing.T) {
	now := time.Now()
	if d := expireFrom("24h"); d.Sub(now) > 25*time.Hour || d.Sub(now) < 23*time.Hour {
		t.Errorf("24h 过期时间异常: %v", d)
	}
	if d := expireFrom("7d"); d.Sub(now) < 6*24*time.Hour {
		t.Errorf("7d 过期时间异常: %v", d)
	}
	if d := expireFrom("forever"); d.Sub(now) < 50*365*24*time.Hour {
		t.Errorf("forever 应为很久以后: %v", d)
	}
	// 未知值回退 7d
	if d := expireFrom(""); d.Sub(now) < 6*24*time.Hour {
		t.Errorf("默认应为 7d: %v", d)
	}
}
