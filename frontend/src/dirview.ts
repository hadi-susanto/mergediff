// Directory comparison window (mergediff LEFTDIR RIGHTDIR, git difftool -d):
// the list of files that differ. Opening a file shows it in a diff window of
// its own (Backend.OpenEntry).

import type {
  Entry,
  Snapshot,
  Stat,
  StatsBatch,
} from "../bindings/github.com/hadi-susanto/mergediff/internal/dirdiff/models";
import { showError } from "./dialogs.ts";
import {
  chipCounts,
  filterRows,
  flatItems,
  groupItems,
  middleEllipsis,
  type Chip,
  type GroupItem,
  type ListItem,
} from "./filelist-core.ts";
import { FileList, renderPath, type Column } from "./filelist.ts";
import { installShortcuts } from "./keys.ts";

/** What the directory view needs from the backend and the window. */
export interface DirBackend {
  entries(): Promise<Entry[]>;
  progress(): Promise<Snapshot>;
  stats(): Promise<StatsBatch>;
  open(path: string): Promise<void>;
  quit(code: number): void;
}

export interface DirViewOptions {
  left: string;
  right: string;
  version: string;
  backend: DirBackend;
}

/** A row of the list. */
export interface DirRow {
  path: string;
  entry: Entry;
  /** Line counts, once computed (changed files only). */
  stat?: Stat;
}

const EXIT_OK = 0;
const EXIT_ERROR = 2;
const HINT = "Double-click or Enter to open the file diff · F7 / Shift+F7 next / previous file";
/** "Comparing…" shows only if the comparison takes longer than this, so a fast one never flashes it. */
const COMPARING_DELAY_MS = 150;
const PROGRESS_INTERVAL_MS = 200;
const STATS_INTERVAL_MS = 150;
const MAX_OPEN_SIZE = 5 * 1024 * 1024;

const STATUS_NAMES: Record<string, string> = {
  modified: "Modified",
  added: "Added",
  deleted: "Deleted",
  unchanged: "Unchanged",
  error: "Error",
};
const BADGES: Record<string, string> = { modified: "M", added: "A", deleted: "D", unchanged: "", error: "!" };

/** Why a row cannot be opened, or null. stat adds what reading the files found (binary, not UTF-8). */
export function blockedReason(e: Entry, stat?: Stat): string | null {
  if (e.note) return e.note;
  if (e.leftSize > MAX_OPEN_SIZE || e.rightSize > MAX_OPEN_SIZE) return "larger than 5 MB";
  return stat?.reason || null;
}

/** What the Lines column shows: line counts, or a label (muted) instead. */
export type LinesCell = { label: string } | { added?: number; deleted?: number };

export function linesCell(e: Entry, stat?: Stat): LinesCell {
  if (e.note) return { label: e.note };
  if (e.status === "unchanged") return { label: "" };
  if (!stat) return { label: "…" };
  if (stat.label) return { label: stat.label };
  if (e.status === "added") return { added: stat.added };
  if (e.status === "deleted") return { deleted: stat.deleted };
  return {
    added: stat.added > 0 || stat.deleted === 0 ? stat.added : undefined,
    deleted: stat.deleted > 0 || stat.added === 0 ? stat.deleted : undefined,
  };
}

/** Status bar counts: "11 changed · 184 unchanged · 2 errors". */
export function countsText(entries: readonly Entry[]): string {
  let changed = 0;
  let unchanged = 0;
  let errors = 0;
  for (const e of entries) {
    if (e.status === "unchanged") unchanged++;
    else if (e.status === "error") errors++;
    else changed++;
  }
  const parts = [`${changed} changed`, `${unchanged} unchanged`];
  if (errors > 0) parts.push(`${errors} ${errors === 1 ? "error" : "errors"}`);
  return parts.join(" · ");
}

/** The "Comparing…" text for a progress snapshot. */
export function progressText(p: Snapshot): string {
  return `Comparing… ${p.found.toLocaleString("en-US")} files found, ${p.compared.toLocaleString("en-US")} compared`;
}

function span(cls: string, text: string): HTMLSpanElement {
  const s = document.createElement("span");
  s.className = cls;
  s.textContent = text;
  return s;
}

