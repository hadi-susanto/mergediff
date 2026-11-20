import assert from "node:assert/strict";
import { existsSync, readdirSync, readFileSync } from "node:fs";
import { describe, it } from "node:test";
import { history, redo, undo } from "@codemirror/commands";
import { EditorState, type TransactionSpec } from "@codemirror/state";
import {
  applyNonConflicting,
  applySide,
  chunkField,
  chunkTracking,
  conflictsLeft,
  ignoreSide,
  initialResult,
  nonConflictingLeft,
  type Tracked,
} from "./chunks.ts";
import type { Kind, MergeChunk } from "./model.ts";

// --- helpers ---------------------------------------------------------------

interface Editor {
  state: EditorState;
}

function editor(chunks: MergeChunk[], base: string, local: string, remote: string, autoApplied = false): Editor {
  const init = initialResult(chunks, base, local, remote, autoApplied);
  return {
    state: EditorState.create({
      doc: init.doc,
      extensions: [history(), EditorState.lineSeparator.of("\n"), chunkTracking(init.tracked)],
    }),
  };
}

function run(ed: Editor, spec: TransactionSpec | null): void {
  assert.ok(spec, "expected a transaction");
  ed.state = ed.state.update(spec).state;
}

/** Runs a StateCommand such as undo/redo. */
function command(ed: Editor, cmd: typeof undo): boolean {
  return cmd({ state: ed.state, dispatch: (tr) => (ed.state = tr.state) });
}

function type(ed: Editor, from: number, insert: string, to = from): void {
  ed.state = ed.state.update({ changes: { from, to, insert }, userEvent: "input.type" }).state;
}

const doc = (ed: Editor) => ed.state.doc.toString();
const tracked = (ed: Editor) => ed.state.field(chunkField);
const textOf = (ed: Editor, t: Tracked) => ed.state.sliceDoc(t.from, t.to);

// --- golden cases shared with the Go engine ---------------------------------

const testdata = new URL("../../internal/merge/testdata/", import.meta.url);

interface Case {
  name: string;
  base: string;
  local: string;
  remote: string;
  expected: string; // the Go helper's output (FromBoth), LF-normalised
  chunks: MergeChunk[];
}

function read(dir: URL, name: string): string {
  const url = new URL(name, dir);
  return existsSync(url) ? readFileSync(url, "utf8").replaceAll("\r\n", "\n") : "";
}

function parseChunks(spec: string): MergeChunk[] {
  return spec
    .trim()
    .split("\n")
    .map((line) => {
      const [kind, b, l, r] = line.split(" ");
      const range = (s: string) => {
        const [start, end] = s.split("-").map(Number);
        return { start, end };
      };
      return { kind: kind as Kind, base: range(b), local: range(l), remote: range(r) };
    });
}

function loadCases(): Case[] {
  return readdirSync(testdata, { withFileTypes: true })
    .filter((d) => d.isDirectory())
    .map((d) => {
      const dir = new URL(`${d.name}/`, testdata);
      return {
        name: d.name,
        base: read(dir, "base"),
        local: read(dir, "local"),
        remote: read(dir, "remote"),
        expected: read(dir, "expected"),
        chunks: parseChunks(read(dir, "chunks")),
      };
    });
}

describe("golden cases (same data as the Go engine)", () => {
  const cases = loadCases();
  it("has the cases", () => assert.ok(cases.length >= 15));

  for (const c of cases) {
    it(c.name, () => {
      // Starts as the base text.
      const ed = editor(c.chunks, c.base, c.local, c.remote);
      assert.equal(doc(ed), c.base);
      const before = tracked(ed);

      // Apply non-conflicting gives exactly the Go helper's output.
      const spec = applyNonConflicting(ed.state);
      if (nonConflictingLeft(before) === 0) {
        assert.equal(spec, null);
      } else {
        run(ed, spec);
      }
      assert.equal(doc(ed), c.expected);
      assert.equal(nonConflictingLeft(tracked(ed)), 0);
      // Conflicts keep the base text and stay unresolved.
      for (const t of tracked(ed)) {
        if (t.chunk.kind === "conflict") {
          const b = c.base.split(/(?<=\n)/).slice(t.chunk.base.start, t.chunk.base.end).join("");
          assert.equal(textOf(ed, t), c.base === "" ? "" : b);
          assert.equal(t.local, "pending");
          assert.equal(t.remote, "pending");
        }
      }

      // --auto-apply lays out the same text and state.
      const auto = editor(c.chunks, c.base, c.local, c.remote, true);
      assert.equal(doc(auto), c.expected);
      assert.deepEqual(tracked(auto), tracked(ed));

      // One undo restores the base text and every chunk state.
      if (spec) {
        assert.ok(command(ed, undo));
        assert.equal(doc(ed), c.base);
        assert.deepEqual(tracked(ed), before);
        assert.ok(command(ed, redo));
        assert.equal(doc(ed), c.expected);
      }
    });
  }
});

