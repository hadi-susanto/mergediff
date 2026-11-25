// Generic file list: a table of rows with configurable columns, path cells,
// optional directory group headers, selection, keyboard navigation and
// "open" handling. Only the rows in view are in the DOM (virtual scrolling),
// so lists of tens of thousands of files stay fast. It knows nothing about
// what the rows mean; the directory view (dirview.ts) and, later, the
// conflicts view configure it. The logic lives in filelist-core.ts.

import {
  scrollToShow,
  Selection,
  splitPath,
  visibleWindow,
  type GroupItem,
  type ListItem,
} from "./filelist-core.ts";

/** Height of every row in px; must match .fl-body .fl-row in styles.css. */
export const ROW_HEIGHT = 32;

/** One column of the list. */
export interface Column<T> {
  /** Used in CSS class names: fl-col-<key>. */
  key: string;
  title: string;
  /** Second heading line, e.g. a branch name. */
  subtitle?: string;
  /** CSS grid track size, e.g. "48px" or "minmax(0, 1fr)". */
  width: string;
  /** Fills the cell for a row. nameOnly: the row is under a directory header. */
  render(row: T, cell: HTMLElement, nameOnly: boolean): void;
}

export interface FileListOptions<T extends { path: string }> {
  columns: Column<T>[];
  /** Fills a directory header row (one cell spanning the columns). */
  renderGroup(group: GroupItem, cell: HTMLElement): void;
  /** A header was collapsed or expanded (click, Enter, ←/→). */
  toggleGroup(dir: string, collapse: boolean): void;
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
  /** Extra CSS classes for a row element, e.g. "fl-faded" for resolved rows. */
  rowClass?(row: T): string;
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

const FOLDER_SVG =
  '<svg width="14" height="12" viewBox="0 0 14 12" aria-hidden="true"><path d="M1 2.5A1.5 1.5 0 0 1 2.5 1h3l1.5 1.5h4.5A1.5 1.5 0 0 1 13 4v5.5A1.5 1.5 0 0 1 11.5 11h-9A1.5 1.5 0 0 1 1 9.5z" fill="none" stroke="currentColor"/></svg>';

/** The start of a directory header: ▾/▸, a folder icon and the directory (full path in a tooltip). */
export function groupHeader(g: GroupItem): HTMLElement[] {
  const chevron = document.createElement("span");
  chevron.className = "fl-group-chevron";
  chevron.textContent = g.collapsed ? "▸" : "▾";
  const icon = document.createElement("span");
  icon.className = "fl-group-icon";
  icon.innerHTML = FOLDER_SVG;
  const label = document.createElement("span");
  label.className = "fl-group-label";
  label.textContent = g.label;
  label.title = g.dir || g.label;
  return [chevron, icon, label];
}

/** How close two presses on the same row must be to count as a double-click. */
export const DOUBLE_CLICK_MS = 500;

/** Whether a press on row index at time completes a double-click after last. */
export function isDoubleClick(last: { index: number; time: number }, index: number, time: number): boolean {
  return last.index === index && index >= 0 && time - last.time <= DOUBLE_CLICK_MS;
}

/** Identifies an item across setItems calls, to keep the selection. */
function itemKey<T extends { path: string }>(item: ListItem<T> | undefined): string | undefined {
  if (!item) return undefined;
  return item.kind === "group" ? `group:${item.dir}` : `row:${item.row.path}`;
}

export class FileList<T extends { path: string }> {
  readonly root: HTMLElement;
  private readonly body: HTMLElement;
  private readonly canvas: HTMLElement;
  private readonly o: FileListOptions<T>;
  private readonly sel: Selection;
  private items: ListItem<T>[] = [];
  private message: HTMLElement | null = null;
  private frame = 0;
  /**
   * Row elements in the DOM, by item index. They stay in place while the
   * selection changes: replacing the element under the mouse would stop
   * the click from focusing the list and the double-click from firing.
   */
  private rendered = new Map<number, HTMLElement>();
  /** The rendered rows show stale content (new items, or redraw()). */
  private dirty = false;
  /**
   * The last plain press, to recognise a double-click ourselves: the
   * browser's dblclick is lost when a redraw (e.g. line counts arriving)
   * replaces the clicked cell between the two presses.
   */
  private lastPress = { index: -1, time: 0 };

