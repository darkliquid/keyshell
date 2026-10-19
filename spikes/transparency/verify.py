#!/usr/bin/env python3
"""Compare a screenshot of the S4 overlay against the desktop beneath it.

Usage:
    verify.py BASELINE.ppm WITH.ppm LAYOUT.json [--transparent]

BASELINE.ppm is the composited desktop captured with no overlay on screen,
WITH.ppm is the same screen while the spike's surface is up, and LAYOUT.json is
the sample list the spike printed (its S4-LAYOUT line).

Each sample is classified by the spike into one of three kinds and checked:

    desktop  the pixel must be unchanged from the baseline - the compositor
             blended a fully transparent surface over it
    opaque   the pixel must be exactly the fill colour
    blend    the pixel must be fill*alpha + baseline*(1-alpha), the result of
             correct premultiplied alpha. Getting this wrong is what a
             straight-vs-premultiplied mistake looks like: white at 50% would
             clip towards white, black at 50% towards black.

Exit code is 0 only when every sample passes, so this can gate a task.
"""

import json
import sys

TOLERANCE = 6  # 8-bit channels; grim output and compositor rounding wiggle a little


def read_ppm(path):
    with open(path, "rb") as f:
        magic = f.readline().strip()
        if magic != b"P6":
            raise SystemExit(f"{path}: expected a P6 PPM, got {magic!r}")
        line = f.readline()
        while line.startswith(b"#"):
            line = f.readline()
        width, height = map(int, line.split())
        maxval = int(f.readline())
        if maxval != 255:
            raise SystemExit(f"{path}: expected maxval 255, got {maxval}")
        data = f.read()
    return width, height, data


def pixel(img, x, y):
    width, _, data = img
    i = (y * width + x) * 3
    return data[i], data[i + 1], data[i + 2]


def close(got, want, tolerance=TOLERANCE):
    return all(abs(g - w) <= tolerance for g, w in zip(got, want))


def main():
    args = sys.argv[1:]
    transparent = "--transparent" in args
    args = [a for a in args if a != "--transparent"]
    if len(args) != 3:
        raise SystemExit(__doc__)
    baseline_path, with_path, layout_path = args

    baseline = read_ppm(baseline_path)
    with_overlay = read_ppm(with_path)
    layout = json.load(open(layout_path))

    failures = 0
    print(f"surface alpha expected: {'yes' if transparent else 'no'}")
    print(f"{'sample':14} {'kind':8} {'baseline':>16} {'with overlay':>16} {'expected':>16}  result")
    for s in layout:
        x, y = s["x"], s["y"]
        base = pixel(baseline, x, y)
        got = pixel(with_overlay, x, y)

        kind = s["kind"]
        if kind == "desktop":
            if transparent:
                want, ok = base, close(got, base)
            else:
                # The control: an opaque surface must NOT show the desktop, and
                # must differ from it measurably. This is what makes a PASS in
                # transparent mode meaningful rather than vacuous.
                ok = not close(got, base, tolerance=2)
                want = None
        elif kind == "opaque":
            want = tuple(s["fill"])
            ok = close(got, want, tolerance=2)
        elif kind == "blend":
            if not transparent:
                # On an opaque surface these blend against the surface's own
                # clear colour, not the desktop, so they are not asserted here:
                # the control exists to prove the desktop assertion can fail.
                print(f"{s['name']:14} {kind:8} {str(base):>16} {str(got):>16} "
                      f"{'n/a':>16}  SKIP  not asserted on an opaque surface")
                continue
            alpha = s["alpha"]
            want = tuple(round(f * alpha + b * (1 - alpha)) for f, b in zip(s["fill"], base))
            ok = close(got, want)
        else:
            raise SystemExit(f"unknown sample kind {kind!r}")

        if not ok:
            failures += 1
        want_str = "-" if want is None else str(want)
        print(f"{s['name']:14} {kind:8} {str(base):>16} {str(got):>16} {want_str:>16}  "
              f"{'PASS' if ok else 'FAIL'}  {s.get('note', '')}")

    print()
    if failures:
        print(f"{failures} sample(s) failed")
        return 1
    if transparent:
        print("all samples passed: the desktop shows through, alpha blends correctly, "
              "opaque stays opaque")
    else:
        print("control passed: on an opaque surface the desktop does NOT show through, "
              "so the transparent run's pass is meaningful")
    return 0


if __name__ == "__main__":
    sys.exit(main())