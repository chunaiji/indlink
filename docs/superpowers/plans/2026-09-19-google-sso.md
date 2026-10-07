# Google SSO 实施计划（含 Apple 登录与账号绑定）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 App 端可以用 Google / Apple 账号登录，并允许已有账号在设置里绑定这两个身份。

**Architecture:** 后端的验签链路（`user/appauth.go`）已完整实现，本计划只补五个缺口：ID Token 取 `email_verified`、登录时带上 email、新增「邮箱已占用」冲突分支、新增绑定/解绑接口、把 `google_sub`/`apple_sub` 改为可空并加唯一索引。客户端从零接入两个 SDK。所有判断逻辑先抽成纯函数再测——本仓库 20 个 `*_test.go` 没有一个碰数据库，测试只测纯函数。

**Tech Stack:** Go 1.x + Gin + GORM v2 + MySQL · Flutter 3.47.5 / Dart 3.13.4 + Riverpod + go_router · `google_sign_in` ^7.x · `sign_in_with_apple` ^7.x

**Spec:** `docs/superpowers/specs/2026-09-19-google-capabilities-design.md`

## Global Constraints

- **Flutter ≥ 3.47.0**，Dart `^3.13.0`（`app/bottles/pubspec.yaml` 的 `environment`）。本机实测 3.47.5 / Dart 3.13.4。
- **不要升 `flutter_secure_storage`（锁 `^9.2.4`）和 `path_provider_foundation`（`dependency_overrides: 2.4.1`）**。10+ 版本的 native-assets hook 引用了 Flutter 3.47 已移除的 `Architecture.arm64e`，升上去 `flutter test` / `flutter build` 在任何平台都直接挂。
- **ID 一律字符串下发**：`json` tag 加 `,string`，前端传 ID 同样 `String()`。
- **多租户**：所有表带 `tenant_id`，所有查询按租户过滤。
- **新增 sysconfig key 必须同步写 `internal/sysconfig/sysconfig.go` 的 defaults**（空串会导致开关逻辑反转），要进后台还需登记 `internal/admin/meta.go` 白名单。本计划**不新增 sysconfig key**（`app_google_client_id` / `app_apple_bundle_id` 已存在）。
- **提交前验证**：后端 `cd server && go build ./... && go vet ./...`；前端 `cd app/bottles && flutter analyze`。
- **Apple 登录是硬性要求**：iOS 上有 Google 登录却无 Apple 登录 = App Store 4.8 直接拒审。
- **iOS 无法在本机验证**：Windows 上 `flutter build ipa` 子命令都不存在。iOS 部分只写代码，需 macOS + Xcode 才能真跑。
- 回复用中文；代码 / 标识符 / commit message 英文（conventional commits）。

---

## File Structure

**后端**

| 文件 | 职责 | 动作 |
|---|---|---|
| `server/internal/common/errs/errs.go` | 错误码 | 修改：加 2 个码 |
| `server/internal/model/model.go` | User 的 OAuth 标识列 | 修改：`string` → `*string` + 唯一索引 |
| `server/internal/bootstrap/migrate.go` | 建唯一索引前的数据清理 | 修改：加 `nullifyEmptyOAuthSubs` |
| `server/internal/user/appauth.go` | JWKS 验签（已有） | 修改：claims 加 `EmailVerified`，抽 `verifyOAuthIdentity` |
| `server/internal/user/oauthlink.go` | **新增**：登录决策与绑定/解绑的纯逻辑 | 创建 |
| `server/internal/user/oauthlink_test.go` | **新增**：上面那些纯函数的测试 | 创建 |
| `server/internal/user/otp.go` | `appIdentity` / `loginOrCreateApp` | 修改：适配 `*string`、插入冲突分支 |
| `server/internal/user/account.go` | `DeleteAccount` | 修改：置 `NULL` 而非 `''` |
| `server/internal/user/handler.go` | 路由与 handler | 修改：加 3 条路由 |

`oauthlink.go` 单独成文件而不是塞进 `appauth.go`：后者已 200+ 行且职责是「验签」，绑定/解绑与登录决策是另一件事。

**前端**

| 文件 | 职责 | 动作 |
|---|---|---|
| `app/bottles/pubspec.yaml` | 依赖 | 修改 |
| `app/bottles/android/app/src/main/AndroidManifest.xml` | — | 无需改（Credential Manager 不要额外配置） |
| `app/bottles/ios/Runner/Info.plist` | Google URL Scheme | 修改 |
| `app/bottles/lib/data/repositories.dart` | Repository 抽象 | 修改：加 4 个方法签名 |
| `app/bottles/lib/data/remote/remote_repositories.dart` | 真实实现 | 修改 |
| `app/bottles/lib/data/mock/mock_repositories.dart` | Mock 实现 | 修改 |
| `app/bottles/lib/core/platform/oauth.dart` | **新增**：包住两个 SDK，只暴露「拿 idToken」 | 创建 |
| `app/bottles/lib/features/auth/auth_controller.dart` | 登录状态机 | 修改 |
| `app/bottles/lib/features/auth/login_page.dart` | A2 加两个按钮 | 修改 |
| `app/bottles/lib/features/auth/oauth_conflict_page.dart` | **新增**：A2k | 创建 |
| `app/bottles/lib/features/settings/account_security_page.dart` | **新增**：A6 / A6b | 创建 |
| `app/bottles/lib/app/routes.dart` + `router.dart` | 路由常量与注册 | 修改 |

`oauth.dart` 把两个 SDK 关在一个文件里：SDK 的 API 变动频繁（`google_sign_in` 7.x 刚换过一次），隔离后升级只改这一处。

---

## Task 1: 错误码与 `email_verified` claim

**Files:**
- Modify: `server/internal/common/errs/errs.go`
- Modify: `server/internal/user/appauth.go:129-133`
- Test: `server/internal/user/oauthlink_test.go`（创建）

**Interfaces:**
- Consumes: 无
- Produces:
  - `errs.CodeOAuthEmailTaken = 2004`
  - `errs.CodeOAuthAlreadyBound = 2005`
  - `errs.CodeOAuthLastMethod = 2006`
  - `idTokenClaims.EmailVerified bool`

- [x] **Step 1: 写失败的测试**

创建 `server/internal/user/oauthlink_test.go`：