  constructor(o: FileListOptions<T>) {
    this.o = o;
    this.sel = new Selection(0, () => true, o.multi ?? false);

    this.root = document.createElement("div");
    this.root.className = "fl";
    this.root.setAttribute("role", "grid");
    this.root.tabIndex = 0;
    this.root.style.setProperty("--fl-columns", o.columns.map((c) => c.width).join(" "));

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
    this.canvas = document.createElement("div");
    this.canvas.className = "fl-canvas";
    this.body.appendChild(this.canvas);
    this.root.append(header, this.body);

    this.root.addEventListener("keydown", (e) => this.onKey(e));
    this.body.addEventListener("scroll", () => this.schedule());
    new ResizeObserver(() => this.schedule()).observe(this.body);
    // One set of mouse handlers for all rows: rows come and go while scrolling.
    this.canvas.addEventListener("mousedown", (e) => this.onMouseDown(e));
  }

  /** Shows items (in the given order), keeping the selected item if it is still listed. */
  setItems(items: ListItem<T>[]): void {
    const keep = itemKey(this.items[this.sel.current]);
    this.items = items;
    this.message?.remove();
    this.message = null;
    const again = keep === undefined ? -1 : items.findIndex((it) => itemKey(it) === keep);
    this.sel.reset(items.length, () => true, again);
    if (this.sel.current < 0) this.sel.edge(1);
    this.canvas.style.height = `${items.length * ROW_HEIGHT}px`;
    this.dirty = true;
    this.reveal();
    this.render();
    this.notify();
  }

  /** Shows a message instead of rows, e.g. "Comparing…" or "No differences". */
  showMessage(text: string): void {
    this.items = [];
    this.sel.reset(0);
    this.canvas.style.height = "0px";
    this.canvas.replaceChildren();
    this.rendered.clear();
    if (!this.message) {
      this.message = document.createElement("div");
      this.message.className = "fl-message";
      this.message.setAttribute("role", "status");
      this.body.appendChild(this.message);
    }
    this.message.textContent = text;
  }

  /** Renders the rows in view again, e.g. after line counts arrived. Batched per frame. */
  redraw(): void {
    this.dirty = true;
    this.schedule();
  }

  /** The items as shown. */
  all(): readonly ListItem<T>[] {
    return this.items;
  }

  /** The current (keyboard focus) item. */
  current(): ListItem<T> | undefined {
    return this.items[this.sel.current];
  }

  /** The selected file rows (group headers excluded). */
  selectedRows(): T[] {
    const out: T[] = [];
    for (const i of this.sel.rows()) {
      const it = this.items[i];
      if (it?.kind === "row") out.push(it.row);
    }
    return out;
  }

  /** Selects item i and scrolls it into view. */
  select(i: number): boolean {
    if (!this.sel.select(i)) return false;
    this.afterMove();
    return true;
  }

  /** Index of the next (dir 1) or previous (dir -1) file row after the current item for which want() holds, or -1. */
  find(dir: 1 | -1, want: (row: T) => boolean): number {
    const from = this.sel.current < 0 ? (dir === 1 ? -1 : this.items.length) : this.sel.current;
    return this.sel.find(from, dir, (i) => {
      const it = this.items[i];
      return it.kind === "row" && want(it.row);
    });
  }

  /** Opens the current row, or reports why it cannot be opened; on a header, toggles it. */
  openCurrent(): void {
    const it = this.current();
    if (!it) return;
    if (it.kind === "group") {
      this.o.toggleGroup(it.dir, !it.collapsed);
      return;
    }
    const reason = this.o.blocked(it.row);
    if (reason === null) this.o.open(it.row);
    else this.o.refused(it.row, reason);
  }

  focus(): void {
    this.root.focus();
  }

  private schedule(): void {
    if (this.frame) return;
    this.frame = requestAnimationFrame(() => {
      this.frame = 0;
      this.render();
    });
  }

  /**
   * Puts the items in view into the DOM: keeps the row elements already
   * there (refilling them when dirty), adds the missing ones, removes those
   * scrolled out, then paints the selection.
   */
  private render(): void {
    const { start, end } = visibleWindow(this.body.scrollTop, this.body.clientHeight, ROW_HEIGHT, this.items.length);
    for (const [i, el] of this.rendered) {
      if (i < start || i >= end) {
        el.remove();
        this.rendered.delete(i);
      }
    }
    for (let i = start; i < end; i++) {
      let el = this.rendered.get(i);
      if (!el) {
        el = document.createElement("div");
        el.setAttribute("role", "row");
        el.dataset.index = String(i);
        el.style.transform = `translateY(${i * ROW_HEIGHT}px)`;
        this.fill(el, i);
        this.rendered.set(i, el);
        this.canvas.appendChild(el);
      } else if (this.dirty) {
        this.fill(el, i);
      }
    }
    this.dirty = false;
    this.paint();
  }

