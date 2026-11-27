import { Compartment, type Extension } from "@codemirror/state";
import { EditorView, type ViewUpdate } from "@codemirror/view";
import { mergeSegments } from "./alignment.ts";
import {
  applyNonConflicting,
  applySide,
  chunkField,
  chunkTracking,
  conflictsLeft,
  ignoreSide,
  initialResult,
  type Tracked,
} from "./chunks.ts";
import { confirmDialog, showError } from "./dialogs.ts";
import { ButtonStrip, type ChunkAction } from "./gutter.ts";
import {
  changedOn,
  charMarks,
  charMarkSet,
  lineOffset,
  resultHighlight,
  setCharMarks,
  sideClass,
  sideDecorations,
  type CharMarkInput,
} from "./highlight.ts";
import { installShortcuts } from "./keys.ts";
import { sliceLines, splitLines, type CharSpan, type MergeChunk, type Side, type Source } from "./model.ts";
import { step, targets, type Direction } from "./nav.ts";
import { createEditablePane, createReadOnlyPane, type Pane } from "./panes.ts";
import { ScrollSync } from "./scrollsync.ts";
import { createMergeToolbar, type MergeToolbarActions } from "./toolbar.ts";

/** What the merge view needs from the backend. */
export interface MergeBackend {
  save(result: string): Promise<void>;
  quit(code: number): void;
  charDiff(left: string, right: string): Promise<CharSpan[]>;
  /**
   * Routes the window's close button to handler instead of closing (the
   * backend's close guard). Called once the view is ready.
   */
  guardClose(handler: () => void): void;
  /** Tells the backend whether the result has unsaved edits (optional). */
  setDirty?(dirty: boolean): void;
}

export interface MergeInput {
  paths: string[];
  base: string;
  local: string;
  remote: string;
  chunks: MergeChunk[];
  initialResult: string;
  autoApplied: boolean;
  /** Branch names for the side pane headers; empty: the file names. */
  localLabel?: string;
  remoteLabel?: string;
  /** The output file was edited since Git stopped: saving asks first. */
  editedOnDisk?: boolean;
}

/** Process exit codes, mirroring internal/command/command.go. */
const EXIT_OK = 0;
const EXIT_CANCEL = 1;

function baseName(path: string): string {
  return path.split(/[\\/]/).pop() || path;
}

function plural(n: number, word: string): string {
  return `${n} ${word}${n === 1 ? "" : "s"}`;
}

/**
 * Highlights, in a side pane, the characters that side changed relative to
 * base, for chunks with lines on both. Runs once; marks arrive async.
 */
async function paintCharMarks(
  pane: Pane,
  side: Side,
  tracked: readonly Tracked[],
  base: string,
  text: string,
  charDiff: MergeBackend["charDiff"],
): Promise<void> {
  const baseLines = splitLines(base);
  const sideLines = splitLines(text);
  const doc = pane.view.state.doc;
  const jobs = tracked
    .filter((t) => changedOn(t.chunk, side) && sideClass(t.chunk, side) !== "added" && sideClass(t.chunk, side) !== "deleted")
    .map(async (t): Promise<CharMarkInput> => {
      const range = t.chunk[side];
      const spans = await charDiff(sliceLines(baseLines, t.chunk.base), sliceLines(sideLines, range));
      return { offset: lineOffset(doc, range.start), spans, kind: "insert", cls: `md-char-${sideClass(t.chunk, side)}` };
    });
  const inputs = await Promise.all(jobs);
  pane.view.dispatch({ effects: setCharMarks.of(charMarkSet(inputs)) });
}

/** What a rendered view hands back to main. */
export interface Rendered {
  /** Static status-bar text. */
  status: string;
  /** The editors, e.g. for applying the language. */
  views: EditorView[];
}

