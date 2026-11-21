import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { EditorState, Text } from "@codemirror/state";
import { Decoration } from "@codemirror/view";
import { charMarks, clearCharMarksOnEdit, diffLineMarks, diffMarks, diffSummary, setCharMarks, setDiffMarks } from "./highlight.ts";
import type { MergeChunk } from "./model.ts";

const r = (start: number, end: number) => ({ start, end });
const chunk = (kind: MergeChunk["kind"], l: [number, number], rr: [number, number]): MergeChunk => ({
  kind,
  base: r(...l),
  local: r(...l),
  remote: r(...rr),
});

describe("diffSummary", () => {
  it("counts modified, added and deleted changes", () => {
    const chunks = [
      chunk("unchanged", [0, 1], [0, 1]),
      chunk("remote", [1, 2], [1, 3]), // modified
      chunk("remote", [2, 2], [3, 4]), // added
      chunk("remote", [2, 4], [4, 4]), // deleted
    ];
    assert.equal(diffSummary(chunks), "3 changes · 1 modified · 1 added · 1 deleted");
    assert.equal(diffSummary([chunk("remote", [0, 1], [0, 2])]), "1 change · 1 modified · 0 added · 0 deleted");
    assert.equal(diffSummary([]), "No differences");
    assert.equal(diffSummary([chunk("unchanged", [0, 3], [0, 3])]), "No differences");
  });
});

describe("scratch highlights", () => {
  const doc = "a\nb\nc\n";
  const chunks = [chunk("unchanged", [0, 1], [0, 1]), chunk("remote", [1, 2], [1, 2]), chunk("unchanged", [2, 3], [2, 3])];

  it("line marks are set by an effect and cleared by an edit", () => {
    let state = EditorState.create({ doc, extensions: [diffMarks] });
    const marks = diffLineMarks(Text.of(doc.split("\n")), chunks, "right");
    state = state.update({ effects: setDiffMarks.of(marks) }).state;
    assert.equal(state.field(diffMarks).size, 1);
    state = state.update({ selection: { anchor: 1 } }).state;
    assert.equal(state.field(diffMarks).size, 1, "selection changes keep them");
    state = state.update({ changes: { from: 0, insert: "x" } }).state;
    assert.equal(state.field(diffMarks).size, 0);
  });

  it("character marks are cleared by an edit when clearCharMarksOnEdit is on", () => {
    const mark = Decoration.mark({ class: "x" }).range(2, 3);
    const make = (clear: boolean) =>
      EditorState.create({ doc, extensions: clear ? [charMarks, clearCharMarksOnEdit] : [charMarks] }).update({
        effects: setCharMarks.of(Decoration.set([mark])),
      }).state;
    const edit = (s: EditorState) => s.update({ changes: { from: 0, insert: "x" } }).state;
    assert.equal(edit(make(true)).field(charMarks).size, 0);
    assert.equal(edit(make(false)).field(charMarks).size, 1, "without it (merge result) marks are remapped");
  });
});
