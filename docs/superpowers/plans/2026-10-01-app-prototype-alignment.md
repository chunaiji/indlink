# App 全页面对齐原型实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> 本计划已由用户指定 **主 agent 内联执行**（Native）。

**Goal:** 让 `app/bottles` 的全部 40 余个页面在中国 / 印度 / 全球主流手机上呈现与 `docs/prototype/v2-screens.html` 一致的观感，并把对齐工作沉淀为可复用的组件库和可重复的比对流程。

**Architecture:** 先修共享组件（一处改动全 App 见效），再按原型分节逐页对齐；每页按规范 §6 登记固定区 / 弹性区 / 可滚动区。海面、引导、捞瓶仪式三类整屏装饰画面走 `DesignCanvas`。每个任务的验证分两层：布局矩阵测试（六档机型不 overflow）在本地跑，视觉比对靠 `tool/proto_render.py` 把原型按真机逻辑尺寸渲染后与真机截图并排。

**Tech Stack:** Flutter 3.47 / Dart 3.13、Riverpod 3、go_router 18；比对工具用 Python 3 + Pillow + 本机 Edge 无头模式。

**Spec:** `docs/superpowers/specs/2026-09-30-app-layout-adaptation-design.md`（下文简称"规范"）。计划里的每个数字都能在规范或原型 CSS 里找到出处。

---

## Global Constraints

逐条来自规范，每个任务默认包含：

- 一切尺寸单位是逻辑像素；字号、间距、圆角**只用 `Dim` 刻度，永远不乘屏幕比例**；禁止 `flutter_screenutil`。
- 设计基准宽 375；**同一套手机布局必须在 360×780 上不溢出**；最矮验证高 667；宽 ≥ 600 只做限宽 480 居中。
- **宽度决定布局，高度只决定装饰区收缩**；矮屏阈值 700，收缩态不换布局。
- 断点只在 `Breakpoints`（`tokens.dart`）定义，页面里不写数字，不用 `LayoutBuilder` 判断设备类型。
- SafeArea 用 `MediaQuery.viewPaddingOf`，键盘用 `MediaQuery.viewInsetsOf`，两者不混用；底部固定按钮下方 padding = `max(viewPadding.bottom, Dim.s4)`。
- 文字缩放已在 App 根钳制 0.9～1.2，页面禁止再包 `MediaQuery` 覆盖，禁止 `TextScaler.noScaling`。
- 每档文字样式显式 `height`；标题类 1.25，正文 1.6，次要与辅助 1.5。
- Material 密度已收到 compact + shrinkWrap，可点区域下限 `Dim.tap`（44）。
- 手机锁竖屏，图片查看器例外（§4.8）。
- 每个页面文件头部必须有 §6 登记注释（类别 / 固定区 / 弹性区 / 可滚动区 / 键盘）。
- 高屏多余高度的去向必须登记；表单类整屏页默认 1:2 分到头部上方与底部，只能用 `Spacer`。
- 按钮语义四级是产品规则：海蓝 = 免费主线，粉 = 要花币或真钱，灰描边 = 次要，警示色 = 破坏性；渐变只用于品牌面。
- 夜场与系统暗色是两根独立的轴，四种组合都要成立。
- 框内图标一律线性图标（`Icons` 或矢量），不用 emoji；emoji 只允许出现在文案里。
- 回复用中文；代码、标识符、commit message 英文（conventional commits）。
- **不主动跑测试 / 脚本，每次运行需用户明确授权**；git 操作仅在明确指示时执行。计划里写的 `Run:` 步骤都要先问。

### 原型 px → pt 换算表

原型手机框 252px = 375pt，系数 **1.488**。常用值：

| 原型 px | pt | 用途 |
|---|---|---|
| 20 | 30 | 小图标按钮、模态 ✕ |
| 23 | 34 | 头图右上圆按钮 `.hdr .ic` |
| 26 | 39 | 大数字（钱包余额、签到）、`.wallet` 上探 |
| 30 | 44 | 导航栏、列表行、`.jn` 按钮、输入框 `.inp .fd`、发送圆钮 |
| 32 | 48 | `.quad` 图标格、签到格 |
| 34 | 50 | `.cta` / `.oauth` / `.fieldr` / `.pay` / `.hi` / `.ab i` |
| 36 | 54 | 登录头部 margin |
| 38 | 56 | 验证码格、`.bigcoin`、`.mvrow` 缩略图 |
| 40 | 60 | `.fab` |
| 42 | 62 | `.pbn` 海面底部按钮、`.quad.roomy` 格 |
| 44 | 65 | `.thumb`、`.rws` 行、`.chs .ch` 最小宽 |
| 52 | 77 | 支付状态图标环 `.pst .rng` |
| 74 | 110 | 头像上传 `.avup` |
| 120 | 178 | 聊天图片消息宽 |
| 178 | 265 | 用户主页 `.hero` 高 |

**线宽不按比例放大**：原型缩略图里 1px 线是为了在小图上可见，真机上 hairline 仍用 1pt；1.5px 描边保持 1.5pt；只有装饰性粗边（滑卡印章 3px）放大到 4pt。**阴影 blur 与 offset 按 1.488 放大**（`0 5px 16px` → `0 7 24`），透明度不变。**圆角走 `Dim.r*`，不换算**。

---

## Review Focus

规范暗含但没有任何任务的测试直接覆盖、最可能在真机上咬人的五种输入。每条已在括号里指明由哪个任务加测试钉住：

1. **聊天室在 360×780、字体缩放 1.2、键盘弹起（viewInsets.bottom = 300）** 时输入栏必须仍可见且消息列表可滚，不能 overflow。（Task 22 加 `viewInsets` 变体）
2. **折叠屏展开态 720×860 下滑卡 `.swc`**：卡片在限宽 480 内保持 3:4 以上的照片区，不能被拉成横向宽卡。（Task 19 加 wide 变体断言卡片宽 ≤ 480）
3. **夜场 + 系统暗色四种组合**下海面文字、按钮、瓶子发光都要可辨认；暗色下红点白边取 `surface`。（Task 14 golden 四组合）
4. **三键导航（viewPadding.bottom = 48）** 下 `BottomActionBar` 与 Tab 栏不被遮挡、也不重复叠加两份底部 padding。（Task 6 加 `viewPadding` 变体）
5. **充值套餐三列网格在 360 宽 + 缩放 1.2** 时"¥59.9 ~~¥79~~"一行不换行、角标不被裁。（Task 30 加 large-text 变体）

---

## 文件结构

```
app/bottles/
  tool/
    proto_render.py        # 新建：原型某屏按指定逻辑尺寸真实渲染（Edge 无头）
    compare.py             # 新建：真机截图 vs 原型渲染并排 + 100dp 参考线
  lib/core/design/
    tokens.dart            # 补 Shadows、Motion 已有；Breakpoints 已有
    theme.dart             # 已收口；本计划不再动
  lib/ui/widgets/
    design_canvas.dart     # 新建：DesignCanvas / CanvasPositioned（§5）
    buttons.dart           # MiniButton 字号、DockButton 迁入
    cards.dart             # ListRowItem / QuadGrid / NoticeBanner / StatsRow 对齐
    chips.dart             # PillChip / TagChip / SegmentedRow 对齐
    coin.dart              # CoinChip 对齐 .coin
    avatar.dart            # AvatarRing 对齐 .ring
    headers.dart           # GradientHeader / NavBar / BottomActionBar 对齐
    overlays.dart          # SheetShell / ModalCard 对齐
    states.dart            # EmptyState / FailureState / Skeleton 对齐
    sea_scene.dart         # 迁到 DesignCanvas 坐标
    chat_bubbles.dart      # 新建：从 chat_room_page 抽出 .bub/.sys/.gift/.imgmsg
    payment_status.dart    # 新建：.pst 三态共用版式，从 payment_flow_page 抽出
  lib/app/shell.dart       # Tab 栏对齐 .tb.hf；宽 ≥ 600 限宽
  lib/features/**          # 逐页对齐 + 登记注释
  test/layout/
    matrix.dart            # 新建：六档配置 + pumpAt 共享 helper
    *_layout_test.dart     # 每个分节一个文件
  test/widgets/
    *_test.dart            # 组件尺寸断言
  assets/fonts/            # 新建：打包字体
```

拆分原则：`chat_room_page.dart`（662 行）和 `payment_flow_page.dart`（456 行）把可复用的视觉单元抽到 `ui/widgets`，页面只剩编排；其余页面不做无关重构。

---

## Phase 0：比对工具与测试骨架

### Task 1: 原型渲染工具 `tool/proto_render.py`

**Files:**
- Create: `app/bottles/tool/proto_render.py`
- Create: `app/bottles/tool/README.md`

**Interfaces:**
- Produces: 命令 `python tool/proto_render.py --screen A2 --size 393x851 [--hide-apple] [--out DIR]`，输出 `DIR/proto_A2_393x851.png`。`--screen` 取原型 `<div class="cap"><b>…</b><i>ID</i>` 里的 ID（`A2`、`B1`、`H4`…）。后续每个页面任务的 Step 1 都调用它。

- [ ] **Step 1: 写脚本**

```python
"""把原型某一屏抽出来，按目标机型的逻辑尺寸真实渲染。

原型手机框 252px = 375pt（0.672），这里反过来 zoom 1.488，
框做成 W/1.488 x H/1.488 CSS px，zoom 后正好 W x H。
"""
import argparse
import io
import re
import subprocess
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]
SRC = ROOT / 'docs' / 'prototype' / 'v2-screens.html'
EDGE = r'C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe'
ZOOM = 375 / 252


def extract(html: str, screen_id: str) -> str:
    cap = re.search(r'<div class="cap"><b>[^<]*</b><i>' + re.escape(screen_id) + r'(?: ·[^<]*)?</i>', html)
    if not cap:
        raise SystemExit(f'screen {screen_id} not found')
    start = html.index('<div class="ph', cap.end())
    depth, j = 0, start
    tag = re.compile(r'<div\b|</div>')
    while True:
        m = tag.search(html, j)
        depth += -1 if m.group(0) == '</div>' else 1
        j = m.end()
        if depth == 0:
            return html[start:j]


def build(screen_id: str, w: int, h: int, hide_apple: bool) -> str:
    html = io.open(SRC, encoding='utf-8').read()
    style = re.search(r'<style>(.*?)</style>', html, re.S).group(1)
    sprite = re.search(r'(<svg[^>]*style="display:none"[^>]*>.*?</svg>)', html, re.S)
    override = f'''
html,body{{margin:0;padding:0;background:#fff}}
.ph{{width:{w / ZOOM:.2f}px;height:{h / ZOOM:.2f}px;max-width:none;aspect-ratio:auto;zoom:{ZOOM:.4f};
    border:0;border-radius:0;box-shadow:none}}
{'.oauth + .oauth{display:none}' if hide_apple else ''}
'''
    return (f'<!doctype html><html lang="zh"><head><meta charset="utf-8"><style>{style}</style>'
            f'<style>{override}</style></head><body>{sprite.group(1) if sprite else ""}'
            f'{extract(html, screen_id)}</body></html>')


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument('--screen', required=True)
    ap.add_argument('--size', default='393x851')
    ap.add_argument('--hide-apple', action='store_true', help='Android 没有 Apple 登录')
    ap.add_argument('--out', default=str(ROOT / '漂流瓶' / '效果图' / 'proto'))
    a = ap.parse_args()
    w, h = (int(x) for x in a.size.split('x'))
    out = Path(a.out)
    out.mkdir(parents=True, exist_ok=True)
    page = out / f'proto_{a.screen}_{a.size}.html'
    png = out / f'proto_{a.screen}_{a.size}.png'
    io.open(page, 'w', encoding='utf-8').write(build(a.screen, w, h, a.hide_apple))
    subprocess.run([EDGE, '--headless=new', '--disable-gpu', '--hide-scrollbars',
                    f'--window-size={w},{h}', '--virtual-time-budget=3000',
                    f'--screenshot={png}', f'file:///{page}'], check=False, capture_output=True)
    print(png)


if __name__ == '__main__':
    main()
```

- [ ] **Step 2: 写 README**

```markdown
# tool/

## proto_render.py
把原型某一屏按真机逻辑尺寸渲染成 PNG，是所有 UI 比对的基准（规范 §8.1）。

    python tool/proto_render.py --screen B1 --size 393x851
    python tool/proto_render.py --screen A2 --size 360x780 --hide-apple

## compare.py
真机截图与原型渲染并排，每 100dp 一条参考线。

    python tool/compare.py --real 漂流瓶/效果图/xxx.jpg --proto 漂流瓶/效果图/proto/proto_B1_393x851.png --out 漂流瓶/效果图/compare_B1.png
```

- [ ] **Step 3: 跑一次验证（需授权）**

Run: `cd app/bottles && python tool/proto_render.py --screen A2 --size 393x851 --hide-apple`
Expected: 打印 PNG 路径，文件 > 20KB（空白页只有 3KB）。用 Read 看图确认是登录屏。

- [ ] **Step 4: Commit**

```bash
git add app/bottles/tool/proto_render.py app/bottles/tool/README.md
git commit -m "chore(app): add prototype renderer for pixel-fair UI comparison"
```

### Task 2: 并排比对工具 `tool/compare.py`

**Files:**
- Create: `app/bottles/tool/compare.py`

**Interfaces:**
- Produces: 命令 `python tool/compare.py --real <jpg> --proto <png> --out <png> [--label-real "…"] [--label-proto "…"]`。真机截图按 393×851 缩放（默认从 EXIF 尺寸推断 DPR：1080 宽 → 2.75）。

- [ ] **Step 1: 写脚本**

```python
"""真机截图 vs 原型真实 pt 渲染，同尺寸并排，2x 放大，每 100dp 一条参考线。"""
import argparse
from PIL import Image, ImageDraw, ImageFont

SCALE, GAP, HEAD = 2, 24, 40


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument('--real', required=True)
    ap.add_argument('--proto', required=True)
    ap.add_argument('--out', required=True)
    ap.add_argument('--label-real', default='真机')
    ap.add_argument('--label-proto', default='原型真实 pt 渲染')
    a = ap.parse_args()

    proto = Image.open(a.proto).convert('RGB')
    w, h = proto.size  # 原型渲染尺寸即逻辑尺寸
    real = Image.open(a.real).convert('RGB').resize((w * SCALE, h * SCALE), Image.LANCZOS)
    proto = proto.resize((w * SCALE, h * SCALE), Image.LANCZOS)

    try:
        font = ImageFont.truetype('C:/Windows/Fonts/msyh.ttc', 22)
    except OSError:
        font = ImageFont.load_default()

    cv = Image.new('RGB', (w * SCALE * 2 + GAP * 3, h * SCALE + HEAD + GAP), (30, 30, 30))
    d = ImageDraw.Draw(cv)
    d.text((GAP, 8), f'{a.label_real} {w}x{h}', fill=(255, 255, 255), font=font)
    d.text((GAP * 2 + w * SCALE, 8), a.label_proto, fill=(255, 255, 255), font=font)
    cv.paste(real, (GAP, HEAD))
    cv.paste(proto, (GAP * 2 + w * SCALE, HEAD))
    for y in range(0, h, 100):
        yy = HEAD + y * SCALE
        d.line([(0, yy), (cv.width, yy)], fill=(255, 80, 80), width=1)
        d.text((4, yy + 2), str(y), fill=(255, 80, 80), font=font)
    cv.save(a.out)
    print(a.out)


if __name__ == '__main__':
    main()
```

