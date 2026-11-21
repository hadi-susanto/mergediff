// Chunk alignment between panes. DOM-free so it runs under node --test.
//
// The panes are cut into the same sequence of segments (one per chunk): for
// every segment each pane has a line range, and consecutive segments touch.
// Scroll positions map segment to segment, proportionally to each pane's
// segment height. Unchanged segments have the same height in every pane, so
// they scroll 1:1; changed segments stretch, which keeps matching chunks
// aligned however different their heights are.

import type { Tracked } from "./chunks.ts";
import type { LineRange, MergeChunk } from "./model.ts";

/** segments[i][p] is the line range of segment i in pane p. */
export type Segments = LineRange[][];

/**
 * Maps position y in a source pane to the target pane. src and dst are the
 * segment boundaries in each pane: boundaries[i] is where segment i starts
 * and boundaries[n] is where the last one ends (any unit, e.g. pixels).
 * Outside the segments, positions map 1:1 from the nearest end.
 */
export function mapPosition(src: readonly number[], dst: readonly number[], y: number): number {
  const n = src.length - 1;
  if (n < 1 || y < src[0]) return dst[0] + (y - src[0]);
  if (y >= src[n]) return dst[n] + (y - src[n]);
  // Last segment whose start is <= y; zero-height segments are skipped
  // because y < src[i + 1] cannot hold for them.
  let lo = 0;
  let hi = n - 1;
  while (lo < hi) {
    const mid = (lo + hi + 1) >> 1;
    if (src[mid] <= y) lo = mid;
    else hi = mid - 1;
  }
  const height = src[lo + 1] - src[lo];
  const fraction = height > 0 ? (y - src[lo]) / height : 0;
  return dst[lo] + fraction * (dst[lo + 1] - dst[lo]);
}

/** Two-way diff: pane 0 is LEFT (the chunk's local range), pane 1 RIGHT. */
export function diffSegments(chunks: readonly MergeChunk[]): Segments {
  return chunks.map((c) => [c.local, c.remote]);
}

/** Minimal view of a document for line lookups (CodeMirror's Text fits). */
export interface LineLookup {
  /** 1-based number of the line containing pos. */
  lineAt(pos: number): { number: number };
  readonly length: number;
  readonly lines: number;
  sliceString(from: number, to: number): string;
}

/**
 * Number of 0-based line indexes in the document, counted like splitLines:
 * a trailing "\n" does not start another line, and an empty doc has none.
 */
export function lineIndexCount(doc: LineLookup): number {
  if (doc.length === 0) return 0;
  return doc.sliceString(doc.length - 1, doc.length) === "\n" ? doc.lines - 1 : doc.lines;
}

/**
 * Three-way merge: panes are 0 = local, 1 = result, 2 = remote. The result
 * pane's ranges come from the tracked chunk positions, so they stay right
 * while the user edits; unchanged chunks fill the gaps between them.
 */
export function mergeSegments(chunks: readonly MergeChunk[], tracked: readonly Tracked[], result: LineLookup): Segments {
  // Line range of every changed chunk in the result.
  const ranges: LineRange[] = [];
  let floor = 0;
  for (const t of tracked) {
    const start = Math.max(floor, result.lineAt(t.from).number - 1);
    const end = Math.max(start, t.to > t.from ? result.lineAt(t.to - 1).number : start);
    ranges.push({ start, end });
    floor = end;
  }
  const total = Math.max(floor, lineIndexCount(result));

  const segments: Segments = [];
  let next = 0; // index into tracked / ranges
  let line = 0; // result line where the current chunk starts
  for (const chunk of chunks) {
    let r: LineRange;
    if (chunk.kind === "unchanged") {
      const end = next < ranges.length ? ranges[next].start : total;
      r = { start: line, end: Math.max(line, end) };
    } else {
      r = ranges[next++];
    }
    segments.push([chunk.local, r, chunk.remote]);
    line = r.end;
  }
  return segments;
}

/** Boundaries of pane p: each segment's start, then the last segment's end. */
export function boundaries(segments: Segments, pane: number, lineTop: (line: number) => number): number[] {
  if (segments.length === 0) return [lineTop(0), lineTop(0)];
  const out = segments.map((s) => lineTop(s[pane].start));
  out.push(lineTop(segments[segments.length - 1][pane].end));
  return out;
}
