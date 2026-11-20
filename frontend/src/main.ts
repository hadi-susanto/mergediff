import { Text } from "@codemirror/state";
import * as App from "../bindings/github.com/hadi-susanto/mergediff/app";
import { Mode, type Session } from "../bindings/github.com/hadi-susanto/mergediff/internal/session/models";
import { showError } from "./dialogs.ts";
import { diffClass, diffHighlight } from "./highlight.ts";
import { renderMerge } from "./merge.ts";
import type { MergeChunk } from "./model.ts";
import { createReadOnlyPane } from "./panes.ts";
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

function quit(code: number): void {
  App.Quit(code).catch((err) => console.error("quit failed", err));
}

/** The generated Chunk type uses enums; the frontend logic uses string unions. */
function chunksOf(session: Session): MergeChunk[] {
  return (session.chunks ?? []) as unknown as MergeChunk[];
}

function setStatus(session: Session, version: string, left: string): void {
  document.getElementById("status-left")!.textContent = left;
  const language = LANGUAGE_NAMES[session.language] ?? "Plain text";
  document.getElementById("status-right")!.textContent = `${language} · ${session.eol} · UTF-8 · mergediff ${version}`;
}

function renderDiff(session: Session): string {
  const chunks = chunksOf(session);
  const [leftPath = "", rightPath = ""] = session.paths ?? [];
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

  const close = document.createElement("button");
  close.className = "btn btn-primary";
  close.textContent = "Close";
  close.addEventListener("click", () => quit(EXIT_OK));
  document.getElementById("toolbar-right")!.appendChild(close);
  document.addEventListener("keydown", (e) => {
    if (e.key === "Escape") quit(EXIT_OK);
  });

  const counts = { added: 0, deleted: 0, modified: 0 };
  for (const c of chunks) if (c.kind !== "unchanged") counts[diffClass(c)]++;
  const changes = counts.added + counts.deleted + counts.modified;
  return changes === 0
    ? "No differences"
    : `${plural(changes, "change")} · ${counts.modified} modified · ${counts.added} added · ${counts.deleted} deleted`;
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
  let status: string;
  if (session.mode === Mode.Diff) {
    status = renderDiff(session);
  } else {
    status = renderMerge(
      {
        paths: session.paths ?? [],
        base: session.base,
        local: session.local,
        remote: session.remote,
        chunks: chunksOf(session),
        initialResult: session.initialResult,
        autoApplied: session.autoApplied,
      },
      { save: (result) => App.Save(result), quit },
    );
  }
  setStatus(session, version, status);
}

void main();
