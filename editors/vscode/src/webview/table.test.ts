import assert from "node:assert/strict";
import { test } from "node:test";

import { JSDOM } from "jsdom";

import type { RenderResult } from "../protocol";
import { tableOf } from "./table";

// The table is drawn with the page's document, as it is in the webview.
const dom = new JSDOM("<!DOCTYPE html><body></body>");
globalThis.document = dom.window.document;

const origin = { uri: "file:///m.sysml", range: { start: { line: 0, character: 0 }, end: { line: 0, character: 4 } }, digest: "d0" };

const result: RenderResult = {
  view: "",
  kind: "table",
  stated: "",
  form: "markdown",
  artifact: "",
  nodes: [],
  edges: [],
  columns: ["Element", "Kind", "Type", "Declared in"],
  rows: [
    { cells: ["M::Wheel", "part def", "", ""], origin },
    { cells: ["<b>x</b>", "part", "Wheel", "M::Car"] },
  ],
  notices: [],
  version: 3,
};

test("tableOf draws a header per column and a row per element, located rows marked", () => {
  const table = tableOf(result) as HTMLTableElement;
  assert.equal(table.tagName, "TABLE");
  assert.equal(table.className, "opensysml-table");
  const heads = [...table.querySelectorAll("thead th")].map((th) => th.textContent);
  assert.deepEqual(heads, ["Element", "Kind", "Type", "Declared in"]);
  const rows = [...table.querySelectorAll<HTMLTableRowElement>("tbody tr")];
  assert.equal(rows.length, 2);
  assert.equal(rows[0].classList.contains("located"), true);
  assert.equal(rows[0].dataset.opensysmlRow, "0");
  assert.equal(rows[1].classList.contains("located"), false);
  assert.equal(rows[1].dataset.opensysmlRow, "1");
  // Cells are text, never markup: a markup-looking cell is written literally.
  const cells = [...rows[1].querySelectorAll("td")].map((td) => td.textContent);
  assert.deepEqual(cells, ["<b>x</b>", "part", "Wheel", "M::Car"]);
  assert.equal(rows[1].querySelector("b"), null);
});

test("tableOf pads a ragged row to the column count and never throws", () => {
  const ragged: RenderResult = { ...result, rows: [{ cells: ["a", "b", "c"], origin }] };
  const table = tableOf(ragged) as HTMLTableElement;
  const cells = [...table.querySelectorAll("tbody tr td")].map((td) => td.textContent);
  assert.deepEqual(cells, ["a", "b", "c", ""]);
});

test("tableOf shows an empty state rather than a blank panel", () => {
  const empty = tableOf({ ...result, rows: [] });
  assert.equal(empty.tagName, "P");
  assert.equal(empty.className, "empty");
  assert.equal(empty.textContent, "No elements to list.");
});

test("tableOf derives an empty header as wide as the widest row when columns are absent", () => {
  const noColumns: RenderResult = {
    ...result,
    columns: undefined,
    rows: [
      { cells: ["a", "b"], origin },
      { cells: ["a", "b", "c", "d"] },
    ],
  };
  const table = tableOf(noColumns) as HTMLTableElement;
  const heads = [...table.querySelectorAll("thead th")].map((th) => th.textContent);
  assert.deepEqual(heads, ["", "", "", ""]);
});