// --- the mockup example, with edits ----------------------------------------

const MB = `package main

import "fmt"

// greet builds a greeting
func greet(name string) string {
	return "Hello, " + name
}

func main() {
	fmt.Println(greet("world"))
}
`;
const ML = MB.replace("builds", "returns").replace("Hello, ", "Hi, ");
const MR = MB.replace(`"Hello, " + name`, `"Hey " + name + "!"`).replace(`"world"`, `"Go"`);
const MCHUNKS = parseChunks(`unchanged 0-4 0-4 0-4
local 4-5 4-5 4-5
unchanged 5-6 5-6 5-6
conflict 6-7 6-7 6-7
unchanged 7-10 7-10 7-10
remote 10-11 10-11 10-11
unchanged 11-12 11-12 11-12`);
// Tracked indexes: 0 = local (line 5), 1 = conflict (line 7), 2 = remote (line 11).

describe("apply after the user edited the result", () => {
  it("uses current positions and is undone in one step", () => {
    const ed = editor(MCHUNKS, MB, ML, MR);
    // Edit an unchanged region before every chunk, and inside the conflict.
    type(ed, 0, "// header\n");
    const conflict = tracked(ed)[1];
    type(ed, conflict.from + 1, "/*edited*/");
    const edited = doc(ed);
    const before = tracked(ed);
    assert.equal(textOf(ed, before[1]), "\t/*edited*/return \"Hello, \" + name\n");

    run(ed, applyNonConflicting(ed.state));
    const want = "// header\n" + MB.replace("builds", "returns").replace(`"world"`, `"Go"`).replace("\treturn", "\t/*edited*/return");
    assert.equal(doc(ed), want);
    assert.equal(conflictsLeft(tracked(ed)), 1);

    assert.ok(command(ed, undo));
    assert.equal(doc(ed), edited);
    assert.deepEqual(tracked(ed), before);
  });

  it("typing right after an apply is a separate undo step", () => {
    // Typing next to the applied text within CodeMirror's grouping delay
    // must not join the apply's history event.
    const ed = editor(MCHUNKS, MB, ML, MR);
    run(ed, applySide(ed.state, 1, "local"));
    const applied = doc(ed);
    type(ed, tracked(ed)[1].from + 1, "x");
    command(ed, undo);
    assert.equal(doc(ed), applied);
    command(ed, undo);
    assert.equal(doc(ed), MB);
  });

  it("from local only / remote only", () => {
    const l = editor(MCHUNKS, MB, ML, MR);
    run(l, applyNonConflicting(l.state, "local"));
    assert.equal(doc(l), MB.replace("builds", "returns"));
    assert.equal(nonConflictingLeft(tracked(l), "local"), 0);
    assert.equal(nonConflictingLeft(tracked(l), "remote"), 1);

    const r = editor(MCHUNKS, MB, ML, MR);
    run(r, applyNonConflicting(r.state, "remote"));
    assert.equal(doc(r), MB.replace(`"world"`, `"Go"`));
    // Then the rest: the local chunk only.
    run(r, applyNonConflicting(r.state));
    assert.equal(doc(r), MB.replace("builds", "returns").replace(`"world"`, `"Go"`));
    assert.equal(applyNonConflicting(r.state), null);
  });

  it("skips chunks already applied or ignored", () => {
    const ed = editor(MCHUNKS, MB, ML, MR);
    run(ed, ignoreSide(ed.state, 0, "local"));
    run(ed, applyNonConflicting(ed.state));
    assert.equal(doc(ed), MB.replace(`"world"`, `"Go"`));
    assert.equal(tracked(ed)[0].local, "ignored");
  });
});

