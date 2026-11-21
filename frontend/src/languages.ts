// Syntax highlighting. The language for a session comes from the file
// extension (internal/session.Language); its parser is loaded on demand, so
// the main bundle only carries the editor.

import { HighlightStyle, StreamLanguage, syntaxHighlighting, type StreamParser } from "@codemirror/language";
import type { Extension } from "@codemirror/state";
import { tags as t } from "@lezer/highlight";

const legacy = (parser: StreamParser<unknown>): Extension => StreamLanguage.define(parser);

/** Language id → loader. Ids without an entry are shown as plain text. */
const loaders: Record<string, () => Promise<Extension>> = {
  go: async () => (await import("@codemirror/lang-go")).go(),
  javascript: async () => (await import("@codemirror/lang-javascript")).javascript({ jsx: true }),
  typescript: async () => (await import("@codemirror/lang-javascript")).javascript({ typescript: true, jsx: true }),
  python: async () => (await import("@codemirror/lang-python")).python(),
  rust: async () => (await import("@codemirror/lang-rust")).rust(),
  java: async () => (await import("@codemirror/lang-java")).java(),
  cpp: async () => (await import("@codemirror/lang-cpp")).cpp(),
  php: async () => (await import("@codemirror/lang-php")).php(),
  html: async () => (await import("@codemirror/lang-html")).html(),
  css: async () => (await import("@codemirror/lang-css")).css(),
  json: async () => (await import("@codemirror/lang-json")).json(),
  markdown: async () => (await import("@codemirror/lang-markdown")).markdown(),
  xml: async () => (await import("@codemirror/lang-xml")).xml(),
  yaml: async () => (await import("@codemirror/lang-yaml")).yaml(),
  sql: async () => (await import("@codemirror/lang-sql")).sql(),
  kotlin: async () => legacy((await import("@codemirror/legacy-modes/mode/clike")).kotlin),
  csharp: async () => legacy((await import("@codemirror/legacy-modes/mode/clike")).csharp),
  ruby: async () => legacy((await import("@codemirror/legacy-modes/mode/ruby")).ruby),
  shell: async () => legacy((await import("@codemirror/legacy-modes/mode/shell")).shell),
  swift: async () => legacy((await import("@codemirror/legacy-modes/mode/swift")).swift),
  toml: async () => legacy((await import("@codemirror/legacy-modes/mode/toml")).toml),
  dockerfile: async () => legacy((await import("@codemirror/legacy-modes/mode/dockerfile")).dockerFile),
};

/** Loads the language extension for id, or null for plain text. */
export async function loadLanguage(id: string): Promise<Extension | null> {
  const load = loaders[id];
  return load ? load() : null;
}

/**
 * Token colours. They are CSS variables (see styles.css), so the light and
 * dark themes switch with the system theme without reconfiguring editors.
 */
export const syntaxColours: Extension = syntaxHighlighting(
  HighlightStyle.define([
    { tag: [t.keyword, t.controlKeyword, t.moduleKeyword, t.operatorKeyword, t.definitionKeyword], color: "var(--syn-keyword)" },
    { tag: [t.string, t.special(t.string), t.regexp, t.character], color: "var(--syn-string)" },
    { tag: [t.number, t.bool, t.null, t.atom], color: "var(--syn-number)" },
    { tag: [t.comment, t.lineComment, t.blockComment, t.docComment], color: "var(--syn-comment)", fontStyle: "italic" },
    { tag: [t.typeName, t.className, t.namespace, t.standard(t.typeName)], color: "var(--syn-type)" },
    { tag: [t.function(t.variableName), t.function(t.propertyName), t.definition(t.function(t.variableName))], color: "var(--syn-function)" },
    { tag: [t.propertyName, t.attributeName], color: "var(--syn-property)" },
    { tag: [t.tagName, t.heading], color: "var(--syn-tag)", fontWeight: "bold" },
    { tag: [t.meta, t.processingInstruction, t.annotation], color: "var(--syn-meta)" },
    { tag: t.invalid, color: "var(--syn-invalid)" },
    { tag: t.emphasis, fontStyle: "italic" },
    { tag: t.strong, fontWeight: "bold" },
    { tag: t.link, textDecoration: "underline" },
  ]),
);
