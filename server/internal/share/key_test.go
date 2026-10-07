package share

import "testing"

func TestDayKey(t *testing.T) {
	k := dayKey(1, 99, "20260625")
	want := "share:1:99:20260625"
	if k != want {
		t.Fatalf("dayKey=%s want=%s", k, want)
	}
}
