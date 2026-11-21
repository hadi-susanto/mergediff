import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { Text } from "@codemirror/state";
import { boundaries, diffSegments, lineIndexCount, mapPosition, mergeSegments } from "./alignment.ts";
import { initialResult } from "./chunks.ts";
import type { MergeChunk } from "./model.ts";

const range = (start: number, end: number) => ({ start, end });

describe("mapPosition", () => {
  // Segments: unchanged 10 lines | changed 2 vs 40 lines | unchanged 10 lines.
  const src = [0, 10, 12, 22];
  const dst = [0, 10, 50, 60];

  it("maps unchanged regions 1:1", () => {
    assert.equal(mapPosition(src, dst, 0), 0);
    assert.equal(mapPosition(src, dst, 5), 5);
    assert.equal(mapPosition(src, dst, 13), 51);
  });

  it("maps changed regions proportionally, keeping the chunk edges aligned", () => {
    assert.equal(mapPosition(src, dst, 10), 10); // start of the chunk
    assert.equal(mapPosition(src, dst, 11), 30); // half way through
    assert.equal(mapPosition(src, dst, 12), 50); // end of the chunk
    // And back again.
    assert.equal(mapPosition(dst, src, 30), 11);
    assert.equal(mapPosition(dst, src, 50), 12);
  });

  it("skips segments that are empty in the source", () => {
    // Middle segment has no lines on the source side (an insertion).
    const s = [0, 10, 10, 20];
    const d = [0, 10, 30, 40];
    assert.equal(mapPosition(s, d, 9.999), 9.999);
    assert.equal(mapPosition(s, d, 10), 30); // lands after the inserted block
    assert.equal(mapPosition(s, d, 15), 35);
  });

  it("extends 1:1 beyond the ends", () => {
    assert.equal(mapPosition(src, dst, -4), -4);
    assert.equal(mapPosition(src, dst, 30), 68);
  });

  it("round-trips at segment boundaries", () => {
    for (let y = 0; y <= 22; y++) {
      const there = mapPosition(src, dst, y);
      assert.ok(Math.abs(mapPosition(dst, src, there) - y) < 1e-9, `y=${y}`);
    }
  });
});

describe("segments", () => {
  const chunks: MergeChunk[] = [
    { kind: "unchanged", base: range(0, 2), local: range(0, 2), remote: range(0, 2) },
    { kind: "local", base: range(2, 3), local: range(2, 6), remote: range(2, 3) },
    { kind: "unchanged", base: range(3, 4), local: range(6, 7), remote: range(3, 4) },
    { kind: "remote", base: range(4, 4), local: range(7, 7), remote: range(4, 6) },
  ];
  const base = "a\nb\nc\nd\n";
  const local = "a\nb\nL1\nL2\nL3\nL4\nd\n";
  const remote = "a\nb\nc\nd\nR1\nR2\n";

  it("diff segments pair LEFT and RIGHT ranges", () => {
    assert.deepEqual(diffSegments(chunks)[1], [range(2, 6), range(2, 3)]);
  });

  it("merge segments follow the result text", () => {
    const init = initialResult(chunks, base, local, remote, false);
    const doc = Text.of(init.doc.split("\n"));
    const segs = mergeSegments(chunks, init.tracked, doc);
    assert.deepEqual(
      segs.map((s) => s[1]),
      [range(0, 2), range(2, 3), range(3, 4), range(4, 4)],
    );
    // Contiguous in every pane.
    for (let p = 0; p < 3; p++) {
      for (let i = 1; i < segs.length; i++) assert.equal(segs[i][p].start, segs[i - 1][p].end);
    }
  });

  it("merge segments after applying everything", () => {
    const init = initialResult(chunks, base, local, remote, true);
    const doc = Text.of(init.doc.split("\n"));
    const segs = mergeSegments(chunks, init.tracked, doc);
    assert.deepEqual(
      segs.map((s) => s[1]),
      [range(0, 2), range(2, 6), range(6, 7), range(7, 9)],
    );
  });

  it("boundaries use each segment's start and the last end", () => {
    const segs = diffSegments(chunks);
    assert.deepEqual(
      boundaries(segs, 0, (l) => l * 20),
      [0, 40, 120, 140, 140],
    );
  });

  it("counts line indexes like splitLines", () => {
    assert.equal(lineIndexCount(Text.of([""])), 0);
    assert.equal(lineIndexCount(Text.of(["a", ""])), 1);
    assert.equal(lineIndexCount(Text.of(["a", "b"])), 2);
  });
});
