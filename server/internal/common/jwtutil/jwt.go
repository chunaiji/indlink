package jwtutil

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims 内含 user_id 与登录平台(wx/alipay)。
type Claims struct {
	UserID   int64  `json:"uid"`
	TenantID int64  `json:"tid"`
	Platform string `json:"plat"`
	jwt.RegisteredClaims
}

var (
	secret      []byte
	expireHours int
)

// Setup 注入密钥与过期时间(启动时调用一次)。
func Setup(s string, hours int) {
	secret = []byte(s)
	expireHours = hours
}

func Generate(userID, tenantID int64, platform string) (string, error) {
	claims := Claims{
		UserID:   userID,
		TenantID: tenantID,
		Platform: platform,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expireHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
}

func Parse(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return secret, nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, errors.New("invalid token")
}