- [ ] **Step 2: 跑一次验证（需授权）**

Run: `cd app/bottles && python tool/compare.py --real "../../漂流瓶/效果图/582e52bac98d3d52719573d89a5b3491.jpg" --proto "../../漂流瓶/效果图/proto/proto_A2_393x851.png" --out "../../漂流瓶/效果图/compare_A2_tool.png"`
Expected: 输出并排图，Read 确认两栏对齐、参考线可见。

- [ ] **Step 3: Commit**

```bash
git add app/bottles/tool/compare.py
git commit -m "chore(app): add side-by-side comparison tool"
```

### Task 3: 布局矩阵共享 helper `test/layout/matrix.dart`

**Files:**
- Create: `app/bottles/test/layout/matrix.dart`
- Modify: `app/bottles/test/layout/auth_layout_test.dart`（改用 helper）

**Interfaces:**
- Produces:
  - `const List<LayoutCase> layoutMatrix`（六档，规范 §8.2）
  - `class LayoutCase { final String name; final Size size; final double textScale; final EdgeInsets viewPadding; final EdgeInsets viewInsets; }`
  - `Future<void> pumpAt(WidgetTester tester, LayoutCase cfg, Widget page, {List<Override> overrides = const []})`：设窗口、缩放、安全区、键盘，pump 后断言 `takeException()` 为 null。
  - `LayoutCase LayoutCase.withInsets(LayoutCase base, {EdgeInsets? viewPadding, EdgeInsets? viewInsets, String? suffix})`：派生三键导航 / 键盘变体。

- [ ] **Step 1: 写 helper**

```dart
import 'package:bottles/core/design/theme.dart';
import 'package:bottles/core/providers.dart';
import 'package:bottles/core/storage/prefs.dart';
import 'package:bottles/l10n/app_localizations.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

/// 规范 §8.2 的机型矩阵。Ahem 测试字体每个字都是正方形、比真实字体宽，
/// 这里能过的真机上一定能过。
class LayoutCase {
  const LayoutCase(
    this.name,
    this.size, {
    this.textScale = 1.0,
    this.viewPadding = EdgeInsets.zero,
    this.viewInsets = EdgeInsets.zero,
  });

  final String name;
  final Size size;
  final double textScale;
  final EdgeInsets viewPadding;
  final EdgeInsets viewInsets;

  LayoutCase withInsets({EdgeInsets? viewPadding, EdgeInsets? viewInsets, String suffix = ''}) =>
      LayoutCase('$name$suffix', size,
          textScale: textScale,
          viewPadding: viewPadding ?? this.viewPadding,
          viewInsets: viewInsets ?? this.viewInsets);
}

const layoutMatrix = <LayoutCase>[
  LayoutCase('narrow 360x780 (Redmi / Samsung)', Size(360, 780)),
  LayoutCase('short 375x667 (iPhone SE)', Size(375, 667)),
  LayoutCase('base 393x851 (Mi 9 / iPhone 15)', Size(393, 851)),
  LayoutCase('tall 430x932 (iPhone Pro Max)', Size(430, 932)),
  LayoutCase('wide 720x860 (fold opened)', Size(720, 860)),
  LayoutCase('large-text 360x780 @1.2', Size(360, 780), textScale: 1.2),
];

/// 三键导航：底部多吃 48dp。
const threeButtonNav = EdgeInsets.only(top: 34, bottom: 48);

/// 印度低端机键盘能占到屏高 45%。
const keyboard300 = EdgeInsets.only(bottom: 300);

Future<void> pumpAt(
  WidgetTester tester,
  LayoutCase cfg,
  Widget page, {
  List<Override> overrides = const [],
}) async {
  SharedPreferences.setMockInitialValues({});
  final prefs = await Prefs.load();

  tester.view.devicePixelRatio = 1.0;
  tester.view.physicalSize = cfg.size;
  tester.view.viewPadding = _fake(cfg.viewPadding);
  tester.view.padding = _fake(cfg.viewPadding);
  tester.view.viewInsets = _fake(cfg.viewInsets);
  tester.platformDispatcher.textScaleFactorTestValue = cfg.textScale;
  addTearDown(() {
    tester.view.reset();
    tester.platformDispatcher.clearTextScaleFactorTestValue();
  });

  await tester.pumpWidget(
    ProviderScope(
      overrides: [prefsProvider.overrideWithValue(prefs), ...overrides],
      child: MaterialApp(
        theme: AppTheme.light(),
        darkTheme: AppTheme.dark(),
        localizationsDelegates: L.localizationsDelegates,
        supportedLocales: L.supportedLocales,
        home: page,
      ),
    ),
  );
  await tester.pump();
  expect(tester.takeException(), isNull, reason: cfg.name);
}

FakeViewPadding _fake(EdgeInsets e) =>
    FakeViewPadding(left: e.left, top: e.top, right: e.right, bottom: e.bottom);
```

- [ ] **Step 2: 改 `auth_layout_test.dart` 用 helper**

把文件里的 `_matrix`、`_harness`、`_pumpAt` 删掉，改成：

```dart
import 'package:bottles/features/auth/auth_controller.dart';
import 'package:bottles/features/auth/credential_flow_page.dart';
import 'package:bottles/features/auth/login_page.dart';
import 'package:bottles/features/auth/otp_page.dart';
import 'package:flutter_test/flutter_test.dart';

import 'matrix.dart';

void main() {
  for (final cfg in layoutMatrix) {
    testWidgets('login fits: ${cfg.name}', (t) => pumpAt(t, cfg, const LoginPage()));
    testWidgets('login fits with keyboard: ${cfg.name}',
        (t) => pumpAt(t, cfg.withInsets(viewInsets: keyboard300, suffix: ' +kb'), const LoginPage()));
    testWidgets('register fits: ${cfg.name}',
        (t) => pumpAt(t, cfg, const CredentialFlowPage(flow: AuthFlow.register)));
    testWidgets('otp fits: ${cfg.name}', (t) => pumpAt(t, cfg, const OtpPage()));
  }
}
```

- [ ] **Step 3: 跑测试（需授权）**

Run: `cd app/bottles && flutter test test/layout/auth_layout_test.dart`
Expected: 全部 PASS。若 `tester.view.viewPadding` 赋值报类型错误，用 `FakeViewPadding` 已在 helper 里处理；若 Flutter 版本不支持 `tester.view.reset()`，改成分别调 `resetPhysicalSize()`、`resetDevicePixelRatio()`、`resetViewPadding()`、`resetViewInsets()`、`resetPadding()`。

- [ ] **Step 4: Commit**

```bash
git add app/bottles/test/layout/
git commit -m "test(app): share the device matrix helper across layout tests"
```

---

## Phase 1：共享组件对齐原型

每个任务的格式：**现值 → 目标值**表来自原型 CSS 换算；测试用 `tester.getSize` 钉住关键尺寸。组件先于页面，因为一处改动全 App 见效，之后页面任务只剩编排。

### Task 4: 阴影与渐变进 tokens

**Files:**
- Modify: `app/bottles/lib/core/design/tokens.dart`（在 `Motion` 之后加 `Shadows`）
- Test: `app/bottles/test/widgets/tokens_test.dart`

**Interfaces:**
- Produces: `class Shadows { static List<BoxShadow> card(AppColors c); static List<BoxShadow> floating(AppColors c); static List<BoxShadow> sheet(); static List<BoxShadow> modal(); static List<BoxShadow> glow(Color color, {double alpha}); }`，后续组件任务只引用这里，不再各写一份 `BoxShadow`。

- [ ] **Step 1: 写测试**

```dart
import 'package:bottles/core/design/tokens.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('shadows follow the prototype blur scaled by 1.488', () {
    // .swc: 0 10px 26px .18 → 0 15 39
    expect(Shadows.floating(AppColors.light).single.blurRadius, closeTo(39, 1));
    expect(Shadows.floating(AppColors.light).single.offset.dy, closeTo(15, 1));
    // .sheet: 0 -8px 28px .2 → 0 -12 42
    expect(Shadows.sheet().single.offset.dy, closeTo(-12, 1));
    // .cta glow: 0 5px 16px → 0 7 24
    final g = Shadows.glow(Colors.blue).single;
    expect(g.blurRadius, closeTo(24, 1));
    expect(g.offset.dy, closeTo(7, 1));
  });
}
```

- [ ] **Step 2: 实现**

```dart
/// 阴影 —— 原型 CSS 的 blur / offset 按 1.488 放大，透明度不变（计划「换算表」）。
/// 组件只引用这里，不各写一份 BoxShadow。
class Shadows {
  const Shadows._();

  /// `.mcd` `.lst` 这类静态卡片：`--shadow` = 0 1px 2px .05 + 0 8px 24px .07。
  static List<BoxShadow> card(AppColors c) => [
        BoxShadow(color: c.ink.withValues(alpha: .05), blurRadius: 3, offset: const Offset(0, 1)),
        BoxShadow(color: c.ink.withValues(alpha: .07), blurRadius: 36, offset: const Offset(0, 12)),
      ];

  /// 悬浮块（滑卡 `.swc`、钱包 `.wallet`、`.pcard`）：0 10px 26px .18。
  static List<BoxShadow> floating(AppColors c) => [
        BoxShadow(color: c.ink.withValues(alpha: .18), blurRadius: 39, offset: const Offset(0, 15)),
      ];

  /// 底部弹层 `.sheet`：0 -8px 28px .2。
  static List<BoxShadow> sheet() => const [
        BoxShadow(color: Color(0x33000000), blurRadius: 42, offset: Offset(0, -12)),
      ];

  /// 居中模态 `.modal`：0 18px 44px .36。
  static List<BoxShadow> modal() => const [
        BoxShadow(color: Color(0x5C000000), blurRadius: 65, offset: Offset(0, 27)),
      ];

  /// 实心按钮同色投影 `.cta`：0 5px 16px，alpha 由调用方按语义给（海蓝 .38 / 粉 .40 / 警示 .34）。
  static List<BoxShadow> glow(Color color, {double alpha = .38}) => [
        BoxShadow(color: color.withValues(alpha: alpha), blurRadius: 24, offset: const Offset(0, 7)),
      ];

  /// 小按钮 `.jn` / `.hi`：0 3px 10px。
  static List<BoxShadow> glowSmall(Color color, {double alpha = .34}) => [
        BoxShadow(color: color.withValues(alpha: alpha), blurRadius: 15, offset: const Offset(0, 4)),
      ];
}
```

- [ ] **Step 3: 跑测试（需授权）** Run: `flutter test test/widgets/tokens_test.dart` Expected: PASS
- [ ] **Step 4: Commit** `git commit -m "feat(app): centralize prototype shadows in tokens"`

### Task 5: 按钮 `buttons.dart`

**Files:**
- Modify: `app/bottles/lib/ui/widgets/buttons.dart`
- Test: `app/bottles/test/widgets/buttons_test.dart`

| 组件 | 原型 | 现值 | 目标 |
|---|---|---|---|
| `AppButton` | `.cta` 34px / t4 800 / 0 5 16 | 高 50、blur 22 offset 6 | 高 50 ✓、阴影改 `Shadows.glow` |
| `MiniButton` | `.jn` 30px / t4 800 / 0 3 10 | 高 44、**字 t2** | 字 **t4**、阴影 `Shadows.glowSmall`、`.jn.gh` 变体 = surface + line 描边无阴影 |
| `RoundIconButton` | `.ab i` 34px 圆 / t6 图标 / page 底 line 描边 | 30 圆 | 圆 **50**，图标 24，`liked` 变体 = brand 底白图标 + glowSmall |
| 新增 `HiButton` | `.hi` 50 高 pill brand t4 + 币 | 无 | 新建，用于 C1 滑卡「打招呼」 |

- [ ] **Step 1: 写测试**

```dart
import 'package:bottles/core/design/theme.dart';
import 'package:bottles/ui/widgets/buttons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

Widget _wrap(Widget w) => MaterialApp(theme: AppTheme.light(), home: Scaffold(body: Center(child: w)));

void main() {
  testWidgets('MiniButton is 44 tall and uses the 17pt label', (t) async {
    await t.pumpWidget(_wrap(const MiniButton(label: '充值')));
    expect(t.getSize(find.byType(MiniButton)).height, 44);
    final text = t.widget<Text>(find.text('充值'));
    expect(text.style!.fontSize, 17);
  });

  testWidgets('RoundIconButton is a 50pt circle', (t) async {
    await t.pumpWidget(_wrap(const RoundIconButton(icon: Icons.favorite)));
    expect(t.getSize(find.byType(RoundIconButton)), const Size(50, 50));
  });

  testWidgets('AppButton keeps 50pt height at text scale 1.2', (t) async {
    t.platformDispatcher.textScaleFactorTestValue = 1.2;
    addTearDown(t.platformDispatcher.clearTextScaleFactorTestValue);
    await t.pumpWidget(_wrap(const AppButton(label: '登录')));
    expect(t.getSize(find.byType(AppButton)).height, 50);
  });
}
```

- [ ] **Step 2: 实现**

`MiniButton` 的 `Text` 字号改 `Dim.t4`，`glow` 改 `Shadows.glowSmall(c.aqua)` / `Shadows.glowSmall(c.brand, alpha: .38)`；`AppButton` 的 `boxShadow` 改 `Shadows.glow(...)`（海蓝 .38、粉 .40、警示 .34）。`RoundIconButton` 外框 `Dim.tap` 改为 50 并加 `liked` 参数：

```dart
class RoundIconButton extends StatelessWidget {
  const RoundIconButton({
    super.key,
    required this.icon,
    this.onTap,
    this.color,
    this.background,
    this.size = 24,
    this.liked = false,
    this.tooltip,
  });
  // …
  // 原型 .ab i：50pt 圆，page 底 + line 描边；.ab.lk：brand 底白图标 + 小投影。
  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final bg = liked ? c.brand : (background ?? c.page);
    final fg = liked ? Colors.white : (color ?? c.ink2);
    final btn = InkResponse(
      onTap: onTap,
      radius: 28,
      child: Container(
        width: 50,
        height: 50,
        decoration: BoxDecoration(
          color: bg,
          shape: BoxShape.circle,
          border: liked ? null : Border.all(color: c.line),
          boxShadow: liked ? Shadows.glowSmall(c.brand, alpha: .4) : null,
        ),
        alignment: Alignment.center,
        child: Icon(icon, size: size, color: fg),
      ),
    );
    return tooltip == null ? btn : Tooltip(message: tooltip!, child: btn);
  }
}
```

新增 `HiButton`：

```dart
/// 滑卡底部「打招呼」（原型 .hi）：占满剩余宽、50 高、粉底、t4/800，价格跟在文案后。
class HiButton extends StatelessWidget {
  const HiButton({super.key, required this.label, required this.coins, this.onTap});
  final String label;
  final int coins;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) =>
      AppButton(label: label, coins: coins, kind: BtnKind.pay, onTap: onTap, height: 50);
}
```

- [ ] **Step 3: 全局搜 `RoundIconButton(` 调用处**，确认没有依赖 30pt 视觉尺寸的布局（`grep -rn "RoundIconButton" lib/features`），有的话把它换成 `HeaderIconButton` 或调 `size`。
- [ ] **Step 4: 跑测试（需授权）** Run: `flutter test test/widgets/buttons_test.dart` Expected: PASS
- [ ] **Step 5: Commit** `git commit -m "feat(app): align buttons with prototype .jn/.ab/.hi metrics"`

