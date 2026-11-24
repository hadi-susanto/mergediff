// Generic file list: a table of rows with configurable columns, path cells,
// selection, keyboard navigation and "open" handling. It knows nothing about
// what the rows mean; the directory view (dirview.ts) and, later, the
// conflicts view configure it. The logic lives in filelist-core.ts.

import { Selection, splitPath } from "./filelist-core.ts";

/** One column of the list. */
export interface Column<T> {
  /** Used in CSS class names: fl-col-<key>. */
  key: string;
  title: string;
  /** Second heading line, e.g. a branch name. */
  subtitle?: string;
  /** CSS grid track size, e.g. "48px" or "minmax(0, 1fr)". */
  width: string;
  /** Fills the cell for a row; called again when the row is updated. */
  render(row: T, cell: HTMLElement): void;
}

export interface FileListOptions<T extends { path: string }> {
  columns: Column<T>[];
  /** null if the row can be opened, otherwise why not. */
  blocked(row: T): string | null;
  /** Double-click or Enter on a row that can be opened. */
  open(row: T): void;
  /** Double-click or Enter on a row that cannot be opened. */
  refused(row: T, reason: string): void;
  /** The selection changed. */
  selected?(rows: T[]): void;
  /** Ctrl/Cmd+click and Shift+click select several rows. */
  multi?: boolean;
}

/** Fills cell with a path: the muted directory part (truncated first) and the bold file name. */
export function renderPath(cell: HTMLElement, path: string, nameOnly = false): void {
  const { dir, name } = splitPath(path);
  cell.replaceChildren();
  cell.classList.add("fl-path");
  cell.title = path;
  if (dir && !nameOnly) {
    const d = document.createElement("span");
    d.className = "fl-dir";
    d.textContent = dir;
    cell.appendChild(d);
  }
  const n = document.createElement("span");
  n.className = "fl-name";
  n.textContent = name;
  cell.appendChild(n);
}

export class FileList<T extends { path: string }> {
  readonly root: HTMLElement;
  private readonly body: HTMLElement;
  private readonly o: FileListOptions<T>;
  private readonly sel: Selection;
  private rows: T[] = [];
  private elements: HTMLElement[] = [];

  constructor(o: FileListOptions<T>) {
    this.o = o;
    this.sel = new Selection(0, () => true, o.multi ?? false);
    const template = o.columns.map((c) => c.width).join(" ");

    this.root = document.createElement("div");
    this.root.className = "fl";
    this.root.setAttribute("role", "grid");
    this.root.tabIndex = 0;
    this.root.style.setProperty("--fl-columns", template);

    const header = document.createElement("div");
    header.className = "fl-row fl-header";
    header.setAttribute("role", "row");
    for (const c of o.columns) {
      const h = document.createElement("div");
      h.className = `fl-cell fl-col-${c.key}`;
      h.setAttribute("role", "columnheader");
      h.textContent = c.title;
      if (c.subtitle) {
        const sub = document.createElement("div");
        sub.className = "fl-subtitle";
        sub.textContent = c.subtitle;
        h.appendChild(sub);
      }
      header.appendChild(h);
    }
    this.body = document.createElement("div");
    this.body.className = "fl-body";
    this.root.append(header, this.body);

    this.root.addEventListener("keydown", (e) => this.onKey(e));
  }

  /** Shows rows (in the given order), keeping the selected path if it is still listed. */
  setRows(rows: T[]): void {
    const keep = this.current()?.path;
    this.rows = rows;
    this.elements = rows.map((row, i) => this.makeRow(row, i));
    this.body.replaceChildren(...this.elements);
    const again = keep === undefined ? -1 : rows.findIndex((r) => r.path === keep);
    this.sel.reset(rows.length, () => true, again);
    if (this.sel.current < 0) this.sel.edge(1);
    this.paintSelection();
  }

  /** Replaces the rows with a message, e.g. "Comparing…" or "No differences". */
  showMessage(text: string): void {
    const m = document.createElement("div");
    m.className = "fl-message";
    m.setAttribute("role", "status");
    m.textContent = text;
    this.rows = [];
    this.elements = [];
    this.body.replaceChildren(m);
    this.sel.reset(0);
  }

  /** Renders row i's cells again, e.g. after its line counts arrived. */
  update(i: number): void {
    const el = this.elements[i];
    if (!el) return;
    this.o.columns.forEach((c, k) => c.render(this.rows[i], el.children[k] as HTMLElement));
    el.classList.toggle("fl-blocked", this.o.blocked(this.rows[i]) !== null);
  }

  /** The rows as shown. */
  all(): readonly T[] {
    return this.rows;
  }

  /** The current (keyboard focus) row. */
  current(): T | undefined {
    return this.rows[this.sel.current];
  }

  /** The selected rows. */
  selectedRows(): T[] {
    return this.sel.rows().map((i) => this.rows[i]);
  }

  /** Selects row i and scrolls it into view. */
  select(i: number): boolean {
    if (!this.sel.select(i)) return false;
    this.paintSelection();
    return true;
  }

  /** Index of the next (dir 1) or previous (dir -1) row after the current one for which want() holds, or -1. */
  find(dir: 1 | -1, want: (row: T) => boolean): number {
    const from = this.sel.current < 0 ? (dir === 1 ? -1 : this.rows.length) : this.sel.current;
    return this.sel.find(from, dir, (i) => want(this.rows[i]));
  }

  /** Opens the current row, or reports why it cannot be opened. */
  openCurrent(): void {
    const row = this.current();
    if (!row) return;
    const reason = this.o.blocked(row);
    if (reason === null) this.o.open(row);
    else this.o.refused(row, reason);
  }

  focus(): void {
    this.root.focus();
  }

  private makeRow(row: T, i: number): HTMLElement {
    const el = document.createElement("div");
    el.className = "fl-row";
    el.setAttribute("role", "row");
    for (const c of this.o.columns) {
      const cell = document.createElement("div");
      cell.className = `fl-cell fl-col-${c.key}`;
      cell.setAttribute("role", "gridcell");
      c.render(row, cell);
      el.appendChild(cell);
    }
    el.classList.toggle("fl-blocked", this.o.blocked(row) !== null);
    el.addEventListener("mousedown", (e) => {
      if (e.button !== 0) return;
      const ok =
        e.shiftKey ? this.sel.extendTo(i) : e.ctrlKey || e.metaKey ? this.sel.toggle(i) : this.sel.select(i);
      if (ok) this.paintSelection(false);
    });
    el.addEventListener("dblclick", () => {
      if (this.sel.select(i)) this.paintSelection(false);
      this.openCurrent();
    });
    return el;
  }

  private onKey(e: KeyboardEvent): void {
    if (e.altKey || e.ctrlKey || e.metaKey) return;
    let moved = false;
    switch (e.key) {
      case "ArrowDown":
        moved = this.sel.move(1);
        break;
      case "ArrowUp":
        moved = this.sel.move(-1);
        break;
      case "Home":
        moved = this.sel.edge(1);
        break;
      case "End":
        moved = this.sel.edge(-1);
        break;
      case "Enter":
        e.preventDefault();
        this.openCurrent();
        return;
      default:
        return;
    }
    e.preventDefault();
    if (moved) this.paintSelection();
  }

  private paintSelection(scroll = true): void {
    this.elements.forEach((el, i) => {
      const on = this.sel.has(i);
      el.classList.toggle("fl-selected", on);
      el.setAttribute("aria-selected", String(on));
    });
    const cur = this.elements[this.sel.current];
    if (scroll && cur) cur.scrollIntoView({ block: "nearest" });
    this.o.selected?.(this.selectedRows());
  }
}
