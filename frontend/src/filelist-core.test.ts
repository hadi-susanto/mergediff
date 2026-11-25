import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  chipCounts,
  countBy,
  filterRows,
  flatItems,
  groupItems,
  matchesText,
  middleEllipsis,
  scrollToShow,
  Selection,
  sortByPath,
  splitPath,
  visibleWindow,
  type Chip,
  type GroupItem,
  type ListItem,
  type RowItem,
} from "./filelist-core.ts";

describe("splitPath", () => {
  it("splits the directory (with its slash) from the file name", () => {
    assert.deepEqual(splitPath("src/main/App.java"), { dir: "src/main/", name: "App.java" });
    assert.deepEqual(splitPath("a/b"), { dir: "a/", name: "b" });
  });
  it("a file in the root has no directory part", () => {
    assert.deepEqual(splitPath("pom.xml"), { dir: "", name: "pom.xml" });
  });
  it("keeps unicode and spaces", () => {
    assert.deepEqual(splitPath("dir with space/ünï 文件.txt"), { dir: "dir with space/", name: "ünï 文件.txt" });
  });
});

describe("sortByPath", () => {
  it("sorts by path without changing the input", () => {
    const rows = [{ path: "b/a" }, { path: "a" }, { path: "a/z" }, { path: "B" }];
    assert.deepEqual(
      sortByPath(rows).map((r) => r.path),
      ["B", "a", "a/z", "b/a"],
    );
    assert.equal(rows[0].path, "b/a");
  });
});

describe("middleEllipsis", () => {
  it("leaves short text alone", () => {
    assert.equal(middleEllipsis("/tmp/x", 10), "/tmp/x");
    assert.equal(middleEllipsis("/tmp/x", 6), "/tmp/x");
  });
  it("keeps both ends", () => {
    assert.equal(middleEllipsis("/tmp/git-difftool.Xk2Pq/left", 11), "/tmp/…/left");
    assert.equal(middleEllipsis("abcdefgh", 4), "ab…h");
  });
  it("counts characters, not UTF-16 units", () => {
    assert.equal(middleEllipsis("😀😀😀😀😀", 3), "😀…😀");
  });
  it("tiny limits", () => {
    assert.equal(middleEllipsis("abc", 1), "…");
    assert.equal(middleEllipsis("abc", 0), "");
  });
});

describe("Selection", () => {
  // Rows 0 and 3 are headers that cannot be selected.
  const selectable = (i: number) => i !== 0 && i !== 3;

  it("moves over unselectable rows and stops at the ends", () => {
    const s = new Selection(6, selectable);
    assert.equal(s.current, -1);
    assert.ok(s.move(1));
    assert.equal(s.current, 1);
    s.move(1);
    s.move(1);
    assert.equal(s.current, 4); // skipped header 3
    s.move(1);
    assert.equal(s.current, 5);
    assert.equal(s.move(1), false);
    assert.equal(s.current, 5);
    s.move(-1);
    s.move(-1);
    assert.equal(s.current, 2);
  });

  it("jumps to the first and last selectable rows", () => {
    const s = new Selection(4, selectable);
    s.edge(-1);
    assert.equal(s.current, 2);
    s.edge(1);
    assert.equal(s.current, 1);
  });

  it("select refuses unselectable or missing rows", () => {
    const s = new Selection(6, selectable);
    assert.equal(s.select(0), false);
    assert.equal(s.select(9), false);
    assert.ok(s.select(4));
    assert.deepEqual(s.rows(), [4]);
  });

  it("find looks for rows matching a condition", () => {
    const s = new Selection(6, selectable);
    const even = (i: number) => i % 2 === 0;
    assert.equal(s.find(-1, 1, even), 2);
    assert.equal(s.find(2, 1, even), 4);
    assert.equal(s.find(4, 1, even), -1);
    assert.equal(s.find(6, -1, even), 4);
  });

  it("without multi-select, toggle and extendTo select one row", () => {
    const s = new Selection(6, selectable);
    s.select(1);
    s.toggle(2);
    assert.deepEqual(s.rows(), [2]);
    s.extendTo(5);
    assert.deepEqual(s.rows(), [5]);
  });

  it("multi-select: toggle and range", () => {
    const s = new Selection(6, selectable, true);
    s.select(1);
    s.toggle(4);
    assert.deepEqual(s.rows(), [1, 4]);
    s.toggle(1);
    assert.deepEqual(s.rows(), [4]);
    s.toggle(4); // the last selected row stays selected
    assert.deepEqual(s.rows(), [4]);
    s.select(1);
    s.extendTo(5);
    assert.deepEqual(s.rows(), [1, 2, 4, 5]); // header 3 skipped
    assert.equal(s.current, 5);
  });

  it("reset keeps the current row when it still exists", () => {
    const s = new Selection(6, selectable, true);
    s.select(4);
    s.toggle(5);
    s.reset(6);
    assert.equal(s.current, 5);
    assert.deepEqual(s.rows(), [5]);
    s.reset(3);
    assert.equal(s.current, -1);
  });
});

