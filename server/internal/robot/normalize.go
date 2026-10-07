package robot

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

var reNonWord = regexp.MustCompile(`[^\p{Han}a-z0-9\s]`)

// normalizeText 转小写、去标点、合并空白，用于缓存 key 计算。不修改发给 LLM 的原文。
func normalizeText(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	text = reNonWord.ReplaceAllString(text, "")
	return strings.Join(strings.Fields(text), " ")
}

// cacheKey 返回 16 位 hex（SHA256 前 8 字节），作为 robot_reply_cache.question_hash。
func cacheKey(text, personaRole string) string {
	normalized := normalizeText(text)
	sum := sha256.Sum256([]byte(normalized + "|" + personaRole))
	return hex.EncodeToString(sum[:8])
}
