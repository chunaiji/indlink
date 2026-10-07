package crypto

import "testing"

func TestEncryptDecryptRoundtrip(t *testing.T) {
	Setup("my-master-key-for-test")
	if !Enabled() {
		t.Fatal("应已启用")
	}
	cases := []string{"hello", "微信支付密钥-APIv3-32bytes-xxxxxx", "", "a"}
	for _, plain := range cases {
		enc, err := Encrypt(plain)
		if plain == "" {
			// 空串加密后非空,但解密回空
			if err != nil {
				t.Fatalf("空串加密出错: %v", err)
			}
		}
		if err != nil {
			t.Fatalf("加密失败: %v", err)
		}
		got, err := Decrypt(enc)
		if err != nil {
			t.Fatalf("解密失败: %v", err)
		}
		if got != plain {
			t.Fatalf("往返不一致: got %q want %q", got, plain)
		}
	}
}

func TestDecryptEmptyReturnsEmpty(t *testing.T) {
	Setup("k")
	got, err := Decrypt("")
	if err != nil || got != "" {
		t.Fatalf("空密文应返回空: got %q err %v", got, err)
	}
}

func TestEncryptWithoutKeyFails(t *testing.T) {
	Setup("") // 关闭
	if Enabled() {
		t.Fatal("不应启用")
	}
	if _, err := Encrypt("x"); err == nil {
		t.Fatal("未配主密钥应拒绝加密")
	}
}
