// Chunk tracking for the Result editor. DOM-free so it runs under node --test.
//
// Every non-unchanged chunk is tracked with its current range in the result
// document and the state of its local and remote side. Ranges follow every
// edit (StateField + ChangeSet.mapPos). Applying or ignoring a side is one
// transaction whose chunk-state effects are registered with invertedEffects,
// so a single undo reverts both the text and the chunk state.

import { invertedEffects, isolateHistory } from "@codemirror/commands";
import {
  StateEffect,
  StateField,
  type ChangeDesc,
  type ChangeSpec,
  type EditorState,
  type TransactionSpec,
} from "@codemirror/state";
import { sliceLines, splitLines, type MergeChunk, type Side, type SideState, type Source } from "./model.ts";

/** A changed chunk as tracked in the result document. */
export interface Tracked {
  readonly chunk: MergeChunk;
  /** Current range in the result document (character offsets). */
  readonly from: number;
  readonly to: number;
  readonly local: SideState;
  readonly remote: SideState;
  /** The chunk's text on each side, inserted when that side is applied. */
  readonly localText: string;
  readonly remoteText: string;
}

interface ChunkUpdate {
  index: number;
  from: number;
  to: number;
  local: SideState;
  remote: SideState;
}

/** Maps a tracked range through document changes made by the user. */
function mapRange(from: number, to: number, changes: ChangeDesc): [number, number] {
  // Text typed at the start of a chunk's first line belongs to the chunk;
  // text typed at `to` (the start of the following line) does not.
  const f = changes.mapPos(from, -1);
  const t = changes.mapPos(to, -1);
  return [f, Math.max(f, t)];
}

/** Sets the range and side states of one tracked chunk. */
const setChunk = StateEffect.define<ChunkUpdate>({
  map: (u, changes) => {
    const [from, to] = mapRange(u.from, u.to, changes);
    return { ...u, from, to };
  },
});

export const chunkField = StateField.define<readonly Tracked[]>({
  create: () => [],
  update(value, tr) {
    let next = value;
    if (!tr.changes.empty) {
      next = next.map((t) => {
        const [from, to] = mapRange(t.from, t.to, tr.changes);
        return from === t.from && to === t.to ? t : { ...t, from, to };
      });
    }
    for (const e of tr.effects) {
      if (e.is(setChunk)) {
        const { index, ...rest } = e.value;
        const copy = next.slice();
        copy[index] = { ...copy[index], ...rest };
        next = copy;
      }
    }
    return next;
  },
});

/** Makes every chunk-state change undoable together with its text change. */
const undoableChunkState = invertedEffects.of((tr) => {
  const before = tr.startState.field(chunkField, false);
  if (!before) return [];
  const inverse: StateEffect<ChunkUpdate>[] = [];
  for (const e of tr.effects) {
    if (e.is(setChunk)) {
      const old = before[e.value.index];
      inverse.push(setChunk.of({ index: e.value.index, from: old.from, to: old.to, local: old.local, remote: old.remote }));
    }
  }
  return inverse;
});

/** Result of laying out the initial result document. */
export interface InitialResult {
  doc: string;
  tracked: Tracked[];
}

/**
 * Builds the initial result text and the tracked chunks. The result is the
 * base text; with autoApplied every non-conflicting chunk already has its
 * changed text (as the Go side's ApplyNonConflicting produces) and is marked
 * applied.
 */
export function initialResult(
  chunks: readonly MergeChunk[],
  base: string,
  local: string,
  remote: string,
  autoApplied: boolean,
): InitialResult {
  const b = splitLines(base);
  const l = splitLines(local);
  const r = splitLines(remote);
  let doc = "";
  const tracked: Tracked[] = [];
  for (const chunk of chunks) {
    const localText = sliceLines(l, chunk.local);
    const remoteText = sliceLines(r, chunk.remote);
    const applied = autoApplied && chunk.kind !== "conflict" && chunk.kind !== "unchanged";
    const text = applied ? (chunk.kind === "remote" ? remoteText : localText) : sliceLines(b, chunk.base);
    if (chunk.kind !== "unchanged") {
      const pending: SideState = applied ? "applied" : "pending";
      tracked.push({
        chunk,
        from: doc.length,
        to: doc.length + text.length,
        local: chunk.kind === "remote" ? "none" : pending,
        remote: chunk.kind === "local" ? "none" : pending,
        localText,
        remoteText,
      });
    }
    doc += text;
  }
  return { doc, tracked };
}

/** Extensions for the Result editor's state: the field and undo support. */
export function chunkTracking(tracked: readonly Tracked[]) {
  return [chunkField.init(() => tracked), undoableChunkState];
}

