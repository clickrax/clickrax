"""Generate build/appicon.png and build/windows/icon.ico from logoclickrax.png."""
from __future__ import annotations

import sys
from pathlib import Path

from PIL import Image

ROOT = Path(__file__).resolve().parents[1]
SRC = ROOT / "logoclickrax.png"
APPICON = ROOT / "build" / "appicon.png"
ICO = ROOT / "build" / "windows" / "icon.ico"
SIZES = [(256, 256), (128, 128), (64, 64), (48, 48), (32, 32), (24, 24), (16, 16)]


def main() -> int:
    if not SRC.is_file():
        print(f"ERROR: source icon not found: {SRC}", file=sys.stderr)
        return 1

    img = Image.open(SRC).convert("RGBA")
    APPICON.parent.mkdir(parents=True, exist_ok=True)
    ICO.parent.mkdir(parents=True, exist_ok=True)

    img.save(APPICON, format="PNG", optimize=True)

    if ICO.exists():
        ICO.unlink()

    img.save(ICO, format="ICO", sizes=SIZES)
    print(f"OK: {ICO} ({ICO.stat().st_size} bytes, sizes: {', '.join(f'{w}' for w, _ in SIZES)})")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