```go
package user

import (
	"encoding/json"
	"testing"
)

// idTokenClaims 必须能吃到 Google 的 email_verified。
// 缺了它，A2k 的冲突判断就只能拿未经验证的邮箱去匹配已有账号——那是账号接管。
func TestIDTokenClaimsEmailVerified(t *testing.T) {
	var c idTokenClaims
	raw := `{"sub":"110","email":"meera@gmail.com","email_verified":true,"name":"Meera"}`
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		t.Fatalf("解析 claims 失败: %v", err)
	}
	if c.Email != "meera@gmail.com" {
		t.Fatalf("email 应为 meera@gmail.com, 实际 %q", c.Email)
	}
	if !c.EmailVerified {
		t.Fatal("email_verified=true 时应解析为 true")
	}
}

// 字段缺失时必须是 false，不能是「未知」。Apple 的 token 里就没有这个字段。
func TestIDTokenClaimsEmailVerifiedAbsent(t *testing.T) {
	var c idTokenClaims
	if err := json.Unmarshal([]byte(`{"sub":"110"}`), &c); err != nil {
		t.Fatalf("解析 claims 失败: %v", err)
	}
	if c.EmailVerified {
		t.Fatal("字段缺失时应为 false")
	}
}
```

- [x] **Step 2: 跑测试确认失败**

Run: `cd server && go test ./internal/user/ -run TestIDTokenClaims -v`
Expected: 编译失败，`c.EmailVerified undefined (type idTokenClaims has no field or method EmailVerified)`

- [x] **Step 3: 加字段**

`server/internal/user/appauth.go`，把 `idTokenClaims` 改成：

```go
type idTokenClaims struct {
	jwt.RegisteredClaims
	Email string `json:"email"`
	// EmailVerified Google 会给；Apple 不给(其 token 无此字段,解析为 false)。
	// 只有它为 true 时,邮箱才可用于「该邮箱已注册」的冲突判断——
	// 拿未经验证的邮箱去匹配已有账号,等于谁都能声称自己拥有那个邮箱。
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
}
```

- [x] **Step 4: 跑测试确认通过**

Run: `cd server && go test ./internal/user/ -run TestIDTokenClaims -v`
Expected: PASS（2 个用例）

- [x] **Step 5: 加错误码**

`server/internal/common/errs/errs.go`，在 `CodeNeedVerify = 2003` 之后加：

```go
	CodeOAuthEmailTaken   = 2004 // 第三方登录:该邮箱已注册,需先登录再绑定(A2k)
	CodeOAuthAlreadyBound = 2005 // 绑定:该第三方账号已被其他用户绑定(A6b)
	CodeOAuthLastMethod   = 2006 // 解绑:解绑后将没有任何可登录方式
```

- [x] **Step 6: 验证并提交**

```bash
cd server && go build ./... && go vet ./... && go test ./internal/user/ -v
git add server/internal/common/errs/errs.go server/internal/user/appauth.go server/internal/user/oauthlink_test.go
git commit -m "feat(user): read email_verified from ID token, add oauth error codes"
```

---

## Task 2: OAuth 标识改为可空 + 唯一索引 + 迁移

**Files:**
- Modify: `server/internal/model/model.go:76-77`
- Modify: `server/internal/bootstrap/migrate.go:12-21`
- Modify: `server/internal/user/account.go`（`DeleteAccount` 里的 `google_sub` / `apple_sub`）
- Modify: `server/internal/user/otp.go`（`loginOrCreateApp` 建号处、`appIdentity.where()`）
- Create: `server/internal/user/oauthlink.go`
- Test: `server/internal/user/oauthlink_test.go`

**Interfaces:**
- Consumes: Task 1 的错误码
- Produces:
  - `func nilIfEmpty(s string) *string`
  - `model.User.GoogleSub *string` / `model.User.AppleSub *string`
  - `bootstrap.nullifyEmptyOAuthSubs(db *gorm.DB) error`

> **为什么必须可空**：现状是 `gorm:"size:64;index"`，数据库层面不阻止两个账号绑同一个 Google 身份，A6b 的拦截若只在应用层，绑定按钮连点两次即可击穿。但不能直接加唯一索引——手机号用户的 `google_sub` 是 `''`，MySQL 唯一索引不允许重复 `''`（`NULL` 可以）。

- [x] **Step 1: 写失败的测试**

追加到 `server/internal/user/oauthlink_test.go`：

```go
// nilIfEmpty 是可空唯一索引列的唯一正确写法：
// 空串在 MySQL 唯一索引里会互相冲突，NULL 不会。
func TestNilIfEmpty(t *testing.T) {
	if nilIfEmpty("") != nil {
		t.Fatal("空串必须转成 nil，否则多行 '' 会撞唯一索引")
	}
	p := nilIfEmpty("abc")
	if p == nil {
		t.Fatal("非空串不应转成 nil")
	}
	if *p != "abc" {
		t.Fatalf("值应为 abc, 实际 %q", *p)
	}
}
```

- [x] **Step 2: 跑测试确认失败**

Run: `cd server && go test ./internal/user/ -run TestNilIfEmpty -v`
Expected: 编译失败，`undefined: nilIfEmpty`

- [x] **Step 3: 创建 `oauthlink.go` 并实现**

创建 `server/internal/user/oauthlink.go`：

```go
package user

// 第三方身份(Google / Apple)的登录决策与绑定/解绑。
//
// 与 appauth.go 的分工：appauth.go 只负责「这个 ID Token 是真的吗」，
// 本文件负责「验过之后该怎么办」——登录、建号、还是拒绝并提示去绑定。

// nilIfEmpty 空串转 nil。
//
// google_sub / apple_sub 带唯一索引，未绑定时必须存 NULL 而不是 ''：
// MySQL 的唯一索引允许多行 NULL，但不允许多行 ''。存 '' 的话，
// 第二个没绑 Google 的用户就插不进去了。
func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
```

- [x] **Step 4: 跑测试确认通过**

Run: `cd server && go test ./internal/user/ -run TestNilIfEmpty -v`
Expected: PASS

- [x] **Step 5: 改模型**

`server/internal/model/model.go`，把第 76-77 行替换为：

```go
	// GoogleSub / AppleSub 第三方身份标识。
	//
	// 必须可空 + 唯一：没有唯一约束时，两个账号可以绑同一个 Google 身份，
	// 之后按 sub 查用户会返回不确定的那一个。未绑定存 NULL 而非 ''——
	// MySQL 唯一索引允许多行 NULL，不允许多行 ''。
	GoogleSub *string `gorm:"size:64;uniqueIndex:uk_tenant_google,priority:2" json:"-"`
	AppleSub  *string `gorm:"size:64;uniqueIndex:uk_tenant_apple,priority:2" json:"-"`
```

同时确认 `TenantID` 字段带上这两个唯一索引的 `priority:1`。若 `TenantID` 当前是 `gorm:"index"`，改为：

```go
	TenantID int64 `gorm:"index;uniqueIndex:uk_tenant_google,priority:1;uniqueIndex:uk_tenant_apple,priority:1" json:"tenant_id,string"`
```

- [x] **Step 6: 写迁移**

`server/internal/bootstrap/migrate.go`，在 `Migrate` 里 `dedupeChats` 之后、`AutoMigrate` 之前插入调用：

