package admin

import (
	"errors"
	"log"
	"os"
	"strings"
	"time"

	"driftbottle/internal/common/response"
	"driftbottle/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// 管理后台 token 与 C 端 JWT 完全隔离:独立 claims(scope=admin)+ 独立签发。
type adminClaims struct {
	AdminID  int64  `json:"aid"`
	Username string `json:"uname"`
	Role     string `json:"role"`
	Scope    string `json:"scope"` // 固定 "admin"
	jwt.RegisteredClaims
}

var (
	secret      []byte
	expireHours = 12
)

// Setup 注入签名密钥(复用 cfg.JWTSecret 即可,scope 隔离保证不与 C 端互通)。
func Setup(s string) { secret = []byte(s) }

func genToken(a *model.AdminUser) (string, error) {
	claims := adminClaims{
		AdminID:  a.ID,
		Username: a.Username,
		Role:     a.Role,
		Scope:    "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expireHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
}

func parseToken(tokenStr string) (*adminClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &adminClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return secret, nil
	})
	if err != nil {
		return nil, err
	}
	if c, ok := token.Claims.(*adminClaims); ok && token.Valid && c.Scope == "admin" {
		return c, nil
	}
	return nil, errors.New("invalid admin token")
}

// authMiddleware 校验 Authorization: Bearer <adminToken>。
func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		if h == "" || !strings.HasPrefix(h, "Bearer ") {
			response.Abort(c, 401, 2001, "未登录")
			return
		}
		claims, err := parseToken(strings.TrimPrefix(h, "Bearer "))
		if err != nil {
			response.Abort(c, 401, 2001, "登录已失效")
			return
		}
		c.Set("admin_id", claims.AdminID)
		c.Set("admin_name", claims.Username)
		c.Set("admin_role", claims.Role)
		c.Next()
	}
}

// Init 建表后调用:无管理员则按 env 播种默认账号。
//
//	ADMIN_DEFAULT_USER(默认 admin)/ ADMIN_DEFAULT_PASSWORD(默认 admin@12345)
func Init(db *gorm.DB) error {
	var n int64
	db.Model(&model.AdminUser{}).Count(&n)
	if n > 0 {
		return nil
	}
	user := getenv("ADMIN_DEFAULT_USER", "admin")
	pass := getenv("ADMIN_DEFAULT_PASSWORD", "admin123")
	hash, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	a := model.AdminUser{Username: user, PwdHash: string(hash), Role: "super", Status: "active", CreatedAt: time.Now()}
	if err := db.Create(&a).Error; err != nil {
		return err
	}
	log.Printf("[admin] 已创建默认管理员账号: %s / %s (请尽快在后台修改密码)", user, pass)
	return nil
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