describe("conflicts", () => {
  const localLine = '\treturn "Hi, " + name\n';
  const remoteLine = '\treturn "Hey " + name + "!"\n';

  for (const order of [["local", "remote"], ["remote", "local"]] as const) {
    it(`applying ${order[0]} then ${order[1]} inserts both, local first`, () => {
      const ed = editor(MCHUNKS, MB, ML, MR);
      run(ed, applySide(ed.state, 1, order[0]));
      assert.equal(conflictsLeft(tracked(ed)), 1);
      run(ed, applySide(ed.state, 1, order[1]));
      assert.equal(textOf(ed, tracked(ed)[1]), localLine + remoteLine);
      assert.equal(conflictsLeft(tracked(ed)), 0);
      // Each apply is its own undo step.
      command(ed, undo);
      assert.equal(textOf(ed, tracked(ed)[1]), order[0] === "local" ? localLine : remoteLine);
      assert.equal(conflictsLeft(tracked(ed)), 1);
    });
  }

  it("apply one side and ignore the other resolves it", () => {
    const ed = editor(MCHUNKS, MB, ML, MR);
    run(ed, applySide(ed.state, 1, "remote"));
    run(ed, ignoreSide(ed.state, 1, "local"));
    assert.equal(conflictsLeft(tracked(ed)), 0);
    assert.equal(doc(ed), MB.replace('\treturn "Hello, " + name\n', remoteLine));
    assert.equal(applySide(ed.state, 1, "local"), null, "ignored side cannot be applied");
    // Undoing the ignore reopens the conflict without touching the text.
    command(ed, undo);
    assert.equal(conflictsLeft(tracked(ed)), 1);
    assert.equal(doc(ed), MB.replace('\treturn "Hello, " + name\n', remoteLine));
  });

  it("joins sides that lack a final newline onto separate lines", () => {
    const chunks = parseChunks("unchanged 0-1 0-1 0-1\nconflict 1-2 1-2 1-2");
    const ed = editor(chunks, "a\nb", "a\nL", "a\nR");
    run(ed, applySide(ed.state, 0, "remote"));
    run(ed, applySide(ed.state, 0, "local"));
    assert.equal(doc(ed), "a\nL\nR");
  });
});

describe("both sides made the same change", () => {
  it("applying either side applies it once", () => {
    const chunks = parseChunks("unchanged 0-1 0-1 0-1\nboth 1-2 1-2 1-2\nunchanged 2-3 2-3 2-3");
    const ed = editor(chunks, "a\nb\nc\n", "a\nX\nc\n", "a\nX\nc\n");
    run(ed, applySide(ed.state, 0, "remote"));
    assert.equal(doc(ed), "a\nX\nc\n");
    assert.equal(tracked(ed)[0].local, "applied");
    assert.equal(applySide(ed.state, 0, "local"), null);
  });
});

describe("tracking through edits", () => {
  it("typing inside or at the start of a chunk keeps it in the chunk", () => {
    const ed = editor(MCHUNKS, MB, ML, MR);
    const c = tracked(ed)[1];
    type(ed, c.from, ">>");
    assert.equal(textOf(ed, tracked(ed)[1]), '>>\treturn "Hello, " + name\n');
  });

  it("typing at the start of the following line stays outside", () => {
    const ed = editor(MCHUNKS, MB, ML, MR);
    const c = tracked(ed)[1];
    type(ed, c.to, "X");
    assert.equal(textOf(ed, tracked(ed)[1]), '\treturn "Hello, " + name\n');
  });

  it("deleting a chunk's text leaves an empty range in place", () => {
    const ed = editor(MCHUNKS, MB, ML, MR);
    const c = tracked(ed)[1];
    type(ed, c.from, "", c.to);
    const t = tracked(ed)[1];
    assert.equal(t.from, t.to);
    run(ed, applySide(ed.state, 1, "local"));
    assert.equal(doc(ed), MB.replace('\treturn "Hello, " + name\n', '\treturn "Hi, " + name\n'));
  });

  it("insert-only chunks are tracked as empty ranges and filled on apply", () => {
    const chunks = parseChunks("unchanged 0-2 0-2 0-2\nremote 2-2 2-2 2-4");
    const ed = editor(chunks, "a\nb\n", "a\nb\n", "a\nb\nc\nd\n");
    const t = tracked(ed)[0];
    assert.equal(t.from, 4);
    assert.equal(t.to, 4);
    run(ed, applySide(ed.state, 0, "remote"));
    assert.equal(doc(ed), "a\nb\nc\nd\n");
    assert.equal(textOf(ed, tracked(ed)[0]), "c\nd\n");
  });
});
