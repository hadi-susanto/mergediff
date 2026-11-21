#!/usr/bin/env python3
"""Generate a large three-way merge for performance checks.

Usage: gen-large.py DIR [LINES]   (used by perf.sh; handy for manual checks too)

Writes DIR/base.go, DIR/local.go and DIR/remote.go: a Go-like file of LINES
lines (default 10000) with about one edit per 30 lines on each side - changed
lines, inserted and deleted blocks of various sizes - some of which overlap
into conflicts. Deterministic (fixed seed).
"""
import os
import random
import sys


def base_lines(n, rnd):
    lines = ["package big\n", "\n", 'import "fmt"\n', "\n"]
    fn = 0
    while len(lines) < n:
        fn += 1
        body = rnd.randint(4, 30)
        lines.append(f"// fn{fn} does step {fn} of the pipeline.\n")
        lines.append(f"func fn{fn}(items []string, limit int) (int, error) {{\n")
        for i in range(body):
            lines.append(f'\tcount{i} := len(items) + {rnd.randint(0, 999)} // note {fn}.{i}\n')
            if i % 7 == 3:
                lines.append(f'\tif count{i} > limit {{ return 0, fmt.Errorf("fn{fn}: too many: %d", count{i}) }}\n')
        lines.append("\treturn limit, nil\n")
        lines.append("}\n")
        lines.append("\n")
    return lines[:n]


def mutate(lines, rnd, tag):
    out = list(lines)
    pos = len(out) - 1
    while pos > 10:
        pos -= rnd.randint(10, 50)
        kind = rnd.random()
        size = rnd.choice([1, 1, 1, 2, 3, 5, 8, 20, 60])
        if kind < 0.5:  # change lines
            for k in range(pos, min(pos + size, len(out))):
                out[k] = out[k].rstrip("\n") + f"  // {tag} edit\n"
        elif kind < 0.8:  # insert a block
            out[pos:pos] = [f"\t// {tag} added line {k}\n" for k in range(size)]
        else:  # delete a block
            del out[pos : pos + size]
    return out


def main():
    d = sys.argv[1]
    n = int(sys.argv[2]) if len(sys.argv) > 2 else 10000
    base = base_lines(n, random.Random(1))
    local = mutate(base, random.Random(2), "local")
    remote = mutate(base, random.Random(3), "remote")
    os.makedirs(d, exist_ok=True)
    for name, lines in (("base", base), ("local", local), ("remote", remote)):
        with open(os.path.join(d, f"{name}.go"), "w") as f:
            f.writelines(lines)


if __name__ == "__main__":
    main()
