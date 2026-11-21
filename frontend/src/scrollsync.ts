import type { EditorView } from "@codemirror/view";
import { boundaries, mapPosition, type Segments } from "./alignment.ts";

/**
 * Keeps several editors scrolled to matching chunks. When one pane scrolls
 * (by the user or by navigation), the point at the middle of its viewport is
 * mapped segment by segment to the other panes, which are scrolled so the
 * mapped point sits at their middle too. Horizontal scrolling is mirrored 1:1.
 */
export class ScrollSync {
  /** Scroll positions we set ourselves, so their scroll events are ignored. */
  private readonly expected = new Map<HTMLElement, number>();
  private segments: Segments | null = null;

  constructor(
    private readonly views: readonly EditorView[],
    private readonly computeSegments: () => Segments,
  ) {
    views.forEach((view, i) => {
      view.scrollDOM.addEventListener("scroll", () => this.onScroll(i), { passive: true });
    });
  }

  /** Call when the documents or chunk positions change. */
  invalidate(): void {
    this.segments = null;
  }

  /** Re-aligns every pane to pane `from` (e.g. after a resize). */
  alignTo(from: number): void {
    this.sync(from);
  }

  private onScroll(i: number): void {
    const el = this.views[i].scrollDOM;
    const want = this.expected.get(el);
    if (want !== undefined) {
      this.expected.delete(el);
      if (Math.abs(el.scrollTop - want) <= 1) return; // our own scroll
    }
    this.sync(i);
  }

  private sync(from: number): void {
    const segments = (this.segments ??= this.computeSegments());
    const src = this.views[from];
    const srcBounds = boundaries(segments, from, (line) => lineTop(src, line));
    const anchor = src.scrollDOM.scrollTop - docOffset(src) + src.scrollDOM.clientHeight / 2;

    this.views.forEach((dst, i) => {
      if (i === from) return;
      const dstBounds = boundaries(segments, i, (line) => lineTop(dst, line));
      const y = mapPosition(srcBounds, dstBounds, anchor);
      const el = dst.scrollDOM;
      const max = el.scrollHeight - el.clientHeight;
      const top = Math.min(Math.max(y + docOffset(dst) - el.clientHeight / 2, 0), Math.max(max, 0));
      if (Math.abs(el.scrollTop - top) >= 1) {
        this.expected.set(el, top);
        el.scrollTop = top;
      }
      if (el.scrollLeft !== src.scrollDOM.scrollLeft) el.scrollLeft = src.scrollDOM.scrollLeft;
    });
  }
}

/** Top of 0-based line index `line`, in document coordinates. */
function lineTop(view: EditorView, line: number): number {
  const doc = view.state.doc;
  if (line < doc.lines) return view.lineBlockAt(doc.line(line + 1).from).top;
  return view.lineBlockAt(doc.length).bottom;
}

/** Distance from the top of the scrollable content to the document's top. */
function docOffset(view: EditorView): number {
  const el = view.scrollDOM;
  return view.documentTop - el.getBoundingClientRect().top + el.scrollTop;
}
