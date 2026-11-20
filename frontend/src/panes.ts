import { EditorState, type Extension } from "@codemirror/state";
import { EditorView, lineNumbers } from "@codemirror/view";

/** A titled editor pane. */
export interface Pane {
  root: HTMLElement;
  view: EditorView;
}

/**
 * Creates a read-only pane with a header and line numbers. The text is shown
 * as-is; selection and copying still work.
 */
export function createReadOnlyPane(title: string, tooltip: string, text: string, extensions: Extension[]): Pane {
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

  const view = new EditorView({
    parent: host,
    state: EditorState.create({
      doc: text,
      extensions: [lineNumbers(), EditorState.readOnly.of(true), EditorView.editable.of(false), ...extensions],
    }),
  });
  return { root, view };
}
