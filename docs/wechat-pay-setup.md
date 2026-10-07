# 微信支付配置指南

> 适用：漂流瓶后端（Go APIv3 · 多租户模式）
> 对应代码：`server/internal/pay/driver_wx.go`

---

## 一、准备文件

将以下文件放入 `server/cert/`（已加入 `.gitignore`，不会提交到 git）：

| 文件 | 来源 | 用途 |
|---|---|---|
| `apiclient_key.pem` | 商户平台 → API安全 → 申请API证书 | 请求签名（商户私钥） |
| `apiclient_cert.pem` | 商户平台 → API安全 → 申请API证书 | 提取商户证书序列号用 |
| `wx_platform_pub.pem` | 商户平台 → API安全 → 微信支付公钥 → 下载 | 回调验签（平台公钥） |

---

## 二、提取商户证书序列号

```bash
openssl x509 -in server/cert/apiclient_cert.pem -noout -serial
# 输出示例: serial=5E62BED8F45E4EA64A2E768E2F7E0EC05323E6C2
# 取 serial= 后面的大写十六进制串，填入「Pay Serial No」字段
```

---

## 三、各字段说明与获取位置

| 字段 | 说明 | 获取路径 |
|---|---|---|
| **AppID** | 小程序 AppID | 微信公众平台 → 开发 → 开发管理 → AppID |
| **登录密钥** | 小程序 AppSecret | 微信公众平台 → 开发 → 开发管理 → AppSecret |
| **商户号** | 10位数字 | 商户平台 → 账户中心 → 商户信息 |
| **Pay Serial No** | 商户证书序列号 | 运行第二节 openssl 命令提取 |
| **APIv3 Key** | 32字节密钥（自己设置）| 商户平台 → 账户中心 → API安全 → APIv3密钥 → 设置 |
| **商户私钥 PEM** | `apiclient_key.pem` 文件内容 | 粘贴文件全文（含 BEGIN/END 行） |
| **平台公钥ID** | 格式：`PUB_KEY_ID_…` | 商户平台 → API安全 → 微信支付公钥 → 公钥ID |
| **微信平台公钥 PEM** | `wx_platform_pub.pem` 文件内容 | 粘贴文件全文（含 BEGIN/END 行） |
| **回调地址** | 支付结果回调 URL | 见第四节 |

> **APIv3 Key ≠ 平台公钥 ID**：APIv3 Key 是你在商户平台自己**设置**的 32 字节字符串，用于 AES-256-GCM 解密回调资源；平台公钥 ID（`PUB_KEY_ID_…`）是微信分配的，用于 RSA 验签，两者完全不同。

---

## 四、回调地址

```
https://你的域名/api/pay/callback/wx
```

服务端通过请求头 `Wechatpay-Serial`（值为平台公钥 ID）自动匹配租户，**无需在 URL 中指定租户 ID**。

- 必须是**公网 HTTPS 地址**，微信服务器需能主动访问
- 本地开发可用 [ngrok](https://ngrok.com) 临时暴露：`ngrok http 8980`

---

## 五、多租户模式配置入口

启用多租户后（`MULTI_TENANT_ENABLED=1`），支付凭证从管理台数据库读取，`.env` 中的 `WX_*` 字段不再生效。

**配置路径**：`https://ambertu.com/message-admin/` → 账号 `admin / admin123` → 凭证管理 → 找到对应微信凭证 → 点「修改凭证」

`.env` 只需保留：

```ini
MULTI_TENANT_ENABLED=1
DEFAULT_TENANT_ID=100        # 默认租户 ID，与 app_credentials.tenant_id 对应
CRED_MASTER_KEY=<32字节随机串>  # 凭证字段加密主密钥，生产环境务必替换
```

---

## 六、验证配置是否生效

重新编译并启动服务后，下单接口返回的 `pay_params` 中：

- ✅ 配置正确：返回 `timeStamp`、`nonceStr`、`package`、`paySign`（真实值）
- ❌ 凭证缺失：返回 `"mock": true`（本地开发 fallback）

```bash
# 快速验证（替换 $TOKEN 为登录后的 token）
curl -s -X POST http://localhost:8980/api/pay/order \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"package_id":1}' | grep -o '"mock"\|"timeStamp"'
```

---

## 七、注意事项

- `apiclient_key.pem` 是商户私钥，**不能上传 git、不能泄露**，`server/cert/` 已在 `.gitignore`
- `CRED_MASTER_KEY` 生产环境必须替换为随机强密钥，泄露后所有凭证密文均可被解密
- 修改凭证后服务端会自动热重载缓存，无需重启
- 旧版带租户 ID 的回调地址（`/api/pay/callback/wx/100`）仍可用，两者兼容
