import { defaultKeymap, history, historyKeymap } from "@codemirror/commands";
import { EditorState, type Extension } from "@codemirror/state";
import { EditorView, keymap, lineNumbers } from "@codemirror/view";

/** A titled editor pane. */
export interface Pane {
  root: HTMLElement;
  view: EditorView;
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
      extensions: [lineNumbers(), lfOnly, EditorState.readOnly.of(true), EditorView.editable.of(false), ...extensions],
    }),
  });
  return { root, view };
}

/** Creates the editable Result pane with undo history. */
export function createEditablePane(title: string, tooltip: string, text: string, extensions: Extension[]): Pane {
  const { root, host } = paneShell(title, tooltip);
  root.classList.add("pane-editable");
  const view = new EditorView({
    parent: host,
    state: EditorState.create({
      doc: text,
      extensions: [lineNumbers(), lfOnly, history(), keymap.of([...defaultKeymap, ...historyKeymap]), ...extensions],
    }),
  });
  return { root, view };
}
