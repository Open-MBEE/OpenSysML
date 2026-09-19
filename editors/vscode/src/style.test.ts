import assert from "node:assert/strict";
import { test } from "node:test";

import { DEFAULT_STYLE, isStyle, paletteOf, PALETTES, pilotLook, STYLE_LABELS, STYLES, styleOf } from "./style";

test("the styles are the theme, the pilot's black and white, and one per server palette, each labelled", () => {
  assert.deepEqual(STYLES.slice(0, 2), ["theme", "pilot"]);
  assert.deepEqual(STYLES.slice(2), [...PALETTES]);
  assert.deepEqual(Object.keys(STYLE_LABELS).sort(), [...STYLES].sort());
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
  assert.deepEqual(STYLES.map(pilotLook), [false, true, ...PALETTES.map(() => true)]);
});
