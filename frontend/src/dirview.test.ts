import assert from "node:assert/strict";
import { describe, it } from "node:test";
import type { Entry } from "../bindings/github.com/hadi-susanto/mergediff/internal/dirdiff/models";
import { blockedReason, countsText, progressText } from "./dirview.ts";

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
