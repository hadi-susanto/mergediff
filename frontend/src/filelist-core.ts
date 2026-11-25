// DOM-free logic of the file list (filelist.ts): path splitting, ordering,
// truncation and the selection model. Shared by the directory diff and, later,
// the conflicts window; runs under node --test.

/** A relative path split into its directory part and file name. */
export interface PathParts {
  /** Directory with its trailing "/", or "" for a file in the root. */
  dir: string;
  name: string;
}

/** Splits a "/"-separated relative path into directory and file name. */
export function splitPath(path: string): PathParts {
  const i = path.lastIndexOf("/");
  return i < 0 ? { dir: "", name: path } : { dir: path.slice(0, i + 1), name: path.slice(i + 1) };
}

/** Compares paths in code-unit order, the order the backend sorts in for ASCII. */
export function comparePaths(a: string, b: string): number {
  return a < b ? -1 : a > b ? 1 : 0;
}

/** Returns rows sorted by path; the input is not changed. */
export function sortByPath<T extends { path: string }>(rows: readonly T[]): T[] {
  return [...rows].sort((a, b) => comparePaths(a.path, b.path));
}

/**
 * Shortens text to at most max characters by replacing its middle with "…",
 * keeping both ends (for absolute paths, whose start and end both matter).
 */
export function middleEllipsis(text: string, max: number): string {
  const chars = Array.from(text);
  if (chars.length <= max) return text;
  if (max <= 1) return "…".slice(0, Math.max(0, max));
  const keep = max - 1;
  const head = Math.ceil(keep / 2);
  const tail = keep - head;
  return chars.slice(0, head).join("") + "…" + (tail > 0 ? chars.slice(chars.length - tail).join("") : "");
}

/**
 * Which rows of a list are selected. Rows are indexes into the rows as
 * shown; rows for which selectable() is false (e.g. group headers) are
 * skipped by keyboard movement. One row is current (the keyboard focus);
 * the selection is that row alone unless multi-select added others.
 */
export class Selection {
  current = -1;
  private extra = new Set<number>();
  private anchor = -1;
  private count: number;
  private selectable: (i: number) => boolean;
  private readonly multi: boolean;

  constructor(count: number, selectable: (i: number) => boolean = () => true, multi = false) {
    this.count = count;
    this.selectable = selectable;
    this.multi = multi;
  }

  /** Resets for a new list of rows, keeping the current row if it still exists. */
  reset(count: number, selectable: (i: number) => boolean = this.selectable, current = this.current): void {
    this.count = count;
    this.selectable = selectable;
    this.extra.clear();
    this.current = current >= 0 && current < count && selectable(current) ? current : -1;
    this.anchor = this.current;
  }

  /** Whether row i is selected. */
  has(i: number): boolean {
    return i === this.current || this.extra.has(i);
  }

  /** The selected rows, ascending. */
  rows(): number[] {
    const all = new Set(this.extra);
    if (this.current >= 0) all.add(this.current);
    return [...all].sort((a, b) => a - b);
  }

  /** Makes row i the only selected row. Returns false if it cannot be selected. */
  select(i: number): boolean {
    if (i < 0 || i >= this.count || !this.selectable(i)) return false;
    this.current = i;
    this.anchor = i;
    this.extra.clear();
    return true;
  }

  /** Multi-select: adds or removes row i (Ctrl/Cmd+click). */
  toggle(i: number): boolean {
    if (!this.multi) return this.select(i);
    if (i < 0 || i >= this.count || !this.selectable(i)) return false;
    if (this.has(i) && this.rows().length > 1) {
      this.extra.delete(i);
      if (this.current === i) this.current = this.rows()[0];
    } else {
      if (this.current >= 0) this.extra.add(this.current);
      this.current = i;
    }
    this.anchor = i;
    return true;
  }

  /** Multi-select: selects the selectable rows from the anchor to i (Shift+click). */
  extendTo(i: number): boolean {
    if (!this.multi || this.anchor < 0) return this.select(i);
    if (i < 0 || i >= this.count || !this.selectable(i)) return false;
    this.extra.clear();
    const [from, to] = this.anchor <= i ? [this.anchor, i] : [i, this.anchor];
    for (let j = from; j <= to; j++) if (this.selectable(j)) this.extra.add(j);
    this.current = i;
    return true;
  }

  /** Moves to the next (dir 1) or previous (dir -1) selectable row; stays at the ends. */
  move(dir: 1 | -1): boolean {
    const from = this.current < 0 ? (dir === 1 ? -1 : this.count) : this.current;
    const i = this.find(from, dir, () => true);
    return i >= 0 && this.select(i);
  }