```go
	if err := nullifyEmptyOAuthSubs(db); err != nil {
		return err
	}
```

并在 `dedupeChats` 函数之后加：

```go
// nullifyEmptyOAuthSubs 把历史遗留的空串 google_sub / apple_sub 刷成 NULL。
//
// 与 dedupeChats 同理：AutoMigrate 会给这两列加唯一索引
// (uk_tenant_google / uk_tenant_apple)，而 MySQL 唯一索引不允许重复的 ''。
// 手机号注册的用户这两列都是 ''，不先清理，建索引会直接失败。
// 幂等；全新库(users 表尚不存在)直接跳过。
func nullifyEmptyOAuthSubs(db *gorm.DB) error {
	if !db.Migrator().HasTable(&model.User{}) {
		return nil
	}
	for _, col := range []string{"google_sub", "apple_sub"} {
		if !db.Migrator().HasColumn(&model.User{}, col) {
			continue
		}
		res := db.Exec("UPDATE users SET "+col+" = NULL WHERE "+col+" = ''")
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected > 0 {
			log.Printf("[migrate] %s 空串置 NULL: %d 行", col, res.RowsAffected)
		}
	}
	return nil
}
```

- [x] **Step 7: 修 `DeleteAccount`（不改会导致第二个账号注销失败）**

`server/internal/user/account.go` 的 `DeleteAccount`，把 `Updates` map 里的这两行：

```go
			"google_sub": "",
			"apple_sub":  "",
```

改成：

```go
			// 置 NULL 而非 ''：这两列带唯一索引，
			// 写 '' 会导致第二个账号注销时撞唯一键。
			"google_sub": nil,
			"apple_sub":  nil,
```

- [x] **Step 8: 修建号处**

`server/internal/user/otp.go` 的 `loginOrCreateApp`，把建号那段里的：

```go
		GoogleSub:      id.GoogleSub,
		AppleSub:       id.AppleSub,
```

改成：

```go
		GoogleSub:      nilIfEmpty(id.GoogleSub),
		AppleSub:       nilIfEmpty(id.AppleSub),
```

`appIdentity.where()` **不需要改** —— 它比较的是 `google_sub = ?` 加一个 `string` 值，与列是否可空无关。

- [x] **Step 9: 编译并跑全量测试**

Run: `cd server && go build ./... && go vet ./... && go test ./... `
Expected: 全部通过。若有其他文件直接读 `u.GoogleSub` 当 `string` 用，编译会报错——按编译器提示逐个改成解引用（注意判 nil）。

- [x] **Step 10: 提交**

```bash
git add server/internal/model/model.go server/internal/bootstrap/migrate.go \
        server/internal/user/account.go server/internal/user/otp.go \
        server/internal/user/oauthlink.go server/internal/user/oauthlink_test.go
git commit -m "feat(user): make oauth subs nullable and unique, migrate empty strings"
```

> **上线前必做**：在生产库快照上跑一次 `Migrate`，确认 `nullifyEmptyOAuthSubs` 的行数符合预期且唯一索引建成功。这一步不能只在空库上验证。

---

## Task 3: 登录决策纯函数 + 接入 Google / Apple 登录

**Files:**
- Modify: `server/internal/user/oauthlink.go`
- Modify: `server/internal/user/appauth.go`（`LoginWithGoogle` / `LoginWithApple`）
- Modify: `server/internal/user/otp.go`（`loginOrCreateApp` 加冲突分支）
- Test: `server/internal/user/oauthlink_test.go`

**Interfaces:**
- Consumes: Task 1 的 `errs.CodeOAuthEmailTaken`、`idTokenClaims.EmailVerified`；Task 2 的 `nilIfEmpty`
- Produces:
  - `type oauthDecision int` 及常量 `oauthLogin` / `oauthCreate` / `oauthEmailTaken`
  - `func decideOAuthLogin(subHit, emailVerified, emailHit bool) oauthDecision`

- [x] **Step 1: 写失败的测试**

追加到 `server/internal/user/oauthlink_test.go`：

```go
// decideOAuthLogin 是「不自动合并」这条产品决策的唯一落点。
// 五种组合必须全部覆盖——漏掉任何一种，要么用户凭空多出第二个账号，
// 要么攻击者能用同名邮箱的 Google 账号接管已有账号。
func TestDecideOAuthLogin(t *testing.T) {
	cases := []struct {
		name          string
		subHit        bool
		emailVerified bool
		emailHit      bool
		want          oauthDecision
	}{
		{"sub 命中即登录，不看邮箱", true, true, true, oauthLogin},
		{"sub 命中，邮箱未占用也照样登录", true, false, false, oauthLogin},
		{"全新用户，建号", false, true, false, oauthCreate},
		{"邮箱已占用但未验证 → 仍建号，未验证的邮箱不参与冲突判断", false, false, true, oauthCreate},
		{"邮箱已验证且已占用 → 冲突，提示去绑定", false, true, true, oauthEmailTaken},
	}
	for _, c := range cases {
		got := decideOAuthLogin(c.subHit, c.emailVerified, c.emailHit)
		if got != c.want {
			t.Errorf("%s: 期望 %v, 实际 %v", c.name, c.want, got)
		}
	}
}
```

- [x] **Step 2: 跑测试确认失败**

Run: `cd server && go test ./internal/user/ -run TestDecideOAuthLogin -v`
Expected: 编译失败，`undefined: decideOAuthLogin`

- [x] **Step 3: 实现**

追加到 `server/internal/user/oauthlink.go`：

```go
// oauthDecision 第三方登录的三种去向。
type oauthDecision int

const (
	oauthLogin      oauthDecision = iota // sub 已存在，直接登录
	oauthCreate                          // 全新身份，建号
	oauthEmailTaken                      // 邮箱已被其他账号占用，拒绝建号并提示去绑定
)

// decideOAuthLogin 决定第三方登录该走哪条路。
//
// 产品决策是「不自动按邮箱合并」：自动合并等于把「Google 说这个邮箱属于他」
// 当成账号所有权证明，这条链路上任何疏漏都会变成账号接管。所以宁可多一步，
// 让用户自己用密码登录后再去绑定。
//
// emailVerified 为 false 时邮箱完全不参与判断——Apple 的 token 根本没有这个
// 字段(解析为 false)，而未经验证的邮箱谁都能声称拥有。
func decideOAuthLogin(subHit, emailVerified, emailHit bool) oauthDecision {
	if subHit {
		return oauthLogin
	}
	if emailVerified && emailHit {
		return oauthEmailTaken
	}
	return oauthCreate
}
```

- [x] **Step 4: 跑测试确认通过**

Run: `cd server && go test ./internal/user/ -run TestDecideOAuthLogin -v`
Expected: PASS（5 个用例）

- [x] **Step 5: 把决策接进登录链路**

