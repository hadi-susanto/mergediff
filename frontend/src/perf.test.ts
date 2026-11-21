// Performance regression guards on the 10,000-line size the UI must handle.
// Budgets are generous (measured values are ~10x lower on a developer
// machine); they exist to catch order-of-magnitude regressions, e.g. after
// a CodeMirror upgrade.

import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { history } from "@codemirror/commands";
import { EditorState } from "@codemirror/state";
import { EditorView } from "@codemirror/view";
import { boundaries, mapPosition, mergeSegments } from "./alignment.ts";
import { applyNonConflicting, chunkField, chunkTracking, initialResult } from "./chunks.ts";
import { resultHighlight } from "./highlight.ts";
import type { Kind, MergeChunk } from "./model.ts";

/** ~10,000 base lines with a changed chunk every 25 unchanged lines. */
function largeMerge() {
  const base: string[] = [];
  const local: string[] = [];
  const remote: string[] = [];
  const chunks: MergeChunk[] = [];
  const kinds: Kind[] = ["local", "remote", "both", "conflict"];
  const push = (kind: Kind, b: string[], l: string[], r: string[]) => {
    chunks.push({
      kind,
      base: { start: base.length, end: base.length + b.length },
      local: { start: local.length, end: local.length + l.length },
      remote: { start: remote.length, end: remote.length + r.length },
    });
    base.push(...b);
    local.push(...l);
    remote.push(...r);
  };
  let n = 0;
  while (base.length < 10000) {
    const same = Array.from({ length: 25 }, (_, i) => `\tcount${i} := len(items) + ${n * 25 + i}\n`);
    push("unchanged", same, same, same);
    const kind = kinds[n % 4];
    const size = [1, 2, 5, 20][n % 4];
    const b = Array.from({ length: size }, (_, i) => `\told ${n}.${i}\n`);
    const l = Array.from({ length: size + (n % 3) }, (_, i) => `\tlocal ${n}.${i}\n`);
    const r = Array.from({ length: Math.max(1, size - 1) }, (_, i) => `\tremote ${n}.${i}\n`);
    push(kind, b, kind === "remote" ? b : kind === "both" ? l : l, kind === "local" ? b : kind === "both" ? l : r);
    n++;
  }
  return { chunks, base: base.join(""), local: local.join(""), remote: remote.join("") };
}

function average(n: number, f: (i: number) => void): number {
  const start = performance.now();
  for (let i = 0; i < n; i++) f(i);
  return (performance.now() - start) / n;
}

describe("performance on a 10,000-line merge", () => {
  const m = largeMerge();
  const init = initialResult(m.chunks, m.base, m.local, m.remote, false);
  const create = () =>
    EditorState.create({
      doc: init.doc,
      extensions: [history(), EditorState.lineSeparator.of("\n"), chunkTracking(init.tracked), resultHighlight],
    });

  it("has the intended size", () => {
    assert.ok(init.tracked.length > 300, `${init.tracked.length} chunks`);
    assert.ok(create().doc.lines > 10000);
  });

  it("a keystroke (chunk mapping + result highlights) stays well within a frame", () => {
    let state = create();
    state.facet(EditorView.decorations);
    const ms = average(100, (i) => {
      state = state.update({ changes: { from: Math.floor(state.doc.length / 2) + i, insert: "x" } }).state;
      state.facet(EditorView.decorations);
    });
    assert.ok(ms < 25, `keystroke took ${ms.toFixed(2)} ms on average (budget 25 ms)`);
  });

  it("apply all non-conflicting is one fast transaction", () => {
    const state = create();
    const ms = average(3, () => {
      state.update(applyNonConflicting(state)!).state.facet(EditorView.decorations);
    });
    assert.ok(ms < 200, `apply all took ${ms.toFixed(2)} ms (budget 200 ms)`);
  });

  it("a scroll event (segments, boundaries of 3 panes, mapping) is cheap", () => {
    const state = create();
    const segs = mergeSegments(m.chunks, state.field(chunkField), state.doc);
    const top = (line: number) => line * 20;
    const ms = average(200, (i) => {
      const b0 = boundaries(segs, 0, top);
      const b1 = boundaries(segs, 1, top);
      boundaries(segs, 2, top);
      mapPosition(b0, b1, (i * 997) % b0[b0.length - 1]);
    });
    assert.ok(ms < 5, `scroll event took ${ms.toFixed(3)} ms (budget 5 ms)`);
  });
});