  /** Selects the first (dir 1) or last (dir -1) selectable row. */
  edge(dir: 1 | -1): boolean {
    const i = this.find(dir === 1 ? -1 : this.count, dir, () => true);
    return i >= 0 && this.select(i);
  }

  /**
   * Index of the first selectable row after (dir 1) or before (dir -1) row
   * from for which want() holds, or -1.
   */
  find(from: number, dir: 1 | -1, want: (i: number) => boolean): number {
    for (let i = from + dir; i >= 0 && i < this.count; i += dir) {
      if (this.selectable(i) && want(i)) return i;
    }
    return -1;
  }
}

/** Whether path contains text, ignoring case. Empty text matches everything. */
export function matchesText(path: string, text: string): boolean {
  if (!text) return true;
  return path.toLowerCase().includes(text.toLowerCase());
}

/** The rows whose path contains text (ignoring case) and that keep() accepts. */
export function filterRows<T extends { path: string }>(
  rows: readonly T[],
  text: string,
  keep: (row: T) => boolean = () => true,
): T[] {
  const needle = text.toLowerCase();
  return rows.filter((r) => (!needle || r.path.toLowerCase().includes(needle)) && keep(r));
}

/** Counts rows by key, e.g. by status. */
export function countBy<T>(rows: readonly T[], key: (row: T) => string): Map<string, number> {
  const counts = new Map<string, number>();
  for (const r of rows) {
    const k = key(r);
    counts.set(k, (counts.get(k) ?? 0) + 1);
  }
  return counts;
}

/** A chip of the filter bar: a name, which rows it keeps, and how many there are. */
export interface Chip<T> {
  id: string;
  label: string;
  keep(row: T): boolean;
}

/** How many rows each chip keeps. */
export function chipCounts<T>(rows: readonly T[], chips: readonly Chip<T>[]): Map<string, number> {
  const counts = new Map<string, number>();
  for (const c of chips) counts.set(c.id, 0);
  for (const r of rows) {
    for (const c of chips) if (c.keep(r)) counts.set(c.id, counts.get(c.id)! + 1);
  }
  return counts;
}

/** Label of the group of files directly in the compared root. */
export const ROOT_LABEL = "(root)";

/** A directory header row in a grouped list. */
export interface GroupItem {
  kind: "group";
  /** The directory with its trailing "/", or "" for the root. */
  dir: string;
  /** What the header shows: dir, or ROOT_LABEL. */
  label: string;
  /** Files in the group by key (e.g. status), before collapsing. */
  counts: Map<string, number>;
  /** How many files the group has. */
  size: number;
  collapsed: boolean;
}

/** A file row; in a grouped list it shows only the file name. */
export interface RowItem<T> {
  kind: "row";
  row: T;
  /** The group's dir in a grouped list; undefined in a flat list. */
  group?: string;
}

/** What a list shows, top to bottom. */
export type ListItem<T> = GroupItem | RowItem<T>;

/** The items of a flat list: every row, in order. */
export function flatItems<T>(rows: readonly T[]): RowItem<T>[] {
  return rows.map((row) => ({ kind: "row", row }));
}

/**
 * Groups rows by parent directory, flat (no tree): the root group first,
 * labelled ROOT_LABEL, then the other directories sorted by path. Rows keep
 * their order within a group. A collapsed group (dir in collapsed) shows
 * only its header. Groups without rows do not appear, so filtering first
 * hides groups left empty.
 */
export function groupItems<T extends { path: string }>(
  rows: readonly T[],
  key: (row: T) => string,
  collapsed: ReadonlySet<string>,
): ListItem<T>[] {
  const groups = new Map<string, T[]>();
  for (const r of rows) {
    const { dir } = splitPath(r.path);
    let g = groups.get(dir);
    if (!g) groups.set(dir, (g = []));
    g.push(r);
  }
  const dirs = [...groups.keys()].sort((a, b) => (a === "" ? -1 : b === "" ? 1 : comparePaths(a, b)));
  const items: ListItem<T>[] = [];
  for (const dir of dirs) {
    const members = groups.get(dir)!;
    const isCollapsed = collapsed.has(dir);
    items.push({
      kind: "group",
      dir,
      label: dir === "" ? ROOT_LABEL : dir,
      counts: countBy(members, key),
      size: members.length,
      collapsed: isCollapsed,
    });
    if (!isCollapsed) for (const row of members) items.push({ kind: "row", row, group: dir });
  }
  return items;
}