### Task 6: 头图、导航栏、底部操作栏 `headers.dart`

**Files:**
- Modify: `app/bottles/lib/ui/widgets/headers.dart`
- Test: `app/bottles/test/widgets/headers_test.dart`

| 组件 | 原型 | 现值 | 目标 |
|---|---|---|---|
| `GradientHeader` | `.hdr` padding 7px/s4/s5，`.slim` 底 s3，`.ttl` t5，`h3` t6 lh1.25 mt s4，`.sc` t2 白 .85 mt s1，`.ic` 23px→34 圆白底 | topRow 高 44、圆钮 30 | topRow 保持 44；`HeaderIconButton` 圆 **34**、图标 20、白底 .94 + 红字 `#C43158`；副标透明度 .85 |
| `GradientHeader.stats` | `.hdr .stats` 白 .16 底 r3、数字 t4 800、标签 t0 .85、竖线白 .22 | 无 | 新增 `HeaderStats(items: [(value, label)])` |
| `NavBar` | `.nv` 30px→44 t4 700 底线 line2 | 48 高（tap+4） | 高 **44**；返回图标 20；标题 t4 700 |
| `BottomActionBar` | `.foot` padding s3 s4 **s4** + 顶线 line2 + 内部 gap s2 | padding 底 s3 | 底 padding = `max(viewPadding.bottom, Dim.s4)`；支持 `children` 列表纵向 gap s2 |

- [ ] **Step 1: 写测试**

```dart
import 'package:bottles/core/design/theme.dart';
import 'package:bottles/ui/widgets/buttons.dart';
import 'package:bottles/ui/widgets/headers.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

Widget _wrap(Widget w, {EdgeInsets padding = EdgeInsets.zero}) => MaterialApp(
      theme: AppTheme.light(),
      home: MediaQuery(
        data: MediaQueryData(viewPadding: padding, padding: padding),
        child: Scaffold(body: Column(children: [const Spacer(), w])),
      ),
    );

void main() {
  testWidgets('NavBar is 44 tall', (t) async {
    await t.pumpWidget(MaterialApp(theme: AppTheme.light(), home: const Scaffold(appBar: NavBar(title: 'x'))));
    expect(t.getSize(find.byType(NavBar)).height, 44);
  });

  testWidgets('BottomActionBar keeps 16 below the button on gesture nav', (t) async {
    await t.pumpWidget(_wrap(const BottomActionBar(child: AppButton(label: 'go'))));
    final bar = t.getRect(find.byType(BottomActionBar));
    final btn = t.getRect(find.byType(AppButton));
    expect(bar.bottom - btn.bottom, 16);
  });

  testWidgets('BottomActionBar absorbs a 48pt three-button nav without doubling', (t) async {
    await t.pumpWidget(_wrap(const BottomActionBar(child: AppButton(label: 'go')),
        padding: const EdgeInsets.only(bottom: 48)));
    final bar = t.getRect(find.byType(BottomActionBar));
    final btn = t.getRect(find.byType(AppButton));
    expect(bar.bottom - btn.bottom, 48);
  });
}
```

- [ ] **Step 2: 实现 `BottomActionBar`**

```dart
/// 底部固定操作栏（原型 .foot）：顶线 line2，padding s3 s4，底部 = max(安全区, s4)。
/// 调用方**不要**再包 SafeArea，否则三键导航机型会叠加两份底部留白（规范 §4.4）。
class BottomActionBar extends StatelessWidget {
  const BottomActionBar({super.key, this.child, this.children = const []})
      : assert(child != null || children.length > 0);

  final Widget? child;
  final List<Widget> children;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final bottom = MediaQuery.viewPaddingOf(context).bottom;
    final items = child != null ? [child!] : children;
    return Container(
      decoration: BoxDecoration(
        color: c.page,
        border: Border(top: BorderSide(color: c.line2)),
      ),
      padding: EdgeInsets.fromLTRB(Dim.gutter, Dim.s3, Dim.gutter, bottom > Dim.s4 ? bottom : Dim.s4),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          for (var i = 0; i < items.length; i++) ...[
            if (i > 0) const SizedBox(height: Dim.s2),
            items[i],
          ],
        ],
      ),
    );
  }
}
```

然后 `grep -rn "BottomActionBar" lib/features` 检查每个调用处外层是否有 `SafeArea(bottom: true)` 或 `SafeArea(child: Column(... BottomActionBar))`，把包住它的 SafeArea 改成 `SafeArea(bottom: false)`。

- [ ] **Step 3: 实现 `NavBar` 高 44、`HeaderIconButton` 34、`HeaderStats`**

`preferredSize` 改 `Size.fromHeight(Dim.tap)`；`HeaderIconButton` 的 30 改 34、图标 17 改 20、颜色 `Colors.white.withValues(alpha: .94)` 底 + `Color(0xFFC43158)` 前景。

```dart
/// 头图内的三格统计（原型 .hdr .stats）：白 .16 底、r3、数字 t4/800、标签 t0。
class HeaderStats extends StatelessWidget {
  const HeaderStats({super.key, required this.items});
  final List<(String value, String label)> items;

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.only(top: Dim.s3),
      clipBehavior: Clip.antiAlias,
      decoration: BoxDecoration(
        color: Colors.white.withValues(alpha: .16),
        borderRadius: Dim.brCard,
      ),
      child: Row(
        children: [
          for (var i = 0; i < items.length; i++)
            Expanded(
              child: Container(
                padding: const EdgeInsets.symmetric(vertical: Dim.s2),
                decoration: i == 0
                    ? null
                    : BoxDecoration(
                        border: Border(left: BorderSide(color: Colors.white.withValues(alpha: .22))),
                      ),
                child: Column(
                  children: [
                    Text(items[i].$1,
                        style: const TextStyle(
                            fontSize: Dim.t4, fontWeight: FontWeight.w800, color: Colors.white, height: 1.25,
                            fontFeatures: [FontFeature.tabularFigures()])),
                    Text(items[i].$2,
                        style: TextStyle(fontSize: Dim.t0, color: Colors.white.withValues(alpha: .85), height: 1.4)),
                  ],
                ),
              ),
            ),
        ],
      ),
    );
  }
}
```

- [ ] **Step 4: 跑测试（需授权）** Run: `flutter test test/widgets/headers_test.dart` Expected: PASS
- [ ] **Step 5: Commit** `git commit -m "feat(app): align nav bar, header and bottom action bar with prototype"`

### Task 7: Tab 栏 `shell.dart`

**Files:**
- Modify: `app/bottles/lib/app/shell.dart:80-170`
- Test: `app/bottles/test/widgets/tab_bar_test.dart`

| 项 | 原型 `.tb.hf` | 现值 | 目标 |
|---|---|---|---|
| 图标 | 18px → **27** | 22 | 27，选中描边粗（用 `Icons.*_rounded` 填充版代替 outlined） |
| 标签 | t1 700 ink3 | t0 | **t1**，选中 brand |
| 上下 padding | 7px → 10 | 5 | 10；总高 = 10 + 27 + 4.5 + 17 + 10 ≈ 68 |
| 红点 | 14px → 21 最小宽，白边 1.5 surface，右上 | `UnreadBadge` | 沿用，确认边色取 `c.surface` |
| 底部 | `env(safe-area-inset-bottom)` | SafeArea ✓ | ✓ |
| 宽 ≥ 600 | 限宽 480 居中（规范 §4.2） | 无 | Shell body 包 `Center(child: ConstrainedBox(maxWidth: 480))`，Tab 栏保持全宽 |

- [ ] **Step 1: 写测试**

```dart
import 'package:bottles/app/shell.dart';
// AppShell 依赖 go_router 的 StatefulNavigationShell，这里只测抽出来的 AppTabBar。
import 'package:bottles/core/design/theme.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:bottles/l10n/app_localizations.dart';

void main() {
  testWidgets('tab bar icons are 27pt and labels 12pt', (t) async {
    await t.pumpWidget(ProviderScope(
      child: MaterialApp(
        theme: AppTheme.light(),
        localizationsDelegates: L.localizationsDelegates,
        supportedLocales: L.supportedLocales,
        home: Scaffold(bottomNavigationBar: AppTabBar(index: 0, onTap: (_) {})),
      ),
    ));
    final icon = t.widget<Icon>(find.byType(Icon).first);
    expect(icon.size, 27);
    final label = t.widget<Text>(find.byType(Text).first);
    expect(label.style!.fontSize, 12);
  });
}
```

- [ ] **Step 2: 实现**：把 `_TabBar` 改名公开为 `AppTabBar`（测试要引用）；`size: 22` → 27；`fontSize: Dim.t0` → `Dim.t1`；`padding` 改 `EdgeInsets.symmetric(horizontal: 6, vertical: 10)`；`SizedBox(height: 58)` 去掉固定高，改由内容撑（`Column(mainAxisSize: min)`）。`AppShell.build` 的 `body` 改：

```dart
      body: Breakpoints.isWide(context)
          ? Center(
              child: ConstrainedBox(
                constraints: const BoxConstraints(maxWidth: Breakpoints.contentMaxWidth),
                child: navigationShell,
              ),
            )
          : navigationShell,
```

- [ ] **Step 3: 跑测试（需授权）** Run: `flutter test test/widgets/tab_bar_test.dart` Expected: PASS
- [ ] **Step 4: Commit** `git commit -m "feat(app): align tab bar with prototype and cap content width on wide screens"`

### Task 8: 列表、卡片、四宫格 `cards.dart`

**Files:**
- Modify: `app/bottles/lib/ui/widgets/cards.dart`
- Test: `app/bottles/test/widgets/cards_test.dart`

| 组件 | 原型 | 现值 | 目标 |
|---|---|---|---|
| `ListGroup` | `.lst` 描边 line、r4、surface | 检查 | r4（20）、描边 `c.line` |
| `ListRowItem` | `.lst a` 最小高 30px→**44**、padding 0 s4、分割线左缩进 34px→**50**、图标 15px→22、标题 t3 600、右文 t2 ink3 | 图标框 24、chevron 18 | 最小高 44、`roomy` 变体 52（`.lst.roomy` 35px）、分割线 `indent: 50`、图标 22、标题 `FontWeight.w600` |
| 新增 `QuadGrid` | `.quad` 4 列 gap s2；格 32px→**48** r3 sea；标签 t1 600 ink2。`.roomy`：格 42px→**62** r4 surface + 描边 line2 + 阴影 card，标签 t2 | `me_page` 里手写 | 抽成组件 `QuadGrid(items: [(icon, label, onTap)], roomy: bool)` |
| `NoticeBanner` | `.warn` gap s2、padding s3、r3、底 coin .14、描边 gold .3、t1 lh1.6 ink2、图标 19 gold | emoji ⚠️ | 图标改 `Icons.warning_amber_rounded` 19pt `c.gold`；禁 emoji |
| `StatsRow` | `.stats` 描边 line r4；数字 t5 800；标签 t1 ink3 mt 1；竖线 line2 | ✓ 大致 | 核对 padding s3 0 |
| `TimelineList` | `.tl` 行最小高 26px→**39**、点 6px→**9** brand、竖线 1.5 mist .55、时间 t1 | 点 7、线 1 | 点 9、线 1.5、行 39；加命名构造 `TimelineList.onDark(stops:)`（B4s 海报用：点白 + 白 .2 外圈 3.5、线白 .38、文字白 700、时间白 .58） |

- [ ] **Step 1: 写测试**

```dart
import 'package:bottles/core/design/theme.dart';
import 'package:bottles/ui/widgets/cards.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

Widget _wrap(Widget w) => MaterialApp(theme: AppTheme.light(), home: Scaffold(body: w));

void main() {
  testWidgets('ListRowItem is at least 44 tall, 52 when roomy', (t) async {
    await t.pumpWidget(_wrap(const ListGroup(children: [
      ListRowItem(title: 'a'),
      ListRowItem(title: 'b', roomy: true),
    ])));
    final rows = find.byType(ListRowItem);
    expect(t.getSize(rows.at(0)).height, greaterThanOrEqualTo(44));
    expect(t.getSize(rows.at(1)).height, greaterThanOrEqualTo(52));
  });

  testWidgets('QuadGrid lays 4 tiles of 48pt, 62 when roomy', (t) async {
    await t.pumpWidget(_wrap(QuadGrid(items: [
      for (var i = 0; i < 4; i++) QuadItem(icon: Icons.star, label: '$i', onTap: () {}),
    ])));
    expect(find.byType(QuadTile), findsNWidgets(4));
    expect(t.getSize(find.byKey(const ValueKey('quad-icon-0'))).height, 48);
  });

  testWidgets('NoticeBanner uses a vector icon, never emoji', (t) async {
    await t.pumpWidget(_wrap(const NoticeBanner(text: 'x')));
    expect(find.byType(Icon), findsOneWidget);
    expect(find.text('⚠️'), findsNothing);
  });
}
```

- [ ] **Step 2: 实现 `QuadGrid`**

```dart
class QuadItem {
  const QuadItem({required this.icon, required this.label, required this.onTap, this.badge});
  final IconData icon;
  final String label;
  final VoidCallback onTap;
  final int? badge;
}

/// 四宫格入口（原型 .quad）。roomy = v2「我的」页的高级版：62pt 格、surface 底、投影。
class QuadGrid extends StatelessWidget {
  const QuadGrid({super.key, required this.items, this.roomy = false});
  final List<QuadItem> items;
  final bool roomy;

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        for (var i = 0; i < items.length; i++) ...[
          if (i > 0) SizedBox(width: roomy ? Dim.s3 : Dim.s2),
          Expanded(child: QuadTile(item: items[i], roomy: roomy, index: i)),
        ],
      ],
    );
  }
}

class QuadTile extends StatelessWidget {
  const QuadTile({super.key, required this.item, required this.roomy, required this.index});
  final QuadItem item;
  final bool roomy;
  final int index;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final size = roomy ? 62.0 : 48.0;
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: item.onTap,
      child: Column(
        children: [
          Container(
            key: ValueKey('quad-icon-$index'),
            width: size,
            height: size,
            decoration: BoxDecoration(
              color: roomy ? c.surface : c.sea,
              borderRadius: roomy ? Dim.brLargeCard : Dim.brCard,
              border: roomy ? Border.all(color: c.line2) : null,
              boxShadow: roomy ? Shadows.card(c) : null,
            ),
            child: Icon(item.icon, size: roomy ? 30 : 22, color: c.ink2),
          ),
          const SizedBox(height: Dim.s1),
          Text(item.label,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(
                  fontSize: roomy ? Dim.t2 : Dim.t1, fontWeight: FontWeight.w600, color: c.ink2, height: 1.4)),
        ],
      ),
    );
  }
}
```

- [ ] **Step 3: `ListRowItem` 加 `roomy` 参数**，最小高 `roomy ? 52 : Dim.tap`，图标 22，标题 `w600`，分割线 `Divider(indent: 50)`；`NoticeBanner` 的 emoji 参数改为 `IconData icon = Icons.warning_amber_rounded`，全局 `grep -rn "NoticeBanner(" lib` 把传 `emoji:` 的调用改掉；`TimelineList` 点 9 线 1.5 行高 39。
- [ ] **Step 4: 跑测试（需授权）** Run: `flutter test test/widgets/cards_test.dart` Expected: PASS
- [ ] **Step 5: Commit** `git commit -m "feat(app): align list rows, quad grid and banners with prototype"`

