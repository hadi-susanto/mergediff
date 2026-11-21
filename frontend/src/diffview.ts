// Two-pane diff: files compared read-only (mergediff LEFT RIGHT), or the
// editable scratch view (mergediff without arguments) where the user pastes
// text into both panes and presses Refresh.

import type { Text } from "@codemirror/state";
import { EditorView, placeholder } from "@codemirror/view";
import { diffSegments, lineIndexCount } from "./alignment.ts";
import { showError } from "./dialogs.ts";
import {
  charMarks,
  charMarkSet,
  clearCharMarksOnEdit,
  diffClass,
  diffLineMarks,
  diffSummary,
  diffMarks,
  lineOffset,
  setCharMarks,
  setDiffMarks,
  type CharMarkInput,
  type DiffSide,
} from "./highlight.ts";
import { installShortcuts } from "./keys.ts";
import type { Rendered } from "./merge.ts";
import { sliceLines, splitLines, type CharSpan, type MergeChunk } from "./model.ts";
import { step, type Direction } from "./nav.ts";
import { createEditablePane, createReadOnlyPane, type Pane } from "./panes.ts";
import { ScrollSync } from "./scrollsync.ts";
import { changeNavButtons } from "./toolbar.ts";

export interface DiffPaneSpec {
  title: string;
  tooltip: string;
  text: string;
}

export interface DiffViewOptions {
  left: DiffPaneSpec;
  right: DiffPaneSpec;
  /** Chunks of the initial texts (empty for scratch). */
  chunks: MergeChunk[];
  /** Scratch mode: both panes editable, with a Refresh action that calls this. */
  compare?: (left: string, right: string) => Promise<MergeChunk[]>;
  charDiff(left: string, right: string): Promise<CharSpan[]>;
  close(): void;
  /** Replaces the left part of the status bar. */
  status(text: string): void;
}

const SIDES: readonly DiffSide[] = ["left", "right"];
const SCRATCH_HINT = "Paste or type text into both panes, then Refresh (F5)";
const STALE_HINT = "Edited — Refresh (F5) to compare again";