const COLUMNS: Column<DirRow>[] = [
  {
    key: "badge",
    title: "",
    width: "44px",
    render(row, cell) {
      const letter = BADGES[row.entry.status] ?? "";
      cell.replaceChildren(letter ? span(`badge st-${row.entry.status}`, letter) : "");
    },
  },
  {
    key: "file",
    title: "File",
    width: "minmax(0, 1fr)",
    render(row, cell, nameOnly) {
      renderPath(cell, row.path, nameOnly);
    },
  },
  {
    key: "lines",
    title: "Lines",
    width: "minmax(120px, 200px)",
    render(row, cell) {
      const c = linesCell(row.entry, row.stat);
      cell.title = "label" in c ? c.label : "";
      if ("label" in c) {
        cell.replaceChildren(span("lines-label mono", c.label));
        return;
      }
      const parts: HTMLElement[] = [];
      if (c.added !== undefined) parts.push(span("lines-added mono", `+${c.added}`));
      if (c.deleted !== undefined) parts.push(span("lines-deleted mono", `−${c.deleted}`));
      cell.replaceChildren(...parts);
    },
  },
  {
    key: "status",
    title: "Status",
    width: "120px",
    render(row, cell) {
      const s = row.entry.status;
      cell.replaceChildren(span(`tag st-${s}`, STATUS_NAMES[s] ?? s));
    },
  },
];

/** The filter chips of the directory view. "All changes" keeps every row shown. */
export const DIR_CHIPS: Chip<DirRow>[] = [
  { id: "all", label: "All changes", keep: () => true },
  { id: "modified", label: "Modified", keep: (r) => r.entry.status === "modified" },
  { id: "added", label: "Added", keep: (r) => r.entry.status === "added" },
  { id: "deleted", label: "Deleted", keep: (r) => r.entry.status === "deleted" },
];

/** What the filter bar is set to. */
export interface DirFilter {
  text: string;
  chip: string;
  showUnchanged: boolean;
  grouped: boolean;
  /** Collapsed group directories (Group by directory). */
  collapsed: ReadonlySet<string>;
}

/** What the list shows for a filter, and the counts the filter bar shows. */
export interface DirView {
  items: ListItem<DirRow>[];
  /** Rows each chip keeps, after the text filter. */
  chipCounts: Map<string, number>;
  /** Unchanged files in the comparison (for "Show unchanged (N)"). */
  unchanged: number;
}

/**
 * Applies the filter bar to the rows (sorted by path): unchanged files only
 * with Show unchanged, then the text filter, then the chip, then grouping.
 */
export function dirItems(rows: readonly DirRow[], f: DirFilter): DirView {
  let unchanged = 0;
  for (const r of rows) if (r.entry.status === "unchanged") unchanged++;
  const base = f.showUnchanged ? rows : rows.filter((r) => r.entry.status !== "unchanged");
  const matched = filterRows(base, f.text);
  const chip = DIR_CHIPS.find((c) => c.id === f.chip) ?? DIR_CHIPS[0];
  const shown = matched.filter((r) => chip.keep(r));
  const items = f.grouped ? groupItems(shown, (r) => r.entry.status, f.collapsed) : flatItems(shown);
  return { items, chipCounts: chipCounts(matched, DIR_CHIPS), unchanged };
}

/** Status tags on a directory header, e.g. "1 modified", "2 added". */
export function groupTags(counts: ReadonlyMap<string, number>): { status: string; text: string }[] {
  const out: { status: string; text: string }[] = [];
  for (const status of ["modified", "added", "deleted", "error", "unchanged"]) {
    const n = counts.get(status) ?? 0;
    if (n > 0) out.push({ status, text: `${n} ${status === "error" ? (n === 1 ? "error" : "errors") : status}` });
  }
  return out;
}

const FOLDER_SVG =
  '<svg width="14" height="12" viewBox="0 0 14 12" aria-hidden="true"><path d="M1 2.5A1.5 1.5 0 0 1 2.5 1h3l1.5 1.5h4.5A1.5 1.5 0 0 1 13 4v5.5A1.5 1.5 0 0 1 11.5 11h-9A1.5 1.5 0 0 1 1 9.5z" fill="none" stroke="currentColor"/></svg>';

function renderGroup(g: GroupItem, cell: HTMLElement): void {
  const chevron = span("fl-group-chevron", g.collapsed ? "▸" : "▾");
  const icon = document.createElement("span");
  icon.className = "fl-group-icon";
  icon.innerHTML = FOLDER_SVG;
  const label = span("fl-group-label", g.label);
  label.title = g.dir || g.label;
  cell.replaceChildren(chevron, icon, label, ...groupTags(g.counts).map((t) => span(`tag st-${t.status}`, t.text)));
}