### Task 9: 标签与分段 `chips.dart`、金币 `coin.dart`、头像 `avatar.dart`

**Files:**
- Modify: `app/bottles/lib/ui/widgets/chips.dart`、`coin.dart`、`avatar.dart`
- Test: `app/bottles/test/widgets/chips_test.dart`

| 组件 | 原型 | 现值 | 目标 |
|---|---|---|---|
| `PillChip` | `.chs .ch` 最小宽 44px→**65**、padding 4px/s4→ **6 / 24**、t3 700、描边 1.5 line、选中 brand2 底 + `0 3 9` 阴影 | 高 34 | 高 **36**（6+24+6）、最小宽 65、阴影 `Shadows.glowSmall(c.brand2, alpha: .36)`；`pk` / `aq` 色变体保留 |
| `TagChip` | `.tg` padding s1 s2、r1、t1 700、sea 底；pk 粉 .14 / pu 紫 .15 | 图标 11 | ✓ 核对，图标 13 |
| `SegmentedRow` | `.seg` 30px→44、page 底、line2 描边、选中铺满 | 高 36 | 高 **44**、与登录页 `ChannelTabs` 合并：`ChannelTabs` 改为调 `SegmentedRow` |
| `CoinChip` | `.coin` t3 800 tabular、padding 4/8/4/4、白 .94 底、`0 1 4 .16` 阴影 | 高 26 | 高 **30**、图标 17 ✓ |
| `AvatarRing` | `.ring` 44 / sm 30；环 conic 金；内圈 inset 2.5px→**4**；surface 边 1.5；在线点 10px→**15**、边 2px→3；sm 点 8→12 边 1.5→2 | 在线点 10 边 1.5 | 点 15 / 12，边 3 / 2 |

- [ ] **Step 1: 写测试**

```dart
import 'package:bottles/core/design/theme.dart';
import 'package:bottles/ui/widgets/avatar.dart';
import 'package:bottles/ui/widgets/chips.dart';
import 'package:bottles/ui/widgets/coin.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

Widget _wrap(Widget w) => MaterialApp(theme: AppTheme.light(), home: Scaffold(body: Center(child: w)));

void main() {
  testWidgets('PillChip is 36 tall and at least 65 wide', (t) async {
    await t.pumpWidget(_wrap(const PillChip(label: '全部')));
    final s = t.getSize(find.byType(PillChip));
    expect(s.height, 36);
    expect(s.width, greaterThanOrEqualTo(65));
  });

  testWidgets('SegmentedRow is 44 tall', (t) async {
    await t.pumpWidget(_wrap(SegmentedRow(labels: const ['a', 'b'], index: 0, onChanged: (_) {})));
    expect(t.getSize(find.byType(SegmentedRow)).height, 44);
  });

  testWidgets('CoinChip is 30 tall', (t) async {
    await t.pumpWidget(_wrap(const CoinChip(coins: 120)));
    expect(t.getSize(find.byType(CoinChip)).height, 30);
  });

  testWidgets('AvatarRing online dot is 15pt at size 44', (t) async {
    await t.pumpWidget(_wrap(const AvatarRing(online: true)));
    expect(t.getSize(find.byKey(const ValueKey('avatar-online-dot'))).width, 15);
  });
}
```

- [ ] **Step 2: 实现**：按表改数值；`AvatarRing` 在线点 `Container` 加 `key: const ValueKey('avatar-online-dot')`，尺寸 `size >= 40 ? 15 : 12`，边 `size >= 40 ? 3 : 2`；`SegmentedRow` 高 44、底 `c.page`、描边 `c.line2`。`login_page.dart` 的 `ChannelTabs.build` 改为：

```dart
    return SegmentedRow(
      labels: [l.authTabPhone, l.authTabEmail],
      index: current == OtpChannel.phone ? 0 : 1,
      onChanged: (i) => onSelect(i == 0 ? OtpChannel.phone : OtpChannel.email),
    );
```

- [ ] **Step 3: 跑测试（需授权）** Run: `flutter test test/widgets/chips_test.dart test/layout/auth_layout_test.dart` Expected: PASS
- [ ] **Step 4: Commit** `git commit -m "feat(app): align chips, coin chip and avatar ring with prototype"`

### Task 10: 弹层与模态 `overlays.dart`

**Files:**
- Modify: `app/bottles/lib/ui/widgets/overlays.dart`
- Test: `app/bottles/test/widgets/overlays_test.dart`

| 组件 | 原型 | 现值 | 目标 |
|---|---|---|---|
| `SheetShell` | `.sheet` padding s2 s4 s4、r5 顶圆角、阴影 `0 -8 28 .2`、拖柄 30px×3→**45×4.5** 下距 s3；最大高 = 屏高 × 0.85（规范 §6） | 拖柄 44×4 blur 28 | 拖柄 45×4.5；阴影 `Shadows.sheet()`；`maxHeight: MediaQuery.sizeOf(context).height * .85`；底 padding `max(viewPadding.bottom, s4)`；内容超出即滚（`SingleChildScrollView`） |
| `ModalCard` | `.modal` 左右 s4、r5、阴影 `0 18 44 .36`；✕ 20px→**30** 圆 ink .09 底 右上 s3；head padding s5 s4 s3 居中 + 金色渐变顶；图标 30px→**45**（svg 54→80）gold；h4 t5；p t2 ink2 lh1.65；body padding 0 s4 s4 gap s2 | 图标 44 mist、✕ 20 | 图标 45 `c.gold`；✕ 30 圆；head 金色渐变 `LinearGradient(top→bottom, coin .26 → coin 0)`；阴影 `Shadows.modal()`；进场 scale .92→1 + opacity `Motion.modal` |
| 新增 `showAppSheet` | 上滑呼出、下拉 40% 或点遮罩关；进 `Motion.sheetIn` 出 `Motion.sheetOut` | 各处直接 `showModalBottomSheet` | 统一入口 `Future<T?> showAppSheet<T>(BuildContext, {required WidgetBuilder builder})`，用 `showModalBottomSheet(isScrollControlled: true, useSafeArea: false, backgroundColor: transparent)` + `SheetShell` |
| 新增 `showAppModal` | 被动弹出不从底部滑入 | 各处 `showDialog` | `Future<T?> showAppModal<T>(BuildContext, {required Widget child})`，`showGeneralDialog` 配 `ScaleTransition(.92→1)` + `FadeTransition`，时长 `Motion.modal`，曲线 `Motion.spring` |

- [ ] **Step 1: 写测试**

```dart
import 'package:bottles/core/design/theme.dart';
import 'package:bottles/ui/widgets/overlays.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('SheetShell caps its height at 85% of the screen and scrolls', (t) async {
    t.view.physicalSize = const Size(393, 851);
    t.view.devicePixelRatio = 1;
    addTearDown(t.view.reset);
    await t.pumpWidget(MaterialApp(
      theme: AppTheme.light(),
      home: Scaffold(body: Builder(builder: (ctx) {
        return TextButton(
          onPressed: () => showAppSheet<void>(ctx, builder: (_) => const SizedBox(height: 2000)),
          child: const Text('open'),
        );
      })),
    ));
    await t.tap(find.text('open'));
    await t.pumpAndSettle();
    expect(t.getSize(find.byType(SheetShell)).height, lessThanOrEqualTo(851 * .85 + 1));
    expect(t.takeException(), isNull);
  });

  testWidgets('ModalCard close button is a 30pt circle', (t) async {
    await t.pumpWidget(MaterialApp(
      theme: AppTheme.light(),
      home: Scaffold(body: ModalCard(title: 't', body: const Text('b'), actions: const [], onClose: () {})),
    ));
    expect(t.getSize(find.byKey(const ValueKey('modal-close'))), const Size(30, 30));
  });
}
```

- [ ] **Step 2: 实现** `SheetShell`：

```dart
class SheetShell extends StatelessWidget {
  const SheetShell({super.key, required this.child});
  final Widget child;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final bottom = MediaQuery.viewPaddingOf(context).bottom;
    return Container(
      constraints: BoxConstraints(maxHeight: MediaQuery.sizeOf(context).height * .85),
      decoration: BoxDecoration(
        color: c.surface,
        borderRadius: const BorderRadius.vertical(top: Radius.circular(Dim.r5)),
        boxShadow: Shadows.sheet(),
      ),
      padding: EdgeInsets.fromLTRB(Dim.gutter, Dim.s2, Dim.gutter, bottom > Dim.s4 ? bottom : Dim.s4),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Container(
            width: 45,
            height: 4.5,
            margin: const EdgeInsets.only(bottom: Dim.s3),
            decoration: BoxDecoration(color: c.line, borderRadius: Dim.brPill),
          ),
          Flexible(child: SingleChildScrollView(child: child)),
        ],
      ),
    );
  }
}

Future<T?> showAppSheet<T>(BuildContext context, {required WidgetBuilder builder}) {
  return showModalBottomSheet<T>(
    context: context,
    isScrollControlled: true,
    useSafeArea: false,
    backgroundColor: Colors.transparent,
    barrierColor: const Color(0x94081420),
    sheetAnimationStyle: AnimationStyle(
      duration: Motion.sheetIn,
      reverseDuration: Motion.sheetOut,
      curve: Motion.spring,
      reverseCurve: Motion.exit,
    ),
    builder: (ctx) => SheetShell(child: builder(ctx)),
  );
}

Future<T?> showAppModal<T>(BuildContext context, {required Widget child, bool dismissible = true}) {
  return showGeneralDialog<T>(
    context: context,
    barrierDismissible: dismissible,
    barrierLabel: 'modal',
    barrierColor: const Color(0x94081420),
    transitionDuration: Motion.modal,
    pageBuilder: (_, __, ___) => Center(
      child: Padding(padding: const EdgeInsets.symmetric(horizontal: Dim.gutter), child: child),
    ),
    transitionBuilder: (_, anim, __, page) {
      final curved = CurvedAnimation(parent: anim, curve: Motion.spring);
      return FadeTransition(
        opacity: anim,
        child: ScaleTransition(scale: Tween(begin: .92, end: 1.0).animate(curved), child: page),
      );
    },
  );
}
```

`ModalCard`：✕ 按钮 `Container(key: ValueKey('modal-close'), width: 30, height: 30, decoration: circle ink .09)`，图标 45 `c.gold`，head 背景 `LinearGradient(begin: top, end: bottom, colors: [c.coin.withValues(alpha: .26), c.coin.withValues(alpha: 0)])`，阴影 `Shadows.modal()`。

- [ ] **Step 3: 全局替换**：`grep -rn "showModalBottomSheet\|showDialog(" lib/features lib/ui/flows`，逐处改为 `showAppSheet` / `showAppModal`（隐私授权门 `_showPrivacyGate` 保留 `barrierDismissible: false` → `dismissible: false`）。
- [ ] **Step 4: 跑测试（需授权）** Run: `flutter test test/widgets/overlays_test.dart` Expected: PASS
- [ ] **Step 5: Commit** `git commit -m "feat(app): unify sheets and modals behind showAppSheet/showAppModal"`

### Task 11: 状态视图 `states.dart`

**Files:**
- Modify: `app/bottles/lib/ui/widgets/states.dart`
- Test: `app/bottles/test/widgets/states_test.dart`

| 组件 | 原型 | 现值 | 目标 |
|---|---|---|---|
| `EmptyState` | `.empty2` 图标 38px→**57**（矢量 92px→137）mist、h5 t5 800 mt s3、p t2 lh1.7 ink3 mt s1、cta mt s5 撑满、左右 padding s6 | 图标 56、标题 t4 | 图标 57、标题 **t5**、正文 t2 lh1.7、cta 前 s5；矮屏（`Breakpoints.isShort`）图标缩到 40 |
| `FailureState` | 同 `.empty2` | 检查 | 同上 |
| `Skeleton` | `.skel` r1、透明度 .5↔1 1.5s | 检查 | r1、`Motion` 1.5s |
| `SkeletonRows` | `.skrow` gap s3 padding s2 0、圆 44 | ✓ | ✓ |
| `DelayedLoading` | 规范：< 300ms 不显示，> 8s 失败 | `Motion.loadingDelay` ✓ | ✓ |

- [ ] **Step 1: 写测试**

```dart
import 'package:bottles/core/design/theme.dart';
import 'package:bottles/ui/widgets/states.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('EmptyState icon is 57 on a regular screen and 40 on a short one', (t) async {
    t.view.devicePixelRatio = 1;
    addTearDown(t.view.reset);
    for (final (h, expected) in [(851.0, 57.0), (667.0, 40.0)]) {
      t.view.physicalSize = Size(393, h);
      await t.pumpWidget(MaterialApp(
        theme: AppTheme.light(),
        home: const Scaffold(body: EmptyState(icon: Icons.inbox, title: 't', body: 'b')),
      ));
      expect(t.widget<Icon>(find.byType(Icon)).size, expected, reason: 'height $h');
    }
  });
}
```

- [ ] **Step 2: 实现**：`EmptyState.build` 里 `final iconSize = Breakpoints.isShort(context) ? 40.0 : 57.0;`，标题 `Dim.t5`，正文 `height: 1.7`，CTA 前 `SizedBox(height: Dim.s5)`；`FailureState` 同步。
- [ ] **Step 3: 跑测试（需授权）** Run: `flutter test test/widgets/states_test.dart` Expected: PASS
- [ ] **Step 4: Commit** `git commit -m "feat(app): align empty/failure states with prototype and shrink on short screens"`

### Task 12: 打包字体（规范 §4.6、D6）

**Files:**
- Create: `app/bottles/assets/fonts/MiSans-Regular.ttf`、`MiSans-Medium.ttf`、`MiSans-Bold.ttf`（子集化后）
- Modify: `app/bottles/pubspec.yaml`、`app/bottles/lib/core/design/theme.dart`（`fontFamily: 'MiSans'`）
- Create: `app/bottles/tool/subset_fonts.py`

**决策（D6 收口）**：选 **MiSans**。理由：小米官方免费商用、Latin 字形与中文同源、字重齐、印度用户也有天城文版本（MiSans Devanagari）留后路。HarmonyOS Sans 的 Latin 更宽，和原型的 Segoe/PingFang 观感差更多。

- [ ] **Step 1: 用户提供原始字体**：请用户把 MiSans 官方包里的 `MiSans-Regular.ttf`、`MiSans-Medium.ttf`、`MiSans-Bold.ttf` 放到 `app/bottles/tool/fonts_src/`（不入库，加 `.gitignore`）。
- [ ] **Step 2: 写子集化脚本**

```python
"""MiSans 子集化：GB2312 一级 + 常用二级 + Latin + 标点 + 货币符号，每字重压到 4MB 内。需 pip install fonttools brotli."""
import subprocess
from pathlib import Path

SRC = Path(__file__).parent / 'fonts_src'
OUT = Path(__file__).parents[1] / 'assets' / 'fonts'
UNICODES = 'U+0000-00FF,U+0100-017F,U+2000-206F,U+20A0-20CF,U+2190-21FF,U+3000-303F,U+FF00-FFEF,U+4E00-9FFF'

for w in ['Regular', 'Medium', 'Bold']:
    OUT.mkdir(parents=True, exist_ok=True)
    subprocess.run(['pyftsubset', str(SRC / f'MiSans-{w}.ttf'), f'--unicodes={UNICODES}',
                    '--layout-features=*', '--no-hinting', f'--output-file={OUT / f"MiSans-{w}.ttf"}'], check=True)
    print(w, (OUT / f'MiSans-{w}.ttf').stat().st_size // 1024, 'KB')
```

