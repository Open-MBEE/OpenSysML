// The panel's viewport: how far the canvas is zoomed and where it is scrolled to.
// Zoom scales the SVG's drawn size and leaves its viewBox alone, so the canvas's
// own coordinates — what hit-testing and drags read through getScreenCTM — are
// untouched; panning is the container's scroll position.

export const ZOOM_MIN = 0.1;
export const ZOOM_MAX = 8;
/** The factor one keyboard or button step zooms by. */
export const ZOOM_STEP = 1.25;
/** How much a wheel notch zooms per 100 units of delta. */
const WHEEL_SENSITIVITY = 0.002;

/** A viewport: the zoom factor and the container's scroll offsets, in screen pixels. */
export interface Viewport {
  zoom: number;
  scrollLeft: number;
  scrollTop: number;
}

/** A point of the container's client area, in screen pixels from its top-left corner. */
export interface ClientPoint {
  x: number;
  y: number;
}

export function clampZoom(zoom: number): number {
  if (!Number.isFinite(zoom) || zoom <= 0) {
    return 1;
  }
  return Math.min(ZOOM_MAX, Math.max(ZOOM_MIN, zoom));
}

/** wheelFactor turns a wheel delta into a zoom factor: scrolling up (negative delta) zooms in. */
export function wheelFactor(deltaY: number): number {
  return Math.exp(-deltaY * WHEEL_SENSITIVITY);
}

/**
 * zoomAbout zooms the viewport by a factor keeping the canvas point under `at`
 * where it is on screen: the point's canvas coordinates are (scroll + at) / zoom,
 * and the new scroll puts them back under `at` at the new zoom.
 */
export function zoomAbout(viewport: Viewport, factor: number, at: ClientPoint): Viewport {
  const zoom = clampZoom(viewport.zoom * factor);
  const ratio = zoom / viewport.zoom;
  return {
    zoom,
    scrollLeft: Math.max(0, (viewport.scrollLeft + at.x) * ratio - at.x),
    scrollTop: Math.max(0, (viewport.scrollTop + at.y) * ratio - at.y),
  };
}

/** fitZoom is the zoom at which a canvas of the size fits a viewport of the size, within the zoom range. */
export function fitZoom(
  canvas: { width: number; height: number },
  viewport: { width: number; height: number },
): number {
  if (canvas.width <= 0 || canvas.height <= 0 || viewport.width <= 0 || viewport.height <= 0) {
    return 1;
  }
  return clampZoom(Math.min(viewport.width / canvas.width, viewport.height / canvas.height));
}

/** canvasSize is the size an SVG's viewBox states, which zoom scales from. */
export function canvasSize(svg: SVGSVGElement): { width: number; height: number } | undefined {
  const parts = (svg.getAttribute("viewBox") ?? "").trim().split(/[\s,]+/).map(Number);
  if (parts.length !== 4 || parts.some((part) => !Number.isFinite(part))) {
    return undefined;
  }
  return { width: parts[2], height: parts[3] };
}

/** sizeCanvas draws an SVG at its viewBox's size scaled by the zoom; the viewBox itself stays, so canvas coordinates do not move. */
export function sizeCanvas(svg: SVGSVGElement, zoom: number): void {
  const size = canvasSize(svg);
  if (!size) {
    return;
  }
  svg.setAttribute("width", String(size.width * zoom));
  svg.setAttribute("height", String(size.height * zoom));
}

/** readViewport reads a saved viewport back, taking defaults for anything missing or malformed. */
export function readViewport(saved: unknown): Viewport {
  const record = (saved ?? {}) as Partial<Record<keyof Viewport, unknown>>;
  const number = (value: unknown, fallback: number): number =>
    typeof value === "number" && Number.isFinite(value) && value >= 0 ? value : fallback;
  return {
    zoom: clampZoom(typeof record.zoom === "number" ? record.zoom : 1),
    scrollLeft: number(record.scrollLeft, 0),
    scrollTop: number(record.scrollTop, 0),
  };
}

/** zoomLabel is the zoom as the toolbar shows it. */
export function zoomLabel(zoom: number): string {
  return `${Math.round(zoom * 100)}%`;
}
