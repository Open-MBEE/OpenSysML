import assert from "node:assert/strict";
import { test } from "node:test";

import {
  ALL_VIEWS,
  chooseView,
  declaredViewEntries,
  DEFAULT_PSEUDO_VIEW,
  expandChoice,
  impliedView,
  panelKey,
  pseudoViewEntries,
  rememberedView,
  viewAt,
  viewPickItems,
  viewTitle,
} from "./views";

test("declaredViewEntries lists the views the server declares, with why one cannot be drawn", () => {
  assert.deepEqual(declaredViewEntries(undefined), []);
  assert.deepEqual(
    declaredViewEntries({
      views: [
        {
          name: "M::Parts",
          kind: "interconnection",
          supported: true,
          range: { start: { line: 2, character: 1 }, end: { line: 4, character: 2 } },
        },
        { name: "M::Shape", kind: "geometry", supported: false, reason: "no geometry renderer" },
      ],
    }),
    [
      {
        value: "M::Parts",
        label: "M::Parts — interconnection",
        kind: "interconnection",
        supported: true,
        reason: undefined,
        range: { start: { line: 2, character: 1 }, end: { line: 4, character: 2 } },
      },
      { value: "M::Shape", label: "M::Shape — geometry", kind: "geometry", supported: false, reason: "no geometry renderer", range: undefined },
    ],
  );
});

test("pseudoViewEntries offers the server's pseudo-views, or the historical set", () => {
  assert.deepEqual(pseudoViewEntries(["#tree", "#sequence"]).map((entry) => [entry.value, entry.label]), [
    ["#tree", "Model tree (no view declared)"],
    ["#sequence", "Message sequence (no view declared)"],
  ]);
  assert.deepEqual(pseudoViewEntries(undefined).map((entry) => entry.value), ["#tree", "#interconnection", "#state", "#action", "#table"]);
  assert.ok(pseudoViewEntries(undefined).every((entry) => entry.supported));
});

test("impliedView is the sole drawable view, the model tree for none, and undefined for several", () => {
  const drawn = (name: string) => ({ value: name, label: name, supported: true });
  const undrawn = (name: string) => ({ value: name, label: name, supported: false, reason: "unsupported" });
  assert.equal(impliedView([]), DEFAULT_PSEUDO_VIEW);
  assert.equal(impliedView([undrawn("M::Shape")]), DEFAULT_PSEUDO_VIEW);
  assert.equal(impliedView([undrawn("M::Shape"), drawn("M::Parts")]), "M::Parts");
  assert.equal(impliedView([drawn("M::Parts"), drawn("M::States")]), undefined);
});

const range = (startLine: number, endLine: number) => ({
  start: { line: startLine, character: 1 },
  end: { line: endLine, character: 2 },
});
const parts = { value: "M::Parts", label: "M::Parts — interconnection", kind: "interconnection", supported: true, range: range(2, 4) };
const states = { value: "M::States", label: "M::States — state", kind: "state", supported: true, range: range(6, 8) };
const shape = { value: "M::Shape", label: "M::Shape — geometry", kind: "geometry", supported: false, reason: "no geometry renderer", range: range(10, 12) };
const pseudo = pseudoViewEntries(["#tree", "#table"]);

test("viewAt is the drawable view whose declaration contains the cursor", () => {
  assert.equal(viewAt([parts, states, shape], { line: 7, character: 0 }), "M::States");
  assert.equal(viewAt([parts, states, shape], { line: 2, character: 1 }), "M::Parts", "the declaration's first character is in it");
  assert.equal(viewAt([parts, states, shape], { line: 4, character: 2 }), "M::Parts", "its closing brace is in it");
  assert.equal(viewAt([parts, states, shape], { line: 5, character: 0 }), undefined, "the gap between declarations is in none");
  assert.equal(viewAt([parts, states, shape], { line: 11, character: 0 }), undefined, "a view that cannot be drawn is not chosen");
  assert.equal(viewAt([parts, states], undefined), undefined);
  assert.equal(viewAt([{ ...parts, range: undefined }, { ...states, range: undefined }], { line: 3, character: 0 }), undefined, "a server without ranges decides nothing");
});

test("rememberedView keeps a choice that still exists and drops one that is gone", () => {
  assert.equal(rememberedView([parts, states], pseudo, "M::States"), "M::States");
  assert.equal(rememberedView([parts, states], pseudo, "#table"), "#table");
  assert.equal(rememberedView([parts, states], pseudo, "M::Gone"), undefined);
  assert.equal(rememberedView([parts, shape], pseudo, "M::Shape"), undefined, "a view that is no longer drawable is dropped");
  assert.equal(rememberedView([parts, states], pseudo, undefined), undefined);
});

test("chooseView takes the implied view, then the cursor's, then the remembered one, then asks", () => {
  assert.deepEqual(chooseView([], pseudo, { line: 0, character: 0 }, "M::Parts"), { view: DEFAULT_PSEUDO_VIEW });
  assert.deepEqual(chooseView([parts, shape], pseudo, undefined, "M::Shape"), { view: "M::Parts" });
  assert.deepEqual(chooseView([parts, states], pseudo, { line: 7, character: 0 }, "M::Parts"), { view: "M::States" });
  assert.deepEqual(chooseView([parts, states], pseudo, { line: 5, character: 0 }, "M::Parts"), { view: "M::Parts" });
  assert.deepEqual(chooseView([parts, states], pseudo, { line: 5, character: 0 }, undefined), { stale: undefined });
  assert.deepEqual(chooseView([parts, states], pseudo, undefined, "M::Gone"), { stale: "M::Gone" });
});

test("viewPickItems lists drawable views by name and kind, then All views, then the pseudo-views", () => {
  const items = viewPickItems([parts, states, shape], pseudo);
  assert.deepEqual(items.map((item) => [item.label, item.detail ?? item.description, item.value]), [
    ["M::Parts", "interconnection", "M::Parts"],
    ["M::States", "state", "M::States"],
    ["All views", "Open the 2 drawable views, one panel each", ALL_VIEWS],
    ["Model tree (no view declared)", "#tree", "#tree"],
    ["Element table (no view declared)", "#table", "#table"],
  ]);
});

test("expandChoice opens every drawable view for All views and the one picked otherwise", () => {
  assert.deepEqual(expandChoice(ALL_VIEWS, [parts, shape, states]), ["M::Parts", "M::States"]);
  assert.deepEqual(expandChoice("M::States", [parts, states]), ["M::States"]);
  assert.deepEqual(expandChoice("#tree", [parts, states]), ["#tree"]);
});

test("panelKey tells the panels of one document apart by view", () => {
  assert.equal(panelKey("file:///a.sysml", "M::Parts"), panelKey("file:///a.sysml", "M::Parts"));
  assert.notEqual(panelKey("file:///a.sysml", "M::Parts"), panelKey("file:///a.sysml", "M::States"));
  assert.notEqual(panelKey("file:///a.sysml", "M::Parts"), panelKey("file:///b.sysml", "M::Parts"));
  assert.notEqual(panelKey("file:///a.sysml", ""), panelKey("file:///a.sysml", "#tree"));
  assert.notEqual(panelKey('file:///a".sysml', "x"), panelKey("file:///a", '".sysml","x'), "a quote in a path cannot forge a key");
});

test("viewTitle is the view's short name, or the pseudo-view spec", () => {
  assert.equal(viewTitle("KitViews::widgetParts"), "widgetParts");
  assert.equal(viewTitle("widgetParts"), "widgetParts");
  assert.equal(viewTitle("#tree"), "#tree");
});