describe("filterRows", () => {
  const rows = [{ path: "src/Main.java" }, { path: "src/util/Strings.java" }, { path: "README.md" }];
  it("matches a substring of the path, ignoring case", () => {
    assert.deepEqual(
      filterRows(rows, "MAIN").map((r) => r.path),
      ["src/Main.java"],
    );
    assert.deepEqual(
      filterRows(rows, "src/").map((r) => r.path),
      ["src/Main.java", "src/util/Strings.java"],
    );
  });
  it("empty text keeps everything", () => {
    assert.equal(filterRows(rows, "").length, 3);
  });
  it("applies keep() too", () => {
    assert.deepEqual(
      filterRows(rows, "java", (r) => !r.path.includes("util")).map((r) => r.path),
      ["src/Main.java"],
    );
  });
  it("matchesText agrees", () => {
    assert.ok(matchesText("src/Main.java", "main"));
    assert.ok(matchesText("x", ""));
    assert.ok(!matchesText("src/Main.java", "test"));
  });
});

describe("countBy and chipCounts", () => {
  const rows = [{ s: "added" }, { s: "modified" }, { s: "added" }, { s: "unchanged" }];
  it("counts by key", () => {
    assert.deepEqual([...countBy(rows, (r) => r.s)], [["added", 2], ["modified", 1], ["unchanged", 1]]);
  });
  it("counts what each chip keeps; chips may overlap", () => {
    const chips: Chip<{ s: string }>[] = [
      { id: "all", label: "All changes", keep: (r) => r.s !== "unchanged" },
      { id: "added", label: "Added", keep: (r) => r.s === "added" },
      { id: "deleted", label: "Deleted", keep: (r) => r.s === "deleted" },
    ];
    assert.deepEqual([...chipCounts(rows, chips)], [["all", 3], ["added", 2], ["deleted", 0]]);
  });
});

describe("groupItems", () => {
  const rows = [
    { path: "pom.xml", s: "modified" },
    { path: "src/b/B.java", s: "added" },
    { path: "src/a/A.java", s: "modified" },
    { path: "src/a/A2.java", s: "added" },
    { path: ".github/ci.yml", s: "deleted" },
    { path: "README.md", s: "added" },
  ];
  const sorted = sortByPath(rows);
  const shape = (items: ListItem<{ path: string; s: string }>[]) =>
    items.map((i) => (i.kind === "group" ? `[${i.label}${i.collapsed ? " +" : ""}]` : i.row.path));

  it("puts the root first, then directories sorted by path, rows in order", () => {
    assert.deepEqual(shape(groupItems(sorted, (r) => r.s, new Set())), [
      "[(root)]",
      "README.md",
      "pom.xml",
      "[.github/]",
      ".github/ci.yml",
      "[src/a/]",
      "src/a/A.java",
      "src/a/A2.java",
      "[src/b/]",
      "src/b/B.java",
    ]);
  });

  it("counts each group's rows by key", () => {
    const items = groupItems(sorted, (r) => r.s, new Set());
    const a = items.find((i) => i.kind === "group" && i.dir === "src/a/") as GroupItem;
    assert.deepEqual([...a.counts], [["modified", 1], ["added", 1]]);
    assert.equal(a.size, 2);
    const root = items[0] as GroupItem;
    assert.equal(root.dir, "");
    assert.deepEqual([...root.counts], [["added", 1], ["modified", 1]]);
  });

  it("collapsed groups show only their header and keep their counts", () => {
    const items = groupItems(sorted, (r) => r.s, new Set(["src/a/", ""]));
    assert.deepEqual(shape(items), ["[(root) +]", "[.github/]", ".github/ci.yml", "[src/a/ +]", "[src/b/]", "src/b/B.java"]);
    assert.equal((items[3] as GroupItem).size, 2);
  });

  it("rows know their group; filtering first hides empty groups", () => {
    const items = groupItems(filterRows(sorted, "java"), (r) => r.s, new Set());
    assert.deepEqual(shape(items), ["[src/a/]", "src/a/A.java", "src/a/A2.java", "[src/b/]", "src/b/B.java"]);
    const row = items[1] as RowItem<{ path: string }>;
    assert.equal(row.group, "src/a/");
  });

  it("no rows, no groups", () => {
    assert.deepEqual(groupItems([], () => "", new Set()), []);
  });

  it("flatItems keeps every row without groups", () => {
    assert.deepEqual(
      flatItems(sorted).map((i) => [i.kind, i.row.path, i.group]),
      sorted.map((r) => ["row", r.path, undefined]),
    );
  });
});

describe("visibleWindow", () => {
  it("covers the viewport plus overscan, clamped to the list", () => {
    assert.deepEqual(visibleWindow(0, 320, 32, 1000, 5), { start: 0, end: 15 });
    assert.deepEqual(visibleWindow(3200, 320, 32, 1000, 5), { start: 95, end: 115 });
    assert.deepEqual(visibleWindow(31900, 320, 32, 1000, 5), { start: 991, end: 1000 });
  });
  it("partly visible rows count", () => {
    assert.deepEqual(visibleWindow(16, 32, 32, 100, 0), { start: 0, end: 2 });
  });
  it("empty lists and odd input", () => {
    assert.deepEqual(visibleWindow(0, 320, 32, 0), { start: 0, end: 0 });
    assert.deepEqual(visibleWindow(-50, 64, 32, 10, 0), { start: 0, end: 2 });
  });
});

describe("scrollToShow", () => {
  it("leaves visible rows alone and scrolls as little as possible", () => {
    assert.equal(scrollToShow(5, 100, 320, 32), 100);
    assert.equal(scrollToShow(2, 100, 320, 32), 64); // above: align top
    assert.equal(scrollToShow(20, 100, 320, 32), 352); // below: align bottom (21*32 - 320)
  });
});