  /** Marks the selected rows among those in the DOM. */
  private paint(): void {
    for (const [i, el] of this.rendered) {
      const selected = this.sel.has(i);
      el.classList.toggle("fl-selected", selected);
      el.setAttribute("aria-selected", String(selected));
    }
  }

  /** Fills row element el with item i's content. */
  private fill(el: HTMLElement, i: number): void {
    const it = this.items[i];
    el.className = "fl-row";
    el.replaceChildren();
    el.removeAttribute("aria-expanded");
    if (it.kind === "group") {
      el.classList.add("fl-group");
      el.setAttribute("aria-expanded", String(!it.collapsed));
      const cell = document.createElement("div");
      cell.className = "fl-cell fl-group-cell";
      cell.setAttribute("role", "gridcell");
      this.o.renderGroup(it, cell);
      el.appendChild(cell);
      return;
    }
    const nameOnly = it.group !== undefined;
    for (const c of this.o.columns) {
      const cell = document.createElement("div");
      cell.className = `fl-cell fl-col-${c.key}`;
      cell.setAttribute("role", "gridcell");
      c.render(it.row, cell, nameOnly);
      el.appendChild(cell);
    }
    if (nameOnly) el.classList.add("fl-in-group");
    el.classList.toggle("fl-blocked", this.o.blocked(it.row) !== null);
    const extra = this.o.rowClass?.(it.row);
    if (extra) el.classList.add(...extra.split(/\s+/).filter(Boolean));
  }

  private indexAt(e: MouseEvent): number {
    const row = (e.target as HTMLElement).closest<HTMLElement>(".fl-row");
    return row?.dataset.index === undefined ? -1 : Number(row.dataset.index);
  }

  private onMouseDown(e: MouseEvent): void {
    if (e.button !== 0) return;
    const i = this.indexAt(e);
    if (i < 0) return;
    const it = this.items[i];
    if (it.kind === "group") {
      this.sel.select(i);
      this.o.toggleGroup(it.dir, !it.collapsed);
      return;
    }
    const plain = !e.shiftKey && !e.ctrlKey && !e.metaKey;
    const ok = e.shiftKey ? this.sel.extendTo(i) : e.ctrlKey || e.metaKey ? this.sel.toggle(i) : this.sel.select(i);
    if (ok) this.afterMove(false);
    // A double-click opens the row under the mouse, never another one: only
    // when this press selected that very row.
    const double = plain && ok && this.sel.current === i && isDoubleClick(this.lastPress, i, e.timeStamp);
    this.lastPress = double || !ok ? { index: -1, time: 0 } : { index: plain ? i : -1, time: e.timeStamp };
    if (double) this.openCurrent();
  }

  private onKey(e: KeyboardEvent): void {
    if (e.altKey || e.ctrlKey || e.metaKey) return;
    const it = this.current();
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
      case "ArrowLeft":
        // On a header: collapse it. On a row in a group: go to its header.
        if (it?.kind === "group" && !it.collapsed) this.o.toggleGroup(it.dir, true);
        else if (it?.kind === "row" && it.group !== undefined) {
          moved = this.sel.select(this.items.findIndex((x) => x.kind === "group" && x.dir === it.group));
        }
        break;
      case "ArrowRight":
        if (it?.kind === "group" && it.collapsed) this.o.toggleGroup(it.dir, false);
        break;
      case "Enter":
        this.openCurrent();
        break;
      default:
        return;
    }
    e.preventDefault();
    if (moved) this.afterMove();
  }

  /** After the selection moved: scroll it into view (unless clicked), repaint, report it. */
  private afterMove(scroll = true): void {
    if (scroll) this.reveal();
    this.render(); // only adds rows scrolled into view; existing rows stay
    this.notify();
  }

  private reveal(): void {
    if (this.sel.current < 0) return;
    const top = scrollToShow(this.sel.current, this.body.scrollTop, this.body.clientHeight, ROW_HEIGHT);
    if (top !== this.body.scrollTop) this.body.scrollTop = top;
  }

  private notify(): void {
    this.o.selected?.(this.selectedRows());
  }
}
