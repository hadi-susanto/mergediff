import { Text } from "@codemirror/state";
import { EditorView } from "@codemirror/view";
import { Events } from "@wailsio/runtime";
import * as App from "../bindings/github.com/hadi-susanto/mergediff/app";
import { Mode, type Session } from "../bindings/github.com/hadi-susanto/mergediff/internal/session/models";
import { diffSegments } from "./alignment.ts";
import { showError } from "./dialogs.ts";
import { charMarks, charMarkSet, diffClass, diffHighlight, lineOffset, setCharMarks, type CharMarkInput } from "./highlight.ts";
import { installShortcuts } from "./keys.ts";
import { renderMerge, type Rendered } from "./merge.ts";
import { sliceLines, splitLines, type CharSpan, type MergeChunk } from "./model.ts";
import { step, type Direction } from "./nav.ts";
import { createReadOnlyPane, setLanguage, type Pane } from "./panes.ts";
import { ScrollSync } from "./scrollsync.ts";
import { installTheme } from "./theme.ts";
import { changeNavButtons } from "./toolbar.ts";
import "./styles.css";

/** Event the backend emits when the window's close button is used (main.go). */
const CLOSE_REQUESTED = "mergediff:close-requested";

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

function charDiff(left: string, right: string): Promise<CharSpan[]> {
  return App.CharDiff(left, right).then((spans) => (spans ?? []) as unknown as CharSpan[]);
}

/** Character marks for a diff: deleted spans on the left, inserted on the right. */
async function paintDiffCharMarks(chunks: readonly MergeChunk[], left: Pane, right: Pane, session: Session): Promise<void> {
  const leftLines = splitLines(session.local);
  const rightLines = splitLines(session.remote);
  const modified = chunks.filter((c) => c.kind !== "unchanged" && diffClass(c) === "modified");
  const results = await Promise.all(
    modified.map((c) => charDiff(sliceLines(leftLines, c.local), sliceLines(rightLines, c.remote))),
  );
  const inputs = (pane: Pane, kind: CharSpan["kind"]): CharMarkInput[] =>
    modified.map((c, i) => ({
      offset: lineOffset(pane.view.state.doc, (kind === "delete" ? c.local : c.remote).start),
      spans: results[i],
      kind,
      cls: "md-char-modified",
    }));
  left.view.dispatch({ effects: setCharMarks.of(charMarkSet(inputs(left, "delete"))) });
  right.view.dispatch({ effects: setCharMarks.of(charMarkSet(inputs(right, "insert"))) });
}

function renderDiff(session: Session): Rendered {
  const chunks = chunksOf(session);
  const [leftPath = "", rightPath = ""] = session.paths ?? [];
  const panes = document.getElementById("panes")!;
  panes.classList.add("panes-2");

  const [left, right] = (
    [
      ["left", leftPath, session.local],
      ["right", rightPath, session.remote],
    ] as const
  ).map(([side, path, text]) => {
    const doc = Text.of(text.split("\n"));
    const pane = createReadOnlyPane(baseName(path), path, text, [diffHighlight(doc, chunks, side), charMarks]);
    panes.appendChild(pane.root);
    return pane;
  });

  const sync = new ScrollSync([left.view, right.view], () => diffSegments(chunks));
  window.addEventListener("resize", () => sync.alignTo(1));

  // Navigation steps from the line at the middle of the right pane, or from
  // the last chunk visited if the user has not scrolled since.
  const starts = chunks.filter((c) => c.kind !== "unchanged").map((c) => c.remote.start);
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
  for (const b of nav.querySelectorAll("button")) b.disabled = starts.length === 0;
  toolbarLeft.appendChild(nav);

  const close = document.createElement("button");
  close.className = "btn btn-primary";
  close.textContent = "Close";
  close.title = "Close (Esc)";
  close.addEventListener("click", () => quit(EXIT_OK));
  document.getElementById("toolbar-right")!.appendChild(close);
  installShortcuts({ next: () => go(1), prev: () => go(-1), cancel: () => quit(EXIT_OK) });

  paintDiffCharMarks(chunks, left, right, session).catch((err) => console.error("character diff failed", err));

  const counts = { added: 0, deleted: 0, modified: 0 };
  for (const c of chunks) if (c.kind !== "unchanged") counts[diffClass(c)]++;
  const changes = counts.added + counts.deleted + counts.modified;
  const status =
    changes === 0
      ? "No differences"
      : `${plural(changes, "change")} · ${counts.modified} modified · ${counts.added} added · ${counts.deleted} deleted`;
  return { status, views: [left.view, right.view] };
}

/** Sets the "theme" setting; the system theme applies until it is known. */
const setTheme = installTheme(document.documentElement, window.matchMedia("(prefers-color-scheme: dark)"));

async function main(): Promise<void> {
  App.Settings()
    .then((settings) => setTheme(settings?.theme))
    .catch((err) => console.error("loading settings failed", err));
  let session: Session;
  let version: string;
  try {
    [session, version] = await Promise.all([App.LoadSession(), App.Version()]);
  } catch (err) {
    showError(`Could not load the files: ${err}`, () => quit(EXIT_ERROR));
    return;
  }
  document.title = `mergediff — ${session.title}`;
  let rendered: Rendered;
  if (session.mode === Mode.Diff) {
    rendered = renderDiff(session);
  } else {
    rendered = renderMerge(
      {
        paths: session.paths ?? [],
        base: session.base,
        local: session.local,
        remote: session.remote,
        chunks: chunksOf(session),
        initialResult: session.initialResult,
        autoApplied: session.autoApplied,
      },
      {
        save: (result) => App.Save(result),
        quit,
        charDiff,
        guardClose(handler) {
          Events.On(CLOSE_REQUESTED, () => {
            App.CloseRequestSeen().catch((err) => console.error("acknowledging close failed", err));
            handler();
          });
          App.GuardClose(true).catch((err) => console.error("guarding close failed", err));
        },
      },
    );
  }
  setStatus(session, version, rendered.status);
  setLanguage(rendered.views, session.language).catch((err) => console.error("loading the language failed", err));
}

void main();
