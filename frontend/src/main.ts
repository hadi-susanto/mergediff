import { Clipboard, Events, Window } from "@wailsio/runtime";
import * as Backend from "../bindings/github.com/hadi-susanto/mergediff/windows";
import type { Side } from "../bindings/github.com/hadi-susanto/mergediff/internal/gitconflict/models";
import { Mode, type Session } from "../bindings/github.com/hadi-susanto/mergediff/internal/session/models";
import { showError } from "./dialogs.ts";
import { renderDiffView, type DiffPaneSpec } from "./diffview.ts";
import { renderDirView } from "./dirview.ts";
import { renderMerge, type Rendered } from "./merge.ts";
import { renderResolveView } from "./resolveview.ts";
import type { CharSpan, MergeChunk } from "./model.ts";
import { setLanguage } from "./panes.ts";
import { installTheme, themeFromURL } from "./theme.ts";
import "./styles.css";

/**
 * Prefix of the event the backend emits when a window's close button is
 * used; the window's name follows (closeRequestedEvent in main.go).
 */
const CLOSE_REQUESTED = "mergediff:close-requested:";
/** Prefix of the event telling a Conflicts window to refresh (windows.go). */
const CONFLICTS_CHANGED = "mergediff:conflicts-changed:";

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

/**
 * A diff pane for a file. An empty path is a file missing on that side
 * (opened from a directory comparison): empty text titled "(missing)".
 */
function paneSpec(path: string, text: string, label = ""): DiffPaneSpec {
  if (label) return { title: label, tooltip: path, text };
  return path ? { title: baseName(path), tooltip: path, text } : { title: "(missing)", tooltip: "", text: "" };
}

/**
 * Calls handler when the backend emits event + this window's name
 * (per-window events: every window receives every event).
 */
function onWindowEvent(event: string, handler: () => void): Promise<void> {
  return Window.Name().then((name) => {
    Events.On(event + name, handler);
  });
}

/** Routes this window's close button to handler (the backend's close guard). */
function guardClose(handler: () => void): void {
  onWindowEvent(CLOSE_REQUESTED, () => {
    Backend.CloseRequestSeen().catch((err) => console.error("acknowledging close failed", err));
    handler();
  })
    .then(() => Backend.GuardClose(true))
    .catch((err) => console.error("guarding close failed", err));
}

function quit(code: number): void {
  Backend.Quit(code).catch((err) => console.error("quit failed", err));
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
  return Backend.CharDiff(left, right).then((spans) => (spans ?? []) as unknown as CharSpan[]);
}

/** Sets the "theme" setting; it starts from the window URL (see main.go). */
const setTheme = installTheme(
  document.documentElement,
  window.matchMedia("(prefers-color-scheme: dark)"),
  themeFromURL(location.search),
);

async function main(): Promise<void> {
  Backend.Settings()
    .then((settings) => setTheme(settings?.theme))
    .catch((err) => console.error("loading settings failed", err));
  let session: Session;
  let version: string;
  try {
    [session, version] = await Promise.all([Backend.LoadSession(), Backend.Version()]);
  } catch (err) {
    showError(`Could not load the files: ${err}`, () => quit(EXIT_ERROR));
    return;
  }
  document.title = `mergediff — ${session.title}`;
  if (session.mode === Mode.DirDiff) {
    const [left = "", right = ""] = session.paths ?? [];
    renderDirView({
      left,
      right,
      version,
      backend: {
        entries: () => Backend.DirEntries().then((e) => e ?? []),
        progress: () => Backend.DirProgress(),
        stats: () => Backend.DirStats(),
        open: (path) => Backend.OpenEntry(path),
        quit,
      },
    });
    return;
  }
  if (session.mode === Mode.Resolve) {
    renderResolveView({
      repo: session.paths?.[0] ?? "",
      version,
      backend: {
        conflicts: () => Backend.Conflicts(),
        accept: (paths, side) => Backend.AcceptSide(paths, side as Side),
        counts: (path) => Backend.MergeCounts(path),
        edited: (paths) => Backend.Edited(paths).then((p) => p ?? []),
        openMerge: (path) => Backend.OpenMerge(path),
        showChanges: (path) => Backend.ShowChanges(path),
        unsavedMerges: () => Backend.UnsavedMerges().then((p) => p ?? []),
        copy: (text) => Clipboard.SetText(text),
        onChanged: (handler) => {
          onWindowEvent(CONFLICTS_CHANGED, handler).catch((err) => console.error("listening for changes failed", err));
        },
        guardClose,
        quit,
      },
    });
    return;
  }
  let rendered: Rendered;
  if (session.mode === Mode.Diff || session.mode === Mode.Scratch) {
    const scratch = session.mode === Mode.Scratch;
    const [leftPath = "", rightPath = ""] = session.paths ?? [];
    rendered = renderDiffView({
      left: scratch ? { title: "Left", tooltip: "", text: "" } : paneSpec(leftPath, session.local, session.localLabel),
      right: scratch ? { title: "Right", tooltip: "", text: "" } : paneSpec(rightPath, session.remote, session.remoteLabel),
      chunks: chunksOf(session),
      compare: scratch ? (l, r) => Backend.Compare(l, r).then((c) => (c ?? []) as unknown as MergeChunk[]) : undefined,
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
        localLabel: session.localLabel,
        remoteLabel: session.remoteLabel,
        editedOnDisk: session.editedOnDisk,
      },
      {
        save: (result) => Backend.Save(result),
        quit,
        charDiff,
        guardClose,
        setDirty: (dirty) => {
          Backend.SetDirty(dirty).catch((err) => console.error("reporting edits failed", err));
        },
      },
    );
  }
  setStatus(session, version, rendered.status);
  setLanguage(rendered.views, session.language).catch((err) => console.error("loading the language failed", err));
}

void main();
