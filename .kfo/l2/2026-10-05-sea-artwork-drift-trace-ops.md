# 海面美术与点击捞瓶 · 漂流轨迹 · 后台改币与在线曲线

> 2026-10-05 · L2 · 关联 [l1/bottle-feed](../l1/bottle-feed.md) [l1/robot](../l1/robot.md) [l1/pay-wallet](../l1/pay-wallet.md) [l1/admin-platform](../l1/admin-platform.md)
> 状态:**COMPLETED**

两条线并行:App 侧把海面做成「有东西可玩」,服务端侧把运营缺的手段补上。

## 1. 海面:真实美术 + 点击即捞 + 昼夜三态

- `BottleSprite` 包 `assets/images/bottle.png`(由供图裁出),六个海面槽位六个倾角,**全部落在地平线以下、避开灯塔**。捞起上提、投掷抛物线、登录页与启动页复用同一个 sprite。
- **点一下漂浮的瓶子就走捞瓶仪式**,删掉原来的预览气泡;发现页「✕」按钮撤掉(左滑仍然算跳过)。
- 海面状态从昼/夜两态扩成三态:**17:00–21:00 黄昏**铺日落图,夜态优先级最高(`dda0618`);随后补上夜间瓶子美术(`424082b`)。

## 2. 漂流轨迹(drift trace)

服务端把 `MatchLog` 聚合成**按城市的节点**(thrown / seen / replied),内嵌进 App 的瓶子详情、
我的瓶子、轨迹三个响应。**捞过这只瓶子的人才能看它的轨迹**。App 的瓶子详情页据此渲染
「这只瓶子去过哪」时间线,捞瓶人那条结尾是「这里」。

## 3. 后台运营手段

| 能力 | 实现 |
|---|---|
| 手动调币 | `wallet.AdminAdjust`(**只动余额**)+ `POST /admin/users/:id/coins`;流水带备注,场景记 `admin` |
| 在线人数曲线 | `/hook-count` 返回 `online`:可配基数 `online_base` 经 24h 曲线整形 + 按分钟的确定性抖动 `online_jitter` |
| 远程启动图 | `app_splash_image_1..5`(image 类型,新「App 启动图」分组)经 `/app-config` 的 `splash.images` 下发 |
| 道具流水 | `GET /item/orders`:按用户的道具账本(买入 / 送出 / 收到) |
| 机器人全量在线 | 发现页机器人恒为在线;同城按 `city_robot_top_m/n` 注入头部 |

## 4. 机器人按语言生成

机器人档案按语言成套生成(中文/英文的名字、城市、兴趣、简介),受 `robot_language` 控制;
后台可创建/编辑机器人的完整档案;**LLM 回复与主动搭讪开场白都跟随该机器人的语言**。

> 这是 App 出海的前置:此前机器人池是中文的,英文用户看到的是一池中文昵称。
