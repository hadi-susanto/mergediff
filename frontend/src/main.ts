import { Events } from "@wailsio/runtime";
import * as App from "../bindings/github.com/hadi-susanto/mergediff/app";
import { Mode, type Session } from "../bindings/github.com/hadi-susanto/mergediff/internal/session/models";
import { showError } from "./dialogs.ts";
import { renderDiffView } from "./diffview.ts";
import { renderMerge, type Rendered } from "./merge.ts";
import type { CharSpan, MergeChunk } from "./model.ts";
import { setLanguage } from "./panes.ts";
import { installTheme, themeFromURL } from "./theme.ts";
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

/** Sets the "theme" setting; it starts from the window URL (see main.go). */
const setTheme = installTheme(
  document.documentElement,
  window.matchMedia("(prefers-color-scheme: dark)"),
  themeFromURL(location.search),
);

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
  if (session.mode === Mode.Diff || session.mode === Mode.Scratch) {
    const scratch = session.mode === Mode.Scratch;
    const [leftPath = "", rightPath = ""] = session.paths ?? [];
    rendered = renderDiffView({
      left: scratch ? { title: "Left", tooltip: "", text: "" } : { title: baseName(leftPath), tooltip: leftPath, text: session.local },
      right: scratch ? { title: "Right", tooltip: "", text: "" } : { title: baseName(rightPath), tooltip: rightPath, text: session.remote },
      chunks: chunksOf(session),
      compare: scratch ? (l, r) => App.Compare(l, r).then((c) => (c ?? []) as unknown as MergeChunk[]) : undefined,
      charDiff,
      close: () => quit(EXIT_OK),
      status: (text) => (document.getElementById("status-left")!.textContent = text),
    });
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