- [ ] **Step 3: pubspec 注册**

```yaml
flutter:
  fonts:
    - family: MiSans
      fonts:
        - asset: assets/fonts/MiSans-Regular.ttf
        - asset: assets/fonts/MiSans-Medium.ttf
          weight: 500
        - asset: assets/fonts/MiSans-Bold.ttf
          weight: 700
```

`theme.dart` 的 `ThemeData(... fontFamily: 'MiSans')`；`_textTheme` 里 `w800` 改 `w700`（只打包到 Bold，w800 会被合成加粗发虚）。全局 `grep -rn "FontWeight.w800" lib` 替换为 `w700`。

- [ ] **Step 4: 跑（需授权）** Run: `python tool/subset_fonts.py && flutter analyze` Expected: 三个文件各 < 4096 KB；analyze 无 issue。
- [ ] **Step 5: Commit** `git commit -m "feat(app): bundle subsetted MiSans so all regions render the same"`

---

## Phase 2：设计画布与海面（B1 系列）

### Task 13: `DesignCanvas` 组件（规范 §5）

**Files:**
- Create: `app/bottles/lib/ui/widgets/design_canvas.dart`
- Test: `app/bottles/test/widgets/design_canvas_test.dart`

**Interfaces:**
- Produces:
  - `class DesignCanvas extends StatelessWidget { const DesignCanvas({required List<Widget> children, Size designSize = const Size(375, 762), Widget? extendBottom, Widget? background}); }`
  - `class CanvasPositioned extends StatelessWidget { const CanvasPositioned({required double left/right/top/bottom (任二), double? width, double? height, required Widget child}); }`
  - `CanvasScope.of(context).scale`：子元素需要按比例缩放自身尺寸时取。

- [ ] **Step 1: 写测试**

```dart
import 'package:bottles/ui/widgets/design_canvas.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('scale follows width: 393 wide → 1.048, child at (100,100) lands at (104.8,104.8)', (t) async {
    t.view.devicePixelRatio = 1;
    t.view.physicalSize = const Size(393, 851);
    addTearDown(t.view.reset);
    await t.pumpWidget(const MaterialApp(
      home: DesignCanvas(children: [
        CanvasPositioned(left: 100, top: 100, width: 10, height: 10, child: SizedBox(key: Key('p'))),
      ]),
    ));
    final r = t.getRect(find.byKey(const Key('p')));
    expect(r.left, closeTo(104.8, .1));
    expect(r.top, closeTo(104.8, .1));
    expect(r.width, closeTo(10.48, .1));
  });

  testWidgets('short screen clips the bottom instead of squashing', (t) async {
    t.view.devicePixelRatio = 1;
    t.view.physicalSize = const Size(375, 600);
    addTearDown(t.view.reset);
    await t.pumpWidget(const MaterialApp(
      home: DesignCanvas(children: [
        CanvasPositioned(left: 0, top: 700, width: 10, height: 10, child: SizedBox(key: Key('low'))),
      ]),
    ));
    expect(t.getRect(find.byKey(const Key('low'))).top, closeTo(700, .1)); // 坐标不变，只是被裁
    expect(t.takeException(), isNull);
  });
}
```

- [ ] **Step 2: 实现**

```dart
import 'package:flutter/material.dart';

/// 装饰画布（规范 §5）：原型 375×762 坐标系，**按宽度等比缩放**，
/// 高度不足裁底、多余用 [extendBottom] 填满。只放装饰，功能控件作它的兄弟节点。
class DesignCanvas extends StatelessWidget {
  const DesignCanvas({
    super.key,
    required this.children,
    this.designSize = const Size(375, 762),
    this.background,
    this.extendBottom,
  });

  final List<Widget> children;
  final Size designSize;

  /// 铺满整个可用区域的底（如海面渐变），不参与缩放。
  final Widget? background;

  /// 画布高度不够铺满时，画布底边以下的填充。
  final Widget? extendBottom;

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(builder: (context, box) {
      final scale = box.maxWidth / designSize.width;
      final canvasH = designSize.height * scale;
      return ClipRect(
        child: Stack(
          fit: StackFit.expand,
          children: [
            if (background != null) background!,
            if (extendBottom != null && box.maxHeight > canvasH)
              Positioned(left: 0, right: 0, top: canvasH, bottom: 0, child: extendBottom!),
            Positioned(
              left: 0,
              top: 0,
              width: box.maxWidth,
              height: canvasH,
              child: CanvasScope(
                scale: scale,
                child: Stack(clipBehavior: Clip.none, children: children),
              ),
            ),
          ],
        ),
      );
    });
  }
}

class CanvasScope extends InheritedWidget {
  const CanvasScope({super.key, required this.scale, required super.child});
  final double scale;

  static CanvasScope of(BuildContext context) =>
      context.dependOnInheritedWidgetOfExactType<CanvasScope>()!;

  @override
  bool updateShouldNotify(CanvasScope old) => old.scale != scale;
}

/// 原型坐标定位。宽高可省略（由子节点决定），给了就跟着缩放。
class CanvasPositioned extends StatelessWidget {
  const CanvasPositioned({
    super.key,
    this.left,
    this.top,
    this.right,
    this.bottom,
    this.width,
    this.height,
    required this.child,
  });

  final double? left, top, right, bottom, width, height;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    final s = CanvasScope.of(context).scale;
    double? m(double? v) => v == null ? null : v * s;
    return Positioned(
      left: m(left), top: m(top), right: m(right), bottom: m(bottom),
      width: m(width), height: m(height),
      child: width == null && height == null
          ? Transform.scale(scale: s, alignment: Alignment.topLeft, child: child)
          : child,
    );
  }
}
```

- [ ] **Step 3: 跑测试（需授权）** Run: `flutter test test/widgets/design_canvas_test.dart` Expected: PASS
- [ ] **Step 4: Commit** `git commit -m "feat(app): add DesignCanvas for prototype-anchored decorative scenes"`

### Task 14: 海面 B1 / B1n / B1a / B1b / Z1 / Z3

**Files:**
- Modify: `app/bottles/lib/ui/widgets/sea_scene.dart`（装饰迁到画布坐标）
- Modify: `app/bottles/lib/features/bottle/ocean_page.dart`、`scoop_flow.dart`
- Test: `app/bottles/test/layout/bottle_layout_test.dart`

**画布设计尺寸：`Size(375, 490)`，只覆盖 `.scene`**（原型 B1 的 `.scene` 是头图与 Tab 栏之间的弹性区，在 512 高的框里约 330px；头图与 Tab 栏在画布外）。原型 CSS 与 B1 markup 的百分比按 375×490 换算：

| 元素 | 原型 | 画布坐标（375×490 系） |
|---|---|---|
| 太阳 `.sun` | top 8%、right 24px、34px 圆 | top 39、right 36、直径 51 |
| 云 1 | left 16px top 11% | left 24、top 54、宽 39 |
| 云 2 | left 54% top 6% scale .82 | left 202、top 29、宽 32 |
| 云 3 | left 30% top 20% scale .62 opacity .85 | left 112、top 98、宽 24 |
| 浪线 `.wave` | top 52% 高 12px | top 255、高 18 |
| 灯塔 `.lh` | left 13px、bottom 104px；塔 12×42、岛 38×11、灯 8×7 | left 19、bottom 155；整体高 89 |
| 海况标签 `.lhtag` | left 58px、bottom 112px | left 86、bottom 167 |
| 瓶子容器 `.sea` | top 42%、bottom 76px | top 206、高 171 |
| 瓶 1（带 3 条回应） | left 22% top 12%（在 `.sea` 内） | (83, 227) |
| 瓶 2 | left 55% top 2% | (206, 209) |
| 瓶 3（带 1 条回应） | left 76% top 26% | (285, 250) |
| 瓶 4 | left 38% top 38% | (143, 271) |
| 瓶子尺寸 `.btl` | 20×32 | 30×48 |
| 涟漪中心 `.ripple` | left 50% top 46% | (187, 225) |
| 底部两个按钮 `.pbn` | 高 42px→62、r4、gap s2、left/right s4、bottom s4 | 画布外，`DockButton` |
| 头图 `.hdr` | 见 Task 6 | 画布外 |

- [ ] **Step 1: 写布局测试**

```dart
import 'package:bottles/features/bottle/ocean_page.dart';
import 'package:flutter_test/flutter_test.dart';

import 'matrix.dart';

void main() {
  for (final cfg in layoutMatrix) {
    testWidgets('ocean fits: ${cfg.name}', (t) => pumpAt(t, cfg, const OceanPage()));
    testWidgets('ocean fits with 3-button nav: ${cfg.name}',
        (t) => pumpAt(t, cfg.withInsets(viewPadding: threeButtonNav, suffix: ' +nav'), const OceanPage()));
  }
}
```

- [ ] **Step 2: 重写 `_Sky`**：删掉 `LayoutBuilder` 百分比定位，改为

```dart
class _Sky extends StatelessWidget {
  const _Sky({required this.night});
  final bool night;

  @override
  Widget build(BuildContext context) {
    return DesignCanvas(
      designSize: const Size(375, 490),
      background: _SeaGradient(night: night),
      extendBottom: _SeaGradient(night: night, bottomOnly: true),
      children: [
        CanvasPositioned(top: 39, right: 36, width: 51, height: 51, child: night ? const _Moon() : const _Sun()),
        if (!night) ...[
          const CanvasPositioned(left: 24, top: 54, width: 39, child: _Cloud()),
          const CanvasPositioned(left: 202, top: 29, width: 32, child: _Cloud()),
          const CanvasPositioned(left: 112, top: 98, width: 24, child: _Cloud(opacity: .85)),
          const CanvasPositioned(left: 0, right: 0, top: 255, height: 18, child: _FoamLine()),
        ],
        CanvasPositioned(left: 19, bottom: 155, child: Lighthouse(night: night)),
      ],
    );
  }
}
```

`SeaScene` 的 `children`（漂浮瓶子）改为由 `OceanPage` 通过 `CanvasPositioned` 传入，瓶位常量：

```dart
/// 原型 B1 四个瓶位（375×490 画布坐标，左上角），瓶子 30×48。
/// 第 5、6 个瓶子原型没画，多出来的瓶子在四个位置之间按 golden ratio 插值，
/// 各瓶随机相位漂浮（相位在 FloatingBottle 内部，避免「军训感」）。
const bottleSlots = <Offset>[
  Offset(83, 227), Offset(206, 209), Offset(285, 250), Offset(143, 271),
  Offset(40, 262), Offset(330, 218),
];
const bottleSize = Size(30, 48);
const rippleCenter = Offset(187, 225);
```

- [ ] **Step 3: `ocean_page.dart` 头部登记**

```dart
/// 类别：场景页（规范 §6）
/// 固定区：头图 GradientHeader、底部 DockButton 两枚
/// 弹性区：SeaScene 画布（高 < 700 时裁底）
/// 可滚动区：无
/// 键盘：无
/// 高屏多余高度：全部给画布（extendBottom 延伸海水）
```

底部按钮 `DockButton` 高 62、r4、t4 800、副文字 t1；`dimmed` 捞取态、B1b 开瓶信纸 `.letter`（顶 4pt 渐变封口 `--grad`、r4、padding s4）保持在 `scoop_flow.dart`，只核对数值：涟漪 `RippleRings` 22→158px 直径按 1.488（33→235）；瓶子放大淡出 220ms；信纸 scaleY .3→1 340ms。

- [ ] **Step 4: 夜场 × 暗色四组合 golden**（Review Focus 3）：

```dart
  for (final night in [false, true]) {
    for (final dark in [false, true]) {
      testWidgets('sea scene night=$night dark=$dark', (t) async {
        await t.pumpWidget(MaterialApp(
          theme: dark ? AppTheme.dark() : AppTheme.light(),
          home: Scaffold(body: SeaScene(night: night)),
        ));
        await expectLater(find.byType(SeaScene), matchesGoldenFile('goldens/sea_${night ? 'night' : 'day'}_${dark ? 'dark' : 'light'}.png'));
      });
    }
  }
```

首次用 `flutter test --update-goldens test/layout/bottle_layout_test.dart` 生成基准图，Read 四张图确认文字与瓶子在暗色下可辨认后再提交。

- [ ] **Step 5: 跑测试（需授权）** Run: `flutter test test/layout/bottle_layout_test.dart` Expected: PASS
- [ ] **Step 6: 渲染原型 B1、B1n 并请用户截真机图比对**：`python tool/proto_render.py --screen B1`、`--screen B1n`；用户截图后 `tool/compare.py` 并排，太阳、灯塔、浪线位置偏差 ≤ 4pt（规范 §8.4）。
- [ ] **Step 7: Commit** `git commit -m "feat(app): move the sea scene onto the design canvas and register the ocean page"`

---

## Phase 3：登录与用户（A 系列剩余）

每个页面任务共用同一套步骤，这里完整写一次，后续任务只列差异表；步骤本身不省略。

### Task 15: A1 启动、A1c 隐私门、A2g / A2p 第三方登录中、A2k 冲突

**Files:**
- Modify: `app/bottles/lib/features/auth/splash_page.dart`、`login_page.dart`（`_showPrivacyGate` 改用 `showAppModal` + `ModalCard`）、`oauth_conflict_page.dart`、`auth_controller.dart`（第三方登录中的整屏遮罩）
- Test: `app/bottles/test/layout/auth_layout_test.dart`（追加）

| 屏 | 原型要点 | 目标 |
|---|---|---|
| A1 `.splash` | 全屏 `--grad` + 三个白色 .18/.13/.11 圆泡；瓶子 44px→**65**；`DRIFT` t6 800 字距 .2em mt s3；标语 t2 白 .82 mt s2；底部进度条 74px×2px→**110×3** 白 .3 底、白 42% 条、距底 s6 | 画布外元素全部居中 Column；进度条 `bottom: max(viewPadding.bottom, s6)`；圆泡用 `CanvasPositioned`（375 系：18%/22% → 67,168 直径 72；82%/16% → 307,122 直径 101；60%/84% → 225,640 直径 140） |
| A1c | `ModalCard`：锁图标、标题 t5、正文 t2 lh1.75 ink2、两个 `.cta`（同意 / `gh` 不同意）；**不能预勾选**；不同意退出 | `_showPrivacyGate` 改 `showAppModal(dismissible: false, child: ModalCard(icon: Icons.lock_outline_rounded, ...))`，actions 两个 `AppButton`（primary / ghost） |
| A2g / A2p | 整屏：品牌 logo + 「正在连接 Google…」+ `.spin` 30px→**45** 圆环 2.5px→3 line 底 aqua 顶 | 新建 `OAuthPendingOverlay(provider)`，`otp.busy && provider != null` 时 `Stack` 叠在登录页上；转圈响应 `MediaQuery.disableAnimationsOf` |
| A2k | `NavBar` + `.empty2` 版式：图标、标题 t5、正文 t2、两个按钮 `.foot`（flex + gap s2） | 用 `EmptyState` + `BottomActionBar(children: [AppButton, AppButton(ghost)])` |

