import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { initialResult } from "./chunks.ts";
import type { MergeChunk } from "./model.ts";
import { step, targets } from "./nav.ts";

describe("step", () => {
  const starts = [10, 20, 30];
  it("finds the next start strictly after pos", () => {
    assert.equal(step(starts, 0, 1), 0);
    assert.equal(step(starts, 10, 1), 1); // on a chunk: go to the following one
    assert.equal(step(starts, 15, 1), 1);
    assert.equal(step(starts, 30, 1), -1);
  });
  it("finds the previous start strictly before pos", () => {
    assert.equal(step(starts, 35, -1), 2);
    assert.equal(step(starts, 30, -1), 1);
    assert.equal(step(starts, 25, -1), 1); // inside a chunk: go to its start
    assert.equal(step(starts, 10, -1), -1);
  });
  it("handles no chunks", () => {
    assert.equal(step([], 5, 1), -1);
    assert.equal(step([], 5, -1), -1);
  });
});

describe("targets", () => {
  const r = (start: number, end: number) => ({ start, end });
  const chunks: MergeChunk[] = [
    { kind: "local", base: r(0, 1), local: r(0, 1), remote: r(0, 1) },
    { kind: "unchanged", base: r(1, 2), local: r(1, 2), remote: r(1, 2) },
    { kind: "conflict", base: r(2, 3), local: r(2, 3), remote: r(2, 3) },
    { kind: "unchanged", base: r(3, 4), local: r(3, 4), remote: r(3, 4) },
    { kind: "conflict", base: r(4, 5), local: r(4, 5), remote: r(4, 5) },
  ];
  const { tracked } = initialResult(chunks, "a\nb\nc\nd\ne\n", "A\nb\nC\nd\nE\n", "a\nb\nX\nd\nY\n", false);

  it("changes are every tracked chunk", () => {
    assert.deepEqual(targets(tracked, "change"), [0, 1, 2]);
  });
  it("conflicts are the unresolved conflict chunks", () => {
    assert.deepEqual(targets(tracked, "conflict"), [1, 2]);
    const resolved = tracked.map((t, i) => (i === 1 ? { ...t, local: "applied" as const, remote: "ignored" as const } : t));
    assert.deepEqual(targets(resolved, "conflict"), [2]);
  });
});