/** Builds the two panes, toolbar and shortcuts of a diff or scratch view. */
export function renderDiffView(o: DiffViewOptions): Rendered {
  const scratch = o.compare !== undefined;
  let chunks: MergeChunk[] = [];
  let compared = false; // a comparison is shown (initial diff, or after Refresh)
  let starts: number[] = []; // right-pane first lines of the changes, for navigation
  let generation = 0; // bumps on every comparison or edit; stale async work checks it

  const panesEl = document.getElementById("panes")!;
  panesEl.classList.add("panes-2");

  const onEdit = EditorView.updateListener.of((u) => {
    if (!u.docChanged || !compared) return;
    compared = false;
    generation++;
    chunks = [];
    setStarts([]);
    sync.invalidate();
    o.status(STALE_HINT);
  });
  const [left, right] = SIDES.map((side) => {
    const spec = side === "left" ? o.left : o.right;
    const shared = [diffMarks, charMarks];
    const pane = scratch
      ? createEditablePane(spec.title, spec.tooltip, spec.text, [
          ...shared,
          clearCharMarksOnEdit,
          placeholder(`Paste or type the ${side} text here`),
          onEdit,
        ])
      : createReadOnlyPane(spec.title, spec.tooltip, spec.text, shared);
    panesEl.appendChild(pane.root);
    return pane;
  }) as [Pane, Pane];
  const views = [left.view, right.view];

  // Without a current comparison the panes scroll proportionally as a whole.
  const sync = new ScrollSync(views, () =>
    compared && chunks.length > 0
      ? diffSegments(chunks)
      : [[{ start: 0, end: lineIndexCount(left.view.state.doc) }, { start: 0, end: lineIndexCount(right.view.state.doc) }]],
  );
  window.addEventListener("resize", () => sync.alignTo(1));

  // Navigation steps from the line at the middle of the right pane, or from
  // the last change visited if the user has not scrolled since.
  let visited = -1;
  let visitedScroll = -1;
  const go = (dir: Direction) => {
    const view = right.view;
    const el = view.scrollDOM;
    let k: number;
    if (visited >= 0 && Math.abs(el.scrollTop - visitedScroll) < 1) {
      k = visited + dir;
      if (k < 0 || k >= starts.length) return;
    } else {
      const middle = el.getBoundingClientRect().top + el.clientHeight / 2 - view.documentTop;
      const line = view.state.doc.lineAt(view.lineBlockAtHeight(middle).from).number - 1;
      k = step(starts, line, dir);
      if (k < 0) return;
    }
    const pos = lineOffset(view.state.doc, starts[k]);
    view.dispatch({ effects: EditorView.scrollIntoView(pos, { y: "center" }) });
    visited = k;
    requestAnimationFrame(() => (visitedScroll = el.scrollTop));
  };

  const toolbarLeft = document.getElementById("toolbar-left")!;
  const nav = document.createElement("div");
  nav.className = "btn-group";
  nav.append(...changeNavButtons(go));
  const setStarts = (next: number[]) => {
    starts = next;
    visited = -1;
    for (const b of nav.querySelectorAll("button")) b.disabled = starts.length === 0;
  };

  const refresh = async () => {
    if (!o.compare) return;
    const docs = views.map((v) => v.state.doc);
    let next: MergeChunk[];
    try {
      next = await o.compare(docs[0].toString(), docs[1].toString());
    } catch (err) {
      showError(`Could not compare the texts: ${err}`);
      return;
    }
    // Typed into while comparing: these chunks describe older text.
    if (views.some((v, i) => v.state.doc !== docs[i])) return;
    show(next);
  };
  if (scratch) {
    const button = document.createElement("button");
    button.className = "btn";
    button.textContent = "⟳ Refresh";
    button.title = "Compare the two texts (F5 or Ctrl+Enter)";
    button.addEventListener("click", () => void refresh());
    toolbarLeft.appendChild(button);
  }
  toolbarLeft.appendChild(nav);

  const close = document.createElement("button");
  close.className = "btn btn-primary";
  close.textContent = "Close";
  // Esc would throw pasted text away, so it only closes a file diff.
  close.title = scratch ? "Close" : "Close (Esc)";
  close.addEventListener("click", o.close);
  document.getElementById("toolbar-right")!.appendChild(close);
  installShortcuts({
    next: () => go(1),
    prev: () => go(-1),
    cancel: scratch ? undefined : o.close,
    refresh: scratch ? () => void refresh() : undefined,
  });

  function show(next: MergeChunk[]): string {
    chunks = next;
    compared = true;
    const gen = ++generation;
    SIDES.forEach((side, i) => {
      views[i].dispatch({ effects: setDiffMarks.of(diffLineMarks(views[i].state.doc, chunks, side)) });
    });
    setStarts(chunks.filter((c) => c.kind !== "unchanged").map((c) => c.remote.start));
    sync.invalidate();
    sync.alignTo(1);
    paintCharMarks(chunks, views[0].state.doc, views[1].state.doc, o.charDiff)
      .then(([l, r]) => {
        if (gen !== generation) return; // edited or compared again meanwhile
        views[0].dispatch({ effects: setCharMarks.of(l) });
        views[1].dispatch({ effects: setCharMarks.of(r) });
      })
      .catch((err) => console.error("character diff failed", err));
    const summary = diffSummary(chunks);
    o.status(summary);
    return summary;
  }

  setStarts([]);
  const status = scratch ? SCRATCH_HINT : show(o.chunks);
  if (scratch) left.view.focus();
  return { status, views };
}

/** Character marks for a diff: deleted spans on the left, inserted on the right. */
async function paintCharMarks(
  chunks: readonly MergeChunk[],
  leftDoc: Text,
  rightDoc: Text,
  charDiff: DiffViewOptions["charDiff"],
) {
  const leftLines = splitLines(leftDoc.toString());
  const rightLines = splitLines(rightDoc.toString());
  const modified = chunks.filter((c) => c.kind !== "unchanged" && diffClass(c) === "modified");
  const results = await Promise.all(
    modified.map((c) => charDiff(sliceLines(leftLines, c.local), sliceLines(rightLines, c.remote))),
  );
  const inputs = (doc: Text, kind: CharSpan["kind"]): CharMarkInput[] =>
    modified.map((c, i) => ({
      offset: lineOffset(doc, (kind === "delete" ? c.local : c.remote).start),
      spans: results[i],
      kind,
      cls: "md-char-modified",
    }));
  return [charMarkSet(inputs(leftDoc, "delete")), charMarkSet(inputs(rightDoc, "insert"))] as const;
}
