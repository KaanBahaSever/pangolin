"""Puts raw Pangolin captures in a window frame for the website and README.

    SHOT_DIR=shots go test -tags screenshots ./internal/ui -run TestScreenshots
    python tools/screenshots/frame.py shots docs

Reads shots/raw-<lang>-<theme>-<screen>.png (rendered at 2x by
internal/ui/screenshots_test.go) and writes docs/screenshot-<lang>-<theme>.png
from the "detail" screen. Needs Pillow.
"""

import os
import sys

from PIL import Image, ImageDraw, ImageFilter, ImageFont

SCALE = 2
TITLE_BAR = 38 * SCALE
RADIUS = 12 * SCALE
MARGIN = 36 * SCALE  # room for the shadow
SCREEN = "detail"

# Title bar colours, a shade off each theme's background.
BAR = {"light": (234, 226, 210), "dark": (12, 16, 33)}
TITLE = {"light": (95, 103, 128), "dark": (163, 172, 198)}
OUTLINE = {"light": (214, 204, 186), "dark": (41, 49, 80)}
LIGHTS = [(255, 95, 87), (254, 188, 46), (40, 200, 64)]

FONTS = [
    "C:/Windows/Fonts/segoeuib.ttf",
    "/System/Library/Fonts/SFNS.ttf",
    "/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf",
]


def font(size):
    for path in FONTS:
        if os.path.exists(path):
            return ImageFont.truetype(path, size)
    return ImageFont.load_default()


def rounded_mask(size, radius):
    mask = Image.new("L", size, 0)
    ImageDraw.Draw(mask).rounded_rectangle((0, 0, size[0] - 1, size[1] - 1), radius, fill=255)
    return mask


def frame(raw, theme, icon):
    width, height = raw.width, raw.height + TITLE_BAR
    window = Image.new("RGBA", (width, height), BAR[theme] + (255,))
    window.paste(raw.convert("RGBA"), (0, TITLE_BAR))

    draw = ImageDraw.Draw(window)
    # Window buttons on the left.
    r, y = 6 * SCALE, TITLE_BAR // 2
    for i, colour in enumerate(LIGHTS):
        x = 20 * SCALE + i * 20 * SCALE
        draw.ellipse((x - r, y - r, x + r, y + r), fill=colour)
    # Icon and title in the middle.
    title_font = font(13 * SCALE)
    text = "Pangolin"
    text_width = draw.textlength(text, font=title_font)
    icon_size = 18 * SCALE
    gap = 7 * SCALE
    left = (width - (icon_size + gap + text_width)) / 2
    small = icon.resize((icon_size, icon_size), Image.LANCZOS)
    window.alpha_composite(small, (int(left), y - icon_size // 2))
    draw.text((left + icon_size + gap, y), text, font=title_font, fill=TITLE[theme], anchor="lm")
    # A hairline between the title bar and the content.
    draw.line((0, TITLE_BAR - 1, width, TITLE_BAR - 1), fill=OUTLINE[theme], width=1)

    window.putalpha(rounded_mask(window.size, RADIUS))
    outline = Image.new("RGBA", window.size, (0, 0, 0, 0))
    ImageDraw.Draw(outline).rounded_rectangle((0, 0, width - 1, height - 1), RADIUS,
                                              outline=OUTLINE[theme] + (255,), width=SCALE)
    window.alpha_composite(outline)

    # Soft shadow beneath, on a transparent canvas.
    canvas = Image.new("RGBA", (width + 2 * MARGIN, height + 2 * MARGIN), (0, 0, 0, 0))
    shadow = Image.new("RGBA", canvas.size, (0, 0, 0, 0))
    ImageDraw.Draw(shadow).rounded_rectangle(
        (MARGIN, MARGIN + 10 * SCALE, MARGIN + width, MARGIN + height + 10 * SCALE),
        RADIUS, fill=(10, 14, 30, 90))
    shadow = shadow.filter(ImageFilter.GaussianBlur(14 * SCALE))
    canvas.alpha_composite(shadow)
    canvas.alpha_composite(window, (MARGIN, MARGIN))
    return canvas


def main():
    if len(sys.argv) != 3:
        sys.exit("usage: frame.py <raw captures dir> <output dir>")
    src, dst = sys.argv[1], sys.argv[2]
    root = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
    icon = Image.open(os.path.join(root, "assets", "icon.png")).convert("RGBA")
    for lang in ("en", "tr"):
        for theme in ("light", "dark"):
            raw = Image.open(os.path.join(src, f"raw-{lang}-{theme}-{SCREEN}.png"))
            out = os.path.join(dst, f"screenshot-{lang}-{theme}.png")
            frame(raw, theme, icon).save(out, optimize=True)
            print(out)


if __name__ == "__main__":
    main()
