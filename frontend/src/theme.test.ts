import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { installTheme, resolveTheme, themeFromURL, type DarkQuery, type ThemeRoot } from "./theme.ts";

describe("resolveTheme", () => {
  it("an explicit setting wins over the system", () => {
    assert.equal(resolveTheme("dark", false), "dark");
    assert.equal(resolveTheme("light", true), "light");
  });
  it("system, missing or unknown values follow the system", () => {
    for (const s of ["system", undefined, "", "blue"]) {
      assert.equal(resolveTheme(s, true), "dark");
      assert.equal(resolveTheme(s, false), "light");
    }
  });
});

function fakes(systemDark: boolean) {
  const root: ThemeRoot = { dataset: {}, style: { colorScheme: "" } };
  const listeners: (() => void)[] = [];
  const query: DarkQuery = { matches: systemDark, addEventListener: (_t, l) => listeners.push(l) };
  const flip = (dark: boolean) => {
    query.matches = dark;
    for (const l of listeners) l();
  };
  return { root, flip, query };
}

describe("installTheme", () => {
  it("applies the system theme before the setting is known", () => {
    const { root, query } = fakes(true);
    installTheme(root, query);
    assert.equal(root.dataset.theme, "dark");
    assert.equal(root.style.colorScheme, "dark");
  });

  it("an explicit setting ignores system changes", () => {
    const { root, query, flip } = fakes(false);
    installTheme(root, query)("dark");
    assert.equal(root.dataset.theme, "dark");
    flip(false);
    assert.equal(root.dataset.theme, "dark");
  });

  it("the system setting follows system changes", () => {
    const { root, query, flip } = fakes(false);
    installTheme(root, query)("system");
    assert.equal(root.dataset.theme, "light");
    flip(true);
    assert.equal(root.dataset.theme, "dark");
    assert.equal(root.style.colorScheme, "dark");
  });
});

describe("themeFromURL", () => {
  it("reads the theme query parameter", () => {
    assert.equal(themeFromURL("?theme=dark"), "dark");
    assert.equal(themeFromURL("?x=1&theme=light"), "light");
    assert.equal(themeFromURL(""), undefined);
  });

  it("an initial dark setting applies before the settings load, on a light system", () => {
    const { root, query, flip } = fakes(false);
    installTheme(root, query, themeFromURL("?theme=dark"));
    assert.equal(root.dataset.theme, "dark");
    flip(false);
    assert.equal(root.dataset.theme, "dark");
  });
});
