import assert from "node:assert/strict";
import { test } from "node:test";

import { JSDOM } from "jsdom";

import {
  canvasSize,
  clampZoom,
  fitZoom,
  readViewport,
  sizeCanvas,
  wheelFactor,
  ZOOM_MAX,
  ZOOM_MIN,
  zoomAbout,
  zoomLabel,
} from "./viewport";

const dom = new JSDOM("<!DOCTYPE html><body></body>");
globalThis.document = dom.window.document;

test("clampZoom keeps the zoom in range and takes 1 for nonsense", () => {
  assert.equal(clampZoom(0.5), 0.5);
  assert.equal(clampZoom(0), 1);
  assert.equal(clampZoom(-2), 1);
  assert.equal(clampZoom(Number.NaN), 1);
  assert.equal(clampZoom(1e9), ZOOM_MAX);
  assert.equal(clampZoom(1e-9), ZOOM_MIN);
});

test("zoomAbout keeps the canvas point under the pointer where it is on screen", () => {
  // The pointer is 100 px right of and 50 px below the viewport's corner; the viewport is
  // scrolled by (200, 100) at zoom 1, so the canvas point under it is (300, 150).
  const before = { zoom: 1, scrollLeft: 200, scrollTop: 100 };
  const after = zoomAbout(before, 2, { x: 100, y: 50 });
  assert.equal(after.zoom, 2);
  // At zoom 2 that canvas point is drawn at (600, 300); under a pointer at (100, 50) the scroll is (500, 250).
  assert.deepEqual([after.scrollLeft, after.scrollTop], [500, 250]);
  const canvasPoint = { x: (after.scrollLeft + 100) / after.zoom, y: (after.scrollTop + 50) / after.zoom };
  assert.deepEqual(canvasPoint, { x: 300, y: 150 });
});

test("zoomAbout clamps at the range's ends and never scrolls to a negative offset", () => {
  const atMax = zoomAbout({ zoom: ZOOM_MAX, scrollLeft: 10, scrollTop: 10 }, 2, { x: 0, y: 0 });
  assert.deepEqual(atMax, { zoom: ZOOM_MAX, scrollLeft: 10, scrollTop: 10 });
  const out = zoomAbout({ zoom: 1, scrollLeft: 0, scrollTop: 0 }, 0.5, { x: 100, y: 100 });
  assert.equal(out.zoom, 0.5);
  assert.deepEqual([out.scrollLeft, out.scrollTop], [0, 0]);
});

test("wheelFactor zooms in on a negative delta and out on a positive one, symmetrically", () => {
  assert.ok(wheelFactor(-100) > 1);
  assert.ok(wheelFactor(100) < 1);
  assert.ok(Math.abs(wheelFactor(-100) * wheelFactor(100) - 1) < 1e-12);
  assert.equal(wheelFactor(0), 1);
});

test("fitZoom fits the canvas's longer side into the viewport", () => {
  assert.equal(fitZoom({ width: 2000, height: 500 }, { width: 1000, height: 1000 }), 0.5);
  assert.equal(fitZoom({ width: 100, height: 400 }, { width: 1000, height: 800 }), 2);
  assert.equal(fitZoom({ width: 0, height: 400 }, { width: 1000, height: 800 }), 1);
  assert.equal(fitZoom({ width: 100, height: 100 }, { width: 0, height: 800 }), 1);
  assert.equal(fitZoom({ width: 1, height: 1 }, { width: 10000, height: 10000 }), ZOOM_MAX);
});

test("sizeCanvas scales the drawn size from the viewBox and leaves the viewBox alone", () => {
  const svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
  svg.setAttribute("viewBox", "-24 -24 400 300");
  svg.setAttribute("width", "400");
  svg.setAttribute("height", "300");
  assert.deepEqual(canvasSize(svg), { width: 400, height: 300 });
  sizeCanvas(svg, 1.5);
  assert.deepEqual([svg.getAttribute("width"), svg.getAttribute("height")], ["600", "450"]);
  assert.equal(svg.getAttribute("viewBox"), "-24 -24 400 300");
  sizeCanvas(svg, 1);
  assert.deepEqual([svg.getAttribute("width"), svg.getAttribute("height")], ["400", "300"]);
  svg.setAttribute("viewBox", "garbage");
  assert.equal(canvasSize(svg), undefined);
  sizeCanvas(svg, 2);
  assert.equal(svg.getAttribute("width"), "400");
});

test("readViewport restores a saved viewport and defaults what is missing or malformed", () => {
  assert.deepEqual(readViewport(undefined), { zoom: 1, scrollLeft: 0, scrollTop: 0 });
  assert.deepEqual(readViewport({ zoom: 2, scrollLeft: 30, scrollTop: 40 }), { zoom: 2, scrollLeft: 30, scrollTop: 40 });
  assert.deepEqual(readViewport({ zoom: "big", scrollLeft: -5, scrollTop: Number.NaN }), { zoom: 1, scrollLeft: 0, scrollTop: 0 });
  assert.equal(readViewport({ zoom: 100 }).zoom, ZOOM_MAX);
});

test("zoomLabel rounds to a percentage", () => {
  assert.equal(zoomLabel(1), "100%");
  assert.equal(zoomLabel(0.4999), "50%");
  assert.equal(zoomLabel(2.5), "250%");
});
