# -*- coding: utf-8 -*-
"""生成底部 tabBar「统计」图标（纯标准库，无第三方依赖）。

输出：uni-app-frontend/static/tabbar/stats.png（未选中，灰 #999999）
      uni-app-frontend/static/tabbar/stats-active.png（选中，苍岭绿 #0F766E）
尺寸：81x81 RGBA，与既有 home/order/profile 图标一致。
图形：三根递增柱状条 + 基线（柱状图语义）。
"""
import os
import struct
import zlib

SIZE = 81


def blank():
    return [[(0, 0, 0, 0) for _ in range(SIZE)] for _ in range(SIZE)]


def rect(px, x0, y0, x1, y1, color):
    for y in range(y0, y1):
        for x in range(x0, x1):
            if 0 <= x < SIZE and 0 <= y < SIZE:
                px[y][x] = color


def write_png(path, px):
    raw = bytearray()
    for row in px:
        raw.append(0)
        for r, g, b, a in row:
            raw += bytes((r, g, b, a))

    def chunk(tag, data):
        c = struct.pack('>I', len(data)) + tag + data
        return c + struct.pack('>I', zlib.crc32(tag + data) & 0xFFFFFFFF)

    ihdr = struct.pack('>IIBBBBB', SIZE, SIZE, 8, 6, 0, 0, 0)
    png = b'\x89PNG\r\n\x1a\n' + chunk(b'IHDR', ihdr)
    png += chunk(b'IDAT', zlib.compress(bytes(raw), 9))
    png += chunk(b'IEND', b'')
    with open(path, 'wb') as f:
        f.write(png)


def build(color):
    px = blank()
    # 三根柱子：高度递增
    bars = [(16, 46), (33, 32), (50, 18)]  # (x0, top_y)
    for x0, top in bars:
        rect(px, x0, top, x0 + 13, 62, color)
    # 基线
    rect(px, 12, 64, 69, 67, color)
    return px


def main():
    base = os.path.join(os.path.dirname(os.path.abspath(__file__)), '..', 'uni-app-frontend', 'static', 'tabbar')
    base = os.path.normpath(base)
    write_png(os.path.join(base, 'stats.png'), build((153, 153, 153, 255)))
    write_png(os.path.join(base, 'stats-active.png'), build((15, 118, 110, 255)))
    print('written to', base)


if __name__ == '__main__':
    main()
