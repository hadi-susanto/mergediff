import { RangeSetBuilder, type Extension, type Text } from "@codemirror/state";
import { Decoration, EditorView } from "@codemirror/view";
import { ChunkKind, type Chunk } from "../bindings/github.com/hadi-susanto/mergediff/internal/merge/models";

/** Which file of a two-way diff a pane shows. */
export type DiffSide = "left" | "right";

/** Line change category, used as a CSS class suffix. */
export type ChangeClass = "added" | "deleted" | "modified";

/**
 * Classifies a diff chunk (LEFT is the chunk's local range, RIGHT its remote
 * range): lines only on the right are added, lines only on the left deleted,
 * otherwise modified.
 */
export function changeClass(chunk: Chunk): ChangeClass {
  const leftLen = chunk.local.end - chunk.local.start;
  const rightLen = chunk.remote.end - chunk.remote.start;
  if (leftLen === 0) return "added";
  if (rightLen === 0) return "deleted";
  return "modified";
}

/** One decoration to place: whole lines, or a gap marker between lines. */
export interface LineMark {
  /** 1-based CodeMirror line number. */
  line: number;
  /** CSS class. */
  cls: string;
}

/**
 * Computes line marks for one side of a two-way diff. Changed lines get
 * `md-line md-<class>`; where this side has no lines for a change, a gap
 * marker is drawn above the line that follows (or below the last line).
 *
 * Chunk ranges are 0-based line indexes into the text split after each "\n";
 * CodeMirror line numbers are 1-based, so line index i is CodeMirror line i+1.
 */
export function diffLineMarks(chunks: readonly Chunk[], side: DiffSide, lineCount: number): LineMark[] {
  const marks: LineMark[] = [];
  for (const chunk of chunks) {
    if (chunk.kind === ChunkKind.Unchanged) continue;
    const cls = changeClass(chunk);
    const range = side === "left" ? chunk.local : chunk.remote;
    if (range.end > range.start) {
      for (let i = range.start; i < range.end; i++) {
        marks.push({ line: i + 1, cls: `md-line md-${cls}` });
      }
    } else if (range.start + 1 <= lineCount) {
      marks.push({ line: range.start + 1, cls: `md-gap-above md-${cls}` });
    } else {
      marks.push({ line: lineCount, cls: `md-gap-below md-${cls}` });
    }
  }
  return marks;
}

/** A static line-decoration extension for one side of a two-way diff. */
export function diffHighlight(doc: Text, chunks: readonly Chunk[], side: DiffSide): Extension {
  const builder = new RangeSetBuilder<Decoration>();
  // Several marks may land on one line (a gap marker and a changed line);
  // RangeSetBuilder needs them in order, so merge classes per line first.
  const byLine = new Map<number, string>();
  for (const mark of diffLineMarks(chunks, side, doc.lines)) {
    const prev = byLine.get(mark.line);
    byLine.set(mark.line, prev ? `${prev} ${mark.cls}` : mark.cls);
  }
  for (const line of [...byLine.keys()].sort((a, b) => a - b)) {
    const from = doc.line(line).from;
    builder.add(from, from, Decoration.line({ class: byLine.get(line)! }));
  }
  return EditorView.decorations.of(builder.finish());
}
