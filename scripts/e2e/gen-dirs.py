#!/usr/bin/env python3
"""Generate two large directory trees for directory diff performance checks.

Usage: gen-dirs.py DIR [FILES] [CHANGED]   (used by perf.sh; handy for manual checks)

Writes DIR/left and DIR/right with FILES files each (default 20000) in
nested directories, of which CHANGED (default 500) differ: a third
modified (half of them with the same size, so they need a byte
comparison), a third only on the right (added) and a third only on the
left (deleted). Text files of 1-8 KB. Deterministic (fixed seed).
"""
import os
import random
import sys


def main():
    root = sys.argv[1]
    files = int(sys.argv[2]) if len(sys.argv) > 2 else 20000
    changed = int(sys.argv[3]) if len(sys.argv) > 3 else 500
    rnd = random.Random(42)
    changed_at = set(rnd.sample(range(files), changed))
    for i in range(files):
        rel = os.path.join(f"pkg{i % 50:02}", f"mod{(i // 50) % 40:02}", f"file{i:05}.txt")
        lines = rnd.randint(16, 128)
        text = "".join(f"line {j} of file {i}: {'x' * (j % 40)}\n" for j in range(lines))
        left = right = text
        if i in changed_at:
            kind = i % 3
            if kind == 0:  # modified; every other one keeps the size
                right = text.replace("line 3 ", "LINE 3 ", 1) if i % 2 else text + "one more line\n"
            elif kind == 1:
                left = None  # added
            else:
                right = None  # deleted
        for side, content in (("left", left), ("right", right)):
            if content is None:
                continue
            path = os.path.join(root, side, rel)
            os.makedirs(os.path.dirname(path), exist_ok=True)
            with open(path, "w") as f:
                f.write(content)


if __name__ == "__main__":
    main()
