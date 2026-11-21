import type { EditorView } from "@codemirror/view";
import { isResolved, type Tracked } from "./chunks.ts";
import { connectorPath, isVisible, type Span } from "./connectors.ts";
import { changedOn, sideClass } from "./highlight.ts";
import type { Side } from "./model.ts";
import { lineTop } from "./scrollsync.ts";

export type ChunkAction = "apply" | "ignore";

const SVG = "http://www.w3.org/2000/svg";

/**
 * The strip between a side pane and the Result pane. It draws a connector
 * from every chunk that side changed to the chunk's place in the result, and
 * apply / ignore buttons next to the first line of each chunk whose side is
 * still pending. It redraws (at most once per animation frame) when either
 * pane scrolls or resizes, and when the chunks change.
 */
export class ButtonStrip {
  readonly root: HTMLElement;
  private readonly body: HTMLElement;
  private readonly svg: SVGSVGElement;
  private readonly buttons: HTMLElement;
  private tracked: readonly Tracked[] = [];
  private frame = 0;

  constructor(
    private readonly side: Side,
    private readonly sideView: EditorView,
    private readonly resultView: EditorView,
    private readonly onAction: (index: number, side: Side, action: ChunkAction) => void,
  ) {
    this.root = document.createElement("div");
    this.root.className = `strip strip-${side}`;
    const spacer = document.createElement("div");
    spacer.className = "pane-header strip-header";
    this.body = document.createElement("div");
    this.body.className = "strip-body";
    this.svg = document.createElementNS(SVG, "svg");
    this.svg.classList.add("strip-connectors");
    this.buttons = document.createElement("div");
    this.buttons.className = "strip-button-layer";
    this.body.append(this.svg, this.buttons);
    this.root.append(spacer, this.body);

    for (const view of [sideView, resultView]) {
      view.scrollDOM.addEventListener("scroll", () => this.schedule(), { passive: true });
      new ResizeObserver(() => this.schedule()).observe(view.dom);
    }
  }

  /** Sets the chunks to draw and redraws. */
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
    const side = this.sideView;
    const result = this.resultView;
    const bodyTop = this.body.getBoundingClientRect().top;
    const width = this.body.clientWidth;
    const height = this.body.clientHeight;
    const sideOffset = side.documentTop - bodyTop;
    const resultOffset = result.documentTop - bodyTop;
    const rdoc = result.state.doc;

    const paths: SVGPathElement[] = [];
    const groups: HTMLElement[] = [];
    this.tracked.forEach((t, index) => {
      if (!changedOn(t.chunk, this.side)) return;
      const range = t.chunk[this.side];
      const s: Span = {
        top: sideOffset + lineTop(side, range.start),
        bottom: sideOffset + lineTop(side, range.end),
      };
      const rTop = result.lineBlockAt(t.from).top;
      const r: Span = {
        top: resultOffset + rTop,
        bottom: resultOffset + (t.to > t.from ? result.lineBlockAt(Math.min(t.to - 1, rdoc.length)).bottom : rTop),
      };
      if (!isVisible(s, r, height)) return;

      const path = document.createElementNS(SVG, "path");
      path.setAttribute("d", connectorPath(width, s, r, this.side === "local"));
      const cls = `md-conn md-${sideClass(t.chunk, this.side)}${isResolved(t) ? " md-resolved" : ""}`;
      path.setAttribute("class", cls);
      paths.push(path);

      if (t[this.side] === "pending" && s.top + 1 >= 0 && s.top <= height) {
        const block = side.lineBlockAt(side.state.doc.line(Math.min(range.start + 1, side.state.doc.lines)).from);
        groups.push(this.group(index, s.top, block.height));
      }
    });
    this.svg.setAttribute("viewBox", `0 0 ${width} ${height}`);
    this.svg.replaceChildren(...paths);
    this.buttons.replaceChildren(...groups);
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
