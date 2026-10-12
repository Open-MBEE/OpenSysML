import assert from "node:assert/strict";
import { test } from "node:test";

import type { Box } from "../webview/layout";
import {
  MAX_ZOOM,
  ZOOM_STEP,
  clampedView,
  pannedView,
  pinchedView,
  refittedView,
  zoomOf,
  zoomedView,
  type View,
} from "./zoom";

const fit: View = { x: 100, y: 50, scale: 0.5 };
const stage: Box = { x: 100, y: 50, width: 800, height: 400 };

const close = (actual: View, expected: View): void => {
  for (const key of ["x", "y", "scale"] as const) {
    assert.ok(Math.abs(actual[key] - expected[key]) < 1e-9, `${key}: ${actual[key]} != ${expected[key]}`);
  }
};

test("the fitted view is already clamped, and zooming below it snaps back to it", () => {
  close(clampedView(fit, fit, stage), fit);
  close(zoomedView(fit, fit, stage, fit.scale / ZOOM_STEP, { x: 500, y: 250 }), fit);
  assert.equal(zoomOf(fit, fit), 1);
});

test("zooming about a point keeps the layout point under it where it was", () => {
  const focal = { x: 300, y: 100 };
  const before = { x: (focal.x - fit.x) / fit.scale, y: (focal.y - fit.y) / fit.scale };
  const view = zoomedView(fit, fit, stage, fit.scale * 2, focal);
  assert.equal(zoomOf(view, fit), 2);
  close({ x: before.x * view.scale + view.x, y: before.y * view.scale + view.y, scale: view.scale }, { ...focal, scale: view.scale });
});

test("zoom stops at MAX_ZOOM", () => {
  const view = zoomedView(fit, fit, stage, fit.scale * 100, { x: 500, y: 250 });
  assert.equal(zoomOf(view, fit), MAX_ZOOM);
});

test("panning cannot pull the diagram's edge inside the stage", () => {
  const zoomed = zoomedView(fit, fit, stage, fit.scale * 2, { x: 500, y: 250 });
  const left = pannedView(zoomed, fit, stage, 10_000, 10_000);
  // At 2x, the fitted window is 1600x800 and must still cover the 800x400 stage.
  close(left, { x: stage.x, y: stage.y, scale: 1 });
  const right = pannedView(zoomed, fit, stage, -10_000, -10_000);
  close(right, { x: stage.x + stage.width - 1600, y: stage.y + stage.height - 800, scale: 1 });
  const small = pannedView(zoomed, fit, stage, -10, 5);
  close(small, { x: zoomed.x - 10, y: zoomed.y + 5, scale: 1 });
});

test("a pinch zooms by the spread of the fingers and slides with their midpoint", () => {
  const from: [typeof fit, typeof fit] = [{ ...fit, x: 400, y: 250 }, { ...fit, x: 600, y: 250 }];
  const to: [typeof fit, typeof fit] = [{ ...fit, x: 290, y: 240 }, { ...fit, x: 690, y: 240 }];
  const view = pinchedView(fit, fit, stage, from, to);
  assert.equal(zoomOf(view, fit), 2);
  // The layout point under the first midpoint (500, 250) is now drawn under the second (490, 240).
  const at = { x: (500 - fit.x) / fit.scale, y: (250 - fit.y) / fit.scale };
  close({ x: at.x * view.scale + view.x, y: at.y * view.scale + view.y, scale: 1 }, { x: 490, y: 240, scale: 1 });
});

test("a pinch of zero spread does not divide by zero", () => {
  const same = { x: 500, y: 250 };
  close(pinchedView(fit, fit, stage, [same, same], [same, same]), fit);
});

test("refitting keeps the zoom and the point at the stage's centre", () => {
  const zoomed = zoomedView(fit, fit, stage, fit.scale * 2, { x: 300, y: 100 });
  const newFit: View = { x: 50, y: 50, scale: 0.25 };
  const newStage: Box = { x: 50, y: 50, width: 400, height: 200 };
  const view = refittedView(zoomed, fit, stage, newFit, newStage);
  assert.equal(zoomOf(view, newFit), 2);
  const centre = { x: (500 - zoomed.x) / zoomed.scale, y: (250 - zoomed.y) / zoomed.scale };
  close({ x: centre.x * view.scale + view.x, y: centre.y * view.scale + view.y, scale: 1 }, { x: 250, y: 150, scale: 1 });
});

test("refitting at zoom 1 is the new fit", () => {
  const newFit: View = { x: 50, y: 50, scale: 0.25 };
  const newStage: Box = { x: 50, y: 50, width: 400, height: 200 };
  close(refittedView(fit, fit, stage, newFit, newStage), newFit);
});
