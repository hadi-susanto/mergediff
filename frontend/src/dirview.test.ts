import assert from "node:assert/strict";
import { describe, it } from "node:test";
import type { Entry, Stat } from "../bindings/github.com/hadi-susanto/mergediff/internal/dirdiff/models";
import {
  blockedReason,
  countsText,
  dirItems,
  groupTags,
  linesCell,
  progressText,
  type DirFilter,
  type DirView,
} from "./dirview.ts";

function entry(status: string, more: Partial<Entry> = {}): Entry {
  return { path: "p", status, left: "", right: "", leftSize: 0, rightSize: 0, note: "", ...more } as Entry;
}

describe("blockedReason", () => {
  it("text files can be opened", () => {
    assert.equal(blockedReason(entry("modified", { leftSize: 10, rightSize: 20 })), null);
  });
  it("errors and symlinks to directories give their note", () => {
    assert.equal(blockedReason(entry("error", { note: "permission denied" })), "permission denied");
    assert.equal(blockedReason(entry("modified", { note: "symlink to directory, not followed" })), "symlink to directory, not followed");
  });
  it("files over 5 MB cannot be opened", () => {
    assert.equal(blockedReason(entry("added", { rightSize: 5 * 1024 * 1024 + 1 })), "larger than 5 MB");
    assert.equal(blockedReason(entry("added", { rightSize: 5 * 1024 * 1024 })), null);
  });
});

describe("countsText", () => {
  it("counts changed, unchanged and errors", () => {
    const es = [entry("modified"), entry("added"), entry("deleted"), entry("unchanged"), entry("error")];
    assert.equal(countsText(es), "3 changed · 1 unchanged · 1 error");
  });
  it("uses thousands separators", () => {
    const es = Array.from({ length: 1200 }, () => entry("unchanged"));
    assert.equal(countsText(es), "0 changed · 1,200 unchanged");
  });
  it("leaves out errors when there are none", () => {
    assert.equal(countsText([entry("modified"), entry("error"), entry("error")]), "1 changed · 0 unchanged · 2 errors");
    assert.equal(countsText([]), "0 changed · 0 unchanged");
  });
});

describe("progressText", () => {
  it("shows both counts with thousands separators", () => {
    assert.equal(progressText({ found: 18240, compared: 3100, done: false }), "Comparing… 18,240 files found, 3,100 compared");
  });
});

function stat(more: Partial<Stat>): Stat {
  return { added: 0, deleted: 0, label: "", reason: "", ...more } as Stat;
}

describe("blockedReason with line counts", () => {
  it("binary and not UTF-8 files found while counting cannot be opened", () => {
    assert.equal(blockedReason(entry("modified"), stat({ label: "binary, differs", reason: "binary file" })), "binary file");
    assert.equal(blockedReason(entry("modified"), stat({ label: "not UTF-8", reason: "not valid UTF-8" })), "not valid UTF-8");
    assert.equal(blockedReason(entry("modified"), stat({ added: 3 })), null);
  });
});

describe("linesCell", () => {
  it("waits for the counts", () => {
    assert.deepEqual(linesCell(entry("modified")), { label: "…" });
  });
  it("shows the note of errors and symlinks to directories", () => {
    assert.deepEqual(linesCell(entry("error", { note: "permission denied" })), { label: "permission denied" });
  });
  it("shows nothing for unchanged files", () => {
    assert.deepEqual(linesCell(entry("unchanged")), { label: "" });
  });
  it("shows a label instead of counts", () => {
    assert.deepEqual(linesCell(entry("modified"), stat({ label: "line endings" })), { label: "line endings" });
  });
  it("added and deleted files show one count", () => {
    assert.deepEqual(linesCell(entry("added"), stat({ added: 63 })), { added: 63 });
    assert.deepEqual(linesCell(entry("deleted"), stat({ deleted: 212 })), { deleted: 212 });
    assert.deepEqual(linesCell(entry("added"), stat({})), { added: 0 });
  });
  it("modified files show the non-zero counts", () => {
    assert.deepEqual(linesCell(entry("modified"), stat({ added: 48, deleted: 12 })), { added: 48, deleted: 12 });
    assert.deepEqual(linesCell(entry("modified"), stat({ added: 2 })), { added: 2, deleted: undefined });
    assert.deepEqual(linesCell(entry("modified"), stat({ deleted: 2 })), { added: undefined, deleted: 2 });
    assert.deepEqual(linesCell(entry("modified"), stat({})), { added: 0, deleted: 0 });
  });
});

describe("dirItems", () => {
  const rows = [
    { path: "a/x.txt", entry: entry("modified", { path: "a/x.txt" }) },
    { path: "a/y.txt", entry: entry("unchanged", { path: "a/y.txt" }) },
    { path: "b/new.ts", entry: entry("added", { path: "b/new.ts" }) },
    { path: "gone.md", entry: entry("deleted", { path: "gone.md" }) },
    { path: "z/bad", entry: entry("error", { path: "z/bad", note: "permission denied" }) },
  ];
  const filter = (more: Partial<DirFilter> = {}): DirFilter => ({
    text: "",
    chip: "all",
    showUnchanged: false,
    grouped: false,
    collapsed: new Set(),
    ...more,
  });
  const paths = (v: DirView) => v.items.map((i) => (i.kind === "group" ? `[${i.label}]` : i.row.path));

  it("hides unchanged files unless asked, and counts them", () => {
    const v = dirItems(rows, filter());
    assert.deepEqual(paths(v), ["a/x.txt", "b/new.ts", "gone.md", "z/bad"]);
    assert.equal(v.unchanged, 1);
    assert.deepEqual(paths(dirItems(rows, filter({ showUnchanged: true }))), ["a/x.txt", "a/y.txt", "b/new.ts", "gone.md", "z/bad"]);
  });

  it("chips keep one status; counts follow the text filter", () => {
    assert.deepEqual(paths(dirItems(rows, filter({ chip: "added" }))), ["b/new.ts"]);
    const v = dirItems(rows, filter({ text: "A/" }));
    assert.deepEqual(paths(v), ["a/x.txt"]);
    assert.deepEqual([...v.chipCounts], [["all", 1], ["modified", 1], ["added", 0], ["deleted", 0]]);
    assert.deepEqual([...dirItems(rows, filter()).chipCounts], [["all", 4], ["modified", 1], ["added", 1], ["deleted", 1]]);
  });

  it("groups by directory with the root first", () => {
    assert.deepEqual(paths(dirItems(rows, filter({ grouped: true }))), [
      "[(root)]",
      "gone.md",
      "[a/]",
      "a/x.txt",
      "[b/]",
      "b/new.ts",
      "[z/]",
      "z/bad",
    ]);
    assert.deepEqual(paths(dirItems(rows, filter({ grouped: true, collapsed: new Set(["a/"]), chip: "modified" }))), ["[a/]"]);
  });

  it("an unknown chip falls back to all", () => {
    assert.equal(dirItems(rows, filter({ chip: "nope" })).items.length, 4);
  });
});

describe("groupTags", () => {
  it("lists the non-zero counts in a fixed order", () => {
    const counts = new Map([["added", 2], ["modified", 1], ["error", 1], ["deleted", 0]]);
    assert.deepEqual(groupTags(counts), [
      { status: "modified", text: "1 modified" },
      { status: "added", text: "2 added" },
      { status: "error", text: "1 error" },
    ]);
  });
});