`server/internal/user/otp.go`，新增一个带冲突判断的版本（保留原 `loginOrCreateApp` 给手机号/邮箱链路用）：

```go
// loginOrCreateOAuth 第三方登录专用：在「查不到就建号」之间插入冲突判断。
//
// email 为 ID Token 里的邮箱，emailVerified 为其 email_verified claim。
// 两者共同决定是否允许建号，见 decideOAuthLogin。
func (s *Service) loginOrCreateOAuth(tenantID int64, id appIdentity, emailVerified bool) (*model.User, bool, error) {
	cond, val := id.where()

	var u model.User
	err := s.db.Where("tenant_id = ?", tenantID).First(&u, cond, val).Error
	subHit := err == nil
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}

	emailHit := false
	if !subHit && id.Email != "" && emailVerified {
		emailHit = s.identityExists(tenantID, appIdentity{Email: id.Email})
	}

	switch decideOAuthLogin(subHit, emailVerified, emailHit) {
	case oauthEmailTaken:
		return nil, false, errs.New(errs.CodeOAuthEmailTaken,
			"该邮箱已注册，请用密码登录后在「账号与安全」里绑定")
	case oauthLogin:
		// 落到下面的既有逻辑
	case oauthCreate:
		// 落到下面的既有逻辑
	}
	return s.loginOrCreateApp(tenantID, id)
}
```

> 建号时 `loginOrCreateApp` 会把 `id.Email` 一并写进 `User.Email`，这正是我们要的——修掉了「Google 注册的账号没有邮箱」那个缺口。

- [x] **Step 6: 改 `LoginWithGoogle` / `LoginWithApple`**

`server/internal/user/appauth.go`，`LoginWithGoogle` 结尾的：

```go
	return s.loginOrCreateApp(tenantID, appIdentity{GoogleSub: claims.Subject})
```

改成：

```go
	// 带上 email：不带的话 Google 注册的账号会没有邮箱，
	// 既做不了冲突判断，用户日后也找不回账号。
	return s.loginOrCreateOAuth(tenantID, appIdentity{
		GoogleSub: claims.Subject,
		Email:     strings.ToLower(claims.Email),
	}, claims.EmailVerified)
```

`LoginWithApple` 同样改为 `loginOrCreateOAuth`，但**第三个参数传 `false`**：

```go
	// Apple 的 identityToken 没有 email_verified，且用户可能选了「隐藏我的邮箱」，
	// 拿到的是 @privaterelay.appleid.com 中继地址——能收信，但不是用户的真实邮箱，
	// 不可用于冲突判断。
	return s.loginOrCreateOAuth(tenantID, appIdentity{
		AppleSub: claims.Subject,
		Email:    strings.ToLower(claims.Email),
	}, false)
```

确认 `appauth.go` 的 import 里有 `strings`。

- [x] **Step 7: 验证并提交**

```bash
cd server && go build ./... && go vet ./... && go test ./internal/user/ -v
git add server/internal/user/
git commit -m "feat(user): reject oauth signup when verified email already taken"
```

---

## Task 4: 绑定与解绑

**Files:**
- Modify: `server/internal/user/oauthlink.go`
- Modify: `server/internal/user/handler.go`
- Test: `server/internal/user/oauthlink_test.go`

**Interfaces:**
- Consumes: Task 1 的 `errs.CodeOAuthAlreadyBound` / `errs.CodeOAuthLastMethod`；Task 2 的 `nilIfEmpty`
- Produces:
  - `type loginMethods struct{ Phone, Password, Google, Apple bool }`
  - `func (m loginMethods) remainingAfterUnbind(provider string) int`
  - `POST /api/auth/google/bind` · `POST /api/auth/apple/bind` · `POST /api/auth/unbind`

- [x] **Step 1: 写失败的测试**

追加到 `server/internal/user/oauthlink_test.go`：

```go
// 解绑后必须至少还剩一种可登录方式，否则用户把自己锁在门外，
// 而且再也没有任何入口能进来改回去。
func TestRemainingAfterUnbind(t *testing.T) {
	cases := []struct {
		name     string
		m        loginMethods
		provider string
		want     int
	}{
		{"只有 Google，解绑后归零", loginMethods{Google: true}, "google", 0},
		{"手机号 + Google，解绑 Google 还剩手机号", loginMethods{Phone: true, Google: true}, "google", 1},
		{"Google + Apple，解绑 Google 还剩 Apple", loginMethods{Google: true, Apple: true}, "google", 1},
		{"只有 Apple，解绑 Apple 归零", loginMethods{Apple: true}, "apple", 0},
		{"解绑一个本来就没绑的，数量不变", loginMethods{Phone: true, Password: true}, "google", 2},
		{"四种全有，解绑 Apple 还剩三种", loginMethods{Phone: true, Password: true, Google: true, Apple: true}, "apple", 3},
	}
	for _, c := range cases {
		if got := c.m.remainingAfterUnbind(c.provider); got != c.want {
			t.Errorf("%s: 期望 %d, 实际 %d", c.name, c.want, got)
		}
	}
}
```

- [x] **Step 2: 跑测试确认失败**

Run: `cd server && go test ./internal/user/ -run TestRemainingAfterUnbind -v`
Expected: 编译失败，`undefined: loginMethods`

- [x] **Step 3: 实现纯函数**

追加到 `server/internal/user/oauthlink.go`：

```go
// loginMethods 一个账号当前拥有的可登录方式。
//
// Password 单列一项而不是并进 Phone/Email：验证码时代注册的老账号有手机号
// 但没有密码，两者不是一回事。
type loginMethods struct {
	Phone    bool
	Password bool
	Google   bool
	Apple    bool
}

// remainingAfterUnbind 解绑 provider 之后还剩几种可登录方式。
// 返回 0 表示不能解绑——用户会被锁在门外，且没有任何入口能改回来。
func (m loginMethods) remainingAfterUnbind(provider string) int {
	switch provider {
	case "google":
		m.Google = false
	case "apple":
		m.Apple = false
	}
	n := 0
	for _, has := range []bool{m.Phone, m.Password, m.Google, m.Apple} {
		if has {
			n++
		}
	}
	return n
}
```

- [x] **Step 4: 跑测试确认通过**

Run: `cd server && go test ./internal/user/ -run TestRemainingAfterUnbind -v`
Expected: PASS（6 个用例）

- [x] **Step 5: 实现 service 方法**

追加到 `server/internal/user/oauthlink.go`（文件头 import 需要 `errors`、`gorm.io/gorm`、`driftbottle/internal/model`、`driftbottle/internal/common/errs`）：

