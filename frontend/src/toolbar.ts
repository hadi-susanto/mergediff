import { conflictsLeft, nonConflictingLeft, type Tracked } from "./chunks.ts";
import type { Source } from "./model.ts";
import type { Direction } from "./nav.ts";

function el<K extends keyof HTMLElementTagNameMap>(tag: K, cls: string, text = ""): HTMLElementTagNameMap[K] {
  const e = document.createElement(tag);
  e.className = cls;
  if (text) e.textContent = text;
  return e;
}

/**
 * A toolbar button. Its accessible name is the visible label; only buttons
 * whose label is a bare symbol get the title as aria-label.
 */
function button(label: string, cls: string, title: string, onClick: () => void): HTMLButtonElement {
  const b = el("button", `btn ${cls}`, label);
  b.title = title;
  if (!/[A-Za-z]/.test(label)) b.setAttribute("aria-label", title);
  b.addEventListener("click", onClick);
  return b;
}

function plural(n: number, word: string): string {
  return `${n} ${word}${n === 1 ? "" : "s"}`;
}

/** ↑ / ↓ buttons for previous / next change. */
export function changeNavButtons(go: (dir: Direction) => void): HTMLButtonElement[] {
  return [
    button("↑", "btn-icon", "Previous change (Shift+F7)", () => go(-1)),
    button("↓", "btn-icon", "Next change (F7)", () => go(1)),
  ];
}

export interface MergeToolbarActions {
  apply(source: Source): void;
  change(dir: Direction): void;
  conflict(dir: Direction): void;
  save(): void;
  cancel(): void;
}

export interface MergeToolbar {
  /** Refreshes counts, the conflict pill and disabled states. */
  update(tracked: readonly Tracked[]): void;
  /** Closes the "apply from one side" menu; returns whether it was open. */
  closeMenu(): boolean;
}

/** Builds the merge-mode toolbar into the left and right toolbar areas. */
export function createMergeToolbar(left: HTMLElement, right: HTMLElement, actions: MergeToolbarActions): MergeToolbar {
  // "Apply non-conflicting changes" with a dropdown for one side only.
  const split = el("div", "split-button");
  const main = el("button", "btn split-main", "✦ Apply non-conflicting changes");
  const caret = el("button", "btn split-caret", "▾");
  caret.title = "Apply from one side only";
  caret.setAttribute("aria-label", "Apply from one side only");
  const menu = el("div", "split-menu");
  menu.hidden = true;
  const fromLocal = el("button", "split-item", "From Local only");
  const fromRemote = el("button", "split-item", "From Remote only");
  menu.append(fromLocal, fromRemote);
  split.append(main, caret, menu);

  const closeMenu = () => {
    const wasOpen = !menu.hidden;
    menu.hidden = true;
    return wasOpen;
  };
  main.addEventListener("click", () => actions.apply("both"));
  caret.addEventListener("click", (e) => {
    e.stopPropagation();
    menu.hidden = !menu.hidden;
  });
  fromLocal.addEventListener("click", () => {
    closeMenu();
    actions.apply("local");
  });
  fromRemote.addEventListener("click", () => {
    closeMenu();
    actions.apply("remote");
  });
  document.addEventListener("click", (e) => {
    if (!split.contains(e.target as Node)) closeMenu();
  });
  main.title = "Apply non-conflicting changes (Ctrl+Shift+A)";

  const [prevChange, nextChange] = changeNavButtons((dir) => actions.change(dir));
  const prevConflict = button("↑ Conflict", "", "Previous unresolved conflict", () => actions.conflict(-1));
  const nextConflict = button("↓ Conflict", "", "Next unresolved conflict", () => actions.conflict(1));
  const nav = el("div", "btn-group");
  nav.append(prevChange, nextChange);
  const conflictNav = el("div", "btn-group");
  conflictNav.append(prevConflict, nextConflict);

  const pill = el("span", "conflict-pill");
  left.append(split, nav, conflictNav, pill);

  const cancel = button("Cancel", "", "Cancel the merge (Esc)", () => actions.cancel());
  const save = button("Save", "btn-primary", "Save the result (Ctrl+S)", () => actions.save());
  right.append(cancel, save);

  return {
    closeMenu,
    update(tracked) {
      const all = nonConflictingLeft(tracked, "both");
      const local = nonConflictingLeft(tracked, "local");
      const remote = nonConflictingLeft(tracked, "remote");
      main.disabled = all === 0;
      caret.disabled = all === 0;
      main.title =
        all === 0 ? "No non-conflicting changes left" : `Apply ${plural(all, "non-conflicting change")} (Ctrl+Shift+A)`;
      fromLocal.disabled = local === 0;
      fromRemote.disabled = remote === 0;
      fromLocal.textContent = `From Local only (${local})`;
      fromRemote.textContent = `From Remote only (${remote})`;
      if (all === 0) closeMenu();

      prevChange.disabled = nextChange.disabled = tracked.length === 0;
      const conflicts = conflictsLeft(tracked);
      prevConflict.disabled = nextConflict.disabled = conflicts === 0;
      pill.textContent = conflicts === 0 ? "All conflicts resolved" : `${plural(conflicts, "conflict")} left`;
      pill.classList.toggle("conflict-pill-done", conflicts === 0);
    },
  };
}
