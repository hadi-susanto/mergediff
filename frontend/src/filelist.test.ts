import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { DOUBLE_CLICK_MS, isDoubleClick } from "./filelist.ts";

describe("isDoubleClick", () => {
  it("two presses on the same row in time", () => {
    assert.ok(isDoubleClick({ index: 3, time: 1000 }, 3, 1000 + DOUBLE_CLICK_MS));
  });
  it("too slow, another row, or no previous press", () => {
    assert.ok(!isDoubleClick({ index: 3, time: 1000 }, 3, 1001 + DOUBLE_CLICK_MS));
    assert.ok(!isDoubleClick({ index: 3, time: 1000 }, 4, 1100));
    assert.ok(!isDoubleClick({ index: -1, time: 0 }, -1, 10));
  });
});