/** The filter bar: text filter, chips, Show unchanged and Group by directory. */
interface FilterBar {
  root: HTMLElement;
  input: HTMLInputElement;
  /** Updates the chip counts, the active chip and the Show unchanged label. */
  update(view: DirView, f: DirFilter): void;
}

function makeFilterBar(f: DirFilter, changed: () => void): FilterBar {
  const root = document.createElement("div");
  root.className = "filter-bar";

  const input = document.createElement("input");
  input.type = "search";
  input.className = "filter-input";
  input.placeholder = "Filter files…";
  input.setAttribute("aria-label", "Filter files");
  input.addEventListener("input", () => {
    f.text = input.value;
    changed();
  });

  const chips = document.createElement("div");
  chips.className = "chips";
  chips.setAttribute("role", "radiogroup");
  const chipButtons = DIR_CHIPS.map((c) => {
    const b = document.createElement("button");
    b.className = "chip";
    b.setAttribute("role", "radio");
    b.addEventListener("click", () => {
      f.chip = c.id;
      changed();
    });
    chips.appendChild(b);
    return b;
  });

  const checkbox = (label: string, on: boolean, set: (v: boolean) => void) => {
    const l = document.createElement("label");
    l.className = "check";
    const box = document.createElement("input");
    box.type = "checkbox";
    box.checked = on;
    box.addEventListener("change", () => {
      set(box.checked);
      changed();
    });
    const text = span("", label);
    l.append(box, text);
    return { l, text };
  };
  const unchanged = checkbox("Show unchanged", f.showUnchanged, (v) => (f.showUnchanged = v));
  const grouped = checkbox("Group by directory", f.grouped, (v) => (f.grouped = v));

  const spacer = document.createElement("div");
  spacer.className = "filter-spacer";
  root.append(input, chips, spacer, unchanged.l, grouped.l);

  return {
    root,
    input,
    update(view, f) {
      DIR_CHIPS.forEach((c, i) => {
        const on = c.id === f.chip;
        chipButtons[i].textContent = `${c.label} ${view.chipCounts.get(c.id) ?? 0}`;
        chipButtons[i].classList.toggle("chip-active", on);
        chipButtons[i].setAttribute("aria-checked", String(on));
      });
      unchanged.l.hidden = view.unchanged === 0;
      unchanged.text.textContent = `Show unchanged (${view.unchanged.toLocaleString("en-US")})`;
    },
  };
}

/** Builds the directory window into the page (index.html's toolbar, panes and status bar). */
export function renderDirView(o: DirViewOptions): void {
  const statusLeft = document.getElementById("status-left")!;
  const statusRight = document.getElementById("status-right")!;
  const setHint = (text = HINT) => (statusLeft.textContent = text);
  setHint();
  statusRight.textContent = `mergediff ${o.version}`;

  renderPathBar(document.getElementById("toolbar-left")!, o.left, o.right);

  // Filter state lives for this window only (no setting).
  const filter: DirFilter = { text: "", chip: "all", showUnchanged: false, grouped: false, collapsed: new Set() };
  let rows: DirRow[] | null = null; // null until the comparison is done

  const show = () => {
    if (!rows) return;
    const view = dirItems(rows, filter);
    bar.update(view, filter);
    if (view.items.length > 0) list.setItems(view.items);
    else if (rows.length === 0) list.showMessage("Both directories are empty");
    else if (filter.text || filter.chip !== "all") list.showMessage("No files match the filter");
    else list.showMessage("No differences");
  };

  const list = new FileList<DirRow>({
    columns: COLUMNS,
    renderGroup,
    toggleGroup(dir, collapse) {
      const c = new Set(filter.collapsed);
      if (collapse) c.add(dir);
      else c.delete(dir);
      filter.collapsed = c;
      show();
    },
    blocked: (row) => blockedReason(row.entry, row.stat),
    open: (row) => {
      setHint();
      o.backend.open(row.path).catch((err) => showError(`Could not open ${row.path}: ${err}`));
    },
    refused: (row, reason) => setHint(`${row.path} cannot be opened: ${reason}`),
    selected: () => setHint(),
  });
  const bar = makeFilterBar(filter, show);
  bar.input.addEventListener("keydown", (e) => {
    if (e.key === "ArrowDown" || e.key === "Enter") {
      e.preventDefault();
      list.focus();
    }
  });

  const panes = document.getElementById("panes")!;
  panes.classList.add("panes-dir");
  panes.append(bar.root, list.root);

  // Next / previous file that can be opened, then open it.
  const step = (dir: 1 | -1) => {
    const i = list.find(dir, (row) => blockedReason(row.entry, row.stat) === null);
    if (i >= 0 && list.select(i)) list.openCurrent();
  };
  installShortcuts({
    next: () => step(1),
    prev: () => step(-1),
    find: () => {
      bar.input.focus();
      bar.input.select();
    },
    // Esc clears the filter first; with an empty filter it closes the window.
    cancel: () => {
      if (filter.text) {
        bar.input.value = "";
        filter.text = "";
        show();
        list.focus();
      } else {
        o.backend.quit(EXIT_OK);
      }
    },
  });

  void load(o, list, statusRight, (loaded) => {
    rows = loaded;
    show();
    list.focus();
  });
}

