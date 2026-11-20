import { conflictsLeft, nonConflictingLeft, type Tracked } from "./chunks.ts";
import type { Source } from "./model.ts";

export interface MergeToolbarActions {
  apply(source: Source): void;
  save(): void;
  cancel(): void;
}

export interface MergeToolbar {
  /** Refreshes counts, the conflict pill and disabled states. */
  update(tracked: readonly Tracked[]): void;
}

function el<K extends keyof HTMLElementTagNameMap>(tag: K, cls: string, text = ""): HTMLElementTagNameMap[K] {
  const e = document.createElement(tag);
  e.className = cls;
  if (text) e.textContent = text;
  return e;
}

function plural(n: number, word: string): string {
  return `${n} ${word}${n === 1 ? "" : "s"}`;
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

  const closeMenu = () => (menu.hidden = true);
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

  const pill = el("span", "conflict-pill");
  left.append(split, pill);

  const cancel = el("button", "btn", "Cancel");
  const save = el("button", "btn btn-primary", "Save");
  cancel.addEventListener("click", () => actions.cancel());
  save.addEventListener("click", () => actions.save());
  right.append(cancel, save);

  return {
    update(tracked) {
      const all = nonConflictingLeft(tracked, "both");
      const local = nonConflictingLeft(tracked, "local");
      const remote = nonConflictingLeft(tracked, "remote");
      main.disabled = all === 0;
      caret.disabled = all === 0;
      main.title = all === 0 ? "No non-conflicting changes left" : `Apply ${plural(all, "non-conflicting change")}`;
      fromLocal.disabled = local === 0;
      fromRemote.disabled = remote === 0;
      fromLocal.textContent = `From Local only (${local})`;
      fromRemote.textContent = `From Remote only (${remote})`;
      if (all === 0) closeMenu();

      const left = conflictsLeft(tracked);
      pill.textContent = left === 0 ? "All conflicts resolved" : `${plural(left, "conflict")} left`;
      pill.classList.toggle("conflict-pill-done", left === 0);
    },
  };
}
