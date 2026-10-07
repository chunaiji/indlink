# tool/

UI 比对与构建辅助脚本。依赖 Python 3 + Pillow，渲染依赖本机 Edge（无头模式）。

## proto_render.py

把原型某一屏按真机逻辑尺寸渲染成 PNG，是所有 UI 比对的基准（规范 §8.1）。
原型手机框 252px = 375pt，脚本按 1.488 放大，输出的 PNG 像素数就是逻辑像素数。

    python tool/proto_render.py --screen B1 --size 393x851
    python tool/proto_render.py --screen A2 --size 360x780 --hide-apple

输出到 `漂流瓶/效果图/proto/proto_<屏>_<尺寸>.png`。`--screen` 取原型每屏标题右侧的 ID。

## compare.py

真机截图与原型渲染并排，每 100dp 一条参考线。真机截图任意分辨率都行，会按原型渲染的尺寸缩放。

    python tool/compare.py --real 漂流瓶/效果图/xxx.jpg --proto 漂流瓶/效果图/proto/proto_B1_393x851.png --out 漂流瓶/效果图/compare_B1.png

## 常用机型逻辑尺寸

| 机型 | 尺寸 |
|---|---|
| Redmi 720p / 三星默认密度 | 360x780 |
| iPhone SE | 375x667 |
| 小米 9 / iPhone 15 | 393x851 |
| iPhone Pro Max | 430x932 |
| 折叠屏展开 | 720x860 |