- [ ] **Step 1: 渲染原型**（需授权）`python tool/proto_render.py --screen A1`、`A1c`、`A2g`、`A2k`，Read 四图。
- [ ] **Step 2: 追加布局测试**

```dart
    testWidgets('splash fits: ${cfg.name}', (t) => pumpAt(t, cfg, const SplashPage()));
    testWidgets('oauth conflict fits: ${cfg.name}',
        (t) => pumpAt(t, cfg, const OAuthConflictPage(message: 'x@y.z 已注册')));
```

- [ ] **Step 3: 实现**，每个页面文件头加 §6 登记注释；A1 登记为「场景页 / 固定区：无 / 弹性区：画布 / 可滚动区：无」。
- [ ] **Step 4: 跑测试（需授权）** Run: `flutter test test/layout/auth_layout_test.dart` Expected: PASS
- [ ] **Step 5: Commit** `git commit -m "feat(app): align splash, privacy gate and oauth states with prototype"`

### Task 16: A4 完善资料、A7 定位说明

**Files:**
- Modify: `app/bottles/lib/features/auth/onboarding_page.dart`、`lib/features/location/location_intro_page.dart`
- Test: `test/layout/auth_layout_test.dart`（追加 `OnboardingPage`、`LocationIntroPage`）

| 屏 | 原型要点 | 目标 |
|---|---|---|
| A4 | `NavBar` 带右侧「第 1 步，共 2 步」`.sp2` t2 ink3；`.prog` 3px 渐变进度条；`.avup` 74px→**110** 圆、虚线 line、sea 底、相机角标 24→**36** brand 白边 2→3；`.selfield` 多选下拉 min 34→**50** r3；`.chs .ch` 兴趣 chips（Task 9）；`.foot` 主按钮 | 头像 `width: 104` 写死 → 110；`NavBar(subtitle:)` 已支持；进度条用 `ThinProgressBar`（高 3，渐变）；语言下拉 `SelectField(chips: [...], onTap)` 新建于 `chips.dart` |
| A7 | `.empty2` 版式：定位图标 57、标题 t5、正文 t2、`.cta` 开启 + `.oauth` 样式「以后再说」 | `EmptyState(ctaLabel, altLabel)`；登记为「场景页 / 弹性区：图标上方留白」 |

- [ ] **Step 1: 渲染原型（需授权）** `--screen A4`、`--screen A7`
- [ ] **Step 2: 追加布局测试**（同 Task 15 Step 2 形状，页面换成 `OnboardingPage()`、`LocationIntroPage()`）
- [ ] **Step 3: 实现 `SelectField`**

```dart
/// 多选「下拉」外观的字段（原型 .selfield）：已选项用小胶囊铺开，右侧 ˅。
class SelectField extends StatelessWidget {
  const SelectField({super.key, required this.selected, required this.placeholder, required this.onTap});
  final List<String> selected;
  final String placeholder;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: onTap,
      child: Container(
        constraints: const BoxConstraints(minHeight: 50),
        padding: const EdgeInsets.symmetric(horizontal: Dim.s3, vertical: Dim.s1),
        decoration: BoxDecoration(color: c.surface, borderRadius: Dim.brCard, border: Border.all(color: c.line)),
        child: Row(
          children: [
            Expanded(
              child: selected.isEmpty
                  ? Text(placeholder, style: TextStyle(fontSize: Dim.t3, color: c.ink3, height: 1.5))
                  : Wrap(spacing: Dim.s2, runSpacing: Dim.s1, children: [
                      for (final s in selected) TagChip(label: s),
                    ]),
            ),
            Icon(Icons.expand_more_rounded, size: 22, color: c.ink3),
          ],
        ),
      ),
    );
  }
}
```

- [ ] **Step 4: 跑测试（需授权）** Run: `flutter test test/layout/auth_layout_test.dart` Expected: PASS
- [ ] **Step 5: Commit** `git commit -m "feat(app): align onboarding and location intro with prototype"`

### Task 17: A6 账号与安全、A6b 绑定被占用

**Files:**
- Modify: `app/bottles/lib/features/me/account_security_page.dart`
- Test: `test/layout/me_layout_test.dart`（新建，本 Phase 起「我的」系页面都写这里）

| 原型要点 | 目标 |
|---|---|
| `.sec` 分组标题 t2 700 ink3；`.lst` 分组，行高「列表行加高」→ `roomy` 52；右侧 `span` t2 ink3 显示已绑定 / 未绑定；注销账号行 `titleColor: c.warn`；A6b 用 `ModalCard` | `ListGroup` + `ListRowItem(roomy: true)`；A6b 走 `showAppModal` |

- [ ] **Step 1: 渲染原型（需授权）** `--screen A6`、`--screen A6b`
- [ ] **Step 2: 写测试** `test/layout/me_layout_test.dart`，矩阵里 `pumpAt(t, cfg, const AccountSecurityPage())`
- [ ] **Step 3: 实现** + 登记注释（列表页）
- [ ] **Step 4: 跑测试（需授权）** Run: `flutter test test/layout/me_layout_test.dart` Expected: PASS
- [ ] **Step 5: Commit** `git commit -m "feat(app): align account security page with prototype"`

---

## Phase 4：漂流瓶（B 系列剩余）

### Task 18: B2 写瓶子、M1 / M2 地图选点、B3 详情、B4 / B4s 轨迹与海报

**Files:**
- Modify: `write_bottle_page.dart`、`location/map_picker_page.dart`、`location/place_field.dart`、`bottle_detail_page.dart`、`my_bottles_page.dart`、`drift_map_page.dart`
- Create: `lib/ui/widgets/poster_card.dart`（B4s `.poster`）
- Test: `test/layout/bottle_layout_test.dart`（追加五页）

| 屏 | 原型要点 | 目标 |
|---|---|---|
| B2 `.ta` | 文本域撑满剩余高（弹性区）、padding s4、描边 line、r4、t3 lh1.75；右下计数 t1 ink3；`.thumb` 44→**65** r2 sea，`add` 虚线；`.chs` 标签行；`.foot` 主按钮 | 登记「表单页 / 弹性区：文本域 / 键盘：文本域收缩」；键盘弹起时 `.foot` 贴键盘 |
| M1 / M2 | 地图占弹性区；底部 `.foot` 两按钮；M2 选点后 `PlaceField` 显示地址 + ✕ | `map_picker_page` 登记「场景页」；`google_maps_flutter` 在国内不可用时显示 `FailureState`（R2） |
| B3 | `.btlc` 强调卡：padding s4、r4、描边 line2、渐变底、顶 4pt `--grad` 封口、正文 t4 lh1.75；作者行 `AvatarRing(size: 30)`；解锁回信 `.blur` 3.5px→**5**；底部 `.foot` 两按钮（回信免费 / 解锁 pay） | 抽 `BottleLetterCard` 到 `cards.dart`；模糊 `ImageFiltered(sigma 5)` |
| B4 | `TimelineList`（Task 8）；顶部 `StatsRow` | 登记「列表页」 |
| B4s `.poster` | r4；深海渐变 168deg `#0D1436→#1A2456 34%→#1E3E68 66%→#26586F`；星点 4 处；标题 t5 800 白；副标 t1 白 .62；时间线白版；`.stats2` 三格 t6；`.quote` 左线 2 白 .35 t3 lh1.7；水印 `.wm2` t1 字距 .16em 白 .5 | 新建 `PosterCard`，`RepaintBoundary` 包住供分享截图 |

- [ ] **Step 1: 渲染原型（需授权）** `--screen B2 M1 M2 B3 B4 B4s`（六次调用）
- [ ] **Step 2: 追加布局测试**，B2 与 D2 一样加 `keyboard300` 变体：

```dart
    testWidgets('write bottle fits with keyboard: ${cfg.name}',
        (t) => pumpAt(t, cfg.withInsets(viewInsets: keyboard300, suffix: ' +kb'), const WriteBottlePage()));
```

- [ ] **Step 3: 实现 `PosterCard`**

```dart
/// 可分享海报（原型 .poster）：深海渐变 + 星点 + 白版时间线。外层调用方包 RepaintBoundary 截图。
class PosterCard extends StatelessWidget {
  const PosterCard({super.key, required this.title, required this.subtitle, required this.stops,
      required this.stats, this.quote});
  final String title;
  final String subtitle;
  final List<(String place, String time)> stops;
  final List<(String value, String label)> stats;
  final String? quote;

  static const _gradient = LinearGradient(
    begin: Alignment(-0.6, -1), end: Alignment(0.6, 1),
    colors: [Color(0xFF0D1436), Color(0xFF1A2456), Color(0xFF1E3E68), Color(0xFF26586F)],
    stops: [0, .34, .66, 1],
  );

  @override
  Widget build(BuildContext context) {
    return Container(
      clipBehavior: Clip.antiAlias,
      padding: const EdgeInsets.all(Dim.s4),
      decoration: BoxDecoration(gradient: _gradient, borderRadius: Dim.brLargeCard, boxShadow: Shadows.floating(context.c)),
      child: Stack(children: [
        const Positioned.fill(child: _Stars()),
        Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          Text(title, style: const TextStyle(fontSize: Dim.t5, fontWeight: FontWeight.w700, color: Colors.white, height: 1.25)),
          Text(subtitle, style: TextStyle(fontSize: Dim.t1, color: Colors.white.withValues(alpha: .62), height: 1.5)),
          const SizedBox(height: Dim.s5),
          TimelineList.onDark(stops: stops),
          const SizedBox(height: Dim.s4),
          _Stats(stats),
          if (quote != null) _Quote(quote!),
          const SizedBox(height: Dim.s4),
          Row(children: [
            Text('DRIFT', style: TextStyle(fontSize: Dim.t1, fontWeight: FontWeight.w700, letterSpacing: 1.9, color: Colors.white.withValues(alpha: .5))),
            const Spacer(),
            Text(subtitle, style: TextStyle(fontSize: Dim.t1, color: Colors.white.withValues(alpha: .5))),
          ]),
        ]),
      ]),
    );
  }
}
```

（`_Stars`：四个 `Positioned` 白色小圆 1.5～2pt，位置 22%/18%、68%/12%、44%/28%、86%/24%，用 `LayoutBuilder` 按卡片宽高百分比放，这是装饰画布内的百分比定位，规范允许。`TimelineList.onDark` 为 Task 8 的 `TimelineList` 加的命名构造：点白 + 白 .2 外圈、线白 .38、文字白 700。）

- [ ] **Step 4: 跑测试（需授权）** Run: `flutter test test/layout/bottle_layout_test.dart` Expected: PASS
- [ ] **Step 5: Commit** `git commit -m "feat(app): align bottle compose, detail, map picker and poster with prototype"`

---

## Phase 5：匹配发现（C 系列）

### Task 19: C1 滑卡

**Files:**
- Modify: `lib/features/discover/discover_page.dart`、`swipe_deck.dart`
- Test: `test/layout/discover_layout_test.dart`

| 原型要点 | 目标 |
|---|---|
| `.deck` margin s6 s3 s3；`.swc` r5、描边 line、阴影 `floating`；后两张 scale .945 / .89、translateY 11px→**16** / 22px→**33**、opacity .62 / .34；前卡 rotate 3° translate(7,-3) | `swipe_deck.dart` 的堆叠常量按此改 |
| `.pic` 照片区 flex 1、渐变占位 `158deg #FFD9E2→#EAD8FF 54%→#CDE9F4`；`.live` 左上 s3 白 .92 pill t1 700 绿点 6px→9；`.stamp` 印章 left s3 top 46px→**68**、旋转 -14°、边 4pt `#2FBF77`、r2、t6 800 字距 .05em、opacity .92 | 印章透明度随位移 0→1，25% 处到满（§21） |
| `.info` padding s4；名字 t6 800；右侧「去看看」t2 700 ink3；bio t2 lh1.6 ink2 mt s2；`.acts` gap s2 mt s4：跳过 `RoundIconButton`、喜欢 `RoundIconButton(liked)`、`HiButton` 占满 | 用 Task 5 的组件 |
| 手势：位移 > 25% 或速度 > 800 判定；旋转 = 位移 × .06° 上限 12°；飞出 280ms easeOutCubic；回弹 `Motion.spring` | 核对 `swipe_deck.dart` 的阈值双通道 |
| 宽 ≥ 600：卡片在限宽 480 内，照片区最小 3:4 | Review Focus 2 |

- [ ] **Step 1: 渲染原型（需授权）** `--screen C1`
- [ ] **Step 2: 写测试**

```dart
import 'package:bottles/features/discover/discover_page.dart';
import 'package:bottles/features/discover/swipe_deck.dart';
import 'package:flutter_test/flutter_test.dart';

import 'matrix.dart';

void main() {
  for (final cfg in layoutMatrix) {
    testWidgets('discover fits: ${cfg.name}', (t) async {
      await pumpAt(t, cfg, const DiscoverPage());
      if (cfg.size.width >= 600) {
        final deck = find.byType(SwipeDeck);
        if (deck.evaluate().isNotEmpty) {
          final s = t.getSize(deck);
          expect(s.width, lessThanOrEqualTo(480));
          expect(s.height / s.width, greaterThanOrEqualTo(4 / 3));
        }
      }
    });
  }
}
```

- [ ] **Step 3: 实现** + 登记注释「场景页 / 固定区：头图 chips 行 / 弹性区：卡片堆 / 可滚动区：无」。
- [ ] **Step 4: 跑测试（需授权）** Run: `flutter test test/layout/discover_layout_test.dart` Expected: PASS
- [ ] **Step 5: Commit** `git commit -m "feat(app): align swipe deck stack, stamp and actions with prototype"`

### Task 20: C2 筛选 · 答题匹配、C3 用户主页

**Files:**
- Modify: `filter_page.dart`、`user_profile_page.dart`
- Test: `test/layout/discover_layout_test.dart`（追加）

| 屏 | 原型要点 | 目标 |
|---|---|---|
| C2 | `.qz` 每题一张 sea 底 r3 padding s3 卡；题干 t3 700；选项 `PillChip`；`.sld` 距离滑块：轨 3、拇指 14px→**21** 白 + brand 边 2；底部 `.foot`「N 人符合」 | 滑块用 `SliderTheme`（已在 theme 里，trackHeight 4 → 3、thumb 10.5 半径）；登记「表单页」 |
| C3 | `.hero` 178px→**265** 高、渐变、右上 `.iconb` 24px→**36** 白圆 + 阴影；`.charm` 金 pill t2 800；`.stats` 三格；`.mvrow` 动态预览行：缩略 38→**57** r2、文案 t2、时间 t0；`.foot` 两按钮 | `height: 300` 写死 → 265；矮屏（< 700）收到 200；登记「详情页 / 弹性区：头图」；Hero 共享元素 tag = `user-photo-<id>`（§21） |

- [ ] **Step 1: 渲染原型（需授权）** `--screen C2`、`--screen C3`
- [ ] **Step 2: 追加测试**（`const FilterPage()`、`const UserProfilePage(userId: 'u1')`）
- [ ] **Step 3: 实现** + 登记注释
- [ ] **Step 4: 跑测试（需授权）** Expected: PASS
- [ ] **Step 5: Commit** `git commit -m "feat(app): align filter quiz and user profile with prototype"`

---

## Phase 6：私聊与关系（D / E 系列）

### Task 21: D1 会话列表、Z2 空态、E1 关系

