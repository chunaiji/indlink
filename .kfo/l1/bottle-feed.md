# L1 设计 — 捞瓶推荐 Feed(bottle.feed)

| 字段 | 值 |
|---|---|
| KFO 层级 | L1 — 设计层(模块级参考) |
| 最后更新 | 2026-06-21 |
| 覆盖模块 | `internal/bottle/feed.go`、`internal/bottle/service.go`(Scoop/热度) |
| 关联 | L0 `l0/architecture.md` · L2 `l2/2026-06-20-driftbottle-v1-and-api-tests.md` · 规划 `l2/2026-06-21-feature-roadmap.md`(#8) · `l1/admin-platform.md`(权重配置) · L4 `l4/cross-platform-api-patterns.md` |

---

> **多租户(已实现)**:`Scoop/rebuild` 候选池加 `tenant_id` 过滤;feed Redis 键为 `user_feed:{tenantID}:{userID}`。跨租户捞瓶不可见(已测)。见 `l1/tenant-saas.md`。

## 一、核心思想:feed 走缓存,不实时查库

每次捞瓶若实时 `查候选 → 打分 → 排序`,高并发直接压垮 MySQL。设计为 **Redis 预生成列表**:用户 feed 提前算好存 `user_feed:{userId}`,捞瓶只从列表弹 ID,命中空时才重建。

```
Next(userId, city, tags):
  LRange user_feed:{uid} 0..(batch-1)     // 取一批(batch=5)
  若为空 → rebuild() → 再取
  LTrim 弹出已取的 ID                       // 取走即消费,避免重复
  按 ID 批量查 bottle(status=active)返回
```

## 二、候选池(rebuild)

```sql
status = 'active' AND expire_at > now() AND user_id <> :self
AND bottle_id NOT IN (SELECT bottle_id FROM match_log WHERE viewer_id = :self)   -- 去重:看过的不再捞
AND user_id    NOT IN (SELECT target_id FROM blocks    WHERE user_id  = :self)   -- 排除被自己拉黑的作者
ORDER BY heat_score DESC, created_at DESC
LIMIT feed_size * 3   -- 多取,打分后截断
```

- **不捞自己**:`user_id <> :self`(产品要求 #8,早已具备)。
- **去重**靠 `match_log`;**拉黑过滤**靠 `blocks`(2026-06-21 #8 补)。
- **筛选(#7)**:`Filter{Scope,Gender}` —— `scope=local` 加硬过滤 `city=:viewerCity`;`gender>0` 加 `user_id IN (SELECT user_id FROM users WHERE gender=?)`。
- `feed_size` 来自 `config`(默认 50),运营可调。
- **缓存键含筛选签名**:`user_feed:{tenant}:{user}:{scope}-{gender}` —— 不同筛选条件各自缓存,互不串味。

## 三、打分公式(#8 精准匹配,2026-06-21 升级,权重运营可调)

```
score = wTag·标签重合度   // 命中数/浏览者标签数(0~1);浏览者标签 = 入参 + 自己发过的瓶子标签
      + wCity·同城        // bottle.city == 浏览者 city(入参优先,空则取资料)
      + wGender·异性优先   // 双方性别已知且相异(作者性别批量查 users,匿名不外泄)
      + wFresh·新鲜度      // 7 天内线性衰减 max(0, 1 - ageHours/168)
      + wHeat·热度归一     // heat_score / maxHeat(本批最大)
      − wRobot·机器人惩罚  // 作者 is_robot 则减分(降低机器人占比)
      + wRandom·随机扰动   // (bottle_id ^ index*黄金比) & 0xffff / 65535
```

- 权重 `wTag/wCity/wGender/wFresh/wHeat/wRobot/wRandom` 全部走 `sysconfig`(管理台「匹配权重」组热调,默认 40/25/15/10/10/20/15)。
- 作者 **性别 / is_robot 批量取**(`authorMeta`,避免 N+1);浏览者画像取 `gender/city` + 兴趣标签聚合。
- 随机因子用 `bottle_id ^ index` 扰动,**不依赖 `rand`/时间**(可重放、无副作用)。
- 打分后插入排序取 score 降序 top `feed_size`。
- `User.IsRobot` 字段已加(默认 false),机器人执行逻辑见规划 #1。

## 四、写回 Redis

```
DEL user_feed:{uid}
RPUSH user_feed:{uid} <排序后的 bottleId 列表>
EXPIRE user_feed:{uid} 10min        // 10 分钟过期 → 定期刷新,避免列表陈旧
```

## 五、热度(异步性弱化版)

- 回信 `+3`、点赞 `+1`(`service.Reply`/`Like` 内更新 `heat_score`),候选池按热度排序。
- V1 同步更新(量不大);后续可改 Redis 计数 + 定时刷库(对齐 L0"行为异步化"原则)。

## 六、V1 取舍(YAGNI)

砍掉了产品文档里的 `emotion_profile` 情绪画像、`user_tag` 画像、`interaction_log` 训练表 —— 那是 V2/V4 的高级推荐。V1 用"标签+同城+热度+随机"轻量打分即可起量。

## 七、已知边界 / 待办

- 单用户 feed 取完(列表空)且无新候选 → 返回空数组(前端提示"海面很安静")。
- `Scoop` 当前不自动记 view(仅 like/skip/reply 记 match_log),意味着同一批可能重复捞到未交互的瓶子 → 待办:捞取即写 view 去重,或前端去重。
- 多实例部署时 feed 仍是 per-user Redis key,天然共享,无需额外改造。