/** Builds the three-pane merge view. */
export function renderMerge(input: MergeInput, backend: MergeBackend): Rendered {
  const [, localPath = "", remotePath = "", mergedPath = ""] = input.paths;
  const init = initialResult(input.chunks, input.base, input.local, input.remote, input.autoApplied);
  if (init.doc !== input.initialResult) {
    // Both sides implement the same layout; a mismatch is a bug.
    console.error("initial result differs from the backend's", { frontend: init.doc, backend: input.initialResult });
  }

  // Side panes: their line highlights depend on chunk states (resolved
  // chunks fade), so they sit in a compartment repainted when states change.
  const sideDeco = { local: new Compartment(), remote: new Compartment() };
  const localPane = createReadOnlyPane(`Local (yours) · ${input.localLabel || baseName(localPath)}`, localPath, input.local, [
    sideDeco.local.of([]),
    charMarks,
  ]);
  const remotePane = createReadOnlyPane(`Remote (theirs) · ${input.remoteLabel || baseName(remotePath)}`, remotePath, input.remote, [
    sideDeco.remote.of([]),
    charMarks,
  ]);
  const paintSides = (tracked: readonly Tracked[]) => {
    for (const [side, pane] of [
      ["local", localPane],
      ["remote", remotePane],
    ] as const) {
      const deco: Extension = EditorView.decorations.of(sideDecorations(pane.view.state.doc, tracked, side));
      pane.view.dispatch({ effects: sideDeco[side].reconfigure(deco) });
    }
  };
  paintSides(init.tracked);

  const onAction = (index: number, side: Side, action: ChunkAction) => {
    const state = result.view.state;
    const spec = action === "apply" ? applySide(state, index, side) : ignoreSide(state, index, side);
    if (spec) result.view.dispatch(spec);
    result.view.focus();
  };
  // Created once the Result editor exists (they draw connectors to it).
  let localStrip: ButtonStrip | null = null;
  let remoteStrip: ButtonStrip | null = null;

  /** Moves the cursor to the previous / next change or conflict. */
  const goTo = (kind: "change" | "conflict", dir: Direction) => {
    const state = result.view.state;
    const tracked = state.field(chunkField);
    const starts = targets(tracked, kind).map((i) => tracked[i].from);
    const k = step(starts, state.selection.main.head, dir);
    if (k >= 0) {
      result.view.dispatch({
        selection: { anchor: starts[k] },
        effects: EditorView.scrollIntoView(starts[k], { y: "center" }),
      });
    }
    result.view.focus();
  };

  const actions: MergeToolbarActions = {
    apply(source: Source) {
      const spec = applyNonConflicting(result.view.state, source);
      if (spec) result.view.dispatch(spec);
      result.view.focus();
    },
    change: (dir) => goTo("change", dir),
    conflict: (dir) => goTo("conflict", dir),
    async save() {
      if (input.editedOnDisk) {
        const ok = await confirmDialog(
          "This file was edited since Git stopped.\n\nSave the result and replace those edits?",
          "Save",
          "Keep editing",
        );
        if (!ok) return;
      }
      const left = conflictsLeft(result.view.state.field(chunkField));
      if (left > 0) {
        const ok = await confirmDialog(
          `${plural(left, "conflict")} ${left === 1 ? "is" : "are"} still unresolved.\n\nSave anyway? Git will treat the merge as not resolved.`,
          "Save anyway",
          "Keep editing",
        );
        if (!ok) return;
      }
      try {
        await backend.save(result.view.state.doc.toString());
      } catch (err) {
        showError(`Could not save the result:\n${err}`);
        return;
      }
      backend.quit(left === 0 ? EXIT_OK : EXIT_CANCEL);
    },
    async cancel() {
      if (result.view.state.doc.toString() !== init.doc) {
        const ok = await confirmDialog("Discard your changes to the result?", "Discard", "Keep editing");
        if (!ok) return;
      }
      backend.quit(EXIT_CANCEL);
    },
  };
  const toolbar = createMergeToolbar(document.getElementById("toolbar-left")!, document.getElementById("toolbar-right")!, actions);

  const refresh = (tracked: readonly Tracked[]) => {
    toolbar.update(tracked);
    localStrip?.update(tracked);
    remoteStrip?.update(tracked);
  };

  let sync: ScrollSync | null = null;
  let dirty = false;
  const onResultUpdate = (u: ViewUpdate) => {
    if (u.docChanged && backend.setDirty) {
      const now = u.state.doc.toString() !== init.doc;
      if (now !== dirty) {
        dirty = now;
        backend.setDirty(now);
      }
    }
    const before = u.startState.field(chunkField);
    const after = u.state.field(chunkField);
    if (u.docChanged || u.geometryChanged || u.viewportChanged) {
      localStrip?.schedule();
      remoteStrip?.schedule();
    }
    if (u.docChanged || before !== after) {
      sync?.invalidate();
      // Keep the side panes aligned with the result while it changes.
      if (u.docChanged) requestAnimationFrame(() => sync?.alignTo(1));
    }
    if (before === after) return;
    // Fade resolved chunks in the side panes too.
    if (after.some((t, i) => t.local !== before[i].local || t.remote !== before[i].remote)) paintSides(after);
    refresh(after);
  };

  const result = createEditablePane("Result · editable", mergedPath, init.doc, [
    chunkTracking(init.tracked),
    resultHighlight,
    EditorView.updateListener.of(onResultUpdate),
  ]);

  localStrip = new ButtonStrip("local", localPane.view, result.view, onAction);
  remoteStrip = new ButtonStrip("remote", remotePane.view, result.view, onAction);

  const panes = document.getElementById("panes")!;
  panes.classList.add("panes-3");
  panes.append(localPane.root, localStrip.root, result.root, remoteStrip.root, remotePane.root);
  refresh(init.tracked);

  sync = new ScrollSync([localPane.view, result.view, remotePane.view], () =>
    mergeSegments(input.chunks, result.view.state.field(chunkField), result.view.state.doc),
  );
  window.addEventListener("resize", () => {
    sync?.alignTo(1);
    localStrip?.schedule();
    remoteStrip?.schedule();
  });

  installShortcuts({
    save: () => void actions.save(),
    next: () => actions.change(1),
    prev: () => actions.change(-1),
    applyAll: () => actions.apply("both"),
    cancel: () => {
      if (!toolbar.closeMenu()) void actions.cancel();
    },
  });

  // The window's close button behaves like Cancel: it asks before
  // discarding edits. Ignored while a dialog is already open.
  backend.guardClose(() => {
    if (!document.querySelector("dialog[open]")) void actions.cancel();
  });

  const logCharDiffError = (err: unknown) => console.error("character diff failed", err);
  paintCharMarks(localPane, "local", init.tracked, input.base, input.local, backend.charDiff).catch(logCharDiffError);
  paintCharMarks(remotePane, "remote", init.tracked, input.base, input.remote, backend.charDiff).catch(logCharDiffError);
  result.view.focus();

  let local = 0;
  let remote = 0;
  let both = 0;
  let conflicts = 0;
  for (const c of input.chunks) {
    if (c.kind === "local") local++;
    else if (c.kind === "remote") remote++;
    else if (c.kind === "both") both++;
    else if (c.kind === "conflict") conflicts++;
  }
  const nonConflicting = local + remote + both;
  const changes = nonConflicting + conflicts;
  const status =
    changes === 0
      ? "No changes"
      : `${plural(changes, "change")} · ${nonConflicting} non-conflicting · ${plural(conflicts, "conflict")}`;
  return { status, views: [localPane.view, result.view, remotePane.view] };
}
