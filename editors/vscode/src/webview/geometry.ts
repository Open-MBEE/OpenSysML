// The canvas's geometry primitives, a leaf the layout and its router share without importing each other.

export interface Box {
  x: number;
  y: number;
  width: number;
  height: number;
}

/** Between slots of one grid, and between a container's border and its slots. */
export const GAP = 32;

/** How a gesture's positions are written, so the model reads back in whole pixels. */
export function snap(value: number): number {
  return Math.round(value);
}
