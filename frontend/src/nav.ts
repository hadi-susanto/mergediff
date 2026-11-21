// Previous / next chunk lookup. DOM-free so it runs under node --test.

import { isResolved, type Tracked } from "./chunks.ts";

export type Direction = 1 | -1;

/**
 * In ascending starts, returns the index of the first start after pos
 * (dir 1) or the last start before pos (dir -1), or -1 if there is none.
 * Being exactly at a start counts as being on that chunk, so stepping moves
 * to its neighbour.
 */
export function step(starts: readonly number[], pos: number, dir: Direction): number {
  if (dir === 1) {
    for (let i = 0; i < starts.length; i++) if (starts[i] > pos) return i;
    return -1;
  }
  for (let i = starts.length - 1; i >= 0; i--) if (starts[i] < pos) return i;
  return -1;
}

/** Indexes (into tracked) of the chunks a navigation kind visits. */
export function targets(tracked: readonly Tracked[], kind: "change" | "conflict"): number[] {
  const out: number[] = [];
  tracked.forEach((t, i) => {
    if (kind === "change" || (t.chunk.kind === "conflict" && !isResolved(t))) out.push(i);
  });
  return out;
}
