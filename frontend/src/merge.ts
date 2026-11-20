import { Compartment, type Extension } from "@codemirror/state";
import { EditorView, type ViewUpdate } from "@codemirror/view";
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
import { resultHighlight, sideDecorations } from "./highlight.ts";
import type { MergeChunk, Side, Source } from "./model.ts";
import { createEditablePane, createReadOnlyPane } from "./panes.ts";
import { createMergeToolbar } from "./toolbar.ts";

/** What the merge view needs from the backend. */
export interface MergeBackend {
  save(result: string): Promise<void>;
  quit(code: number): void;
}

export interface MergeInput {
  paths: string[];
  base: string;
  local: string;
  remote: string;
  chunks: MergeChunk[];
  initialResult: string;
  autoApplied: boolean;
}

/** Process exit codes, mirroring cli.go. */
const EXIT_OK = 0;
const EXIT_CANCEL = 1;

function baseName(path: string): string {
  return path.split(/[\\/]/).pop() || path;
}

function plural(n: number, word: string): string {
  return `${n} ${word}${n === 1 ? "" : "s"}`;
}

/** Builds the three-pane merge view. Returns the static status-bar text. */
export function renderMerge(input: MergeInput, backend: MergeBackend): string {
  const [, localPath = "", remotePath = "", mergedPath = ""] = input.paths;
  const init = initialResult(input.chunks, input.base, input.local, input.remote, input.autoApplied);
  if (init.doc !== input.initialResult) {
    // Both sides implement the same layout; a mismatch is a bug.
    console.error("initial result differs from the backend's", { frontend: init.doc, backend: input.initialResult });
  }

  // Side panes: their highlights depend on chunk states (resolved chunks
  // fade), so they sit in a compartment repainted when the states change.
  const sideDeco = { local: new Compartment(), remote: new Compartment() };
  const localPane = createReadOnlyPane(`Local (yours) · ${baseName(localPath)}`, localPath, input.local, [
    sideDeco.local.of([]),
  ]);
  const remotePane = createReadOnlyPane(`Remote (theirs) · ${baseName(remotePath)}`, remotePath, input.remote, [
    sideDeco.remote.of([]),
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
  const localStrip = new ButtonStrip("local", localPane.view, onAction);
  const remoteStrip = new ButtonStrip("remote", remotePane.view, onAction);

  const toolbar = createMergeToolbar(document.getElementById("toolbar-left")!, document.getElementById("toolbar-right")!, {
    apply(source: Source) {
      const spec = applyNonConflicting(result.view.state, source);
      if (spec) result.view.dispatch(spec);
      result.view.focus();
    },
    async save() {
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
  });

  const refresh = (tracked: readonly Tracked[]) => {
    toolbar.update(tracked);
    localStrip.update(tracked);
    remoteStrip.update(tracked);
  };

  const onResultUpdate = (u: ViewUpdate) => {
    const before = u.startState.field(chunkField);
    const after = u.state.field(chunkField);
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

  const panes = document.getElementById("panes")!;
  panes.classList.add("panes-3");
  panes.append(localPane.root, localStrip.root, result.root, remoteStrip.root, remotePane.root);
  refresh(init.tracked);
  window.addEventListener("resize", () => {
    localStrip.schedule();
    remoteStrip.schedule();
  });
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
  return changes === 0
    ? "No changes"
    : `${plural(changes, "change")} · ${nonConflicting} non-conflicting · ${plural(conflicts, "conflict")}`;
}