```go
// BindOAuth 把已验签的第三方身份绑到当前登录用户上。
//
// 与登录的唯一区别是最后一步：登录按 sub 查用户，绑定按 sub 查「有没有别人先绑了」，
// 没有才写到当前用户上。验签那一步两者完全共用。
func (s *Service) BindOAuth(tenantID, userID int64, provider, sub string) error {
	col := "google_sub"
	if provider == "apple" {
		col = "apple_sub"
	}

	var other model.User
	err := s.db.Where("tenant_id = ?", tenantID).First(&other, col+" = ?", sub).Error
	if err == nil {
		if other.UserID == userID {
			return nil // 幂等：已绑到自己身上
		}
		// 不透露是哪个账号——用户自己的账号自己知道，
		// 攻击者不该从这里问出关联关系。
		return errs.New(errs.CodeOAuthAlreadyBound, "该账号已被其他用户绑定")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return s.db.Model(&model.User{}).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Update(col, nilIfEmpty(sub)).Error
}

// UnbindOAuth 解绑。解绑后必须仍有可登录方式。
func (s *Service) UnbindOAuth(tenantID, userID int64, provider string) error {
	if provider != "google" && provider != "apple" {
		return errs.New(errs.CodeBadRequest, "不支持的绑定类型")
	}
	var u model.User
	if err := s.db.Where("tenant_id = ?", tenantID).First(&u, "user_id = ?", userID).Error; err != nil {
		return errs.New(errs.CodeNotFound, "用户不存在")
	}
	m := loginMethods{
		Phone:    u.Phone != "",
		Password: u.PasswordHash != "",
		Google:   u.GoogleSub != nil,
		Apple:    u.AppleSub != nil,
	}
	if m.remainingAfterUnbind(provider) == 0 {
		return errs.New(errs.CodeOAuthLastMethod, "这是你唯一的登录方式，请先绑定手机号或设置密码")
	}
	col := "google_sub"
	if provider == "apple" {
		col = "apple_sub"
	}
	return s.db.Model(&model.User{}).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Update(col, nil).Error
}
```

> `PasswordHash` 是 `model.go:75` 的实际字段名（`gorm:"size:72" json:"-"`，bcrypt 密文）。

- [x] **Step 6: 抽出共用的验签入口**

`server/internal/user/appauth.go` 增加（供绑定复用，登录侧不改）：

```go
// verifyOAuthSub 验签并返回 sub / email / emailVerified。
// 登录与绑定共用：两者的验签规则完全一致，区别只在验完之后做什么。
func (s *Service) verifyOAuthSub(appid, provider, idToken string) (sub, email string, verified bool, tenantID int64, err error) {
	tenantID, _, _, _, err = s.resolveLogin(PlatformApp, appid)
	if err != nil {
		return "", "", false, 0, err
	}
	switch provider {
	case "google":
		clientID := sysconfig.GetString(tenantID, sysconfig.KeyAppGoogleClientID)
		if clientID == "" {
			return "", "", false, 0, errs.New(errs.CodeLoginFailed, "未配置 Google Client ID")
		}
		claims, e := verifyIDToken(googleJWKS, idToken, clientID)
		if e != nil {
			return "", "", false, 0, errs.New(errs.CodeLoginFailed, "Google 登录校验失败")
		}
		if !googleIssuers[claims.Issuer] {
			return "", "", false, 0, errs.New(errs.CodeLoginFailed, "Google 令牌签发方不正确")
		}
		return claims.Subject, strings.ToLower(claims.Email), claims.EmailVerified, tenantID, nil
	case "apple":
		bundleID := sysconfig.GetString(tenantID, sysconfig.KeyAppAppleBundleID)
		if bundleID == "" {
			return "", "", false, 0, errs.New(errs.CodeLoginFailed, "未配置 Apple Bundle ID")
		}
		claims, e := verifyIDToken(appleJWKS, idToken, bundleID)
		if e != nil {
			return "", "", false, 0, errs.New(errs.CodeLoginFailed, "Apple 登录校验失败")
		}
		if claims.Issuer != appleIssuer {
			return "", "", false, 0, errs.New(errs.CodeLoginFailed, "Apple 令牌签发方不正确")
		}
		return claims.Subject, strings.ToLower(claims.Email), false, tenantID, nil
	}
	return "", "", false, 0, errs.New(errs.CodeBadRequest, "不支持的登录方式")
}
```

> `resolveLogin` 的签名是 `(platform, appid string) (tenantID int64, wxAppID, wxSecret, aliAppID string, err error)`（`user/service.go:37`），故上面用 5 个返回值接。

- [x] **Step 7: 加路由与 handler**

`server/internal/user/handler.go`，在 `g := api.Group("/user", auth)` **之前**加（绑定需要登录态，所以要单独挂 `auth`）：

```go
	api.POST("/auth/google/bind", auth, h.bindGoogle)
	api.POST("/auth/apple/bind", auth, h.bindApple)
	api.POST("/auth/unbind", auth, h.unbind)
```

在 `loginApple` 之后加：

```go
func (h *Handler) bindGoogle(c *gin.Context) { h.bindOAuth(c, "google") }
func (h *Handler) bindApple(c *gin.Context)  { h.bindOAuth(c, "apple") }

func (h *Handler) bindOAuth(c *gin.Context, provider string) {
	var req oauthLoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	sub, _, _, tenantID, err := h.svc.verifyOAuthSub(req.AppID, provider, req.IDToken)
	if err == nil {
		err = h.svc.BindOAuth(tenantID, middleware.UserID(c), provider, sub)
	}
	if err != nil {
		if be, ok := err.(*errs.BizError); ok {
			response.Fail(c, be.Code, be.Msg)
			return
		}
		response.Fail(c, errs.CodeServerError, "绑定失败")
		return
	}
	response.OK(c, gin.H{"provider": provider, "bound": true})
}

func (h *Handler) unbind(c *gin.Context) {
	var req struct {
		Provider string `json:"provider" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errs.CodeBadRequest, "参数错误")
		return
	}
	if err := h.svc.UnbindOAuth(middleware.TenantID(c), middleware.UserID(c), req.Provider); err != nil {
		if be, ok := err.(*errs.BizError); ok {
			response.Fail(c, be.Code, be.Msg)
			return
		}
		response.Fail(c, errs.CodeServerError, "解绑失败")
		return
	}
	response.OK(c, gin.H{"provider": req.Provider, "bound": false})
}
```

- [x] **Step 8: 让 profile 能告诉前端绑定状态**

A6 页面要显示「已绑定 / 未绑定」。在 `internal/common/appdto` 的 `FromUser` 里补两个布尔字段：

```go
	GoogleBound bool `json:"google_bound"`
	AppleBound  bool `json:"apple_bound"`
