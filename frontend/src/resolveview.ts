// Conflicts window (mergediff resolve): every conflicted file of the
// repository with its status on each side, like the JetBrains Conflicts
// dialog. Files are resolved with Accept Yours / Accept Theirs (several at
// once) or merged in a window of their own. The logic lives in
// resolve-core.ts; this file builds the DOM.

import type { Operation } from "../bindings/github.com/hadi-susanto/mergediff/internal/gitconflict/models";
import type { MergeCounts } from "../bindings/github.com/hadi-susanto/mergediff/internal/session/models";
import { confirmDialog, showError } from "./dialogs.ts";
import { middleEllipsis, type GroupItem } from "./filelist-core.ts";
import { FileList, groupHeader, renderPath, type Column } from "./filelist.ts";
import { makeFilterBar } from "./filterbar.ts";
import { installShortcuts } from "./keys.ts";
import {
  actionsFor,
  banner,
  canShowChanges,
  confirmText,
  describe,
  groupSummary,
  mergeBlocked,
  pillText,
  RESOLVE_CHIPS,
  resolveItems,
  stateClass,
  stateText,
  statusText,
  type ConflictRow,
  type ResolveFilter,
  type Side,
  unsavedText,
} from "./resolve-core.ts";

/** The rows and operation the backend reports (session.ConflictList). */
export interface ConflictListData {
  operation: Operation;
  rows: ConflictRow[] | null;
  unresolved: number;
  /** Paths open in merge windows. */
  open: string[] | null;
}

/** What the conflicts view needs from the backend and the window. */
export interface ResolveBackend {
  conflicts(): Promise<ConflictListData>;
  accept(paths: string[], side: Side): Promise<ConflictListData>;
  counts(path: string): Promise<MergeCounts>;
  openMerge(path: string): Promise<void>;
  showChanges(path: string): Promise<void>;
  /** Files open in merge windows with unsaved edits. */
  unsavedMerges(): Promise<string[]>;
  copy(text: string): Promise<void>;
  /** Calls handler when a merge window opens or closes. */
  onChanged(handler: () => void): void;
  /** Routes the window's close button to handler. */
  guardClose(handler: () => void): void;
  quit(code: number): void;
}

export interface ResolveViewOptions {
  /** The repository's top-level directory. */
  repo: string;
  version: string;
  backend: ResolveBackend;
}

const EXIT_OK = 0;
const EXIT_ERROR = 2;
const HINT = "Double-click or Enter to merge · Ctrl+Shift+Y yours · Ctrl+Shift+T theirs";

function el<K extends keyof HTMLElementTagNameMap>(tag: K, cls = "", text = ""): HTMLElementTagNameMap[K] {
  const e = document.createElement(tag);
  if (cls) e.className = cls;
  if (text) e.textContent = text;
  return e;
}

function tag(cls: string, text: string): HTMLSpanElement {
  return el("span", `tag ${cls}`, text);
}

function columns(op: Operation): Column<ConflictRow>[] {
  return [
    {
      key: "file",
      title: "File",
      width: "minmax(0, 1fr)",
      render: (row, cell, nameOnly) => renderPath(cell, row.path, nameOnly),
    },
    {
      key: "yours",
      title: "Yours",
      subtitle: op.yoursLabel,
      width: "150px",
      render: (row, cell) => cell.replaceChildren(tag(`st-${row.yours}`, statusText(row.yours))),
    },
    {
      key: "theirs",
      title: "Theirs",
      subtitle: op.theirsLabel,
      width: "150px",
      render: (row, cell) => cell.replaceChildren(tag(`st-${row.theirs}`, statusText(row.theirs))),
    },
    {
      key: "state",
      title: "State",
      width: "150px",
      render: (row, cell) => cell.replaceChildren(tag(stateClass(row), stateText(row))),
    },
  ];
}

function renderGroup(g: GroupItem, cell: HTMLElement): void {
  const s = groupSummary(g.counts, g.size);
  cell.replaceChildren(...groupHeader(g), el("span", s.done ? "fl-group-summary fl-group-done" : "fl-group-summary", s.text));
}

/** The operation bar: the sentence (names in bold) and the pill. */
function renderOperation(host: HTMLElement, pill: HTMLElement, op: Operation, unresolved: number, total: number): void {
  const sentence = el("div", "op-sentence");
  for (const p of op.sentence ?? []) {
    sentence.appendChild(p.strong ? el("strong", "", p.text) : document.createTextNode(p.text));
  }
  host.replaceChildren(sentence);
  const t = pillText(unresolved, total);
  pill.textContent = t.text;
  pill.classList.toggle("conflict-pill-done", t.done);
}

