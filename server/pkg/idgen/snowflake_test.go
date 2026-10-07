package idgen

import "testing"

func TestNext_Unique(t *testing.T) {
	seen := make(map[int64]struct{}, 10000)
	for i := 0; i < 10000; i++ {
		id := Next()
		if id <= 0 {
			t.Fatalf("ID 应为正数, got %d", id)
		}
		if _, dup := seen[id]; dup {
			t.Fatalf("ID 重复: %d", id)
		}
		seen[id] = struct{}{}
	}
}
