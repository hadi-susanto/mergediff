import type { EditorView } from "@codemirror/view";
import type { Tracked } from "./chunks.ts";
import type { Side } from "./model.ts";

export type ChunkAction = "apply" | "ignore";

/**
 * The strip between a side pane and the Result pane. It shows apply / ignore
 * buttons next to the first line of every chunk whose side is still pending,
 * aligned with the side pane's scroll position.
 */
export class ButtonStrip {
  readonly root: HTMLElement;
  private readonly body: HTMLElement;
  private tracked: readonly Tracked[] = [];
  private frame = 0;

  constructor(
    private readonly side: Side,
    private readonly sideView: EditorView,
    private readonly onAction: (index: number, side: Side, action: ChunkAction) => void,
  ) {
    this.root = document.createElement("div");
    this.root.className = `strip strip-${side}`;
    const spacer = document.createElement("div");
    spacer.className = "pane-header strip-header";
    this.body = document.createElement("div");
    this.body.className = "strip-body";
    this.root.append(spacer, this.body);

    sideView.scrollDOM.addEventListener("scroll", () => this.schedule(), { passive: true });
    new ResizeObserver(() => this.schedule()).observe(sideView.dom);
  }

  /** Sets the chunks to show buttons for and redraws. */
  update(tracked: readonly Tracked[]): void {
    this.tracked = tracked;
    this.schedule();
  }

  /** Redraws on the next animation frame (at most once per frame). */
  schedule(): void {
    if (this.frame) return;
    this.frame = requestAnimationFrame(() => {
      this.frame = 0;
      this.draw();
    });
  }

  private draw(): void {
    const view = this.sideView;
    const doc = view.state.doc;
    const bodyTop = this.body.getBoundingClientRect().top;
    const height = this.body.clientHeight;
    const groups: HTMLElement[] = [];
    this.tracked.forEach((t, index) => {
      if (t[this.side] !== "pending") return;
      const line = Math.min(t.chunk[this.side].start + 1, doc.lines);
      const block = view.lineBlockAt(doc.line(line).from);
      const top = view.documentTop + block.top - bodyTop;
      if (top + block.height < 0 || top > height) return;
      groups.push(this.group(index, top, block.height));
    });
    this.body.replaceChildren(...groups);
  }

  private group(index: number, top: number, lineHeight: number): HTMLElement {
    const g = document.createElement("div");
    g.className = "strip-buttons";
    g.style.top = `${top}px`;
    g.style.height = `${lineHeight}px`;
    const ignore = this.button("×", "Ignore this change", () => this.onAction(index, this.side, "ignore"));
    const apply =
      this.side === "local"
        ? this.button("»", "Apply this change to the result", () => this.onAction(index, this.side, "apply"))
        : this.button("«", "Apply this change to the result", () => this.onAction(index, this.side, "apply"));
    // Apply points at the Result pane: "× »" on the left strip, "« ×" on the right.
    if (this.side === "local") g.append(ignore, apply);
    else g.append(apply, ignore);
    return g;
  }

  private button(label: string, title: string, onClick: () => void): HTMLButtonElement {
    const b = document.createElement("button");
    b.className = "strip-btn";
    b.textContent = label;
    b.title = title;
    b.setAttribute("aria-label", title);
    b.addEventListener("click", onClick);
    return b;
  }
}
