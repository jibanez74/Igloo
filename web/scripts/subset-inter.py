#!/usr/bin/env python3
"""Split Inter's variable font into unicode-range subsets.

The full InterVariable.woff2 is 352 KB, and every page paid for all of it,
though the UI's own text needs only Latin. Split by script, the browser
fetches a subset only when a page shows one of its characters: the Latin
file for the app's own text, the others when a title needs them. Every
codepoint the font maps lands in exactly one subset, so nothing Inter draws
is lost; the last subset holds whatever the named scripts leave.

Run from the repository root with fonttools and brotli installed (a venv is
enough: python3 -m venv /tmp/ft && /tmp/ft/bin/pip install fonttools brotli),
passing the InterVariable.woff2 of an Inter release
(https://github.com/rsms/inter/releases, currently 4.001):

    /tmp/ft/bin/python web/scripts/subset-inter.py ~/Downloads/InterVariable.woff2

It writes web/src/assets/fonts/inter-<subset>.woff2 and inter.css, the
@font-face rules boot.css imports. Commit all of them together.
"""

import sys
from pathlib import Path

from fontTools import subset
from fontTools.ttLib import TTFont

OUT_DIR = Path(__file__).resolve().parent.parent / "src" / "assets" / "fonts"

# The OpenType features kept: those browsers apply by default, so the text
# shapes exactly as it did with the full font, plus tnum, the one optional
# feature the UI turns on (tabular-nums). Every other feature (fractions,
# case forms, stylistic sets) pulls in alternate glyphs nothing asks for.
# Add a feature here before the UI uses it.
FEATURES = [
    "rvrn",
    "ccmp",
    "locl",
    "mark",
    "mkmk",
    "kern",
    "calt",
    "liga",
    "clig",
    "rlig",
    "rclt",
    "curs",
    "tnum",
]

# In priority order: a codepoint goes to the first subset whose ranges hold
# it. The Latin subset carries everything the UI's own strings use (general
# punctuation, arrows, the minus and comparison signs) and what precomposed
# Western European text needs. The combining marks (U+0300-036F) go with
# latin-ext: they would add ~28 KB to every page for the rare decomposed
# title. Vietnamese is Latin Extended, so it rides with latin-ext too.
SUBSETS = [
    (
        "latin",
        [
            (0x0000, 0x00FF),
            (0x0131, 0x0131),
            (0x0152, 0x0153),
            (0x02BB, 0x02BC),
            (0x02C6, 0x02C6),
            (0x02DA, 0x02DA),
            (0x02DC, 0x02DC),
            (0x2000, 0x206F),
            (0x20AC, 0x20AC),
            (0x2122, 0x2122),
            (0x2190, 0x2199),
            (0x2212, 0x2212),
            (0x2215, 0x2215),
            (0x2264, 0x2265),
            (0xFEFF, 0xFEFF),
            (0xFFFD, 0xFFFD),
        ],
    ),
    (
        "latin-ext",
        [
            (0x0100, 0x036F),
            (0x1D00, 0x1DBF),
            (0x1E00, 0x1EFF),
            (0x20A0, 0x20CF),
            (0x2113, 0x2113),
            (0x2C60, 0x2C7F),
            (0xA720, 0xA7FF),
        ],
    ),
    (
        "cyrillic",
        [
            (0x0400, 0x052F),
            (0x1C80, 0x1C8F),
            (0x2116, 0x2116),
            (0x2DE0, 0x2DFF),
            (0xA640, 0xA69F),
            (0xFE2E, 0xFE2F),
        ],
    ),
    ("greek", [(0x0370, 0x03FF), (0x1F00, 0x1FFF)]),
    # Whatever is left: symbols, arrows, math, box drawing.
    ("symbols", [(0x0000, 0x10FFFF)]),
]


def partition(codepoints):
    """Assigns each codepoint to the first subset whose ranges hold it."""
    taken = set()
    parts = []
    for name, ranges in SUBSETS:
        part = sorted(
            cp
            for cp in codepoints
            if cp not in taken and any(lo <= cp <= hi for lo, hi in ranges)
        )
        taken.update(part)
        parts.append((name, part))
    return parts


def unicode_range(codepoints):
    """Compacts sorted codepoints into a CSS unicode-range value."""
    spans = []
    start = prev = codepoints[0]
    for cp in codepoints[1:]:
        if cp == prev + 1:
            prev = cp
            continue
        spans.append((start, prev))
        start = prev = cp
    spans.append((start, prev))
    return ", ".join(
        f"U+{lo:04X}" if lo == hi else f"U+{lo:04X}-{hi:04X}" for lo, hi in spans
    )


def write_subset(source, codepoints, path):
    options = subset.Options()
    options.flavor = "woff2"
    # Variation tables (the wght and opsz axes) are kept by default.
    options.layout_features = FEATURES
    options.name_IDs = ["*"]
    options.name_languages = ["*"]
    options.notdef_outline = True
    font = TTFont(source)
    subsetter = subset.Subsetter(options)
    subsetter.populate(unicodes=codepoints)
    subsetter.subset(font)
    font.flavor = "woff2"
    font.save(path)


def main():
    if len(sys.argv) != 2:
        sys.exit(__doc__)
    source = Path(sys.argv[1])
    codepoints = set(TTFont(source).getBestCmap())

    OUT_DIR.mkdir(parents=True, exist_ok=True)
    rules = []
    for name, part in partition(codepoints):
        if not part:
            continue
        filename = f"inter-{name}.woff2"
        write_subset(source, part, OUT_DIR / filename)
        size = (OUT_DIR / filename).stat().st_size
        print(f"{filename}: {len(part)} codepoints, {size / 1024:.1f} KB")
        rules.append(
            "@font-face {\n"
            "  font-family: Inter;\n"
            "  font-style: normal;\n"
            "  font-weight: 100 900;\n"
            "  font-display: swap;\n"
            f'  src: url("./{filename}") format("woff2");\n'
            f"  unicode-range: {unicode_range(part)};\n"
            "}\n"
        )

    header = (
        "/* Generated by web/scripts/subset-inter.py from InterVariable.woff2;\n"
        "   do not edit. One face per script subset: the browser fetches a file\n"
        "   only once the page shows one of its characters. */\n"
    )
    (OUT_DIR / "inter.css").write_text(header + "\n".join(rules), encoding="utf-8")


if __name__ == "__main__":
    main()
