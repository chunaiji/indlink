package ad

import "testing"

func TestDayKey(t *testing.T) {
	if k := dayKey(1, 9, "20260625"); k != "adreward:1:9:20260625" {
		t.Fatalf("dayKey=%s", k)
	}
}
