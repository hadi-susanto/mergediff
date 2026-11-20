import { Text } from "@codemirror/state";
import * as App from "../bindings/github.com/hadi-susanto/mergediff/app";
import { ChunkKind, type Chunk } from "../bindings/github.com/hadi-susanto/mergediff/internal/merge/models";
import { Mode, type Session } from "../bindings/github.com/hadi-susanto/mergediff/internal/session/models";
import { changeClass, diffHighlight } from "./highlight";
import { createReadOnlyPane } from "./panes";
import "./styles.css";

/** Process exit codes, mirroring cli.go. */
const EXIT_OK = 0;
const EXIT_ERROR = 2;

const LANGUAGE_NAMES: Record<string, string> = {
  go: "Go", javascript: "JavaScript", typescript: "TypeScript", python: "Python", rust: "Rust",
  java: "Java", kotlin: "Kotlin", cpp: "C/C++", csharp: "C#", php: "PHP", ruby: "Ruby",
  shell: "Shell", html: "HTML", css: "CSS", json: "JSON", markdown: "Markdown", xml: "XML",
  yaml: "YAML", toml: "TOML", sql: "SQL", swift: "Swift", dockerfile: "Dockerfile", makefile: "Makefile",
};

function baseName(path: string): string {
  return path.split(/[\\/]/).pop() || path;
}

function plural(n: number, word: string): string {
  return `${n} ${word}${n === 1 ? "" : "s"}`;
}

/** Shows a modal error; closing it calls onClose. */
function showError(message: string, onClose: () => void): void {
  const dialog = document.createElement("dialog");
  dialog.className = "error-dialog";
  const text = document.createElement("p");
  text.textContent = message;
  const button = document.createElement("button");
  button.className = "btn btn-primary";
  button.textContent = "Close";
  button.addEventListener("click", () => dialog.close());
  dialog.addEventListener("close", onClose);
  dialog.append(text, button);
  document.body.appendChild(dialog);
  dialog.showModal();
}

function quit(code: number): void {
  App.Quit(code).catch((err) => console.error("quit failed", err));
}

function button(label: string, cls: string, onClick: () => void): HTMLButtonElement {
  const b = document.createElement("button");
  b.className = `btn ${cls}`;
  b.textContent = label;
  b.addEventListener("click", onClick);
  return b;
}

function setStatus(session: Session, version: string, left: string): void {
  document.getElementById("status-left")!.textContent = left;
  const language = LANGUAGE_NAMES[session.language] ?? "Plain text";
  document.getElementById("status-right")!.textContent = `${language} · ${session.eol} · UTF-8 · mergediff ${version}`;
}

function renderDiff(session: Session, version: string): void {
  const chunks: Chunk[] = session.chunks ?? [];
  const [leftPath, rightPath] = session.paths ?? ["", ""];
  const panes = document.getElementById("panes")!;
  panes.classList.add("panes-2");

  for (const [side, path, text] of [
    ["left", leftPath, session.local],
    ["right", rightPath, session.remote],
  ] as const) {
    const doc = Text.of(text.split("\n"));
    const pane = createReadOnlyPane(baseName(path), path, text, [diffHighlight(doc, chunks, side)]);
    panes.appendChild(pane.root);
  }

  const counts = { added: 0, deleted: 0, modified: 0 };
  for (const c of chunks) if (c.kind !== ChunkKind.Unchanged) counts[changeClass(c)]++;
  const changes = counts.added + counts.deleted + counts.modified;
  setStatus(
    session,
    version,
    changes === 0
      ? "No differences"
      : `${plural(changes, "change")} · ${counts.modified} modified · ${counts.added} added · ${counts.deleted} deleted`,
  );

  document.getElementById("toolbar-right")!.appendChild(button("Close", "btn-primary", () => quit(EXIT_OK)));
  document.addEventListener("keydown", (e) => {
    if (e.key === "Escape") quit(EXIT_OK);
  });
}

async function main(): Promise<void> {
  let session: Session;
  let version: string;
  try {
    [session, version] = await Promise.all([App.LoadSession(), App.Version()]);
  } catch (err) {
    showError(`Could not load the files: ${err}`, () => quit(EXIT_ERROR));
    return;
  }
  document.title = `mergediff — ${session.title}`;
  if (session.mode === Mode.Diff) {
    renderDiff(session, version);
  } else {
    // The three-pane merge view arrives in Phase 3.
    showError("Merge mode is not implemented yet.", () => quit(EXIT_ERROR));
  }
}

void main();
