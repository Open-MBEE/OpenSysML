// The canvas's geometry primitives, a leaf the layout and its router share without importing each other.

import type { RenderPoint } from "../protocol";

export interface Box {
  x: number;
  y: number;
  width: number;
  height: number;
}

export type Side = "north" | "east" | "south" | "west";

export interface PortPosition {
  side: Side;
  offset: number;
}

/** Between slots of one grid, and between a container's border and its slots. */
export const GAP = 32;
export const PORT_SIZE = 10;

/** portCenter puts a port on the border of its current box. */
export function portCenter(box: Box, port: PortPosition): RenderPoint {
  if (port.side === "north" || port.side === "south") {
    return {
      x: box.x + box.width * port.offset,
      y: port.side === "north" ? box.y : box.y + box.height,
    };
  }
  return {
    x: port.side === "west" ? box.x : box.x + box.width,
    y: box.y + box.height * port.offset,
  };
}

/** portFace moves the edge endpoint half a port square outward from its border. */
export function portFace(box: Box, port: PortPosition): RenderPoint {
  const center = portCenter(box, port);
  switch (port.side) {
    case "north":
      return { x: center.x, y: center.y - PORT_SIZE / 2 };
    case "east":
      return { x: center.x + PORT_SIZE / 2, y: center.y };
    case "south":
      return { x: center.x, y: center.y + PORT_SIZE / 2 };
    case "west":
      return { x: center.x - PORT_SIZE / 2, y: center.y };
  }
}

/** portBox centers the visible square on its node's border. */
export function portBox(box: Box, port: PortPosition): Box {
  const center = portCenter(box, port);
  return { x: center.x - PORT_SIZE / 2, y: center.y - PORT_SIZE / 2, width: PORT_SIZE, height: PORT_SIZE };
}

/** How a gesture's positions are written, so the model reads back in whole pixels. */
export function snap(value: number): number {
  return Math.round(value);
}
