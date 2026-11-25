// The filter bar above a file list: a text filter, one chip per category
// (one active, each with its count), optional checkboxes and "Group by
// directory". Shared by the directory view and the conflicts view; the
// filtering itself lives in filelist-core.ts.

import type { Chip } from "./filelist-core.ts";

/** What the filter bar is set to; the bar edits it in place. */
export interface FilterState {
  text: string;
  chip: string;
  grouped: boolean;
}

/** A checkbox of the bar (e.g. "Show unchanged"). */
export interface FilterCheckbox {
  label: HTMLLabelElement;
  /** The visible text, to update counts in it. */
  text: HTMLSpanElement;
}

export interface FilterBar {
  root: HTMLElement;
  input: HTMLInputElement;
  /** Updates the chip labels with their counts and marks the active chip. */
  update(counts: ReadonlyMap<string, number>, active: string): void;
  /** Adds a checkbox before "Group by directory"; set runs before changed. */
  checkbox(label: string, on: boolean, set: (on: boolean) => void): FilterCheckbox;
}

/** n with thousands separators: 84,317. */
export function num(n: number): string {
  return n.toLocaleString("en-US");
}

/** The chip's label with its count, e.g. "Unresolved 7". */
export function chipText(label: string, count: number): string {
  return `${label} ${num(count)}`;
}

/** Builds the bar for chips; changed() runs after every edit of f. */
export function makeFilterBar<T>(f: FilterState, chips: readonly Chip<T>[], changed: () => void): FilterBar {
  const root = document.createElement("div");
  root.className = "filter-bar";

  const input = document.createElement("input");
  input.type = "search";
  input.className = "filter-input";
  input.placeholder = "Filter files…";
  input.setAttribute("aria-label", "Filter files");
  input.addEventListener("input", () => {
    f.text = input.value;
    changed();
  });

  const chipBox = document.createElement("div");
  chipBox.className = "chips";
  chipBox.setAttribute("role", "radiogroup");
  const chipButtons = chips.map((c) => {
    const b = document.createElement("button");
    b.className = "chip";
    b.setAttribute("role", "radio");
    b.addEventListener("click", () => {
      f.chip = c.id;
      changed();
    });
    chipBox.appendChild(b);
    return b;
  });

  const makeCheckbox = (text: string, on: boolean, set: (v: boolean) => void): FilterCheckbox => {
    const label = document.createElement("label");
    label.className = "check";
    const box = document.createElement("input");
    box.type = "checkbox";
    box.checked = on;
    box.addEventListener("change", () => {
      set(box.checked);
      changed();
    });
    const span = document.createElement("span");
    span.textContent = text;
    label.append(box, span);
    return { label, text: span };
  };
  const grouped = makeCheckbox("Group by directory", f.grouped, (v) => (f.grouped = v));

  const spacer = document.createElement("div");
  spacer.className = "filter-spacer";
  root.append(input, chipBox, spacer, grouped.label);

  return {
    root,
    input,
    update(counts, active) {
      chips.forEach((c, i) => {
        const on = c.id === active;
        chipButtons[i].textContent = chipText(c.label, counts.get(c.id) ?? 0);
        chipButtons[i].classList.toggle("chip-active", on);
        chipButtons[i].setAttribute("aria-checked", String(on));
      });
    },
    checkbox(text, on, set) {
      const cb = makeCheckbox(text, on, set);
      root.insertBefore(cb.label, grouped.label);
      return cb;
    },
  };
}
