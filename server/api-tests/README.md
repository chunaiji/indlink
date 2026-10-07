# 漂流瓶接口测试(Postman + Newman)

核心链路接口回归:登录 → 扔/捞瓶 → 回信/解锁 → 开聊 → 充值 → 风控。按 `api-testing` skill 6 步产出,适配 Go(Gin)后端。

## 适配说明(与 skill 原 Spring Boot 模板的差异)

- **鉴权**:无 OAuth `tokenUrl`;改为 `POST /auth/login {platform,code}` 取 JWT,`00 准备` 文件夹登录双用户并把 token 写入 collection 变量 `tokenA/tokenB`。
- **成功判定**:统一 `{code,msg,data}`,`code === 0` 成功(约定 B);鉴权失败为 **HTTP 401 + code 2001**,业务/参数错误为 **HTTP 200 + code≠0**。
- **dev mock 登录**:未配微信凭证时任意 `code` 可登录,测试用 `apiA_{{runId}}` / `apiB_{{runId}}`,`runId` 每次跑测随机生成 → 全新用户(各送 50 金币),余额断言确定。
- **int64 ID**:后端已改为字符串序列化,Postman `pm.response.json()` 不丢精度。

## 前置

```
npm i -g newman newman-reporter-htmlextra
```

后端需在 dev 起着(MySQL+Redis 已连):

```
cd ..               # server/
go run ./cmd/api     # 监听 :8980
```

## 跑

```
# Windows
./run.ps1 -e dev
./run.ps1 -e dev -m core
./run.ps1 -e dev -m core -f "05 钱包 / 充值(钱·幂等核心)"

# *nix / Git Bash
./run.sh -e dev
./run.sh -e dev -m core
```

## 报告

- `reports/core.html` — 人看的用例明细
- `reports/core.junit.xml` — CI 用
- `reports/core.json` — newman 原始(含完整请求/响应)
- `reports/core.responses.jsonl` / `.responses.md` — 跑完自动派生的逐请求响应留底

```bash
# 看所有失败响应
jq -c 'select(.assertionsFailed > 0)' reports/core.responses.jsonl
```

## 用例矩阵

每个接口覆盖 6 类(正常/鉴权/参数/业务/边界/幂等),N/A 写原因。矩阵表写在 collection 各文件夹的 description 里,Postman 打开或看 `core.postman_collection.json` 即可对照。

## 维护

- 接口变更 → 改 `collections/core.postman_collection.json`(或改生成脚本重生成)
- 新 bug 场景 → 补到对应文件夹
- 偶发失败 → 登记 `known-flaky.md`
