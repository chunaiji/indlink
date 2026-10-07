package chat

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// 免费会话是火花与机器人触达共用的唯一入口。这条用 AST 盯住它的函数体里
// 不出现任何钱包调用——哪天有人顺手加一行 Debit,两个功能会同时开始扣币,
// 而那种错误在没有 DB 测试基建的仓库里跑不出来。
func TestEnsureFreeChatNeverTouchesWallet(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "service.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	ast.Inspect(f, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "EnsureFreeChat" {
			return true
		}
		found = true
		ast.Inspect(fn.Body, func(m ast.Node) bool {
			sel, ok := m.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == "wlt" {
				t.Errorf("EnsureFreeChat 里出现了钱包调用 wlt.%s —— 免费会话不能扣费", sel.Sel.Name)
			}
			if strings.Contains(sel.Sel.Name, "Debit") {
				t.Errorf("EnsureFreeChat 里出现了 %s —— 免费会话不能扣费", sel.Sel.Name)
			}
			return true
		})
		return false
	})
	if !found {
		t.Fatal("没找到 EnsureFreeChat")
	}
}
