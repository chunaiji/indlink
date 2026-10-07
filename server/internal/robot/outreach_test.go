package robot

import "testing"

func TestInWindow(t *testing.T) {
	cases := []struct {
		hour, start, end int
		want             bool
	}{
		{20, 19, 22, true},
		{19, 19, 22, true},  // 含开始
		{22, 19, 22, false}, // 不含结束
		{18, 19, 22, false},
		{10, 19, 22, false},
		{5, 0, 0, false}, // start>=end 视为关闭
	}
	for _, c := range cases {
		if got := inWindow(c.hour, c.start, c.end); got != c.want {
			t.Errorf("inWindow(%d,%d,%d)=%v want %v", c.hour, c.start, c.end, got, c.want)
		}
	}
}

func TestSampleCount(t *testing.T) {
	if n := sampleCount(100, 10); n != 10 {
		t.Errorf("sampleCount(100,10)=%d want 10", n)
	}
	if n := sampleCount(5, 10); n != 0 { // 向下取整
		t.Errorf("sampleCount(5,10)=%d want 0", n)
	}
	if n := sampleCount(0, 50); n != 0 {
		t.Errorf("sampleCount(0,50)=%d want 0", n)
	}
	if n := sampleCount(100, 0); n != 0 {
		t.Errorf("sampleCount(100,0)=%d want 0", n)
	}
}
