package user

import (
	"testing"
	"time"
)

func TestAgeOf(t *testing.T) {
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	cases := map[string]int{
		"2000-10-04": 26, // 生日当天算满
		"2000-10-05": 25, // 明天才满
		"2008-01-01": 18,
		"2030-01-01": 0, // 未来日期不出负数
	}
	for b, want := range cases {
		d, _ := time.Parse("2006-01-02", b)
		if got := ageOf(d, now); got != want {
			t.Errorf("ageOf(%s) = %d, want %d", b, got, want)
		}
	}
}
