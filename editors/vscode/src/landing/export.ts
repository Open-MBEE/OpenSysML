import type { Box } from "../webview/layout";

const SVG_NS = "http://www.w3.org/2000/svg";

/** Room left around the diagram in an export, in layout units. */
export const EXPORT_PAD = 24;
/** Device pixels per layout unit in an exported PNG. */
export const EXPORT_SCALE = 2;
/** A canvas edge longer than this fails to allocate in most browsers. */
export const MAX_RASTER_EDGE = 8192;

/** The styles the page's stylesheet gives the diagram that an export must carry along. */
export const INLINED_STYLES = [
  "display",
  "visibility",
  "opacity",
  "fill",
  "fill-opacity",
  "stroke",
  "stroke-opacity",
  "stroke-width",
  "stroke-dasharray",
  "stroke-linecap",
  "stroke-linejoin",
  "font-family",
  "font-size",
  "font-weight",
  "font-style",
  "letter-spacing",
  "text-anchor",
  "dominant-baseline",
  "paint-order",
] as const;

export interface RasterSize {
  width: number;
  height: number;
  /** Device pixels per layout unit, lowered from EXPORT_SCALE when the frame is very large. */
  scale: number;
}

/** exportFrame is the diagram's extent padded on every side. */
export function exportFrame(layout: { origin: { x: number; y: number }; width: number; height: number }, pad = EXPORT_PAD): Box {
  return {
    x: layout.origin.x - pad,
    y: layout.origin.y - pad,
    width: layout.width + 2 * pad,
    height: layout.height + 2 * pad,
  };
}

/** rasterSize is the frame at `scale` pixels per unit, shrunk to fit within `maxEdge`. */
export function rasterSize(frame: Box, scale = EXPORT_SCALE, maxEdge = MAX_RASTER_EDGE): RasterSize {
  const longest = Math.max(frame.width, frame.height);
  const fitted = longest * scale > maxEdge ? maxEdge / longest : scale;
  return {
    width: Math.max(1, Math.round(frame.width * fitted)),
    height: Math.max(1, Math.round(frame.height * fitted)),
    scale: fitted,
  };
}

interface Styled {
  getPropertyValue(name: string): string;
}

// inlineStyles writes each INLINED_STYLES value the page computes for an element of `live` onto
// the matching element of `clone`, which has the same shape, so the clone draws alike without
// the page's stylesheet.
export function inlineStyles(live: Element, clone: Element, styleOf: (element: Element) => Styled): void {
  const style = styleOf(live);
  const declarations: string[] = [];
  for (const name of INLINED_STYLES) {
    const value = style.getPropertyValue(name);
    if (value !== "") {
      declarations.push(`${name}: ${value}`);
    }
  }
  if (declarations.length > 0) {
    clone.setAttribute("style", declarations.join("; "));
  }
  const liveChildren = live.children;
  const cloneChildren = clone.children;
  for (let i = 0; i < liveChildren.length && i < cloneChildren.length; i++) {
    inlineStyles(liveChildren[i], cloneChildren[i], styleOf);
  }
}

/** The hero's gradient, which the drawing's white wires and labels are made to sit on. */
export interface Backdrop {
  from: string;
  to: string;
}

/** exportSvg is a standalone SVG of `content`'s drawing (less what `skip` picks out), framed by `frame` on `backdrop`. */
export function exportSvg(
  content: SVGGElement,
  frame: Box,
  size: RasterSize,
  backdrop: Backdrop,
  skip: (node: Element) => boolean,
): SVGSVGElement {
  const svg = document.createElementNS(SVG_NS, "svg");
  svg.setAttribute("xmlns", SVG_NS);
  svg.setAttribute("width", String(size.width));
  svg.setAttribute("height", String(size.height));
  svg.setAttribute("viewBox", `${frame.x} ${frame.y} ${frame.width} ${frame.height}`);
  const defs = document.createElementNS(SVG_NS, "defs");
  const gradient = document.createElementNS(SVG_NS, "linearGradient");
  gradient.id = "osml-export-backdrop";
  for (const [name, value] of Object.entries({ x1: "0", y1: "0", x2: "1", y2: "0.6" })) {
    gradient.setAttribute(name, value);
  }
  for (const [offset, color] of [["0", backdrop.from], ["1", backdrop.to]]) {
    const stop = document.createElementNS(SVG_NS, "stop");
    stop.setAttribute("offset", offset);
    stop.setAttribute("stop-color", color);
    gradient.append(stop);
  }
  defs.append(gradient);
  const background = document.createElementNS(SVG_NS, "rect");
  background.setAttribute("x", String(frame.x));
  background.setAttribute("y", String(frame.y));
  background.setAttribute("width", String(frame.width));
  background.setAttribute("height", String(frame.height));
  background.setAttribute("fill", `url(#${gradient.id})`);
  svg.append(defs, background);
  for (const child of content.children) {
    if (skip(child)) {
      continue;
    }
    const clone = child.cloneNode(true) as Element;
    inlineStyles(child, clone, (element) => getComputedStyle(element));
    svg.append(clone);
  }
  return svg;
}

/** rasterize draws `svg` onto a canvas of its own width and height and encodes it as a PNG. */
export function rasterize(svg: SVGSVGElement): Promise<Blob> {
  const width = Number(svg.getAttribute("width"));
  const height = Number(svg.getAttribute("height"));
  const source = new XMLSerializer().serializeToString(svg);
  const url = URL.createObjectURL(new Blob([source], { type: "image/svg+xml;charset=utf-8" }));
  return new Promise<Blob>((resolve, reject) => {
    const image = new Image(width, height);
    image.onload = () => {
      URL.revokeObjectURL(url);
      const canvas = document.createElement("canvas");
      canvas.width = width;
      canvas.height = height;
      const context = canvas.getContext("2d");
      if (!context) {
        reject(new Error("the browser gave no 2D canvas"));
        return;
      }
      context.drawImage(image, 0, 0, width, height);
      canvas.toBlob((blob) => (blob ? resolve(blob) : reject(new Error("the browser could not encode a PNG"))), "image/png");
    };
    image.onerror = () => {
      URL.revokeObjectURL(url);
      reject(new Error("the browser could not draw the diagram's SVG"));
    };
    image.src = url;
  });
}

/** download offers `blob` to the visitor as a file named `filename`. */
export function download(blob: Blob, filename: string): void {
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  link.rel = "noopener";
  document.body.append(link);
  link.click();
  link.remove();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
