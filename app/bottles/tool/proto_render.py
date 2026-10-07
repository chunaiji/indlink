"""把原型某一屏抽出来，按目标机型的逻辑尺寸真实渲染（规范 §8.1）。

原型手机框 252px = 375pt（0.672），这里反过来 zoom 1.488，
框做成 W/1.488 x H/1.488 CSS px，zoom 后正好 W x H。

    python tool/proto_render.py --screen B1 --size 393x851
    python tool/proto_render.py --screen A2 --size 360x780 --hide-apple
"""
import argparse
import io
import re
import shutil
import subprocess
import tempfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]
SRC = ROOT / 'docs' / 'prototype' / 'v2-screens.html'
EDGE = r'C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe'
ZOOM = 375 / 252


def extract(html: str, screen_id: str) -> str:
    cap = re.search(
        r'<div class="cap"><b>[^<]*</b><i>' + re.escape(screen_id) + r'(?: ·[^<]*)?</i>', html)
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
    ap.add_argument('--screen', required=True, help='原型 cap 里的 ID，如 A2 / B1 / H4')
    ap.add_argument('--size', default='393x851')
    ap.add_argument('--hide-apple', action='store_true', help='Android 没有 Apple 登录')
    ap.add_argument('--out', default=str(ROOT / '漂流瓶' / '效果图' / 'proto'))
    a = ap.parse_args()
    w, h = (int(x) for x in a.size.split('x'))
    # 必须是绝对路径：相对路径拼成 file:///../x.html 后 Edge 导航失败且永不退出。
    out = Path(a.out).resolve()
    out.mkdir(parents=True, exist_ok=True)
    page = out / f'proto_{a.screen}_{a.size}.html'
    png = out / f'proto_{a.screen}_{a.size}.png'
    io.open(page, 'w', encoding='utf-8').write(build(a.screen, w, h, a.hide_apple))
    # 旧版 --headless + 一次性 user-data-dir：复用目录时，上一只挂死的 Edge 握着 SingletonLock，新实例会把活交给它然后一起挂住。
    # 三路都接 DEVNULL：stdin 不接 Edge 会挂住；stdout/stderr 接管道则子进程继承后 capture_output 等到超时。
    profile = Path(tempfile.mkdtemp(prefix='proto_render_'))
    try:
        subprocess.run([EDGE, '--headless', '--disable-gpu', '--hide-scrollbars',
                        '--no-first-run', '--no-default-browser-check',
                        f'--user-data-dir={profile}',
                        f'--window-size={w},{h}', '--virtual-time-budget=3000',
                        f'--screenshot={png}', f'file:///{page}'],
                       check=False, stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL,
                       stderr=subprocess.DEVNULL, timeout=60)
    except subprocess.TimeoutExpired:
        raise SystemExit(f'edge timed out rendering {a.screen}; close stray msedge processes and retry')
    finally:
        shutil.rmtree(profile, ignore_errors=True)
    if not png.exists():
        raise SystemExit(f'edge produced no screenshot for {a.screen}')
    print(png)


if __name__ == '__main__':
    main()
