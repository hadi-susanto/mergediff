// DOM-free logic of the conflicts window (resolveview.ts): chips, texts,
// which actions the selection allows and what they are called. Runs under
// node --test, so it only imports types from the bindings.

import type { MergeCounts } from "../bindings/github.com/hadi-susanto/mergediff/internal/session/models";
import {
  chipCounts,
  filterRows,
  flatItems,
  groupItems,
  type Chip,
  type ListItem,
} from "./filelist-core.ts";

/**
 * A row of the list: session.ConflictRow with plain strings, so the
 * bindings' enums (values Node cannot import) are not needed here.
 */
export interface ConflictRow {
  path: string;
  /** "modified" | "added" | "deleted" */
  yours: string;
  theirs: string;
  /** "text" | "binary" | "notUTF8" | "symlink" | "submodule" */
  kind: string;
  /** "conflict" | "modifyDelete" | "binary" | "symlink" | "submodule" */
  state: string;
  resolved: boolean;
}

/** The user's sides, as the backend names them (gitconflict.Side). */
export type Side = "yours" | "theirs";

export const RESOLVE_CHIPS: Chip<ConflictRow>[] = [
  { id: "all", label: "All", keep: () => true },
  { id: "unresolved", label: "Unresolved", keep: (r) => !r.resolved },
  { id: "resolved", label: "Resolved", keep: (r) => r.resolved },
];

/** What the filter bar is set to. */
export interface ResolveFilter {
  text: string;
  chip: string;
  grouped: boolean;
  collapsed: ReadonlySet<string>;
}

/** What the list shows for a filter, and the chip counts. */
export interface ResolveView {
  items: ListItem<ConflictRow>[];
  chipCounts: Map<string, number>;
}

/** Group headers count their rows by this key. */
export function groupKey(r: ConflictRow): string {
  return r.resolved ? "resolved" : "unresolved";
}

/** Applies the text filter, then the chip, then grouping. Rows come sorted by path. */
export function resolveItems(rows: readonly ConflictRow[], f: ResolveFilter): ResolveView {
  const matched = filterRows(rows, f.text);
  const chip = RESOLVE_CHIPS.find((c) => c.id === f.chip) ?? RESOLVE_CHIPS[0];
  const shown = matched.filter((r) => chip.keep(r));
  const items = f.grouped ? groupItems(shown, groupKey, f.collapsed) : flatItems(shown);
  return { items, chipCounts: chipCounts(matched, RESOLVE_CHIPS) };
}

export function plural(n: number, word: string): string {
  return `${n} ${word}${n === 1 ? "" : "s"}`;
}

const STATUS_NAMES: Record<string, string> = { modified: "Modified", added: "Added", deleted: "Deleted" };

/** "Modified", "Added" or "Deleted". */
export function statusText(status: string): string {
  return STATUS_NAMES[status] ?? status;
}

const STATE_NAMES: Record<string, string> = {
  conflict: "Conflict",
  modifyDelete: "Modify/delete",
  binary: "Binary",
  symlink: "Symlink",
  submodule: "Submodule",
};

/** The State column: the conflict's state, or "✓ Resolved". */
export function stateText(r: ConflictRow): string {
  return r.resolved ? "✓ Resolved" : (STATE_NAMES[r.state] ?? r.state);
}

/** CSS class of the State tag. */
export function stateClass(r: ConflictRow): string {
  return r.resolved ? "st-resolved" : "st-conflict";
}

/** A group header's summary: "2 files · 1 unresolved", or "1 file · all resolved" (done). */
export function groupSummary(counts: ReadonlyMap<string, number>, size: number): { text: string; done: boolean } {
  const open = counts.get("unresolved") ?? 0;
  return open === 0
    ? { text: `${plural(size, "file")} · all resolved`, done: true }
    : { text: `${plural(size, "file")} · ${open} unresolved`, done: false };
}

/** The operation bar's pill: "7 of 9 files unresolved", or "All conflicts resolved" (done). */
export function pillText(unresolved: number, total: number): { text: string; done: boolean } {
  return unresolved === 0
    ? { text: "All conflicts resolved", done: true }
    : { text: `${unresolved} of ${plural(total, "file")} unresolved`, done: false };
}

/** Why Merge… cannot open a row, or null when it can. */
export function mergeBlocked(r: ConflictRow): string | null {
  if (r.resolved) return "already resolved";
  switch (r.state) {
    case "conflict":
      return null;
    case "modifyDelete":
      return "one side deleted it; keep or delete it instead";
    case "binary":
      return r.kind === "notUTF8" ? "it is not valid UTF-8 text" : "it is a binary file";
    case "symlink":
      return "it is a symbolic link";
    case "submodule":
      return "it is a submodule; resolve it with Git";
  }
  return `it cannot be merged (${r.state})`;
}

/** One action button's state. */
export interface ActionState {
  enabled: boolean;
  label: string;
  /** Tooltip: what it does, or why it is disabled. */
  title: string;
}

/** The side panel's actions for the selected rows. */
export interface Actions {
  merge: ActionState;
  yours: ActionState;
  theirs: ActionState;
  /** The rows Accept would resolve (unresolved, not submodules). */
  targets: ConflictRow[];
}

/** "Keep modified (yours)", "Delete (theirs)", … for one modify/delete row. */
function sideLabel(status: string, side: Side): string {
  switch (status) {
    case "deleted":
      return `Delete (${side})`;
    case "added":
      return `Keep added (${side})`;
    default:
      return `Keep modified (${side})`;
  }
}

