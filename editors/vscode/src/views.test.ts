import assert from "node:assert/strict";
import { test } from "node:test";

import {
  ALL_VIEWS,
  chooseView,
  CHOSEN_VIEWS_KEY,
  ChosenViews,
  ChoiceStore,
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
  assert.equal(viewAt([parts, states, shape], { line: 4, character: 1 }), "M::Parts", "its closing brace is in it");
  assert.equal(viewAt([parts, states, shape], { line: 4, character: 2 }), undefined, "the range's end is past the declaration");
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

// A ChoiceStore over a value, standing in for workspaceState.
function memory(initial?: Record<string, string>): ChoiceStore & { writes: (Record<string, string> | undefined)[] } {
  let value = initial;
  const writes: (Record<string, string> | undefined)[] = [];
  return {
    writes,
    get: (key) => (key === CHOSEN_VIEWS_KEY ? value : undefined),
    update: async (key, next) => {
      assert.equal(key, CHOSEN_VIEWS_KEY);
      value = next;
      writes.push(next);
    },
  };
}

const car = "file:///w/car.sysml";
const boat = "file:///w/fleet/boat.sysml";

test("a chosen view is remembered, read back, replaced and forgotten", async () => {
  const store = memory();
  const chosen = new ChosenViews(store);
  assert.equal(chosen.get(car), undefined);
  await chosen.remember(car, "M::Parts");
  await chosen.remember(car, "M::Parts");
  assert.equal(chosen.get(car), "M::Parts");
  assert.deepEqual(store.writes, [{ [car]: "M::Parts" }], "remembering the same view again writes nothing");
  await chosen.remember(car, "M::States");
  assert.equal(chosen.get(car), "M::States");
  await chosen.remember(car, undefined);
  assert.equal(chosen.get(car), undefined);
  assert.equal(store.writes.at(-1), undefined, "an empty record is removed from the store");
  await chosen.remember(car, undefined);
  assert.equal(store.writes.length, 3, "forgetting a document never chosen writes nothing");
});

test("a rename carries the choice to the new name, and a delete drops it", async () => {
  const store = memory({ [car]: "M::Parts", [boat]: "F::Hull" });
  const chosen = new ChosenViews(store);
  await chosen.rename(car, "file:///w/auto.sysml");
  assert.equal(chosen.get(car), undefined);
  assert.equal(chosen.get("file:///w/auto.sysml"), "M::Parts");
  assert.equal(chosen.get(boat), "F::Hull", "the other document keeps its choice");
  await chosen.rename("file:///w/other.sysml", "file:///w/else.sysml");
  assert.equal(store.writes.length, 1, "a rename touching no chosen document writes nothing");
  await chosen.clear("file:///w/auto.sysml");
  assert.equal(chosen.get("file:///w/auto.sysml"), undefined);
  assert.deepEqual(store.writes.at(-1), { [boat]: "F::Hull" });
});

test("renaming or deleting a folder carries or drops every choice inside it", async () => {
  const store = memory({ [car]: "M::Parts", [boat]: "F::Hull" });
  const chosen = new ChosenViews(store);
  await chosen.rename("file:///w/fleet", "file:///w/navy");
  assert.equal(chosen.get("file:///w/navy/boat.sysml"), "F::Hull");
  assert.equal(chosen.get(boat), undefined);
  await chosen.clear("file:///w/navy/");
  assert.equal(chosen.get("file:///w/navy/boat.sysml"), undefined);
  assert.deepEqual(store.writes.at(-1), { [car]: "M::Parts" });
});

test("choice mutations fired together land in order, none overwriting another", async () => {
  const store = memory();
  const chosen = new ChosenViews(store);
  const a = chosen.remember(car, "M::Parts");
  const b = chosen.remember(boat, "F::Hull");
  const c = chosen.rename(car, "file:///w/auto.sysml");
  await Promise.all([a, b, c]);
  assert.deepEqual(store.writes.at(-1), { [boat]: "F::Hull", "file:///w/auto.sysml": "M::Parts" });
});
