package admin

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	maxTagRunes  = 16  // 单个标签最大字符数
	maxTagsRunes = 255 // 整串最大字符数(对齐 varchar(255))
)

// sanitizeTags 清洗逗号分隔标签串:trim、去空、去重(保序)。
// 单个标签超长或总长超限返回错误;空串表示清空标签,合法。
func sanitizeTags(raw string) (string, error) {
	parts := strings.Split(raw, ",")
	seen := map[string]bool{}
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		tag := strings.TrimSpace(p)
		if tag == "" || seen[tag] {
			continue
		}
		if utf8.RuneCountInString(tag) > maxTagRunes {
			return "", fmt.Errorf("标签「%s」超过 %d 字", tag, maxTagRunes)
		}
		seen[tag] = true
		out = append(out, tag)
	}
	joined := strings.Join(out, ",")
	if utf8.RuneCountInString(joined) > maxTagsRunes {
		return "", fmt.Errorf("标签总长超过 %d 字", maxTagsRunes)
	}
	return joined, nil
}
