import assert from "node:assert/strict";
import { test } from "node:test";

import {
  cameoLook,
  DEFAULT_OPEN_IN,
  DEFAULT_ROUTING,
  DEFAULT_STYLE,
  DRAWING_STYLES,
  drawingStyleOf,
  isStyle,
  OPEN_INS,
  openInOf,
  paletteOf,
  PALETTES,
  pilotLook,
  ROUTINGS,
  routingOf,
  SHOW_SETTING,
  shownOf,
  STYLE_LABELS,
  STYLES,
  styleOf,
} from "./style";

test("the show setting lists the optional notation elements to draw, each once; anything else names none", () => {
  assert.equal(SHOW_SETTING, "opensysml.diagram.show");
  assert.deepEqual(shownOf(["edge-labels", "parameter-symbols", "edge-labels"]), ["edge-labels", "parameter-symbols"]);
  assert.deepEqual(shownOf(["all"]), ["all"]);
  assert.deepEqual(shownOf(["", 3, "typings"]), ["typings"]);
  assert.deepEqual(shownOf("edge-labels"), []);
  assert.deepEqual(shownOf(undefined), []);
});

test("the styles are the theme, the pilot's black and white, Cameo's look, and one per server palette, each labelled", () => {
  assert.deepEqual(STYLES.slice(0, 3), ["theme", "pilot", "cameo"]);
  assert.deepEqual(STYLES.slice(3), [...PALETTES]);
  assert.deepEqual([...DRAWING_STYLES], ["pilot", "cameo"]);
  assert.deepEqual(
    Object.keys(STYLE_LABELS).sort((a, b) => a.localeCompare(b)),
    [...STYLES].sort((a, b) => a.localeCompare(b)),
  );
  assert.equal(DEFAULT_STYLE, "theme");
});

test("a setting or saved state names a style or falls back to the default", () => {
  assert.equal(isStyle("okabe-ito"), true);
  assert.equal(isStyle("sysmlbw"), false);
  assert.equal(isStyle(undefined), false);
  assert.equal(styleOf("pilot"), "pilot");
  assert.equal(styleOf("no-such-look"), DEFAULT_STYLE);
  assert.equal(styleOf(3), DEFAULT_STYLE);
});

test("only a palette style asks the server for a palette; every style but the theme draws the pilot look", () => {
  assert.equal(paletteOf("theme"), undefined);
  assert.equal(paletteOf("pilot"), undefined);
  assert.equal(paletteOf("tol-bright"), "tol-bright");
  assert.deepEqual(STYLES.map(pilotLook), [false, true, true, ...PALETTES.map(() => true)]);
});

test("only the cameo style asks the server for a drawing style and draws Cameo's look", () => {
  assert.equal(drawingStyleOf("theme"), undefined);
  assert.equal(drawingStyleOf("pilot"), undefined);
  assert.equal(drawingStyleOf("okabe-ito"), undefined);
  assert.equal(drawingStyleOf("cameo"), "cameo");
  assert.equal(paletteOf("cameo"), undefined);
  assert.deepEqual(STYLES.filter(cameoLook), ["cameo"]);
});

test("the routing setting is the model's routes or the router's, and the model's for anything else", () => {
  assert.deepEqual([...ROUTINGS], ["model", "router"]);
  assert.equal(DEFAULT_ROUTING, "model");
  assert.equal(routingOf("router"), "router");
  assert.equal(routingOf("model"), "model");
  for (const value of [undefined, null, "", "elk", "Router", 1]) {
    assert.equal(routingOf(value), "model");
  }
});

test("the openIn setting is beside the source or a new window, and beside for anything else", () => {
  assert.deepEqual([...OPEN_INS], ["beside", "newWindow"]);
  assert.equal(DEFAULT_OPEN_IN, "beside");
  assert.equal(openInOf("newWindow"), "newWindow");
  for (const value of [undefined, "", "floating", "newwindow", true]) {
    assert.equal(openInOf(value), "beside");
  }
});
