// Connector shapes between a side pane and the Result pane. DOM-free so it
// runs under node --test; gutter.ts draws them into the strips.

/** Vertical extent of a chunk in one pane, in strip coordinates. */
export interface Span {
  top: number;
  bottom: number;
}

/**
 * SVG path of a filled band from the side chunk to the result chunk across
 * a strip `width` wide. The side pane is on the left of the strip when
 * sideOnLeft (the Local strip), otherwise on the right (the Remote strip).
 * Edges are cubic curves with horizontal tangents at both panes.
 */
export function connectorPath(width: number, side: Span, result: Span, sideOnLeft: boolean): string {
  const [x0, x1] = sideOnLeft ? [0, width] : [width, 0];
  const [a, b] = sideOnLeft ? [side, result] : [result, side];
  const [left, right] = sideOnLeft ? [x0, x1] : [x1, x0];
  const mid = width / 2;
  const n = (v: number) => Math.round(v * 10) / 10;
  return (
    `M${n(left)},${n(a.top)} C${n(mid)},${n(a.top)} ${n(mid)},${n(b.top)} ${n(right)},${n(b.top)} ` +
    `L${n(right)},${n(b.bottom)} C${n(mid)},${n(b.bottom)} ${n(mid)},${n(a.bottom)} ${n(left)},${n(a.bottom)} Z`
  );
}

/** Whether a connector touches the visible strip height at all. */
export function isVisible(side: Span, result: Span, height: number): boolean {
  const top = Math.min(side.top, result.top);
  const bottom = Math.max(side.bottom, result.bottom);
  return bottom >= 0 && top <= height;
}