/**
 * Which actions the selection allows. locked holds the paths open in a
 * merge window (they cannot be accepted until it closes).
 */
export function actionsFor(selected: readonly ConflictRow[], locked: ReadonlySet<string>): Actions {
  const targets = selected.filter((r) => !r.resolved && r.state !== "submodule");
  const lockedOnes = targets.filter((r) => locked.has(r.path));
  const one = selected.length === 1 ? selected[0] : undefined;

  let yoursLabel = "Accept Yours";
  let theirsLabel = "Accept Theirs";
  if (one && targets.length === 1 && one.state === "modifyDelete") {
    yoursLabel = sideLabel(one.yours, "yours");
    theirsLabel = sideLabel(one.theirs, "theirs");
  }
  let acceptTitle = "";
  const acceptOK = targets.length > 0 && lockedOnes.length === 0;
  if (selected.length === 0) acceptTitle = "Select a file";
  else if (targets.length === 0) {
    acceptTitle = one?.state === "submodule" && !one.resolved ? "Resolve submodules with Git" : "Already resolved";
  } else if (lockedOnes.length > 0) {
    acceptTitle = `Close the merge window of ${lockedOnes[0].path} first`;
  }
  const accept = (label: string, keys: string): ActionState => ({
    enabled: acceptOK,
    label,
    title: acceptOK ? `${label} (${keys})` : acceptTitle,
  });

  let merge: ActionState = { enabled: false, label: "Merge…", title: "Select a file" };
  if (selected.length > 1) merge = { enabled: false, label: "Merge…", title: "Select one file to merge" };
  else if (one) {
    const reason = mergeBlocked(one);
    merge = reason !== null
      ? { enabled: false, label: "Merge…", title: `Cannot merge: ${reason}` }
      : locked.has(one.path)
        ? { enabled: true, label: "Merge…", title: "Show its open merge window (Enter)" }
        : { enabled: true, label: "Merge…", title: "Open the 3-way merge (Enter)" };
  }
  return {
    merge,
    yours: accept(yoursLabel, "Ctrl+Shift+Y"),
    theirs: accept(theirsLabel, "Ctrl+Shift+T"),
    targets,
  };
}

/**
 * Whether "Show changes" applies to the selection: one unresolved
 * modify/delete text file that one side kept.
 */
export function canShowChanges(selected: readonly ConflictRow[]): boolean {
  if (selected.length !== 1) return false;
  const r = selected[0];
  return !r.resolved && r.state === "modifyDelete" && r.kind === "text" && (r.yours !== "deleted" || r.theirs !== "deleted");
}

/** The all-resolved banner, or null while conflicts remain or the list is empty. */
export function banner(unresolved: number, total: number, next: string): { text: string; command: string } | null {
  if (unresolved > 0 || total === 0) return null;
  return { text: "All conflicts resolved. Next, run:", command: next || "git commit" };
}

/** The question before closing with merge windows that have unsaved edits. */
export function unsavedText(paths: readonly string[]): string {
  const which = paths.length === 1 ? paths[0] : plural(paths.length, "file");
  return `The merge of ${which} has unsaved edits.\n\nClose anyway and discard ${paths.length === 1 ? "them" : "all of them"}?`;
}

/** The side panel's one-line description of the selection. */
export function describe(selected: readonly ConflictRow[], counts?: MergeCounts, locked?: ReadonlySet<string>): string {
  if (selected.length === 0) return "No file selected.";
  if (selected.length === 1 && !selected[0].resolved && locked?.has(selected[0].path)) {
    return "Open in a merge window. Save or cancel it to accept a side here.";
  }
  if (selected.length > 1) {
    const open = selected.filter((r) => !r.resolved).length;
    return `${plural(selected.length, "file")} selected, ${open} unresolved.`;
  }
  const r = selected[0];
  if (r.resolved) return "Resolved. The result is staged in Git's index.";
  switch (r.state) {
    case "conflict": {
      const what = r.yours === "added" && r.theirs === "added" ? "Both sides added this file." : "Both sides changed this file.";
      if (!counts) return what;
      return `${what} ${plural(counts.conflicts, "conflict")}, ${plural(counts.nonConflicting, "non-conflicting change")}.`;
    }
    case "modifyDelete":
      if (r.yours === "deleted" && r.theirs === "deleted") return "Both sides deleted or renamed this file. Accept either side to delete it.";
      if (r.yours === "deleted") return `Yours deleted this file; theirs ${r.theirs === "added" ? "added" : "modified"} it.`;
      return `Yours ${r.yours === "added" ? "added" : "modified"} this file; theirs deleted it.`;
    case "binary":
      return r.kind === "notUTF8"
        ? "This file is not valid UTF-8 text, so it cannot be merged line by line. Accept yours or theirs."
        : "Binary file: it cannot be merged line by line. Accept yours or theirs.";
    case "symlink":
      return "Symbolic link: accept yours or theirs.";
    case "submodule":
      return "Submodule: mergediff cannot resolve it. Check out the commit you want in the submodule, then git add it.";
  }
  return "";
}

/** The confirmation before accepting several files. */
export function confirmText(n: number, side: Side): string {
  return `Accept ${side === "yours" ? "Yours" : "Theirs"} for ${plural(n, "file")}?`;
}
