import assert from "node:assert/strict";
import { test } from "node:test";

import { EXPORT_PAD, EXPORT_SCALE, INLINED_STYLES, MAX_RASTER_EDGE, exportFrame, inlineStyles, rasterSize } from "./export";

test("exportFrame pads the diagram's extent on every side", () => {
  const frame = exportFrame({ origin: { x: 10, y: -5 }, width: 300, height: 120 });
  assert.deepEqual(frame, { x: 10 - EXPORT_PAD, y: -5 - EXPORT_PAD, width: 300 + 2 * EXPORT_PAD, height: 120 + 2 * EXPORT_PAD });
  assert.deepEqual(exportFrame({ origin: { x: 0, y: 0 }, width: 10, height: 10 }, 0), { x: 0, y: 0, width: 10, height: 10 });
});

test("rasterSize scales by EXPORT_SCALE until an edge would pass MAX_RASTER_EDGE", () => {
  assert.deepEqual(rasterSize({ x: 0, y: 0, width: 400, height: 250 }), { width: 800, height: 500, scale: EXPORT_SCALE });
  const huge = rasterSize({ x: 0, y: 0, width: 10_000, height: 2_000 });
  assert.equal(huge.width, MAX_RASTER_EDGE);
  assert.equal(huge.height, Math.round((2_000 * MAX_RASTER_EDGE) / 10_000));
  assert.ok(huge.scale < EXPORT_SCALE);
  assert.deepEqual(rasterSize({ x: 0, y: 0, width: 0.1, height: 0.1 }), { width: 1, height: 1, scale: EXPORT_SCALE });
});

interface FakeElement {
  tag: string;
  computed: Record<string, string>;
  attrs: Record<string, string>;
  children: FakeElement[];
  setAttribute(name: string, value: string): void;
}

const fake = (tag: string, computed: Record<string, string>, children: FakeElement[] = []): FakeElement => ({
  tag,
  computed,
  attrs: {},
  children,
  setAttribute(name, value) {
    this.attrs[name] = value;
  },
});

const bare = (tag: string, children: FakeElement[] = []): FakeElement => fake(tag, {}, children);

test("inlineStyles writes the computed styles of each live element onto the clone of the same shape", () => {
  const live = fake("g", { opacity: "0.28" }, [
    fake("rect", { fill: "rgb(227, 242, 253)", stroke: "rgb(21, 101, 192)", "stroke-width": "1px" }),
    fake("text", { "font-family": "Helvetica, Arial, sans-serif", "font-weight": "700", filter: "drop-shadow(0 0 1px #fff)" }),
  ]);
  const clone = bare("g", [bare("rect"), bare("text")]);
  inlineStyles(live as unknown as Element, clone as unknown as Element, (element) => {
    const computed = (element as unknown as FakeElement).computed;
    return { getPropertyValue: (name: string) => computed[name] ?? "" };
  });
  assert.equal(clone.attrs.style, "opacity: 0.28");
  assert.equal(clone.children[0].attrs.style, "fill: rgb(227, 242, 253); stroke: rgb(21, 101, 192); stroke-width: 1px");
  assert.equal(clone.children[1].attrs.style, "font-family: Helvetica, Arial, sans-serif; font-weight: 700");
  assert.ok(!(INLINED_STYLES as readonly string[]).includes("filter"), "a focus glow is page chrome, not part of the drawing");
});

test("inlineStyles leaves an element alone when nothing is computed for it", () => {
  const live = fake("g", {}, [fake("rect", {})]);
  const clone = bare("g", [bare("rect")]);
  inlineStyles(live as unknown as Element, clone as unknown as Element, () => ({ getPropertyValue: () => "" }));
  assert.equal(clone.attrs.style, undefined);
  assert.equal(clone.children[0].attrs.style, undefined);
});
