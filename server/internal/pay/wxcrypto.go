package pay

import (
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
)

// 微信支付 APIv3 加解密工具(纯标准库实现)。
//
// - 请求签名 / paySign:商户私钥 RSA-SHA256(PKCS#1 v1.5)
// - 回调验签:微信平台公钥 RSA-SHA256 验证
// - 回调资源解密:APIv3 密钥 AES-256-GCM

// parsePrivateKeyPEM 解析商户 API 私钥 PEM 内容(支持 PKCS#8 / PKCS#1)。
func parsePrivateKeyPEM(data []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("无效的私钥 PEM")
	}
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if rk, ok := key.(*rsa.PrivateKey); ok {
			return rk, nil
		}
		return nil, errors.New("私钥不是 RSA 类型")
	}
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

// loadPrivateKeyFile 从文件读取并解析(单租户 .env 路径用)。
func loadPrivateKeyFile(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parsePrivateKeyPEM(data)
}

// parsePublicKeyPEM 解析平台公钥 PEM 内容(支持 PKIX 公钥 / X.509 证书)。
func parsePublicKeyPEM(data []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("无效的公钥/证书 PEM")
	}
	// 先按公钥解析
	if pub, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		if rk, ok := pub.(*rsa.PublicKey); ok {
			return rk, nil
		}
	}
	// 再按证书解析
	if cert, err := x509.ParseCertificate(block.Bytes); err == nil {
		if rk, ok := cert.PublicKey.(*rsa.PublicKey); ok {
			return rk, nil
		}
	}
	return nil, errors.New("无法解析出 RSA 公钥")
}

// readFile 读取文件内容(单租户 .env 密钥路径用)。
func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// signSHA256 用商户私钥对消息做 SHA256+RSA 签名,返回 base64。
func signSHA256(priv *rsa.PrivateKey, message string) (string, error) {
	h := sha256.Sum256([]byte(message))
	sig, err := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, h[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

// verifySHA256 用平台公钥校验签名(base64)。
func verifySHA256(pub *rsa.PublicKey, message, signatureB64 string) error {
	sig, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil {
		return err
	}
	h := sha256.Sum256([]byte(message))
	return rsa.VerifyPKCS1v15(pub, crypto.SHA256, h[:], sig)
}

// decryptAEAD 用 APIv3 密钥 AES-256-GCM 解密回调资源。
// ciphertextB64 为 base64(密文+16字节tag),nonce/associatedData 为明文。
func decryptAEAD(apiv3Key, ciphertextB64, nonce, associatedData string) ([]byte, error) {
	if len(apiv3Key) != 32 {
		return nil, fmt.Errorf("APIv3 密钥长度应为 32,实际 %d", len(apiv3Key))
	}
	ciphertext, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher([]byte(apiv3Key))
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, []byte(nonce), ciphertext, []byte(associatedData))
}

// randNonce 生成随机串(用于请求 nonce_str / paySign nonceStr)。
func randNonce() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
