#!/usr/bin/env python3
"""
Utility script to turn the ANSI art in assets/banner_color.ansi into a retina-friendly PNG
for documentation. Re-run whenever the banner changes.
"""

from __future__ import annotations

import re
from pathlib import Path

from PIL import Image, ImageDraw, ImageFont

ROOT = Path(__file__).resolve().parent
ANSI_PATH = ROOT / "assets" / "banner_color.ansi"
OUTPUT_PATH = ROOT / "assets" / "banner_preview.png"


def parse_ansi(text: str) -> list[list[tuple[str, tuple[int, int, int]]]]:
    """Return list of lines containing (character, rgb-color) tuples."""
    pattern = re.compile(r"\x1b\[([0-9;]+)m")
    parsed: list[list[tuple[str, tuple[int, int, int]]]] = []
    current_color = (255, 255, 255)

    for raw_line in text.splitlines():
        line: list[tuple[str, tuple[int, int, int]]] = []
        idx = 0
        while idx < len(raw_line):
            if raw_line[idx] == "\x1b":
                match = pattern.match(raw_line, idx)
                if match:
                    seq = match.group(1).split(";")
                    idx = match.end()
                    if seq == ["0"]:
                        current_color = (255, 255, 255)
                    elif len(seq) >= 5 and seq[0] == "38" and seq[1] == "2":
                        current_color = tuple(map(int, seq[2:5]))  # type: ignore[arg-type]
                    continue
                idx += 1
                continue

            line.append((raw_line[idx], current_color))
            idx += 1
        parsed.append(line)
    return parsed


def render(parsed: list[list[tuple[str, tuple[int, int, int]]]]) -> Image.Image:
    font = ImageFont.truetype("DejaVuSansMono.ttf", 24)
    glyph_width = font.getbbox("M")[2]
    glyph_height = font.getbbox("M")[3]

    margin = 24
    cols = max(len(line) for line in parsed)
    rows = len(parsed)

    img = Image.new(
        "RGB",
        (margin * 2 + glyph_width * cols, margin * 2 + glyph_height * rows),
        "#111218",
    )
    draw = ImageDraw.Draw(img)

    for y, line in enumerate(parsed):
        for x, (ch, color) in enumerate(line):
            draw.text(
                (margin + x * glyph_width, margin + y * glyph_height),
                ch,
                font=font,
                fill=color,
            )
    return img


def main() -> None:
    parsed = parse_ansi(ANSI_PATH.read_text())
    image = render(parsed)
    image.save(OUTPUT_PATH, dpi=(220, 220))
    print(f"Updated {OUTPUT_PATH.relative_to(ROOT)}")


if __name__ == "__main__":
    main()
