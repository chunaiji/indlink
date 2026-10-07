package upload

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"driftbottle/internal/common/errs"
	"driftbottle/internal/common/middleware"
	"driftbottle/internal/common/response"
	"driftbottle/pkg/idgen"

	"github.com/gin-gonic/gin"
)

// Handler 本地图片上传(联调期临时方案;生产切对象存储,只需替换 save + url 拼装)。
type Handler struct {
	dir           string // 落盘目录
	publicBaseURL string // 对外前缀(空 → 按请求 Host 推导)
	// OnUploaded C 端上传成功回调(main 注入微信图片异步检测;nil 跳过)
	OnUploaded func(tenantID, userID int64, url string)
}

func NewHandler(dir, publicBaseURL string) *Handler {
	_ = os.MkdirAll(dir, 0o755)
	return &Handler{dir: dir, publicBaseURL: strings.TrimRight(publicBaseURL, "/")}
}

const (
	maxSize  = 10 << 20 // 10MB
	staticMt = "/static"
)

var allowExt = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true}

func (h *Handler) Register(api *gin.RouterGroup, auth gin.HandlerFunc) {
	api.POST("/upload", auth, h.upload)
}

// upload 接收 multipart 字段 file,落盘后返回可访问 URL。
func (h *Handler) upload(c *gin.Context) {
	url, err := SaveImage(c, h.dir, h.publicBaseURL)
	if err != nil {
		if be, ok := err.(*errs.BizError); ok {
			response.Fail(c, be.Code, be.Msg)
			return
		}
		response.Fail(c, errs.CodeServerError, "保存失败")
		return
	}
	if h.OnUploaded != nil {
		h.OnUploaded(middleware.TenantID(c), middleware.UserID(c), url) // 图片内容安全异步检测
	}
	response.OK(c, gin.H{"url": url, "media_url": url})
}

// SaveImage 保存 multipart 字段 file 到 dir,返回可访问 URL。C 端与管理后台上传共用。
func SaveImage(c *gin.Context, dir, publicBaseURL string) (string, error) {
	file, err := c.FormFile("file")
	if err != nil {
		return "", errs.New(errs.CodeBadRequest, "请选择文件")
	}
	if file.Size > maxSize {
		return "", errs.New(errs.CodeBadRequest, "图片不能超过 10MB")
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !allowExt[ext] {
		return "", errs.New(errs.CodeBadRequest, "仅支持 jpg/png/gif/webp")
	}

	// 按日期分目录 + 雪花命名,避免重名与单目录文件过多
	day := time.Now().Format("20060102")
	subDir := filepath.Join(dir, day)
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		return "", errs.New(errs.CodeServerError, "存储目录创建失败")
	}
	name := fmt.Sprintf("%d%s", idgen.Next(), ext)
	dst := filepath.Join(subDir, name)
	if err := c.SaveUploadedFile(file, dst); err != nil {
		return "", errs.New(errs.CodeServerError, "保存失败")
	}

	relPath := fmt.Sprintf("%s/%s/%s", staticMt, day, name)
	base := strings.TrimRight(publicBaseURL, "/")
	if base == "" {
		// 按请求 Host 推导(联调常用)
		scheme := "http"
		if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
			scheme = "https"
		}
		return fmt.Sprintf("%s://%s%s", scheme, c.Request.Host, relPath), nil
	}
	return base + relPath, nil
}

// StaticDir 返回静态服务的物理目录(供 main 注册 r.Static)。
func StaticDir(dir string) string { return dir }

// StaticMount 静态访问前缀。
func StaticMount() string { return staticMt }