**Files:**
- Modify: `chat_list_page.dart`、`me/relations_page.dart`
- Test: `test/layout/chat_layout_test.dart`

| 原型要点 | 目标 |
|---|---|
| `.rws a` 行最小高 44px→**65**、padding s2 0、分割线左缩进 38px→**57**；`AvatarRing(size: 44)`；名字 t3 700；预览 t2 ink3 单行省略；右侧时间 t1 ink3 + `.ub2` 未读 pill 最小宽 14px→**21** brand t0 800 | 抽 `ConversationRow` 到 `cards.dart`；E1 复用 + `MiniButton` 右侧 |
| Z2 `EmptyState` | Task 11 |

- [ ] **Step 1: 渲染原型（需授权）** `--screen D1 Z2 E1`
- [ ] **Step 2: 写测试**（`ChatListPage()`、`RelationsPage()`）
- [ ] **Step 3: 实现** + 登记注释（列表页）
- [ ] **Step 4: 跑测试（需授权）** Expected: PASS
- [ ] **Step 5: Commit** `git commit -m "feat(app): align conversation rows and relations with prototype"`

### Task 22: D2 聊天窗、Z7 会话失效、H4 礼物面板（聊天内入口）

**Files:**
- Create: `lib/ui/widgets/chat_bubbles.dart`（从 `chat_room_page.dart` 抽出）
- Modify: `chat_room_page.dart`、`ui/flows/gift_flows.dart`
- Test: `test/layout/chat_layout_test.dart`（追加）、`test/widgets/chat_bubbles_test.dart`

| 元素 | 原型 | 目标 |
|---|---|---|
| `.bub` | max-width 76%、padding s2 s3、t3 lh1.55；对方 sea 底 r3 r3 r3 **r1**；我方 brand 白字 r3 r3 **r1** r3；已读 `.rd2` t0 .8 右对齐 mt 2 | `MessageBubble(mine: bool, text, readLabel)` |
| `.sys` | 居中 pill、max 82%、t0 ink3、line2 底、padding 2.5/s3 | `SystemPill(text)` |
| `.gift` | 居中 pill、coin .2 底、t2 ink2、padding s2 s3 | `GiftPill(text, icon)` |
| `.imgmsg` | 宽 120px→**178**、r 21、4:3、阴影 `0 4 12 .2` → `0 6 18`、已读角标覆盖右下 | `ImageMessage(url, readLabel)` |
| `.inp` | padding s2 s4 s3、顶线 line2；输入框 `.fd` 高 30→**44** pill page 底 t3；发送圆钮 30→**44** brand；左侧图标 19px→28 ink2 | `ChatInputBar`；底 padding `max(viewPadding.bottom, s3)`；键盘弹起随 `viewInsets` |
| H4 `.sheet` + `.gitem` | 网格 4 列；格 padding s2 0、描边 1.5 透明、r3；选中 brand 描边 + 粉 .07 底；图标 20px→**30**；价 t1 700 gold；持有数 `em` 角标 ink 底 t0；`.qty` 步进 50 高 pill | `gift_flows.dart` 用 `showAppSheet`；格子抽 `GiftTile` |

- [ ] **Step 1: 渲染原型（需授权）** `--screen D2 Z7 H4`
- [ ] **Step 2: 写组件测试**

```dart
import 'package:bottles/core/design/theme.dart';
import 'package:bottles/ui/widgets/chat_bubbles.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('bubble never exceeds 76% of the row', (t) async {
    t.view.devicePixelRatio = 1;
    t.view.physicalSize = const Size(360, 780);
    addTearDown(t.view.reset);
    await t.pumpWidget(MaterialApp(
      theme: AppTheme.light(),
      home: Scaffold(body: Column(children: [MessageBubble(mine: true, text: 'x' * 400)])),
    ));
    expect(t.getSize(find.byType(MessageBubble)).width, lessThanOrEqualTo(360 * .76 + 1));
  });

  testWidgets('input bar field and send button are 44 tall', (t) async {
    await t.pumpWidget(MaterialApp(
      theme: AppTheme.light(),
      home: Scaffold(body: ChatInputBar(controller: TextEditingController(), onSend: (_) {}, onPickImage: () {})),
    ));
    expect(t.getSize(find.byKey(const ValueKey('chat-send'))).height, 44);
  });
}
```

- [ ] **Step 3: 追加布局测试**（Review Focus 1）

```dart
    testWidgets('chat room fits: ${cfg.name}', (t) => pumpAt(t, cfg, const ChatRoomPage(conversationId: 'c1')));
    testWidgets('chat room fits with keyboard: ${cfg.name}',
        (t) => pumpAt(t, cfg.withInsets(viewInsets: keyboard300, suffix: ' +kb'), const ChatRoomPage(conversationId: 'c1')));
```

- [ ] **Step 4: 实现**：`chat_room_page.dart` 只保留编排 + 登记注释「聊天页 / 固定区：NavBar、ChatInputBar / 可滚动区：消息列表（reverse: true）/ 键盘：输入栏随 viewInsets 上移，列表 `resizeToAvoidBottomInset`」。
- [ ] **Step 5: 跑测试（需授权）** Run: `flutter test test/widgets/chat_bubbles_test.dart test/layout/chat_layout_test.dart` Expected: PASS
- [ ] **Step 6: Commit** `git commit -m "feat(app): extract chat bubbles and input bar, align with prototype"`

---

## Phase 7：动态（F 系列）

### Task 23: F1 动态广场、F2 发动态、F3 动态详情

**Files:**
- Modify: `moment_feed_page.dart`、`post_moment_page.dart`、`moment_detail_page.dart`
- Create: `lib/ui/widgets/photo_grid.dart`（`.g9` 九宫格，F2 选图与 F1/F3 展示共用）
- Test: `test/layout/moment_layout_test.dart`

| 屏 | 原型要点 | 目标 |
|---|---|---|
| F1 `.mcd` | 卡 padding s3、描边 line、r4、surface；作者行 `AvatarRing(30)`；正文 t3 lh1.6 mt s2；图片行 gap 3px→**4.5** r1 正方形；底栏 `.bar5` gap s5 t2 ink3；`.fab` 60 圆 aqua 距底 58px→**86** 距右 s4 | `MomentCard` 抽到 `cards.dart`；FAB 用 `Positioned` 在 Tab 页内，z 高于列表 |
| F2 | `.ta` 文本域 + `PhotoGrid` 3 列 gap s2 r2，空格虚线 `+`，`.num` 序号角标 14px→**21** brand；`.ovl` 上传中遮罩 ink .52 白 t1 700 | 登记「表单页 / 键盘：文本域收缩」 |
| F3 | `MomentCard` 展开 + 评论 `.rws`；底部 `ChatInputBar` 变体（评论） | 复用 Task 22 的 `ChatInputBar(hint:)` |
| 图片查看器 | 规范 §4.8 唯一的横屏例外 | `image_viewer.dart` 进入时 `SystemChrome.setPreferredOrientations([portraitUp, landscapeLeft, landscapeRight])`，`dispose` 时恢复 `[portraitUp]`；大图 `memCacheWidth` = 屏宽 × DPR |

- [ ] **Step 1: 渲染原型（需授权）** `--screen F1 F2 F3`
- [ ] **Step 2: 写测试**（`MomentFeedPage()`、`PostMomentPage()` 含 `keyboard300` 变体、`MomentDetailPage(momentId: 'm1')`）
- [ ] **Step 3: 实现 `PhotoGrid`**

```dart
/// 九宫格（原型 .g9）：3 列、gap s2、r2、正方形。编辑态显示序号角标与「+」空位。
class PhotoGrid extends StatelessWidget {
  const PhotoGrid({super.key, required this.urls, this.max = 9, this.onAdd, this.onTap, this.uploading = const {}});
  final List<String> urls;
  final int max;
  final VoidCallback? onAdd;
  final ValueChanged<int>? onTap;
  final Set<int> uploading;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    final showAdd = onAdd != null && urls.length < max;
    final count = urls.length + (showAdd ? 1 : 0);
    return GridView.builder(
      shrinkWrap: true,
      physics: const NeverScrollableScrollPhysics(),
      gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(crossAxisCount: 3, mainAxisSpacing: Dim.s2, crossAxisSpacing: Dim.s2),
      itemCount: count,
      itemBuilder: (_, i) {
        if (i == urls.length) {
          return GestureDetector(
            onTap: onAdd,
            child: Container(
              decoration: BoxDecoration(borderRadius: Dim.brButton, border: Border.all(color: c.line, style: BorderStyle.solid)),
              child: Icon(Icons.add_rounded, size: 30, color: c.ink3),
            ),
          );
        }
        return GestureDetector(
          onTap: onTap == null ? null : () => onTap!(i),
          child: ClipRRect(
            borderRadius: Dim.brButton,
            child: Stack(fit: StackFit.expand, children: [
              CachedNetworkImage(
                imageUrl: urls[i],
                fit: BoxFit.cover,
                // 规范 §4.10：列表缩略图按格子逻辑宽 × DPR 解码，不解码原图（印度 3～4GB 机型会掉帧）。
                memCacheWidth: (MediaQuery.sizeOf(context).width / 3 * MediaQuery.devicePixelRatioOf(context)).round(),
              ),
              if (onAdd != null)
                Positioned(right: 4, top: 4, child: Container(
                  width: 21, height: 21, alignment: Alignment.center,
                  decoration: BoxDecoration(color: c.brand, shape: BoxShape.circle),
                  child: Text('${i + 1}', style: const TextStyle(fontSize: Dim.t0, fontWeight: FontWeight.w700, color: Colors.white, height: 1)),
                )),
              if (uploading.contains(i))
                ColoredBox(color: const Color(0x85081420), child: Center(child: Text('上传中', style: TextStyle(fontSize: Dim.t1, fontWeight: FontWeight.w700, color: Colors.white)))),
            ]),
          ),
        );
      },
    );
  }
}
```

（虚线边框 Flutter 无内置，用 `DottedBorder` 手绘 `CustomPainter`，或退而求其次用 `line` 实线，在注释里注明与原型的差异。）

- [ ] **Step 4: 图片查看器横屏例外**：`image_viewer.dart` 改为 `StatefulWidget`，`initState` 放开横屏、`dispose` 恢复竖屏；`PhotoGrid` 与查看器都走 `CachedNetworkImage(memCacheWidth:)`。
- [ ] **Step 5: 跑测试（需授权）** Expected: PASS
- [ ] **Step 6: Commit** `git commit -m "feat(app): align moment feed, composer and detail with prototype"`

---

## Phase 8：我的、变现与支付（G / H 系列）

### Task 24: H1 我的、G2 通知中心、G3 举报

**Files:**
- Modify: `me_page.dart`、`notifications_page.dart`、`ui/flows/safety_flows.dart`
- Test: `test/layout/me_layout_test.dart`（追加）

| 屏 | 原型要点 | 目标 |
|---|---|---|
| H1 | `GradientHeader` + `HeaderStats`（Task 6）+ `.editc` 编辑 pill 白 .94 红字 t2 800；`.wallet` 卡上探 -26px→**-39**、padding s4、r4、`floating` 阴影、描边 line2、右上金色光斑、顶 4pt 金线；`.bigcoin` 56 圆；余额 26px→**39** 800 tabular；「充值」`MiniButton(pay)` 高 32→**48**；`.wlinks` 三链接 t2 600 mt s3 顶线；`QuadGrid(roomy)`；`ListGroup` `roomy` | `me_page` 手写的钱包卡抽 `WalletHeroCard` 到 `cards.dart`；`padding bottom 40` 改成随 `showWallet` 的 39 |
| G2 | `.rws` 行版式（Task 21 `ConversationRow` 变体：无未读 pill、左图标 sea 圆） | 复用 |
| G3 | `showAppSheet` + 单选列表 `.pay` 行 50 高 r3 描边 1.5、选中 brand + 圆点 19 边 6 | `RadioRow` 抽到 `chips.dart`，H3 支付方式复用 |

- [ ] **Step 1: 渲染原型（需授权）** `--screen H1 G2 G3`
- [ ] **Step 2: 追加测试**（`MePage()`、`NotificationsPage()`）
- [ ] **Step 3: 实现 `RadioRow`**

```dart
/// 单选行（原型 .pay）：50 高、r3、1.5 描边；选中 brand 描边 + 右侧实心圆点（19，边 6）。
class RadioRow extends StatelessWidget {
  const RadioRow({super.key, required this.label, required this.selected, required this.onTap, this.icon, this.trailing});
  final String label;
  final bool selected;
  final VoidCallback onTap;
  final IconData? icon;
  final Widget? trailing;

  @override
  Widget build(BuildContext context) {
    final c = context.c;
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: onTap,
      child: Container(
        constraints: const BoxConstraints(minHeight: 50),
        padding: const EdgeInsets.symmetric(horizontal: Dim.s4),
        decoration: BoxDecoration(
          color: c.surface, borderRadius: Dim.brCard,
          border: Border.all(color: selected ? c.brand : c.line, width: 1.5),
        ),
        child: Row(children: [
          if (icon != null) ...[Icon(icon, size: 25, color: c.ink2), const SizedBox(width: Dim.s3)],
          Expanded(child: Text(label, style: TextStyle(fontSize: Dim.t3, color: c.ink, height: 1.5))),
          ?trailing,
          const SizedBox(width: Dim.s3),
          Container(
            width: 19, height: 19,
            decoration: BoxDecoration(shape: BoxShape.circle, border: Border.all(color: selected ? c.brand : c.line, width: selected ? 6 : 1.5)),
          ),
        ]),
      ),
    );
  }
}
```

- [ ] **Step 4: 跑测试（需授权）** Expected: PASS
- [ ] **Step 5: Commit** `git commit -m "feat(app): align me page hero, notifications and report sheet with prototype"`

### Task 25: H2 钱包流水、H10 充值记录、H11 退款撤销

**Files:**
- Modify: `wallet_page.dart`、`pay_records_page.dart`、`voided_page.dart`
- Test: `test/layout/me_layout_test.dart`（追加）

| 原型要点 | 目标 |
|---|---|
| `.pcard` 余额卡上探 -22px→**-33**、padding s3 s4、r4、`0 6 20 .16` → `0 9 30`；金额 t7 800 tabular；单位 t1 ink3 | 复用 `WalletHeroCard(compact: true)` |
| `.ordr a` 行最小高 44、padding s2 0、底线 line2；`.oid` 订单号等宽 t1 ink3 sea 底 r1 padding 2/6；`.stt` 状态标 t0 800 padding 1.5/6 r1：ok 绿 .14 / ing aqua .14 / err warn .12 / rfd sea ink3 | 抽 `StatusTag(kind)`、`OrderIdChip(text)` 到 `chips.dart` |
| H11 | `.pst` 错误态版式（Task 26） | 复用 |

- [ ] **Step 1: 渲染原型（需授权）** `--screen H2 H10 H11`
- [ ] **Step 2: 追加测试**（三页）
- [ ] **Step 3: 实现** + 登记注释（列表页）
- [ ] **Step 4: 跑测试（需授权）** Expected: PASS
- [ ] **Step 5: Commit** `git commit -m "feat(app): align wallet, pay records and voided pages with prototype"`

### Task 26: H7a / H7b / H8 / H9 支付状态三态

