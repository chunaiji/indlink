"""真机截图 vs 原型真实 pt 渲染，同尺寸并排，2x 放大，每 100dp 一条参考线。

    python tool/compare.py --real 漂流瓶/效果图/xxx.jpg \
        --proto 漂流瓶/效果图/proto/proto_B1_393x851.png \
        --out 漂流瓶/效果图/compare_B1.png
"""
import argparse

from PIL import Image, ImageDraw, ImageFont

SCALE, GAP, HEAD = 2, 24, 40


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument('--real', required=True, help='真机截图，任意分辨率')
    ap.add_argument('--proto', required=True, help='proto_render.py 的输出，像素数即逻辑尺寸')
    ap.add_argument('--out', required=True)
    ap.add_argument('--label-real', default='真机')
    ap.add_argument('--label-proto', default='原型真实 pt 渲染')
    a = ap.parse_args()

    proto = Image.open(a.proto).convert('RGB')
    w, h = proto.size
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
