import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { connectorPath, isVisible } from "./connectors.ts";

describe("connectorPath", () => {
  it("joins the side chunk on the left to the result chunk on the right", () => {
    const d = connectorPath(50, { top: 10, bottom: 30 }, { top: 100, bottom: 140 }, true);
    assert.equal(d, "M0,10 C25,10 25,100 50,100 L50,140 C25,140 25,30 0,30 Z");
  });

  it("mirrors for the remote strip (side pane on the right)", () => {
    const d = connectorPath(50, { top: 10, bottom: 30 }, { top: 100, bottom: 140 }, false);
    assert.equal(d, "M0,100 C25,100 25,10 50,10 L50,30 C25,30 25,140 0,140 Z");
  });

  it("draws empty ranges as a line (zero height on one side)", () => {
    const d = connectorPath(40, { top: 20, bottom: 20 }, { top: 20, bottom: 60 }, true);
    assert.equal(d, "M0,20 C20,20 20,20 40,20 L40,60 C20,60 20,20 0,20 Z");
  });

  it("rounds coordinates to one decimal", () => {
    const d = connectorPath(10, { top: 1.234, bottom: 2.25 }, { top: 3.333, bottom: 4 }, true);
    assert.ok(d.startsWith("M0,1.2 C5,1.2 5,3.3 10,3.3"), d);
  });
});

describe("isVisible", () => {
  it("culls connectors entirely above or below the strip", () => {
    assert.equal(isVisible({ top: -50, bottom: -10 }, { top: -40, bottom: -20 }, 500), false);
    assert.equal(isVisible({ top: 600, bottom: 700 }, { top: 510, bottom: 520 }, 500), false);
    assert.equal(isVisible({ top: -50, bottom: -10 }, { top: 490, bottom: 520 }, 500), true);
  });
});
