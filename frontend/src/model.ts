// DOM-free types mirroring the JSON the Go side sends. They use string
// literal unions instead of the generated enums so modules that import them
// run under `node --test` (Node strips types but cannot run enums).

export type Kind = "unchanged" | "local" | "remote" | "both" | "conflict";

/** Half-open range of 0-based line indexes. */
export interface LineRange {
  start: number;
  end: number;
}

export interface MergeChunk {
  kind: Kind;
  base: LineRange;
  local: LineRange;
  remote: LineRange;
}

export type Side = "local" | "remote";

/**
 * State of one side of a chunk in the result:
 * none = that side did not change this chunk (nothing to apply),
 * pending = not decided yet, applied / ignored = decided.
 */
export type SideState = "none" | "pending" | "applied" | "ignored";

/** Which non-conflicting changes "Apply non-conflicting changes" applies. */
export type Source = "both" | "local" | "remote";

/**
 * Splits text into lines that keep their "\n", like textio.SplitLines: the
 * last line has no terminator if the text does not end with one, and empty
 * text has no lines. Chunk line indexes refer to these lines.
 */
export function splitLines(text: string): string[] {
  if (text === "") return [];
  const lines = text.split(/(?<=\n)/);
  return lines;
}

/** Joins lines [range.start, range.end) of lines. */
export function sliceLines(lines: readonly string[], range: LineRange): string {
  return lines.slice(range.start, range.end).join("");
}
