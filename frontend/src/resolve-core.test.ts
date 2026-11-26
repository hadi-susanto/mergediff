import assert from "node:assert/strict";
import { describe as suite, it } from "node:test";
import {
  actionsFor,
  banner,
  canShowChanges,
  confirmText,
  describe,
  groupSummary,
  mergeBlocked,
  pillText,
  resolveItems,
  stateText,
  statusText,
  unsavedText,
  type ConflictRow,
  type ResolveFilter,
} from "./resolve-core.ts";

function row(path: string, more: Partial<ConflictRow> = {}): ConflictRow {
  return {
    path,
    yours: "modified",
    theirs: "modified",
    kind: "text",
    state: "conflict",
    resolved: false,
    ...more,
  };
}

const ROWS = [
  row("a.txt"),
  row("config/app.yml", { resolved: true }),
  row("src/b.java", { theirs: "deleted", state: "modifyDelete" }),
  row("src/c.java"),
  row("img/logo.png", { kind: "binary", state: "binary" }),
];

function filter(more: Partial<ResolveFilter> = {}): ResolveFilter {
  return { text: "", chip: "all", grouped: false, collapsed: new Set(), ...more };
}

suite("resolveItems", () => {
  it("counts the chips after the text filter", () => {
    const v = resolveItems(ROWS, filter({ text: "SRC" }));
    assert.deepEqual([...v.chipCounts], [["all", 2], ["unresolved", 2], ["resolved", 0]]);
    assert.equal(v.items.length, 2);
  });
  it("the Unresolved and Resolved chips keep their rows", () => {
    assert.equal(resolveItems(ROWS, filter({ chip: "unresolved" })).items.length, 4);
    const resolved = resolveItems(ROWS, filter({ chip: "resolved" })).items;
    assert.equal(resolved.length, 1);
    assert.equal(resolved[0].kind === "row" && resolved[0].row.path, "config/app.yml");
  });
  it("groups by directory with resolved counts", () => {
    const items = resolveItems(ROWS, filter({ grouped: true })).items;
    const headers = items.filter((i) => i.kind === "group");
    assert.deepEqual(
      headers.map((h) => h.kind === "group" && h.label),
      ["(root)", "config/", "img/", "src/"],
    );
    const config = headers[1];
    assert.ok(config.kind === "group");
    assert.deepEqual(groupSummary(config.counts, config.size), { text: "1 file · all resolved", done: true });
    const src = headers[3];
    assert.ok(src.kind === "group");
    assert.deepEqual(groupSummary(src.counts, src.size), { text: "2 files · 2 unresolved", done: false });
  });
});

suite("texts", () => {
  it("pill", () => {
    assert.deepEqual(pillText(7, 9), { text: "7 of 9 files unresolved", done: false });
    assert.deepEqual(pillText(1, 1), { text: "1 of 1 file unresolved", done: false });
    assert.deepEqual(pillText(0, 9), { text: "All conflicts resolved", done: true });
  });
  it("status and state", () => {
    assert.equal(statusText("added"), "Added");
    assert.equal(stateText(row("x", { state: "modifyDelete" })), "Modify/delete");
    assert.equal(stateText(row("x", { state: "binary", resolved: true })), "✓ Resolved");
    assert.equal(stateText(row("x", { state: "submodule" })), "Submodule");
  });
  it("describes the selection", () => {
    assert.equal(describe([]), "No file selected.");
    assert.equal(describe([row("a")]), "Both sides changed this file.");
    assert.equal(
      describe([row("a")], { conflicts: 3, nonConflicting: 5 }),
      "Both sides changed this file. 3 conflicts, 5 non-conflicting changes.",
    );
    assert.equal(
      describe([row("a", { yours: "added", theirs: "added" })], { conflicts: 1, nonConflicting: 1 }),
      "Both sides added this file. 1 conflict, 1 non-conflicting change.",
    );
    assert.equal(describe([row("a", { theirs: "deleted", state: "modifyDelete" })]), "Yours modified this file; theirs deleted it.");
    assert.equal(describe([row("a", { yours: "deleted", state: "modifyDelete" })]), "Yours deleted this file; theirs modified it.");
    assert.match(describe([row("a", { kind: "notUTF8", state: "binary" })]), /not valid UTF-8/);
    assert.match(describe([row("a", { state: "submodule", kind: "submodule" })]), /Submodule/);
    assert.equal(describe([row("a"), row("b", { resolved: true })]), "2 files selected, 1 unresolved.");
  });
  it("confirmation names how many", () => {
    assert.equal(confirmText(3, "theirs"), "Accept Theirs for 3 files?");
    assert.equal(confirmText(2, "yours"), "Accept Yours for 2 files?");
  });
});

