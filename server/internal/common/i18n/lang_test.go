package i18n

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestParse(t *testing.T) {
	cases := []struct {
		name   string
		header string
		want   Lang
	}{
		{"空头回退中文", "", ZhCN},
		{"后台发的中文", "zh-CN", ZhCN},
		{"后台发的英文", "en", En},
		{"App 发的英文标签", "en-US", En},
		{"带 q 权重取首个", "en-GB,en;q=0.9,zh;q=0.8", En},
		{"首个是中文就按中文", "zh-CN,en;q=0.9", ZhCN},
		{"大小写不敏感", "EN-us", En},
		{"两侧空白", "  en  ", En},
		{"不认识的语言回退中文", "fr-FR", ZhCN},
		{"印地语暂未支持,回退中文", "hi-IN", ZhCN},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Parse(tc.header); got != tc.want {
				t.Errorf("Parse(%q) = %q, 期望 %q", tc.header, got, tc.want)
			}
		})
	}
}

func TestFromContextDefaultsToChinese(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	// 没挂中间件 —— C 端路由走的就是这条路径,必须恒定中文。
	if got := FromContext(c); got != ZhCN {
		t.Errorf("无中间件时 FromContext = %q, 期望 %q", got, ZhCN)
	}
}

func TestMiddlewareWritesLang(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	var seen Lang
	r.Use(Middleware())
	r.GET("/t", func(c *gin.Context) { seen = FromContext(c); c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/t", nil)
	req.Header.Set("Accept-Language", "en")
	r.ServeHTTP(httptest.NewRecorder(), req)

	if seen != En {
		t.Errorf("中间件写入 = %q, 期望 %q", seen, En)
	}
}