/** Builds the conflicts window into the page (index.html's toolbar, panes and status bar). */
export function renderResolveView(o: ResolveViewOptions): void {
  const statusLeft = document.getElementById("status-left")!;
  const statusRight = document.getElementById("status-right")!;
  const setHint = (text = HINT) => (statusLeft.textContent = text);
  setHint();
  statusRight.textContent = `${o.repo} · mergediff ${o.version}`;
  statusRight.title = o.repo;

  const opHost = document.getElementById("toolbar-left")!;
  opHost.parentElement?.classList.add("toolbar-operation");
  const pill = el("span", "conflict-pill");
  pill.setAttribute("role", "status");
  document.getElementById("toolbar-right")!.appendChild(pill);

  const filter: ResolveFilter = { text: "", chip: "all", grouped: false, collapsed: new Set() };
  let rows: ConflictRow[] = [];
  /** Paths open in a merge window: they cannot be accepted. */
  let locked = new Set<string>();
  /** Conflict counts by path, for the description; cleared on every refresh. */
  let counts = new Map<string, MergeCounts>();
  let list: FileList<ConflictRow> | null = null;
  let busy = false;

  // Side panel.
  const side = el("aside", "resolve-side");
  const mergeBtn = el("button", "btn btn-primary resolve-btn", "Merge…");
  const yoursBtn = el("button", "btn resolve-btn", "Accept Yours");
  const theirsBtn = el("button", "btn resolve-btn", "Accept Theirs");
  const selLabel = el("div", "resolve-selected-label", "Selected:");
  const selPath = el("div", "resolve-selected-path mono");
  const desc = el("div", "resolve-desc");
  desc.setAttribute("role", "status");
  const changesBtn = el("button", "link-btn", "Show changes");
  changesBtn.title = "Compare base with the side that kept the file";
  const closeBtn = el("button", "btn resolve-btn", "Close");
  side.append(mergeBtn, yoursBtn, theirsBtn, selLabel, selPath, desc, changesBtn, el("div", "resolve-spacer"), closeBtn);

  // All-resolved banner: the command that finishes the operation.
  const bannerEl = el("div", "resolve-banner");
  bannerEl.setAttribute("role", "status");
  const bannerText = el("span", "resolve-banner-text");
  const bannerCmd = el("code", "resolve-banner-cmd mono");
  const copyBtn = el("button", "btn", "Copy");
  copyBtn.title = "Copy the command";
  bannerEl.append(bannerText, bannerCmd, copyBtn);
  bannerEl.hidden = true;

  const selected = (): ConflictRow[] => list?.selectedRows() ?? [];

  const updatePanel = () => {
    const sel = selected();
    const a = actionsFor(sel, locked);
    for (const [btn, st] of [
      [mergeBtn, a.merge],
      [yoursBtn, a.yours],
      [theirsBtn, a.theirs],
    ] as const) {
      btn.textContent = st.label;
      btn.disabled = busy || !st.enabled;
      btn.title = st.title;
    }
    selLabel.hidden = sel.length !== 1;
    selPath.textContent = sel.length === 1 ? sel[0].path : "";
    const one = sel.length === 1 ? sel[0] : undefined;
    desc.textContent = describe(sel, one ? counts.get(one.path) : undefined, locked);
    changesBtn.hidden = !canShowChanges(sel);
    if (one && !one.resolved && one.state === "conflict" && !counts.has(one.path)) {
      const path = one.path;
      o.backend
        .counts(path)
        .then((n) => {
          counts.set(path, n);
          const now = selected();
          if (now.length === 1 && now[0].path === path) desc.textContent = describe(now, n, locked);
        })
        .catch((err) => console.error("counting conflicts failed", err));
    }
  };

  const show = () => {
    if (!list) return;
    const view = resolveItems(rows, filter);
    bar.update(view.chipCounts, filter.chip);
    if (view.items.length > 0) list.setItems(view.items);
    else list.showMessage(rows.length === 0 ? "No conflicts" : "No files match the filter");
    updatePanel();
  };

  const apply = (data: ConflictListData) => {
    rows = data.rows ?? [];
    locked = new Set(data.open ?? []);
    counts = new Map();
    renderOperation(opHost, pill, data.operation, data.unresolved, rows.length);
    const b = banner(data.unresolved, rows.length, data.operation.next);
    bannerEl.hidden = b === null;
    if (b) {
      bannerText.textContent = b.text;
      bannerCmd.textContent = b.command;
    }
    if (!list) list = makeList(data.operation);
    show();
  };

  const makeList = (op: Operation): FileList<ConflictRow> => {
    const l = new FileList<ConflictRow>({
      columns: columns(op),
      renderGroup,
      toggleGroup(dir, collapse) {
        const c = new Set(filter.collapsed);
        if (collapse) c.add(dir);
        else c.delete(dir);
        filter.collapsed = c;
        show();
      },
      blocked: mergeBlocked,
      open: (row) => {
        setHint();
        o.backend.openMerge(row.path).catch((err) => {
          showError(`Could not open ${row.path}: ${err}`);
          refresh();
        });
      },
      refused: (row, reason) => setHint(`${row.path} cannot be merged: ${reason}`),
      selected: () => {
        setHint();
        updatePanel();
      },
      multi: true,
      rowClass: (row) => (row.resolved ? "fl-faded" : locked.has(row.path) ? "fl-locked" : ""),
    });
    panes.insertBefore(l.root, side);
    return l;
  };

  const refresh = () => {
    o.backend
      .conflicts()
      .then(apply)
      .catch((err) => showError(`Could not read the conflicts: ${err}`));
  };

  const accept = async (s: Side) => {
    const a = actionsFor(selected(), locked);
    const st = s === "yours" ? a.yours : a.theirs;
    if (busy || !st.enabled) return;
    const paths = a.targets.map((r) => r.path);
    if (paths.length > 1 && !(await confirmDialog(confirmText(paths.length, s), st.label, "Cancel"))) {
      list?.focus();
      return;
    }
    busy = true;
    updatePanel();
    try {
      apply(await o.backend.accept(paths, s));
    } catch (err) {
      showError(`Could not resolve ${paths.length === 1 ? paths[0] : `${paths.length} files`}: ${err}`);
      refresh();
    } finally {
      busy = false;
      updatePanel();
      list?.focus();
    }
  };

  yoursBtn.addEventListener("click", () => void accept("yours"));
  theirsBtn.addEventListener("click", () => void accept("theirs"));
  mergeBtn.addEventListener("click", () => list?.openCurrent());
  changesBtn.addEventListener("click", () => {
    const sel = selected();
    if (!canShowChanges(sel)) return;
    o.backend.showChanges(sel[0].path).catch((err) => showError(`Could not show the changes of ${sel[0].path}: ${err}`));
  });
  copyBtn.addEventListener("click", () => {
    o.backend
      .copy(bannerCmd.textContent ?? "")
      .then(() => setHint(`Copied: ${bannerCmd.textContent}`))
      .catch((err) => showError(`Could not copy: ${err}`));
  });

  /** Closes the window, asking first when merge windows have unsaved edits. */
  let closing = false;
  const close = async () => {
    if (closing || document.querySelector("dialog[open]")) return;
    closing = true;
    try {
      const unsaved = await o.backend.unsavedMerges().catch(() => [] as string[]);
      if (unsaved.length > 0 && !(await confirmDialog(unsavedText(unsaved), "Close", "Keep open"))) {
        list?.focus();
        return;
      }
      o.backend.quit(EXIT_OK);
    } finally {
      closing = false;
    }
  };
  closeBtn.addEventListener("click", () => void close());

  const bar = makeFilterBar(filter, RESOLVE_CHIPS, show);
  bar.input.addEventListener("keydown", (e) => {
    if (e.key === "ArrowDown" || e.key === "Enter") {
      e.preventDefault();
      list?.focus();
    }
  });

  const panes = document.getElementById("panes")!;
  panes.classList.add("panes-resolve");
  panes.append(bar.root, bannerEl, side);

  installShortcuts({
    acceptYours: () => void accept("yours"),
    acceptTheirs: () => void accept("theirs"),
    refresh,
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
        list?.focus();
      } else {
        void close();
      }
    },
  });
  o.backend.onChanged(refresh);
  o.backend.guardClose(() => void close());

  o.backend
    .conflicts()
    .then((data) => {
      apply(data);
      list?.focus();
    })
    .catch((err) => showError(`Could not read the conflicts: ${err}`, () => o.backend.quit(EXIT_ERROR)));

  // The repository path in the status bar, shortened in the middle when long.
  const fitRepo = () => {
    const room = Math.max(20, Math.floor(statusRight.parentElement!.clientWidth / 2 / 7) - 20);
    statusRight.textContent = `${middleEllipsis(o.repo, room)} · mergediff ${o.version}`;
  };
  fitRepo();
  window.addEventListener("resize", fitRepo);
}