suite("actionsFor", () => {
  const none = new Set<string>();
  it("nothing selected: all disabled", () => {
    const a = actionsFor([], none);
    assert.equal(a.merge.enabled || a.yours.enabled || a.theirs.enabled, false);
  });
  it("a text conflict can be merged and accepted", () => {
    const a = actionsFor([row("a")], none);
    assert.equal(a.merge.enabled, true);
    assert.equal(a.yours.label, "Accept Yours");
    assert.equal(a.theirs.enabled, true);
    assert.equal(a.yours.title, "Accept Yours (Ctrl+Shift+Y)");
  });
  it("Merge… on a locked row focuses its window", () => {
    const a = actionsFor([row("a")], new Set(["a"]));
    assert.equal(a.merge.enabled, true);
    assert.equal(a.merge.title, "Show its open merge window (Enter)");
    assert.equal(a.yours.enabled, false);
  });
  it("modify/delete names the buttons by what they do", () => {
    const a = actionsFor([row("a", { theirs: "deleted", state: "modifyDelete" })], none);
    assert.equal(a.merge.enabled, false);
    assert.equal(a.yours.label, "Keep modified (yours)");
    assert.equal(a.theirs.label, "Delete (theirs)");
    const b = actionsFor([row("a", { yours: "deleted", theirs: "added", state: "modifyDelete" })], none);
    assert.equal(b.yours.label, "Delete (yours)");
    assert.equal(b.theirs.label, "Keep added (theirs)");
  });
  it("several rows: plain labels, only unresolved non-submodule targets", () => {
    const sel = [row("a", { theirs: "deleted", state: "modifyDelete" }), row("b", { resolved: true }), row("c")];
    const a = actionsFor(sel, none);
    assert.equal(a.yours.label, "Accept Yours");
    assert.equal(a.merge.enabled, false);
    assert.deepEqual(a.targets.map((r) => r.path), ["a", "c"]);
  });
  it("binary and symlink: accept only", () => {
    for (const state of ["binary", "symlink"]) {
      const a = actionsFor([row("a", { state })], none);
      assert.equal(a.merge.enabled, false);
      assert.equal(a.yours.enabled, true);
    }
  });
  it("submodules and resolved rows: no actions", () => {
    const sub = actionsFor([row("m", { state: "submodule", kind: "submodule" })], none);
    assert.equal(sub.yours.enabled || sub.theirs.enabled || sub.merge.enabled, false);
    assert.equal(sub.yours.title, "Resolve submodules with Git");
    const done = actionsFor([row("a", { resolved: true })], none);
    assert.equal(done.yours.enabled, false);
    assert.equal(done.yours.title, "Already resolved");
  });
  it("a row open in a merge window cannot be accepted", () => {
    const a = actionsFor([row("a"), row("b")], new Set(["b"]));
    assert.equal(a.yours.enabled, false);
    assert.equal(a.theirs.title, "Close the merge window of b first");
  });
  it("mergeBlocked says why", () => {
    assert.equal(mergeBlocked(row("a")), null);
    assert.equal(mergeBlocked(row("a", { state: "binary", kind: "notUTF8" })), "it is not valid UTF-8 text");
    assert.equal(mergeBlocked(row("a", { resolved: true })), "already resolved");
  });
});

suite("Show changes, banner, unsaved", () => {
  it("Show changes only for one unresolved modify/delete text file", () => {
    assert.equal(canShowChanges([row("a", { theirs: "deleted", state: "modifyDelete" })]), true);
    assert.equal(canShowChanges([row("a", { yours: "deleted", theirs: "added", state: "modifyDelete" })]), true);
    assert.equal(canShowChanges([row("a", { yours: "deleted", theirs: "deleted", state: "modifyDelete" })]), false);
    assert.equal(canShowChanges([row("a", { theirs: "deleted", state: "modifyDelete", kind: "binary" })]), false);
    assert.equal(canShowChanges([row("a", { theirs: "deleted", state: "modifyDelete", resolved: true })]), false);
    assert.equal(canShowChanges([row("a")]), false);
    assert.equal(canShowChanges([row("a", { theirs: "deleted", state: "modifyDelete" }), row("b")]), false);
  });
  it("the banner appears once everything is resolved", () => {
    assert.equal(banner(1, 9, "git merge --continue"), null);
    assert.equal(banner(0, 0, "git commit"), null);
    assert.deepEqual(banner(0, 9, "git merge --continue"), {
      text: "All conflicts resolved. Next, run:",
      command: "git merge --continue",
    });
    assert.equal(banner(0, 1, "")?.command, "git commit");
  });
  it("unsaved merges are named", () => {
    assert.match(unsavedText(["a.txt"]), /^The merge of a\.txt has unsaved edits/);
    assert.match(unsavedText(["a", "b"]), /2 files .*all of them/s);
  });
  it("a locked row says so", () => {
    assert.match(describe([row("a")], undefined, new Set(["a"])), /Open in a merge window/);
  });
});