```

赋值 `u.GoogleSub != nil` / `u.AppleSub != nil`。**不要下发 sub 本身**——那是第三方的用户标识，没有任何前端用途。

- [x] **Step 9: 验证并提交**

```bash
cd server && go build ./... && go vet ./... && go test ./... -v
git add server/internal/user/ server/internal/common/appdto/
git commit -m "feat(user): add oauth bind and unbind endpoints"
```

- [x] **Step 10: 手工联调**

后端跑起来（`cd server && go run ./cmd/api`），用 curl 验证三条新路由**在没有登录态时返回 401**，确认 `auth` 中间件挂对了：

```bash
curl -s -X POST http://localhost:8980/api/auth/unbind -H 'Content-Type: application/json' -d '{"provider":"google"}'
```
Expected: 返回 `2001`（`CodeUnauthorized`），**不是** `1001`。

---

## Task 5: Flutter 依赖与平台配置

**Files:**
- Modify: `app/bottles/pubspec.yaml`
- Create: `app/bottles/lib/core/platform/oauth.dart`
- Modify: `app/bottles/ios/Runner/Info.plist`

**Interfaces:**
- Consumes: 无
- Produces:
  - `abstract class OAuthClient { Future<String?> googleIdToken(); Future<String?> appleIdToken(); }`
  - `final oauthClientProvider = Provider<OAuthClient>(...)`

- [ ] **Step 1: 加依赖**

`app/bottles/pubspec.yaml` 的 `dependencies` 里加（**放在 `flutter_secure_storage` 那两条注释之外，不要碰那两条锁版本**）：

```yaml
  google_sign_in: ^7.0.0
  sign_in_with_apple: ^7.0.0
```

- [ ] **Step 2: 拉依赖并确认没破坏 native-assets**

Run: `cd app/bottles && flutter pub get`
Expected: `Got dependencies.`，**不出现任何 `Architecture.arm64e` 或 native-assets 相关报错**。若出现，立刻 `git checkout pubspec.yaml pubspec.lock` 并停下——那说明新依赖间接升了被锁住的包。

- [ ] **Step 3: 写 OAuth 封装**

创建 `app/bottles/lib/core/platform/oauth.dart`：

```dart
import 'dart:io' show Platform;

import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:google_sign_in/google_sign_in.dart';
import 'package:sign_in_with_apple/sign_in_with_apple.dart';

/// 把两个第三方登录 SDK 关在这一个文件里。
///
/// 对外只暴露「给我一个 idToken」，因为服务端要的就只有这个——
/// 昵称、头像、邮箱一律以服务端验签后的 claims 为准，不信客户端。
///
/// 单独成文件是因为这两个 SDK 的 API 变动频繁（google_sign_in 7.x 刚换到
/// Credential Manager），隔离后升级只改这一处。
abstract class OAuthClient {
  /// 返回 null 表示用户主动取消，不是错误，调用方应静默返回。
  Future<String?> googleIdToken();

  /// 仅 iOS 可用。Android 上返回 null。
  Future<String?> appleIdToken();
}

class RealOAuthClient implements OAuthClient {
  @override
  Future<String?> googleIdToken() async {
    final account = await GoogleSignIn.instance.authenticate();
    return account?.authentication.idToken;
  }

  @override
  Future<String?> appleIdToken() async {
    if (!Platform.isIOS) return null;
    final cred = await SignInWithApple.getAppleIDCredential(
      scopes: [
        AppleIDAuthorizationScopes.email,
        AppleIDAuthorizationScopes.fullName,
      ],
    );
    return cred.identityToken;
  }
}

final oauthClientProvider = Provider<OAuthClient>((ref) => RealOAuthClient());

/// Apple 登录按钮是否显示。
///
/// 先判 kIsWeb 再判 Platform.isIOS：Web 端访问 dart:io 的 Platform 会直接抛异常，
/// 顺序反了 Web 构建会在运行期崩。
/// App Store 4.8 只约束 iOS，Android 上不显示是对的。
bool get showAppleSignIn => !kIsWeb && Platform.isIOS;
```

> `google_sign_in` 7.x 的确切 API（`GoogleSignIn.instance.authenticate()`）以 `flutter pub get` 后 `.pub-cache` 里的实际版本为准。若签名不同，按该版本的 README 调整**本文件内部**，不要改 `OAuthClient` 接口——隔离 SDK 变动正是这个文件存在的理由。

- [ ] **Step 4: iOS 配置**

`app/bottles/ios/Runner/Info.plist` 加 Google 的 URL Scheme（值为 iOS Client ID 的**反转形式**，从 Google Cloud Console 下载的 `GoogleService-Info.plist` 里的 `REVERSED_CLIENT_ID`）：

```xml
<key>CFBundleURLTypes</key>
<array>
  <dict>
    <key>CFBundleURLSchemes</key>
    <array>
      <string>com.googleusercontent.apps.REPLACE-WITH-REVERSED-CLIENT-ID</string>
    </array>
  </dict>
</array>
```

Xcode 侧还需在 Signing & Capabilities 里加 **Sign in with Apple** capability。**本机（Windows）无法完成这一步**，记录在案，交由有 macOS 的环境执行。

- [ ] **Step 5: 验证并提交**

```bash
cd app/bottles && flutter analyze
git add app/bottles/pubspec.yaml app/bottles/pubspec.lock \
        app/bottles/lib/core/platform/oauth.dart app/bottles/ios/Runner/Info.plist
git commit -m "feat(app): add google and apple sign-in clients"
```

---

## Task 6: Flutter 登录 UI（A2 / A2g / A2p / A2k）

**Files:**
- Modify: `app/bottles/lib/data/repositories.dart`
- Modify: `app/bottles/lib/data/remote/remote_repositories.dart`
- Modify: `app/bottles/lib/data/mock/mock_repositories.dart`
- Modify: `app/bottles/lib/features/auth/auth_controller.dart`
- Modify: `app/bottles/lib/features/auth/login_page.dart`
- Create: `app/bottles/lib/features/auth/oauth_conflict_page.dart`
- Modify: `app/bottles/lib/app/routes.dart` · `router.dart`

**Interfaces:**
- Consumes: Task 5 的 `OAuthClient` / `oauthClientProvider` / `showAppleSignIn`；Task 3 的 `POST /api/auth/google` 返回的 `2004`
- Produces:
  - `AuthRepository.loginWithOAuth(String provider, String idToken)`
  - `Routes.oauthConflict = '/auth/oauth-conflict'`

- [ ] **Step 1: 扩 Repository 接口**

`app/bottles/lib/data/repositories.dart` 的 `AuthRepository` 抽象类加：

```dart
  /// provider 为 'google' 或 'apple'。
  /// 服务端返回 2004 时抛 OAuthEmailTakenException，由 UI 跳 A2k。
  Future<AuthResult> loginWithOAuth(String provider, String idToken);