export function isResolved(t: Tracked): boolean {
  return t.local !== "pending" && t.remote !== "pending";
}

/** Number of conflict chunks not yet resolved. */
export function conflictsLeft(tracked: readonly Tracked[]): number {
  return tracked.filter((t) => t.chunk.kind === "conflict" && !isResolved(t)).length;
}

/** Whether "apply non-conflicting" from source would apply chunk t. */
function appliesTo(t: Tracked, source: Source): boolean {
  switch (t.chunk.kind) {
    case "local":
      return source !== "remote" && t.local === "pending";
    case "remote":
      return source !== "local" && t.remote === "pending";
    case "both":
      return t.local === "pending";
    default:
      return false;
  }
}

/** Number of chunks "apply non-conflicting" from source would apply. */
export function nonConflictingLeft(tracked: readonly Tracked[], source: Source = "both"): number {
  return tracked.filter((t) => appliesTo(t, source)).length;
}

const historyAnnotations = [isolateHistory.of("full")];

/**
 * Applies one side of chunk index. Replaces the chunk's current result text
 * with that side's text; if the other side of a conflict was already
 * applied, inserts it next to it instead, local first. Returns null when the
 * side has nothing pending.
 */
export function applySide(state: EditorState, index: number, side: Side): TransactionSpec | null {
  const t = state.field(chunkField)[index];
  if (!t || t[side] !== "pending") return null;
  const other: Side = side === "local" ? "remote" : "local";
  const text = side === "local" ? t.localText : t.remoteText;

  let change: ChangeSpec;
  const from = t.from;
  let to: number;
  if (t.chunk.kind === "conflict" && t[other] === "applied") {
    const current = state.sliceDoc(t.from, t.to);
    if (side === "local") {
      // Local goes first: insert before the remote text already there.
      const insert = text !== "" && current !== "" && !text.endsWith("\n") ? text + "\n" : text;
      change = { from: t.from, insert };
      to = t.to + insert.length;
    } else {
      const insert = current !== "" && text !== "" && !current.endsWith("\n") ? "\n" + text : text;
      change = { from: t.to, insert };
      to = t.to + insert.length;
    }
  } else {
    change = { from: t.from, to: t.to, insert: text };
    to = t.from + text.length;
  }

  // Both sides made the same change: applying either one applies it.
  const both = t.chunk.kind === "both";
  const update: ChunkUpdate = {
    index,
    from,
    to,
    local: side === "local" || both ? "applied" : t.local,
    remote: side === "remote" || both ? "applied" : t.remote,
  };
  return { changes: change, effects: setChunk.of(update), annotations: historyAnnotations, userEvent: "input.apply" };
}

/** Marks one side of chunk index as ignored. Null if nothing is pending. */
export function ignoreSide(state: EditorState, index: number, side: Side): TransactionSpec | null {
  const t = state.field(chunkField)[index];
  if (!t || t[side] !== "pending") return null;
  const both = t.chunk.kind === "both";
  const update: ChunkUpdate = {
    index,
    from: t.from,
    to: t.to,
    local: side === "local" || both ? "ignored" : t.local,
    remote: side === "remote" || both ? "ignored" : t.remote,
  };
  return { effects: setChunk.of(update), annotations: historyAnnotations, userEvent: "input.apply" };
}

/**
 * Applies every unresolved non-conflicting chunk from source in a single
 * transaction (one undo step), using the chunks' current positions.
 * Conflicts and chunks already applied or ignored are left alone. Null if
 * there is nothing to apply.
 */
export function applyNonConflicting(state: EditorState, source: Source = "both"): TransactionSpec | null {
  const tracked = state.field(chunkField);
  const changes: ChangeSpec[] = [];
  const effects: StateEffect<ChunkUpdate>[] = [];
  let delta = 0; // length change so far, to place ranges in the new document
  tracked.forEach((t, index) => {
    if (!appliesTo(t, source)) return;
    const side: Side = t.chunk.kind === "remote" ? "remote" : "local";
    const text = side === "local" ? t.localText : t.remoteText;
    changes.push({ from: t.from, to: t.to, insert: text });
    const from = t.from + delta;
    effects.push(
      setChunk.of({
        index,
        from,
        to: from + text.length,
        local: t.chunk.kind === "remote" ? t.local : "applied",
        remote: t.chunk.kind === "local" ? t.remote : "applied",
      }),
    );
    delta += text.length - (t.to - t.from);
  });
  if (changes.length === 0) return null;
  return { changes, effects, annotations: historyAnnotations, userEvent: "input.apply" };
}
