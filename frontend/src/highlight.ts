import { RangeSetBuilder, type EditorState, type Text } from "@codemirror/state";
import { Decoration, EditorView, type DecorationSet } from "@codemirror/view";
import { chunkField, isResolved, type Tracked } from "./chunks.ts";
import type { LineRange, MergeChunk, Side } from "./model.ts";

/** Line change category, used as a CSS class suffix. */
export type ChangeClass = "added" | "deleted" | "modified" | "conflict";

/** One decoration to place: whole lines, or a gap marker between lines. */
export interface LineMark {
  /** 1-based CodeMirror line number. */
  line: number;
  /** CSS classes. */
  cls: string;
}

/**
 * Line marks for a range of 0-based line indexes. Chunk line indexes refer
 * to the text split after each "\n"; CodeMirror line numbers are 1-based, so
 * index i is CodeMirror line i+1. An empty range gets a gap marker above the
 * line that follows it (or below the last line).
 */
function rangeMarks(range: LineRange, lineCount: number, cls: string): LineMark[] {
  if (range.end > range.start) {
    const marks: LineMark[] = [];
    for (let i = range.start; i < range.end; i++) marks.push({ line: i + 1, cls: `md-line ${cls}` });
    return marks;
  }
  if (range.start + 1 <= lineCount) return [{ line: range.start + 1, cls: `md-gap-above ${cls}` }];
  return [{ line: lineCount, cls: `md-gap-below ${cls}` }];
}

/** Builds line decorations, merging the classes of marks on the same line. */
function buildDecorations(doc: Text, marks: LineMark[]): DecorationSet {
  const byLine = new Map<number, string>();
  for (const mark of marks) {
    const prev = byLine.get(mark.line);
    byLine.set(mark.line, prev ? `${prev} ${mark.cls}` : mark.cls);
  }
  const builder = new RangeSetBuilder<Decoration>();
  for (const line of [...byLine.keys()].sort((a, b) => a - b)) {
    const from = doc.line(line).from;
    builder.add(from, from, Decoration.line({ class: byLine.get(line)! }));
  }
  return builder.finish();
}

// --- two-way diff -----------------------------------------------------------

/** Which file of a two-way diff a pane shows. */
export type DiffSide = "left" | "right";

/**
 * Classifies a diff chunk (LEFT is the chunk's local range, RIGHT its remote
 * range): lines only on the right are added, lines only on the left deleted,
 * otherwise modified.
 */
export function diffClass(chunk: MergeChunk): Exclude<ChangeClass, "conflict"> {
  if (chunk.local.end === chunk.local.start) return "added";
  if (chunk.remote.end === chunk.remote.start) return "deleted";
  return "modified";
}

/** A static line-decoration extension for one side of a two-way diff. */
export function diffHighlight(doc: Text, chunks: readonly MergeChunk[], side: DiffSide) {
  const marks = chunks
    .filter((c) => c.kind !== "unchanged")
    .flatMap((c) => rangeMarks(side === "left" ? c.local : c.remote, doc.lines, `md-${diffClass(c)}`));
  return EditorView.decorations.of(buildDecorations(doc, marks));
}

// --- three-way merge --------------------------------------------------------

/** Whether a chunk changes this side relative to base. */
export function changedOn(chunk: MergeChunk, side: Side): boolean {
  return chunk.kind === side || chunk.kind === "both" || chunk.kind === "conflict";
}

/**
 * Colour class of a chunk as seen from one side: conflicts are red;
 * otherwise added (base had no lines), deleted (side has no lines) or
 * modified.
 */
export function sideClass(chunk: MergeChunk, side: Side): ChangeClass {
  if (chunk.kind === "conflict") return "conflict";
  if (chunk.base.end === chunk.base.start) return "added";
  if (chunk[side].end === chunk[side].start) return "deleted";
  return "modified";
}

function stateClass(t: Tracked): string {
  return isResolved(t) ? " md-resolved" : "";
}

/** Decorations for the Local or Remote pane: the chunks that side changed. */
export function sideDecorations(doc: Text, tracked: readonly Tracked[], side: Side): DecorationSet {
  const marks = tracked
    .filter((t) => changedOn(t.chunk, side))
    .flatMap((t) => rangeMarks(t.chunk[side], doc.lines, `md-${sideClass(t.chunk, side)}${stateClass(t)}`));
  return buildDecorations(doc, marks);
}

/** Decorations for the Result pane, following the tracked chunk ranges. */
export function resultDecorations(state: EditorState): DecorationSet {
  const doc = state.doc;
  const marks: LineMark[] = [];
  for (const t of state.field(chunkField)) {
    const side: Side = t.chunk.kind === "remote" ? "remote" : "local";
    const cls = `md-${sideClass(t.chunk, side)}${stateClass(t)}`;
    if (t.to > t.from) {
      const first = doc.lineAt(t.from).number;
      const last = doc.lineAt(t.to - 1).number;
      for (let n = first; n <= last; n++) marks.push({ line: n, cls: `md-line ${cls}` });
    } else {
      marks.push({ line: doc.lineAt(t.from).number, cls: `md-gap-above ${cls}` });
    }
  }
  return buildDecorations(doc, marks);
}

/** Result-pane extension: decorations recomputed when chunks or text change. */
export const resultHighlight = EditorView.decorations.compute(["doc", chunkField], resultDecorations);
