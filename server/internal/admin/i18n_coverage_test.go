package admin

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"driftbottle/internal/common/i18n"
)

// i18nExempt 是**有意不翻译**的消息。
//
// 带格式化占位符的消息无法用精确匹配的消息表翻译 —— 运行时字符串里
// 已经填入了具体数值,与表里的格式串对不上。这两条是标签长度校验的
// 边缘提示,数量极少,保持中文。
var i18nExempt = map[string]bool{
	"标签「%s」超过 %d 字": true,
	"标签总长超过 %d 字":   true,
}

// 会把中文消息下发给后台的函数。扫这些调用里的中文字符串字面量。
var i18nMsgFuncs = map[string]bool{
	"response.Fail":    true,
	"response.FailErr": true,
	"response.Abort":   true,
	"errs.New":         true,
	"errors.New":       true,
	"fmt.Errorf":       true,
}

func containsHan(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

// callName 还原 pkg.Func 形式的调用名,非此形式返回空串。
func callName(call *ast.CallExpr) string {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return ""
	}
	return pkg.Name + "." + sel.Sel.Name
}

// adminMsgLiterals 扫 admin 包(排除测试文件)里所有会下发的中文消息字面量。
func adminMsgLiterals(t *testing.T) map[string]token.Position {
	t.Helper()

	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("扫描源文件失败: %v", err)
	}

	fset := token.NewFileSet()
	out := map[string]token.Position{}

	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("解析 %s 失败: %v", name, err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !i18nMsgFuncs[callName(call)] {
				return true
			}
			for _, arg := range call.Args {
				lit, ok := arg.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				s, err := strconv.Unquote(lit.Value)
				if err != nil || !containsHan(s) {
					continue
				}
				if _, seen := out[s]; !seen {
					out[s] = fset.Position(lit.Pos())
				}
			}
			return true
		})
	}
	return out
}

// TestAdminMessagesHaveEnglish 断言 admin 包里每一条会下发的中文消息
// 都在消息表中有英文译文。
//
// 为什么要 AST 扫源码:消息表以中文原文为 key,原文被改动时译文会**静默**
// 失效(回退中文,接口照常 200)。没有这个测试就只能靠人眼发现。
func TestAdminMessagesHaveEnglish(t *testing.T) {
	var missing []string
	for msg, pos := range adminMsgLiterals(t) {
		if i18nExempt[msg] {
			continue
		}
		if !i18n.Has(i18n.En, msg) {
			missing = append(missing, pos.String()+"  "+strconv.Quote(msg))
		}
	}
	if len(missing) > 0 {
		sortStrings(missing)
		t.Errorf("以下 %d 条中文消息缺英文译文,请补进 internal/common/i18n/catalog.go:\n%s",
			len(missing), strings.Join(missing, "\n"))
	}
}

// TestI18nExemptEntriesStillExist 防止豁免名单变成僵尸。
//
// 消息被改写或删除后,豁免项会永远留在名单里,下一个人无从判断它还作不作数。
func TestI18nExemptEntriesStillExist(t *testing.T) {
	found := adminMsgLiterals(t)
	for msg := range i18nExempt {
		if _, ok := found[msg]; !ok {
			t.Errorf("豁免项 %q 在源码中已不存在,请从 i18nExempt 移除", msg)
		}
	}
}

// TestNoConcatenatedMessages 禁止 response.Fail(c, code, "中文前缀:"+err.Error())。
//
// 拼出来的整串带着运行时的错误详情,永远命中不了以原文为 key 的消息表,
// 会**静默**漏翻 —— 而且上面那个覆盖测试也扫不到(拼接不是 BasicLit)。
// 正确写法是 response.FailErr(c, code, "中文前缀", err)。
func TestNoConcatenatedMessages(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("扫描源文件失败: %v", err)
	}

	fset := token.NewFileSet()
	var bad []string

	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("解析 %s 失败: %v", name, err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !i18nMsgFuncs[callName(call)] {
				return true
			}
			for _, arg := range call.Args {
				bin, ok := arg.(*ast.BinaryExpr)
				if !ok {
					continue
				}
				ast.Inspect(bin, func(in ast.Node) bool {
					lit, ok := in.(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						return true
					}
					if s, err := strconv.Unquote(lit.Value); err == nil && containsHan(s) {
						bad = append(bad, fset.Position(lit.Pos()).String()+"  "+strconv.Quote(s))
					}
					return true
				})
			}
			return true
		})
	}

	if len(bad) > 0 {
		sortStrings(bad)
		t.Errorf("以下 %d 处把中文消息与运行时内容拼接,消息表命中不了,改用 response.FailErr:\n%s",
			len(bad), strings.Join(bad, "\n"))
	}
}

// sortStrings 就地排序,让缺失清单的输出稳定(map 遍历顺序随机)。
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