```

并在同文件定义异常（放在文件末尾）：

```dart
/// 第三方登录时邮箱已被占用。对应服务端 errs.CodeOAuthEmailTaken (2004)。
/// 单独成类型而不是靠错误码字符串比较——UI 要据此跳转到专门的一屏。
class OAuthEmailTakenException implements Exception {
  const OAuthEmailTakenException(this.message, this.email);
  final String message;
  final String email;
}
```

- [ ] **Step 2: 实现 remote**

`remote_repositories.dart` 的 `AuthRepository` 实现类加：

```dart
  @override
  Future<AuthResult> loginWithOAuth(String provider, String idToken) async {
    try {
      final data = await _client.post('/auth/$provider', body: {
        'appid': AppConfig.appId,
        'id_token': idToken,
      });
      return AuthResult.fromJson(data as Map<String, dynamic>);
    } on ApiException catch (e) {
      if (e.code == 2004) {
        throw OAuthEmailTakenException(e.message, '');
      }
      rethrow;
    }
  }
```

> `ApiException` 的构造是 `ApiException(this.code, this.message, {this.data})`，`code` 为 `int`（`core/network/api_exception.dart:25`），故可直接比较 `e.code == 2004`。`_client.post` 的确切签名见 `core/network/api_client.dart`。

- [ ] **Step 3: 实现 mock（保持 Mock 后端可用）**

`mock_repositories.dart` 加：

```dart
  @override
  Future<AuthResult> loginWithOAuth(String provider, String idToken) async {
    // Mock 后端固定放行，用于没有网络时调 UI。
    // idToken 为 'conflict' 时模拟 A2k，方便单独调那一屏。
    if (idToken == 'conflict') {
      throw const OAuthEmailTakenException('该邮箱已注册，请用密码登录后绑定', 'meera@gmail.com');
    }
    return _backend.oauthLogin(provider);
  }
```

若 `MockBackend` 没有 `oauthLogin`，在 `mock_backend.dart` 里加一个，复用已有的建号逻辑。

- [ ] **Step 4: 跑 analyze 确认三处实现都补齐了**

Run: `cd app/bottles && flutter analyze`
Expected: 无 `missing_concrete_implementation` 之类的报错。若 Mock 没补会在这里暴露。

- [ ] **Step 5: 加 controller 方法**

`auth_controller.dart` 加：

```dart
  /// 第三方登录。用户取消(idToken 为 null)时静默返回，不报错。
  Future<void> signInWithOAuth(String provider) async {
    state = state.copyWith(loading: true, error: null);
    try {
      final client = ref.read(oauthClientProvider);
      final idToken = provider == 'google'
          ? await client.googleIdToken()
          : await client.appleIdToken();
      if (idToken == null) {
        state = state.copyWith(loading: false); // 用户取消，不是错误
        return;
      }
      final result = await _repo.loginWithOAuth(provider, idToken);
      await _persist(result);
    } on OAuthEmailTakenException catch (e) {
      state = state.copyWith(loading: false, conflictEmail: e.email);
    } catch (e) {
      state = state.copyWith(loading: false, error: '登录失败，请重试');
    }
  }
```

`AuthState` 需要相应增加 `conflictEmail` 字段；`_persist` 用现有的落 token 逻辑。

- [ ] **Step 6: 改登录页（A2）**

`login_page.dart` 在「还没有账号？注册」之下加分隔线与两个按钮，对齐原型 A2：

```dart
  const _Divider(text: '或'),
  _OAuthButton(
    label: '用 Google 继续',
    onTap: () => ref.read(authControllerProvider.notifier).signInWithOAuth('google'),
  ),
  if (showAppleSignIn)
    _OAuthButton(
      label: '用 Apple 继续',
      onTap: () => ref.read(authControllerProvider.notifier).signInWithOAuth('apple'),
    ),
```

> **两个按钮的尺寸 / 圆角 / 文案 / logo 都有强制品牌规范，自己画会被打回。** 用官方 SDK 提供的按钮组件：Apple 用 `SignInWithAppleButton`，Google 按其品牌指南实现。原型只标了位置。

- [ ] **Step 7: 建冲突页（A2k）**

创建 `oauth_conflict_page.dart`，按原型 A2k：标题「这个邮箱已经有账号了」、正文说明不会自动合并、一个提示条（请先用密码登录再到「账号与安全」绑定）、两个按钮「用密码登录」与「换一个 Google 账号」。

在 `routes.dart` 加 `static const oauthConflict = '/auth/oauth-conflict';`，在 `router.dart` 注册；controller 里 `conflictEmail` 非空时导航过去。

- [ ] **Step 8: 验证并提交**

```bash
cd app/bottles && flutter analyze && flutter test
git add app/bottles/lib/
git commit -m "feat(app): google and apple sign-in on the login screen"
```

---

## Task 7: Flutter 账号与安全页（A6 / A6b）

**Files:**
- Create: `app/bottles/lib/features/settings/account_security_page.dart`
- Modify: `app/bottles/lib/data/repositories.dart` · `remote_repositories.dart` · `mock_repositories.dart`
- Modify: `app/bottles/lib/app/routes.dart` · `router.dart`
- Modify: 「我的」页（加入口）

**Interfaces:**
- Consumes: Task 4 的三条路由与 `google_bound` / `apple_bound` 字段
- Produces: `Routes.accountSecurity = '/settings/account-security'`

- [ ] **Step 1: 扩 Repository**

`repositories.dart` 的 `AuthRepository` 加：

```dart
  Future<void> bindOAuth(String provider, String idToken);
  Future<void> unbindOAuth(String provider);
```

并加异常：

```dart
/// 该第三方账号已被其他用户绑定。对应服务端 2005。
class OAuthAlreadyBoundException implements Exception {
  const OAuthAlreadyBoundException(this.message);
  final String message;
}

/// 解绑后将没有任何可登录方式。对应服务端 2006。
class OAuthLastMethodException implements Exception {
  const OAuthLastMethodException(this.message);
  final String message;
}
```

- [ ] **Step 2: 实现 remote**

```dart
  @override
  Future<void> bindOAuth(String provider, String idToken) async {
    try {
      await _client.post('/auth/$provider/bind', body: {
        'appid': AppConfig.appId,
        'id_token': idToken,
      });
    } on ApiException catch (e) {
      if (e.code == 2005) throw OAuthAlreadyBoundException(e.message);
      rethrow;
    }
  }

  @override
  Future<void> unbindOAuth(String provider) async {
    try {
      await _client.post('/auth/unbind', body: {'provider': provider});
    } on ApiException catch (e) {
      if (e.code == 2006) throw OAuthLastMethodException(e.message);
      rethrow;
    }
  }
