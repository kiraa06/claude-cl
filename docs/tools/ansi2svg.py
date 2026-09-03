"""Convert a captured ANSI frame into a standalone SVG terminal window.

Renders with explicit fill attributes and no CSS or scripting, so the result
survives GitHub's SVG sanitising when referenced from a README.
"""
import re
import sys
from html import escape


def xterm256():
    """The xterm 256-colour palette."""
    base = [
        (0, 0, 0), (205, 0, 0), (0, 205, 0), (205, 205, 0),
        (0, 0, 238), (205, 0, 205), (0, 205, 205), (229, 229, 229),
        (127, 127, 127), (255, 0, 0), (0, 255, 0), (255, 255, 0),
        (92, 92, 255), (255, 0, 255), (0, 255, 255), (255, 255, 255),
    ]
    palette = list(base)
    steps = [0, 95, 135, 175, 215, 255]
    for r in steps:
        for g in steps:
            for b in steps:
                palette.append((r, g, b))
    for i in range(24):
        v = 8 + i * 10
        palette.append((v, v, v))
    return palette


PALETTE = xterm256()
SGR = re.compile(r"\x1b\[([0-9;]*)m")
# Non-SGR escapes only: the character class deliberately excludes "m" so this
# does not eat the colour sequences the parser below depends on.
OTHER_ESC = re.compile(r"\x1b\[[0-9;?]*[a-ln-zA-LN-Z]|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)")

FG_DEFAULT = "#c9d1d9"
BG = "#0d1117"
CHROME = "#161b22"


def parse(text):
    """Yield [(text, fill, bold), ...] per line."""
    lines = []
    fill, bold = FG_DEFAULT, False
    for raw in text.split("\n"):
        raw = OTHER_ESC.sub("", raw)
        spans, pos = [], 0
        for m in SGR.finditer(raw):
            if m.start() > pos:
                spans.append((raw[pos:m.start()], fill, bold))
            codes = [c for c in m.group(1).split(";") if c != ""] or ["0"]
            i = 0
            while i < len(codes):
                c = int(codes[i])
                if c == 0:
                    fill, bold = FG_DEFAULT, False
                elif c == 1:
                    bold = True
                elif c == 22:
                    bold = False
                elif c == 39:
                    fill = FG_DEFAULT
                elif c == 38 and i + 2 < len(codes) and codes[i + 1] == "5":
                    r, g, b = PALETTE[int(codes[i + 2]) % 256]
                    fill = "#%02x%02x%02x" % (r, g, b)
                    i += 2
                elif 30 <= c <= 37:
                    r, g, b = PALETTE[c - 30]
                    fill = "#%02x%02x%02x" % (r, g, b)
                elif 90 <= c <= 97:
                    r, g, b = PALETTE[c - 90 + 8]
                    fill = "#%02x%02x%02x" % (r, g, b)
                i += 1
            pos = m.end()
        if pos < len(raw):
            spans.append((raw[pos:], fill, bold))
        lines.append(spans)
    return lines


def render(lines, title):
    cw, lh = 7.8, 17.0          # character cell width and line height
    pad_x, pad_y = 18, 46       # room for the window chrome
    cols = max((sum(len(t) for t, _, _ in ln) for ln in lines), default=80)
    width = int(cols * cw + pad_x * 2)
    height = int(len(lines) * lh + pad_y + 18)

    out = [
        f'<svg xmlns="http://www.w3.org/2000/svg" width="{width}" height="{height}" '
        f'viewBox="0 0 {width} {height}" font-family="ui-monospace,SFMono-Regular,'
        'Menlo,Consolas,monospace" font-size="12.5">',
        f'<rect width="{width}" height="{height}" rx="10" fill="{BG}"/>',
        f'<path d="M0 10a10 10 0 0 1 10-10h{width - 20}a10 10 0 0 1 10 10v22H0z" fill="{CHROME}"/>',
    ]
    for i, colour in enumerate(("#ff5f56", "#ffbd2e", "#27c93f")):
        out.append(f'<circle cx="{20 + i * 18}" cy="16" r="5.5" fill="{colour}"/>')
    out.append(
        f'<text x="{width / 2}" y="20.5" fill="#8b949e" font-size="11.5" '
        f'text-anchor="middle">{escape(title)}</text>'
    )

    for row, spans in enumerate(lines):
        y = pad_y + row * lh
        col = 0
        for text, fill, bold in spans:
            if text.strip():
                weight = ' font-weight="600"' if bold else ""
                out.append(
                    f'<text x="{pad_x + col * cw:.1f}" y="{y:.1f}" fill="{fill}"'
                    f'{weight} textLength="{len(text) * cw:.1f}"'
                    ' lengthAdjust="spacingAndGlyphs"'
                    f' xml:space="preserve">{escape(text)}</text>'
                )
            col += len(text)
    out.append("</svg>")
    return "\n".join(out)


if __name__ == "__main__":
    src, dst, title = sys.argv[1], sys.argv[2], sys.argv[3]
    frame = open(src, encoding="utf-8").read().rstrip("\n")
    with open(dst, "w", encoding="utf-8") as f:
        f.write(render(parse(frame), title))
    print("wrote", dst)
