package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config 全局配置,从环境变量(.env)加载。
// 密钥类配置只走环境变量,不入库、不进 git。
type Config struct {
	Env            string
	Port           string
	JWTSecret      string
	JWTExpireHours int

	MySQLDSN string

	// Redis:优先用连接串 RedisURL(redis://:pwd@host:port/db);为空时回退拆分字段。
	RedisURL      string
	RedisAddr     string
	RedisPassword string
	RedisDB       int

	WX     WXConfig
	Alipay AlipayConfig

	// 本地图片上传(联调期临时方案,后续切对象存储)
	UploadDir     string // 文件落盘目录
	PublicBaseURL string // 对外可访问前缀(空则按请求 Host 推导)

	// SaaS 多租户
	MultiTenant      bool   // 开=走凭证表 + appid 解析;关=沿用 .env 单租户
	DefaultTenantID  int64  // 单租户/默认租户 ID
	CredMasterKey    string // 凭证字段加密主密钥(AES-GCM)
	DefaultWxAppID   string // 多租户:wx appid 缺省时回退的默认小程序 appid

	// App 端默认租户:非 0 时,App 请求的 appid 在凭证表查不到就落到这个租户,
	// 免去为 App 单独维护 app_credentials 行。与 DefaultTenantID 分开——
	// App 按 T1 决策用独立 tenant_id,与小程序隔离。
	AppDefaultTenantID int64
}

type WXConfig struct {
	AppID             string
	Secret            string
	MchID             string
	PayAPIv3Key       string // APIv3 密钥(32字节),用于回调资源 AES-GCM 解密
	PaySerialNo       string // 商户证书序列号,放入请求 Authorization
	PayPrivateKeyPath string // 商户 API 私钥(apiclient_key.pem),用于请求签名 & paySign
	// 回调验签:微信平台公钥(公钥模式 pub_key.pem)或平台证书(cert.pem,自动取公钥)
	PayPlatformKeyPath string
	PayPlatformSerial  string // 平台公钥ID / 平台证书序列号,与回调头 Wechatpay-Serial 比对
	NotifyURL          string
}

type AlipayConfig struct {
	AppID          string
	PrivateKeyPath string
	PublicKeyPath  string
	NotifyURL      string
}

var C *Config

// Load 读取 .env(若存在)并填充全局配置。
func Load() *Config {
	_ = godotenv.Load() // 生产环境直接用系统环境变量时找不到 .env 也不报错

	C = &Config{
		Env:            getEnv("APP_ENV", "dev"),
		Port:           getEnv("APP_PORT", "8980"),
		JWTSecret:      getEnv("JWT_SECRET", "dev-insecure-secret"),
		JWTExpireHours: getEnvInt("JWT_EXPIRE_HOURS", 168),
		MySQLDSN:       getEnv("MYSQL_DSN", "root:root@tcp(127.0.0.1:3306)/driftbottle?charset=utf8mb4&parseTime=true&loc=Local"),
		RedisURL:       getEnv("REDIS_URL", getEnv("REDIS_CONN_STRING", "")),
		RedisAddr:      getEnv("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword:  getEnv("REDIS_PASSWORD", ""),
		RedisDB:        getEnvInt("REDIS_DB", 0),
		WX: WXConfig{
			AppID:             getEnv("WX_APPID", ""),
			Secret:            getEnv("WX_SECRET", ""),
			MchID:             getEnv("WX_MCH_ID", ""),
			PayAPIv3Key:        getEnv("WX_PAY_APIV3_KEY", ""),
			PaySerialNo:        getEnv("WX_PAY_SERIAL_NO", ""),
			PayPrivateKeyPath:  getEnv("WX_PAY_PRIVATE_KEY_PATH", ""),
			PayPlatformKeyPath: getEnv("WX_PAY_PLATFORM_KEY_PATH", ""),
			PayPlatformSerial:  getEnv("WX_PAY_PLATFORM_SERIAL", ""),
			NotifyURL:          getEnv("WX_PAY_NOTIFY_URL", ""),
		},
		Alipay: AlipayConfig{
			AppID:          getEnv("ALIPAY_APPID", ""),
			PrivateKeyPath: getEnv("ALIPAY_PRIVATE_KEY_PATH", ""),
			PublicKeyPath:  getEnv("ALIPAY_PUBLIC_KEY_PATH", ""),
			NotifyURL:      getEnv("ALIPAY_NOTIFY_URL", ""),
		},
		UploadDir:        getEnv("UPLOAD_DIR", "./uploads"),
		PublicBaseURL:    getEnv("PUBLIC_BASE_URL", ""),
		MultiTenant:      getEnv("MULTI_TENANT_ENABLED", "0") == "1",
		DefaultTenantID:  int64(getEnvInt("DEFAULT_TENANT_ID", 1)),
		CredMasterKey:    getEnv("CRED_MASTER_KEY", ""),
		DefaultWxAppID:   getEnv("DEFAULT_WX_APPID", "wxe48b23d248677356"),

		AppDefaultTenantID: getEnvInt64("APP_DEFAULT_TENANT_ID", 0),
	}
	return C
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// getEnvInt64 用于雪花 ID 这类必然超出 32 位的值,不能走 Atoi。
func getEnvInt64(key string, def int64) int64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return def
}
