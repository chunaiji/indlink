package moderation

import "testing"

func TestCheckText_BlocksContactInfo(t *testing.T) {
	s := &Service{sensitive: []string{"赌博"}}
	cases := []struct {
		name    string
		text    string
		blocked bool
	}{
		{"normal", "今天有点失眠,想找人聊聊", false},
		{"sensitive", "有没有赌博的群", true},
		{"phone", "加我手机 13812345678", true},
		{"wechat", "加我微信 abc_123", true},
		{"qq", "我的qq是123456", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := s.CheckText(c.text)
			if c.blocked && err == nil {
				t.Errorf("期望拦截但通过: %q", c.text)
			}
			if !c.blocked && err != nil {
				t.Errorf("期望通过但被拦截: %q", c.text)
			}
		})
	}
}
