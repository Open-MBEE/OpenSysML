import type { RenderPoint } from "../protocol";
import type { Box } from "../webview/layout";

/** The content's transform in hero pixels: layout point p is drawn at p * scale + (x, y). */
export interface View {
  x: number;
  y: number;
  scale: number;
}

/** Zoom is relative to the fitted view: 1 shows the whole diagram, MAX_ZOOM is four times closer. */
export const MIN_ZOOM = 1;
export const MAX_ZOOM = 4;
/** One wheel notch or button press changes the zoom by this factor. */
export const ZOOM_STEP = 1.25;

export function zoomOf(view: View, fit: View): number {
  return view.scale / fit.scale;
}

// clampedView keeps the zoom within [MIN_ZOOM, MAX_ZOOM] and the pan where the part of the
// diagram the fitted view showed through `stage` still covers the stage: no empty gutters.
export function clampedView(view: View, fit: View, stage: Box): View {
  const scale = Math.min(fit.scale * MAX_ZOOM, Math.max(fit.scale * MIN_ZOOM, view.scale));
  const window = {
    x: (stage.x - fit.x) / fit.scale,
    y: (stage.y - fit.y) / fit.scale,
    width: stage.width / fit.scale,
    height: stage.height / fit.scale,
  };
  const clamp = (value: number, lo: number, hi: number): number => Math.min(hi, Math.max(lo, value));
  return {
    scale,
    x: clamp(view.x, stage.x + stage.width - (window.x + window.width) * scale, stage.x - window.x * scale),
    y: clamp(view.y, stage.y + stage.height - (window.y + window.height) * scale, stage.y - window.y * scale),
  };
}

// scaledAbout changes the scale while the layout point drawn under `focal` stays put.
function scaledAbout(view: View, scale: number, focal: RenderPoint): View {
  const factor = scale / view.scale;
  return {
    scale,
    x: focal.x - (focal.x - view.x) * factor,
    y: focal.y - (focal.y - view.y) * factor,
  };
}

/** zoomedView zooms to `scale` about `focal` (hero pixels), within the limits. */
export function zoomedView(view: View, fit: View, stage: Box, scale: number, focal: RenderPoint): View {
  const bounded = Math.min(fit.scale * MAX_ZOOM, Math.max(fit.scale * MIN_ZOOM, scale));
  return clampedView(scaledAbout(view, bounded, focal), fit, stage);
}

/** pannedView slides the view by a pixel delta, within the limits. */
export function pannedView(view: View, fit: View, stage: Box, dx: number, dy: number): View {
  return clampedView({ ...view, x: view.x + dx, y: view.y + dy }, fit, stage);
}

/** pinchedView is `start` zoomed and slid as two fingers moved from `from` to `to`. */
export function pinchedView(
  start: View,
  fit: View,
  stage: Box,
  from: [RenderPoint, RenderPoint],
  to: [RenderPoint, RenderPoint],
): View {
  const span = (pair: [RenderPoint, RenderPoint]): number => Math.hypot(pair[1].x - pair[0].x, pair[1].y - pair[0].y);
  const mid = (pair: [RenderPoint, RenderPoint]): RenderPoint => ({
    x: (pair[0].x + pair[1].x) / 2,
    y: (pair[0].y + pair[1].y) / 2,
  });
  const before = span(from);
  const scale = before === 0 ? start.scale : (start.scale * span(to)) / before;
  const zoomed = zoomedView(start, fit, stage, scale, mid(from));
  const m0 = mid(from);
  const m1 = mid(to);
  return pannedView(zoomed, fit, stage, m1.x - m0.x, m1.y - m0.y);
}

// refittedView keeps the zoom and the layout point at the stage's centre when the fitted view
// changes, as it does whenever the hero is resized.
export function refittedView(view: View, fit: View, stage: Box, newFit: View, newStage: Box): View {
  const zoom = zoomOf(view, fit);
  const centre = { x: stage.x + stage.width / 2, y: stage.y + stage.height / 2 };
  const at = { x: (centre.x - view.x) / view.scale, y: (centre.y - view.y) / view.scale };
  const scale = newFit.scale * zoom;
  return clampedView(
    {
      scale,
      x: newStage.x + newStage.width / 2 - at.x * scale,
      y: newStage.y + newStage.height / 2 - at.y * scale,
    },
    newFit,
    newStage,
  );
}
