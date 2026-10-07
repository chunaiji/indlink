package pay

import (
	"strings"
	"testing"
)

// orderScope 是为了能在没有数据库的情况下守住一条约束:user_id 必须在 WHERE 里。
//
// 这个测试看着很轻,但它守的正是「有人图省事把 user_id 从条件里删掉」——
// 那一改动会把查单接口变成一个能遍历他人订单的接口,而且不会有任何报错。
func TestOrderScopeAlwaysFiltersByUser(t *testing.T) {
	where, args := orderScope(42, "NO123")

	if !strings.Contains(where, "user_id") {
		t.Fatalf("查单条件必须含 user_id,否则能查到别人的订单: %q", where)
	}
	if !strings.Contains(where, "order_no") {
		t.Fatalf("查单条件必须含 order_no: %q", where)
	}
	if len(args) != 2 {
		t.Fatalf("应有两个绑定参数(order_no, user_id),实得 %d 个", len(args))
	}
	if args[0] != "NO123" || args[1] != int64(42) {
		t.Fatalf("绑定参数顺序或内容不符: %#v", args)
	}
}
