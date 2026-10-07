// server/internal/spark/deliver_test.go
package spark

import (
	"encoding/json"
	"strings"
	"testing"
)

// 文案:后台没配(空串)时用内置兜底;{nickname} 要被替换掉,不能把占位符推给用户看。
func TestRenderCopy(t *testing.T) {
	if got := renderCopy("", "{nickname} 与你碰撞出了火花", "小鱼"); got != "小鱼 与你碰撞出了火花" {
		t.Errorf("空配置应用兜底文案, got %q", got)
	}
	if got := renderCopy("你和 {nickname} 对上眼了", "兜底", "小鱼"); got != "你和 小鱼 对上眼了" {
		t.Errorf("配置了就用配置, got %q", got)
	}
	if got := renderCopy("没有占位符", "兜底", "小鱼"); got != "没有占位符" {
		t.Errorf("不写占位符也合法, got %q", got)
	}
	if strings.Contains(renderCopy("{nickname}", "兜底", ""), "{nickname}") {
		t.Error("昵称为空时也要把占位符去掉")
	}
}

// 真人配对的载荷**不能**带 chat_id:会话要等用户点了 /spark/accept 才建,
// 载荷里给了 chat_id 等于告诉客户端「已经有会话了」,它会直接跳进一个不存在的房间。
func TestBuildPayloadOmitsChatIDForRealMatch(t *testing.T) {
	p := buildPayload(0, Candidate{UserID: 42, Nickname: "小鱼", Avatar: "a.png"}, "标题", "正文", "T", "B")
	if p.ChatID != "" {
		t.Fatalf("真人配对不该有 chat_id, got %q", p.ChatID)
	}
	if p.PeerID != "42" || p.Peer.Nickname != "小鱼" || p.Type != "spark" {
		t.Fatalf("%+v", p)
	}
	// ID 一律字符串下发(JS 端 int64 精度会丢)
	b, _ := json.Marshal(p)
	if strings.Contains(string(b), `"peer_id":42`) {
		t.Fatal("ID 必须以字符串下发")
	}

	p2 := buildPayload(777, Candidate{UserID: 42, Nickname: "小鱼"}, "标题", "正文", "T", "B")
	if p2.ChatID != "777" {
		t.Fatalf("机器人配对要带 chat_id, got %q", p2.ChatID)
	}
}

// 追加到 server/internal/spark/deliver_test.go
// 防白嫖闸门:没被匹配过的 peer 必须拒绝。这条如果失效,任何登录用户
// 都能拿 /spark/accept 和任意人免费开聊,price_chat 就白设了。
func TestAcceptRejectsSelf(t *testing.T) {
	s := New(Deps{})
	if _, err := s.Accept(1, 100, 100); err == nil {
		t.Fatal("和自己开聊应被拒")
	}
}
