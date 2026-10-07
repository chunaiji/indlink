"""MiSans 子集化（规范 §4.6、计划 Task 12）。

把 MiSans 官方包里的 MiSans-Regular.ttf / MiSans-Medium.ttf / MiSans-Bold.ttf
放到 tool/fonts_src/（不入库），运行后输出到 assets/fonts/，每字重应在 4MB 内。

    pip install fonttools brotli
    python tool/subset_fonts.py

子集范围：Latin + 常用标点 + 货币符号 + 箭头 + CJK 标点 + 全角 + 基本汉字区。
"""
import subprocess
from pathlib import Path

SRC = Path(__file__).parent / 'fonts_src'
OUT = Path(__file__).parents[1] / 'assets' / 'fonts'
UNICODES = ','.join([
    'U+0000-00FF',  # Basic Latin + Latin-1
    'U+0100-017F',  # Latin Extended-A
    'U+2000-206F',  # 常用标点
    'U+20A0-20CF',  # 货币符号（₹ ¥ € $）
    'U+2190-21FF',  # 箭头
    'U+3000-303F',  # CJK 标点
    'U+FF00-FFEF',  # 全角
    'U+4E00-9FFF',  # 基本汉字
])


def main() -> None:
    OUT.mkdir(parents=True, exist_ok=True)
    for weight in ['Regular', 'Medium', 'Bold']:
        src = SRC / f'MiSans-{weight}.ttf'
        if not src.exists():
            raise SystemExit(f'missing {src}: put the official MiSans TTFs in tool/fonts_src/')
        dst = OUT / f'MiSans-{weight}.ttf'
        subprocess.run([
            'pyftsubset', str(src), f'--unicodes={UNICODES}',
            '--layout-features=*', '--no-hinting', f'--output-file={dst}',
        ], check=True)
        print(weight, dst.stat().st_size // 1024, 'KB')


if __name__ == '__main__':
    main()