/** Waits for the comparison, showing progress if it takes a while, then hands over the rows and loads the line counts. */
async function load(
  o: DirViewOptions,
  list: FileList<DirRow>,
  statusRight: HTMLElement,
  loaded: (rows: DirRow[]) => void,
): Promise<void> {
  let polling: number | undefined;
  const poll = () => {
    o.backend
      .progress()
      .then((p) => {
        if (!p.done && polling !== undefined) list.showMessage(progressText(p));
      })
      .catch((err) => console.error("progress failed", err));
  };
  const comparing = window.setTimeout(() => {
    list.showMessage("Comparing…");
    poll();
    polling = window.setInterval(poll, PROGRESS_INTERVAL_MS);
  }, COMPARING_DELAY_MS);

  let entries: Entry[];
  try {
    entries = await o.backend.entries();
  } catch (err) {
    showError(`Could not compare the directories: ${err}`, () => o.backend.quit(EXIT_ERROR));
    return;
  } finally {
    clearTimeout(comparing);
    clearInterval(polling);
    polling = undefined;
  }

  statusRight.textContent = `${countsText(entries)} · mergediff ${o.version}`;
  const rows = entries.map((entry) => ({ path: entry.path, entry }));
  loaded(rows);
  await loadStats(o, list, rows);
}

/** Fills in the line counts as the backend computes them. */
async function loadStats(o: DirViewOptions, list: FileList<DirRow>, rows: DirRow[]): Promise<void> {
  const byPath = new Map(rows.map((r) => [r.path, r]));
  for (;;) {
    let batch: StatsBatch;
    try {
      batch = await o.backend.stats();
    } catch (err) {
      console.error("line counts failed", err);
      return;
    }
    for (const ps of batch.stats ?? []) {
      const row = byPath.get(ps.path);
      if (row) row.stat = ps.stat;
    }
    if (batch.stats?.length) list.redraw();
    if (batch.done) return;
    await new Promise((resolve) => setTimeout(resolve, STATS_INTERVAL_MS));
  }
}

/** "Left <path> ↔ Right <path>", each path shortened in the middle to fit, full path in a tooltip. */
function renderPathBar(host: HTMLElement, left: string, right: string): void {
  const bar = document.createElement("div");
  bar.className = "path-bar";
  const l = span("path-bar-path mono", left);
  const r = span("path-bar-path mono", right);
  l.title = left;
  r.title = right;
  bar.append(span("path-bar-label", "Left"), l, span("path-bar-arrow", "↔"), span("path-bar-label", "Right"), r);
  host.appendChild(bar);
  host.parentElement?.classList.add("toolbar-paths");

  const fit = () => {
    l.textContent = left;
    r.textContent = right;
    const room = host.clientWidth;
    if (bar.scrollWidth <= room) return;
    const paths = l.offsetWidth + r.offsetWidth;
    const labels = bar.scrollWidth - paths;
    const charWidth = paths / (Array.from(left).length + Array.from(right).length);
    const perPath = Math.max(8, Math.floor((room - labels) / 2 / charWidth));
    l.textContent = middleEllipsis(left, perPath);
    r.textContent = middleEllipsis(right, perPath);
  };
  fit();
  requestAnimationFrame(fit); // again once the layout has settled
  window.addEventListener("resize", fit);
}
