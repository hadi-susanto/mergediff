import assert from "node:assert/strict";
import { describe, it } from "node:test";
import type { Entry, Stat } from "../bindings/github.com/hadi-susanto/mergediff/internal/dirdiff/models";
import { blockedReason, countsText, linesCell, progressText } from "./dirview.ts";

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
