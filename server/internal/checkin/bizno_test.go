package checkin

import (
	"strings"
	"testing"
)

func TestBizNo(t *testing.T) {
	got := bizNo(123, "20260625")
	if !strings.HasPrefix(got, "checkin:") {
		t.Fatalf("bizNo 前缀异常: %s", got)
	}
	if got != "checkin:123:20260625" {
		t.Fatalf("bizNo 格式异常: %s", got)
	}
}