**Files:**
- Create: `lib/ui/widgets/payment_status.dart`（从 `payment_flow_page.dart` 抽出 `.pst`）
- Modify: `payment_flow_page.dart`
- Test: `test/widgets/payment_status_test.dart`、`test/payment_flow_test.dart`（现有，确认仍过）

| 原型要点 | 目标 |
|---|---|
| `.pst` 居中列、padding 0 s5、gap s2；图标环 52px→**77** 圆，ok 绿 .14 底 + 绿 .45 边 2、err warn .12 底 + warn .45 边 2；`.spin` 30→**45** 圆环 line 底 aqua 顶 2.5→3；标题 t5 800；正文 t2 ink2 lh1.65；金额 `.amt2` 26px→**39** 800 gold；`.oid` 订单号 | `PaymentStatusView(kind: ok/err/pending, title, body, amount?, orderNo?, actions)`；转圈响应 `MediaQuery.disableAnimationsOf`（规范 §4.10 同源的 reduced-motion 要求） |

- [ ] **Step 1: 渲染原型（需授权）** `--screen H7a H8 H9`
- [ ] **Step 2: 写测试**

```dart
import 'package:bottles/core/design/theme.dart';
import 'package:bottles/ui/widgets/payment_status.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('status ring is 77pt and hides the spinner when animations are disabled', (t) async {
    await t.pumpWidget(MaterialApp(
      theme: AppTheme.light(),
      home: const MediaQuery(
        data: MediaQueryData(disableAnimations: true),
        child: Scaffold(body: PaymentStatusView(kind: PaymentStatusKind.pending, title: 't', body: 'b')),
      ),
    ));
    expect(t.getSize(find.byKey(const ValueKey('pst-ring'))).width, 77);
    expect(find.byType(CircularProgressIndicator), findsNothing);
  });
}
```

- [ ] **Step 3: 实现** + `payment_flow_page.dart` 六个屏改为组合 `PaymentStatusView`；状态机 `payment_controller.dart` **不动**（CLAUDE.md 关键机制）。
- [ ] **Step 4: 跑测试（需授权）** Run: `flutter test test/widgets/payment_status_test.dart test/payment_flow_test.dart` Expected: PASS
- [ ] **Step 5: Commit** `git commit -m "feat(app): extract payment status view and align with prototype"`

### Task 27: H3 充值、H3i IAP 商品列表、H5 / Z4 模态、H6 签到

**Files:**
- Modify: `recharge_page.dart`、`ui/flows/purchase_flows.dart`、`rewards_page.dart`
- Test: `test/layout/me_layout_test.dart`（追加）、`test/widgets/package_grid_test.dart`

| 屏 | 原型要点 | 目标 |
|---|---|---|
| H3 `.pkg` | 三列 gap s2；格 padding s3 0、描边 1.5 line、r3；选中 brand 描边 + 粉 .06 底；币数 t5 800 + 币图标；价格 t2 ink3、原价 `del` .7；`em` 角标 top -7px→**-10** brand t0 800 pill | 抽 `PackageTile`；**360 宽 + 1.2 缩放价格不换行**（Review Focus 5）：价格行 `FittedBox(fit: scaleDown)` |
| H3 `.pay` | `RadioRow`（Task 24） | 复用 |
| H3i | iOS 整屏替换：`ListGroup` 行 + 右侧价格；`.foot` 恢复购买 ghost | 平台分叉已有，只对齐行高 52 |
| H5 / Z4 `.modal` + `.offer` | `ModalCard`（Task 10）；`.offer` 推荐套餐行：描边 1.5 brand、r3、粉 .06 底、gap s3、右侧价格 t5 800 gold | `purchase_flows.dart` 三个 `show*Modal` 改 `showAppModal` |
| H6 `.bal` | 余额卡 padding s4 r4 描边 line2 粉紫渐变；数字 26→**39**；`.ck` 七格 gap 3px→**4.5**、最小高 32→**48**、r2、sea 底 t0 700；ok 绿 .2 / now aqua 白 + 阴影 / soon 虚线 | 抽 `CheckInStrip(days)` |

- [ ] **Step 1: 渲染原型（需授权）** `--screen H3 H3i H5 Z4 H6`
- [ ] **Step 2: 写测试**

```dart
import 'package:bottles/core/design/theme.dart';
import 'package:bottles/ui/widgets/cards.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('three package tiles fit 360 wide at text scale 1.2 without overflow', (t) async {
    t.view.devicePixelRatio = 1;
    t.view.physicalSize = const Size(360, 780);
    t.platformDispatcher.textScaleFactorTestValue = 1.2;
    addTearDown(() { t.view.reset(); t.platformDispatcher.clearTextScaleFactorTestValue(); });
    await t.pumpWidget(MaterialApp(
      theme: AppTheme.light(),
      home: Scaffold(body: Padding(
        padding: const EdgeInsets.all(16),
        child: Row(children: [
          for (final p in [(60, '¥6', null, null), (600, '¥59.9', '¥79', '最划算'), (3000, '¥299', null, '送 300')]) ...[
            Expanded(child: PackageTile(coins: p.$1, price: p.$2, original: p.$3, badge: p.$4, selected: p.$1 == 600, onTap: () {})),
            const SizedBox(width: 8),
          ],
        ]),
      )),
    ));
    expect(t.takeException(), isNull);
  });
}
```

- [ ] **Step 3: 实现** + 登记注释
- [ ] **Step 4: 跑测试（需授权）** Expected: PASS
- [ ] **Step 5: Commit** `git commit -m "feat(app): align recharge packages, quota modals and rewards with prototype"`

### Task 28: 状态兜底 Z5 / Z6 / Z8 与 G1 媒体上传

**Files:**
- Modify: `states.dart`（`FailureState.offline` 图标）、`core/media/*`（上传失败态回调）、`location/location_intro_page.dart`（Z6 已拒绝分支）
- Test: `test/widgets/states_test.dart`（追加）

| 屏 | 原型要点 | 目标 |
|---|---|---|
| Z5 网络断了 | `.empty2` + 「重试」`.cta` | `FailureState(offline: true)` 图标 `Icons.wifi_off_rounded` |
| Z6 定位被拒 | `.empty2` + 「去设置开启」+ 「先逛逛」 | `EmptyState(ctaLabel, altLabel)`；`canAsk == false` 时不给「开启」按钮（shell.dart 已有的规则） |
| Z8 上传失败 | 缩略图上 `.ovl` 错误遮罩 + 重试 | `PhotoGrid(failed: Set<int>, onRetry)` 加参数 |
| G1 | 上传中 `.ovl` 遮罩 | `PhotoGrid.uploading` 已有 |

- [ ] **Step 1: 渲染原型（需授权）** `--screen Z5 Z6 Z8`
- [ ] **Step 2: 追加测试**（`FailureState(offline: true)` 找到 `Icons.wifi_off_rounded`）
- [ ] **Step 3: 实现**
- [ ] **Step 4: 跑测试（需授权）** Expected: PASS
- [ ] **Step 5: Commit** `git commit -m "feat(app): align fallback states with prototype"`

---

## Phase 9：收口

### Task 29: 全量矩阵与真机核对清单

**Files:**
- Create: `app/bottles/test/layout/all_pages_test.dart`（把各分节测试文件在一个入口里 `import` 并调用其 `main()`，方便一条命令跑全量）
- Modify: `docs/superpowers/specs/2026-09-30-app-layout-adaptation-design.md` §8.2「现状」段落
- Modify: `app/bottles/README.md`（加 `tool/` 与布局测试说明）

- [ ] **Step 1: 写入口**

```dart
import 'auth_layout_test.dart' as auth;
import 'bottle_layout_test.dart' as bottle;
import 'chat_layout_test.dart' as chat;
import 'discover_layout_test.dart' as discover;
import 'me_layout_test.dart' as me;
import 'moment_layout_test.dart' as moment;

void main() {
  auth.main();
  bottle.main();
  chat.main();
  discover.main();
  me.main();
  moment.main();
}
```

- [ ] **Step 2: 跑全量（需授权）** Run: `cd app/bottles && flutter analyze && flutter test` Expected: analyze 无 issue；test 全 PASS。
- [ ] **Step 3: 真机清单**（规范 §8.3）：请用户在小米 9（标准字体 + 大字体各一次）上过一遍全部页面并截图到 `漂流瓶/效果图/`，主 agent 用 `tool/compare.py` 逐页并排，偏差 > 4pt 的记到本计划末尾「返工清单」再修。
- [ ] **Step 4: 规范 §8.2 现状段改为**「布局矩阵覆盖全部页面（`test/layout/all_pages_test.dart`），海面四组合有 golden；其余 golden 待补」。
- [ ] **Step 5: Commit** `git commit -m "test(app): run the device matrix over every page"`

### Task 30: 复盘归档

**Files:**
- Modify: `docs/APP_V1_PROGRESS.md`（新增「UI 对齐」小节：做了什么、组件清单、比对流程）
- Modify: `docs/prototype/v2-screens.html`（每屏 `.tp` 里补「伸缩规则」一条，格式同 A2）

- [ ] **Step 1: 写进度文档**：列出本计划新增的组件（`Shadows`、`HeaderStats`、`QuadGrid`、`SelectField`、`RadioRow`、`StatusTag`、`OrderIdChip`、`PackageTile`、`CheckInStrip`、`DesignCanvas`、`MessageBubble` 等）与每个页面的登记类别。
- [ ] **Step 2: 原型补注**：按 §6 表给每屏 `.tp` 加 `<dt>伸缩规则</dt>`，内容与页面文件头登记注释一致。
- [ ] **Step 3: Commit** `git commit -m "docs: record the UI alignment pass and per-screen scaling rules"`

---

## 返工清单

（执行过程中真机比对发现偏差 > 4pt 的项记在这里，修完划掉。）

### 2026-10-04 · 小米 9 第二批截图（B2 / B3 / M1 / C2 / C3 / F1 / F2）

比对图：`效果图/compare_<屏>.png`（`tool/compare.py`，原型按 393×851 渲染）。

- [x] **全部六个二级页带着 Tab 栏**：子路由都在 `StatefulShellRoute` 分支里渲染。→ 子路由全部 `parentNavigatorKey: _rootKey`，`test/app/router_test.dart` 守。
- [x] **B2 / C2 标签成了整宽胶囊**：`PillChip` 的 `Container(alignment:)` 在 `Wrap` 里撑满。→ `Center(widthFactor: 1)`。
- [x] **F1 Tab 胶囊居中**：`ChipBar` 横向 ScrollView 在 `Column` 里按内容收缩后被居中。→ `width: double.infinity`。
- [x] **F1 动态图片两列大图**：原型 `.pics` 一行 flex。→ 2～3 张一行等分，4 张起 3 列。「送礼」改 ink3。
- [x] **F1 头图高 12pt**：slim 头图顶行仍按 44 撑。→ slim 时 34。
- [x] **F2 发布在底部 + 缺「谁可以看」**：→ 导航栏右侧文字动作；公开 / 仅自己（后端只认这两个），`visible` 随请求。
- [x] **C3 名字与统计之间 61pt 空档**：空串 bio 占了一行。→ 标签 / 简介 / 共同点都「有内容才占位」。左键「魅力」→「送礼」，底栏去分割线（`BottomActionBar(flat:)`），⋯ 与 ← 同为 36 白圆（`IconCircleButton`）。
- [x] **C2 年龄摊着整条滑块**：原型是值字段。→ `ValueField` + 弹层改区间；答题小标题并成一行。
- [x] **M1 返回箭头 / 地址横幅**：原型 ✕ + `.lst` 卡。→ `closeIcon` + `ListGroup/ListRowItem`，定位钮换 `IconCircleButton`。
- [x] **B3「漂了 N 天」粉色**：原型 `.tg` 默认 sea 底。→ `TagTone.plain`。
- [ ] 待真机复核：以上七屏 + 受全局改动影响的会话列表 / 我的瓶子 / 关系 / 钱包（ChipBar）。

### 2026-10-04 · 第三轮口头反馈（装机后）

- [x] **「发布」「全部已读」没贴右沿**：NavBar 把副标 / actions 放进 `Flexible`，Row 先按 flex 均分，空隙留在右边。→ 两者按内容定宽（上限 35% / 45%）+ `FittedBox` 缩放，标题是唯一让步的。
- [x] **发现 / 消息页胶囊行压在头图上**：`ChipBar(raise:)` 上提 18；原型 `.chs.raise` 其实没有负边距。→ 删掉 raise，胶囊行在头图下方留 s2。
- [x] **发现页金币角标挂错键**：花金币的是「找回划走的人」。→ 撤回键挂价格角标；✕ 只有后台开了 `app_discover_skip_charge_enabled`（默认 0）才带价。
- [x] **我的页钱包卡盖住关注 / 粉丝 / 魅力**：`Positioned(bottom: 0)` 让上探量 = 卡高 - 39。→ 占位 + `bottom: 39`，上探恒为 39。
- [x] **二级页底部没留白**：底栏 / 弹层 / 输入栏用 `max(安全区, s4)`，手势导航机型上按钮骑在手势条上；无底栏的列表页最后一项贴屏底。→ 一律 `s4 + 安全区`；列表页用 `context.pagePadding()`（底部 gutter + 安全区）。

### 2026-10-04 · 第四轮（截图 6 张 + 口头反馈）

- [x] **礼物面板**：手里有 ×1 仍标全价、算余额。→ 服务端 chat / moment 送礼统一「背包先抵扣、差额扣币」，每件记 ItemOrder(target)；客户端只对差额标价，首帧先定选中项再算抵扣；数量下拉 ≥ 84 宽；格子比例 1.2。
- [x] **瓶子详情整瓶解锁**：→ 按条解锁（`/bottle/reply/:rid/unlock`），每条锁着的回信挂「解锁 · 价」小标；「匿名 · 0」改按有无性别 / 年龄拼接。
- [x] **我的页「充值」是个圆**：`MiniButton` 最窄 1.5 倍高。
- [x] **钱包余额卡缩在中间 / 充值档位卡缩成一小块**：通栏 + 铺满格宽，内容整体可缩。
- [x] **资料页「送礼」只弹 toast 没发**：走和 TA 的现有会话真发，没聊过提示先打招呼。
- [x] **聊天「余额不足」提示却不扣费**：按条收费（`price_msg`）为 0 时不再发低余额系统提示。规则本身见服务端：开聊扣 `price_chat`、每条扣 `price_msg`（默认 0）。
- [x] **聊天扣费规则**（用户拍板：开聊扣 M、每条扣 N、前 L 条免费，全部后台配）：新增 `chat_free_msgs`，三键并入「价格」分组；服务端按会话数发送方已发条数判免费；`/app-config` 下发 `pricing`，App 打招呼按钮与聊天页提示按配置显示（之前写死 5）。
- [ ] 「还差 N 金币」补差折扣：仅规划，见会话记录；待拍板后立项。

---

## 执行节奏建议

- Phase 0 → 1 一口气做完再让用户装机：组件层改动会同时影响所有页面，分批看会反复。
- Phase 2 海面单独一次装机验证：它是最难对齐的一屏，也是 `DesignCanvas` 的首个案例。
- Phase 3～8 每个 Phase 结束装一次机，用户按 §8.3 截图，主 agent 并排比对。
- 每个任务的 `Run:` 都需要用户授权；建议每个 Phase 开始时一次性授权该 Phase 内的 `flutter analyze` / `flutter test` / `tool/*.py`，减少往返。
