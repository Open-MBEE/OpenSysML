import assert from "node:assert/strict";
import { test } from "node:test";

import {
  cameoLook,
  DEFAULT_STYLE,
  DRAWING_STYLES,
  drawingStyleOf,
  isStyle,
  paletteOf,
  PALETTES,
  pilotLook,
  STYLE_LABELS,
  STYLES,
  styleOf,
} from "./style";

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
