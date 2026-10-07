package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
)

// 凭证敏感字段加密:AES-256-GCM,主密钥来自 env CRED_MASTER_KEY。
// 密文格式:base64(nonce[12] || ciphertext || tag)。
// 主密钥任意长度,内部取 SHA-256 派生 32 字节。

var key []byte

// Setup 注入主密钥(启动时调用一次)。空则视为未启用加密。
func Setup(master string) {
	if master == "" {
		key = nil
		return
	}
	h := sha256.Sum256([]byte(master))
	key = h[:]
}

// Enabled 是否已配置主密钥。
func Enabled() bool { return len(key) == 32 }

// Encrypt 加密明文,返回 base64 密文。未配主密钥时返回错误(避免明文误存)。
func Encrypt(plain string) (string, error) {
	if !Enabled() {
		return "", errors.New("未配置 CRED_MASTER_KEY,无法加密凭证")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(ct), nil
}

// Decrypt 解密 base64 密文。
func Decrypt(b64 string) (string, error) {
	if b64 == "" {
		return "", nil
	}
	if !Enabled() {
		return "", errors.New("未配置 CRED_MASTER_KEY,无法解密凭证")
	}
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(data) < gcm.NonceSize() {
		return "", errors.New("密文长度非法")
	}
	nonce, ct := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
