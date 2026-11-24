// Directory comparison window (mergediff LEFTDIR RIGHTDIR, git difftool -d):
// the list of files that differ. Opening a file shows it in a diff window of
// its own (Backend.OpenEntry).

import type { Entry, Snapshot } from "../bindings/github.com/hadi-susanto/mergediff/internal/dirdiff/models";
import { showError } from "./dialogs.ts";
import { middleEllipsis } from "./filelist-core.ts";
import { FileList, renderPath, type Column } from "./filelist.ts";
import { installShortcuts } from "./keys.ts";

/** What the directory view needs from the backend and the window. */
export interface DirBackend {
  entries(): Promise<Entry[]>;
  progress(): Promise<Snapshot>;
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
}

const EXIT_OK = 0;
const EXIT_ERROR = 2;
const HINT = "Double-click or Enter to open the file diff · F7 / Shift+F7 next / previous file";
/** "Comparing…" shows only if the comparison takes longer than this, so a fast one never flashes it. */
const COMPARING_DELAY_MS = 150;
const PROGRESS_INTERVAL_MS = 200;
const MAX_OPEN_SIZE = 5 * 1024 * 1024;

const STATUS_NAMES: Record<string, string> = {
  modified: "Modified",
  added: "Added",
  deleted: "Deleted",
  unchanged: "Unchanged",
  error: "Error",
};
const BADGES: Record<string, string> = { modified: "M", added: "A", deleted: "D", unchanged: "", error: "!" };

/** Why a row cannot be opened, or null. */
export function blockedReason(e: Entry): string | null {
  if (e.note) return e.note;
  if (e.leftSize > MAX_OPEN_SIZE || e.rightSize > MAX_OPEN_SIZE) return "larger than 5 MB";
  return null;
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
    render(row, cell) {
      renderPath(cell, row.path);
    },
  },
  {
    key: "lines",
    title: "Lines",
    width: "minmax(120px, 200px)",
    render(row, cell) {
      const reason = row.entry.note;
      cell.title = reason;
      cell.replaceChildren(span("lines-label mono", reason || "…"));
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

/** Builds the directory window into the page (index.html's toolbar, panes and status bar). */
export function renderDirView(o: DirViewOptions): void {
  const statusLeft = document.getElementById("status-left")!;
  const statusRight = document.getElementById("status-right")!;
  const setHint = (text = HINT) => (statusLeft.textContent = text);
  setHint();
  statusRight.textContent = `mergediff ${o.version}`;

  renderPathBar(document.getElementById("toolbar-left")!, o.left, o.right);

  const list = new FileList<DirRow>({
    columns: COLUMNS,
    blocked: (row) => blockedReason(row.entry),
    open: (row) => {
      setHint();
      o.backend.open(row.path).catch((err) => showError(`Could not open ${row.path}: ${err}`));
    },
    refused: (row, reason) => setHint(`${row.path} cannot be opened: ${reason}`),
    selected: () => setHint(),
  });
  const panes = document.getElementById("panes")!;
  panes.classList.add("panes-dir");
  panes.appendChild(list.root);

  // Next / previous file that can be opened, then open it.
  const step = (dir: 1 | -1) => {
    const i = list.find(dir, (row) => blockedReason(row.entry) === null);
    if (i >= 0 && list.select(i)) list.openCurrent();
  };
  installShortcuts({
    next: () => step(1),
    prev: () => step(-1),
    cancel: () => o.backend.quit(EXIT_OK),
  });

  void load(o, list, statusRight);
}

/** Waits for the comparison, showing progress if it takes a while, then shows the list. */
async function load(o: DirViewOptions, list: FileList<DirRow>, statusRight: HTMLElement): Promise<void> {
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
  const rows = entries.filter((e) => e.status !== "unchanged").map((entry) => ({ path: entry.path, entry }));
  if (rows.length === 0) {
    list.showMessage(entries.length === 0 ? "Both directories are empty" : "No differences");
  } else {
    list.setRows(rows);
  }
  list.focus();
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
  window.addEventListener("resize", fit);
}