```

Mock 侧同样补两个方法（`bindOAuth` 直接成功，`unbindOAuth` 在只剩一种方式时抛 `OAuthLastMethodException`，方便调那一屏）。

- [ ] **Step 3: 跑 analyze**

Run: `cd app/bottles && flutter analyze`
Expected: 无未实现抽象方法的报错。

- [ ] **Step 4: 建页面**

`account_security_page.dart`，按原型 A6：

- 「登录方式」分组：手机号（脱敏）、邮箱（脱敏）、Google（已绑定 / 未绑定 + 操作）、Apple（非 iOS 显示「仅 iOS 可绑定」且不可点）
- 「安全」分组：修改密码、删除账号
- 底部提示条：至少保留一种登录方式；只剩一种时该项解绑入口置灰

**脱敏必须做**：这一屏虽然要登录态才能进，但截屏外泄是常态。手机号 `+91 987•• ••210`、邮箱 `me•••a@gmail.com`。

- [ ] **Step 5: 建绑定结果弹层（A6b）**

在同一文件内用 `showDialog` 实现两态：

- **成功**：对勾图标 +「已绑定 meera@gmail.com」
- **被占用**：禁止图标 +「这个 Google 账号已被占用」+ 提示（如果那个账号也是你的，先用它登录后解绑）+ 两个按钮

**被占用时不要显示是哪个账号**（不显示手机号尾号之类）——用户自己的账号自己知道，攻击者不该从这里问出关联关系。

- [ ] **Step 6: 加入口与路由**

`routes.dart` 加 `accountSecurity`；`router.dart` 注册；「我的」页的设置区加一行「账号与安全」。

- [ ] **Step 7: 验证并提交**

```bash
cd app/bottles && flutter analyze && flutter test
git add app/bottles/lib/
git commit -m "feat(app): account security page with oauth bind and unbind"
```

- [ ] **Step 8: 端到端手工验证**

后端跑起来，Android 模拟器（`Pixel_7_API_36`）里：

1. 用手机号注册一个账号，退出
2. 点「用 Google 继续」，选一个**邮箱与步骤 1 不同**的 Google 账号 → 应登录成功并建新号
3. 退出，用步骤 1 的账号密码登录 → 进「账号与安全」→ 绑定 Google，选步骤 2 用过的那个账号 → 应弹 **A6b 被占用**
4. 换一个未用过的 Google 账号绑定 → 应成功，列表变「已绑定」
5. 解绑 Google → 应成功（还有手机号）
6. 构造一个只有 Google 的账号，尝试解绑 → 应被 `2006` 拦住

> 步骤 2 需要两个不同的 Google 账号。模拟器里可以在系统设置添加多个。

---

## 实施记录（2026-09-19，Task 1–4 已完成）

后端四个任务已实施并提交（`e86140e` / `c66e0d9` / `41cbe71` / `774389a`）。
与计划不一致之处，接手 Task 5–7 前请先读：

1. **`verifyOAuthSub` 用的是 `resolveAppTenant` 而非 `resolveLogin`。** 计划写错了——
   App 端有自己的租户解析（`otp.go:89`，凭证表 → `APP_DEFAULT_TENANT_ID` → 单租户默认），
   `resolveLogin` 是小程序 code2session 那条路。
2. **邮箱归一用仓库已有的 `normalizeEmail`**（`otp.go:38`，小写 + 去空白），
   不是计划里写的 `strings.ToLower`，因此 `appauth.go` 无需新增 `strings` import。
3. **🆕 计划没预见的一处 bug，已修**：`appIdentity.where()` 的分支优先级原本是
   Phone → Email → GoogleSub → AppleSub。让 OAuth 登录同时带 `Email` 和 `GoogleSub`
   之后，它会按 **email** 查——那正好就是被否决的「按邮箱自动合并」。
   已改为 sub 优先，并加了 `TestAppIdentityWherePrefersSub` 钉住。
   **这个错误完全静默**：不报错，只是登录悄悄变成合并。
4. **`User.TenantID` 的 tag 也要改**（计划只说了 `GoogleSub`/`AppleSub`）：
   两个唯一索引的 `priority:1` 挂在 `TenantID` 上，第三方身份是**租户内**唯一。
5. **Task 4 Step 10 的手工 curl 换成了自动化测试**。本机没有 MySQL/Redis，
   起不了服务；改为 `cmd/api/routes_test.go` 里的 `TestOAuthBindRoutesRequireAuth`，
   用会 401 中断的哨兵中间件替代 `auth`，断言三条绑定路由拿 401、两条登录路由不拿。
   已验证该测试在去掉 `auth` 时确实失败。
6. **`internal/robot` 有 3 个既有的不稳定测试**（`identity_guard_test.go`，
   每次失败的用例和提示语都不同）。与本次改动无关，已在 HEAD 上验证同样失败。
   跑全量测试时用 `go test $(go list ./... | grep -v /internal/robot)` 绕开。

**尚未验证**：`nullifyEmptyOAuthSubs` 只在编译层面通过，**没有在真实数据库上跑过**。
上线前必须在生产库快照上演练一次，确认刷新行数符合预期且唯一索引建得出来。

---

## 自查

**Spec 覆盖**（对照 spec §四）：

| Spec 要求 | 落点 |
|---|---|
| 缺口 1 claims 取 `email_verified` | Task 1 |
| 缺口 2 `LoginWithGoogle` 带 email | Task 3 Step 6 |
| 缺口 3 冲突分支 | Task 3 |
| 缺口 4 绑定/解绑接口 | Task 4 |
| 缺口 5 唯一索引 | Task 2 |
| 客户端两个 SDK | Task 5 |
| Apple 按钮仅 iOS | Task 5（`showAppleSignIn`）+ Task 6 Step 6 |
| 解绑前置校验 | Task 4（`remainingAfterUnbind`） |
| 原型 A2/A2g/A2k/A2p | Task 6 |
| 原型 A6/A6b | Task 7 |

**计划外但必须做的一条**（spec 未覆盖，实施时发现）：`DeleteAccount` 把 `google_sub`/`apple_sub` 置 `''`，加唯一索引后**第二个账号注销会撞唯一键失败**。已加为 Task 2 Step 7。

**不在本计划**（属 spec 的遗留项）：`Phone` / `Email` 的唯一索引；release keystore 与 SHA-1 登记（决策 4，属开户工作）；OAuth Client ID 的申请。

---

## 前置条件（代码之外，必须先就绪）

1. **Google Cloud Console 建 3 个 OAuth Client ID**：Web（填后台 `app_google_client_id`）、Android（绑包名 `com.ambertu.bottles` + 签名 SHA-1）、iOS（绑 Bundle ID）。**`AIzaSy…` 开头的是 Maps API Key，不能用于登录。**
2. **后台「App 登录」分组填 `app_google_client_id`**（Web 那个）与 `app_apple_bundle_id`。
3. **Apple Developer 配 Sign in with Apple**（需付费账号）。
4. **Android 的 debug 与 release 两套 SHA-1 都要登记**；启用 Play App Signing 后，release 的 SHA-1 要从 **Play Console「应用完整性」页**取，不是本地 keystore。
