import { defaultKeymap, history, historyKeymap } from "@codemirror/commands";
import { Compartment, EditorState, Prec, type Extension } from "@codemirror/state";
import { EditorView, keymap, lineNumbers, type Command, type KeyBinding } from "@codemirror/view";
import { loadLanguage, syntaxColours } from "./languages.ts";

/** A titled editor pane. */
export interface Pane {
  root: HTMLElement;
  view: EditorView;
}

/** Holds each editor's language; filled in once the parser has loaded. */
const languageSlot = new Compartment();

/** Extensions every pane shares. */
function common(): Extension[] {
  return [lineNumbers(), lfOnly, syntaxColours, languageSlot.of([])];
}

/**
 * Loads the language for id and applies it to every view. Unknown ids and
 * load failures leave the panes as plain text.
 */
export async function setLanguage(views: readonly EditorView[], id: string): Promise<void> {
  const ext = await loadLanguage(id);
  if (!ext) return;
  for (const view of views) view.dispatch({ effects: languageSlot.reconfigure(ext) });
}

// Only "\n" separates lines. CodeMirror would otherwise also split on a lone
// "\r", and saving would turn it into "\n".
const lfOnly = EditorState.lineSeparator.of("\n");

function paneShell(title: string, tooltip: string): { root: HTMLElement; host: HTMLElement } {
  const root = document.createElement("section");
  root.className = "pane";

  const header = document.createElement("div");
  header.className = "pane-header";
  header.textContent = title;
  header.title = tooltip;
  root.appendChild(header);

  const host = document.createElement("div");
  host.className = "pane-editor";
  root.appendChild(host);
  return { root, host };
}

/**
 * Creates a read-only pane with a header and line numbers. The text is shown
 * as-is; selection and copying still work.
 */
export function createReadOnlyPane(title: string, tooltip: string, text: string, extensions: Extension[]): Pane {
  const { root, host } = paneShell(title, tooltip);
  const view = new EditorView({
    parent: host,
    state: EditorState.create({
      doc: text,
      extensions: [...common(), EditorState.readOnly.of(true), EditorView.editable.of(false), ...extensions],
    }),
  });
  return { root, view };
}

/**
 * Undo/redo bindings that run exactly once per keystroke.
 *
 * WebKitGTK delivers Ctrl+Z twice: as a keydown, which CodeMirror's keymap
 * handles, and as a native "historyUndo" beforeinput event, which history()
 * also handles (it needs to, for menu-driven undo). So one keystroke undid
 * two steps. When a history key binding has just run, the matching
 * beforeinput event is dropped; a beforeinput on its own still works.
 */
function singleUndo(): Extension {
  let lastKeyRun = 0;
  const mark =
    (run: Command): Command =>
    (view) => {
      lastKeyRun = Date.now();
      return run(view);
    };
  const bindings: KeyBinding[] = historyKeymap.map((b) => (b.run ? { ...b, run: mark(b.run) } : b));
  const dropDuplicate = EditorView.domEventHandlers({
    beforeinput(e) {
      if ((e.inputType === "historyUndo" || e.inputType === "historyRedo") && Date.now() - lastKeyRun < 500) {
        e.preventDefault();
        return true;
      }
      return false;
    },
  });
  return [keymap.of(bindings), Prec.highest(dropDuplicate)];
}

/** Creates the editable Result pane with undo history. */
export function createEditablePane(title: string, tooltip: string, text: string, extensions: Extension[]): Pane {
  const { root, host } = paneShell(title, tooltip);
  root.classList.add("pane-editable");
  const view = new EditorView({
    parent: host,
    state: EditorState.create({
      doc: text,
      extensions: [...common(), history(), singleUndo(), keymap.of(defaultKeymap), ...extensions],
    }),
  });
  return { root, view };
}
