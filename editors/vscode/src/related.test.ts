import assert from "node:assert/strict";
import { test } from "node:test";

import type { RelatedElementsResult } from "./protocol";
import { hiddenFrom, includeAfterPick, relatedPickItems, supportsInclude, supportsRelated } from "./related";

const origin = { uri: "file:///m.sysml", range: { start: { line: 0, character: 0 }, end: { line: 0, character: 4 } }, digest: "d0" };
const related: RelatedElementsResult = {
  element: { fqn: "Rel::Car", name: "Car", kind: "part def", declaredHere: true, origin },
  groups: [
    { kind: "generalizations", label: "Generalizations (specializes, subsets, redefines)", elements: [{ fqn: "Rel::Vehicle", name: "Vehicle", kind: "part def", declaredHere: true, origin }] },
    { kind: "typedUsages", label: "Typed usages (types)", elements: [
      { fqn: "Rel::car", name: "car", kind: "part", declaredHere: true, origin },
      { fqn: "Fleet::taxi", name: "taxi", kind: "part", origin: { ...origin, uri: "file:///fleet.sysml" } },
    ] },
    { kind: "owner", label: "Owner", elements: [] },
  ],
};

test("supportsInclude and supportsRelated read the server's capabilities", () => {
  assert.equal(supportsInclude(undefined), false);
  assert.equal(supportsInclude({ openSysmlRenderInclude: true }), true);
  assert.equal(supportsRelated({ openSysmlRenderInclude: true }), false);
  assert.equal(supportsRelated({ openSysmlRelatedElements: ["generalizations", "owner"] }), true);
  assert.equal(supportsRelated({ openSysmlRelatedElements: true }), false);
});

test("relatedPickItems heads each group with its count and checks what is already included", () => {
  assert.deepEqual(relatedPickItems(related, ["Fleet::taxi", "Other::thing"]), [
    { label: "Generalizations (specializes, subsets, redefines) (1)", heading: true },
    { label: "Vehicle", description: "part def", detail: "Rel::Vehicle", picked: false, fqn: "Rel::Vehicle" },
    { label: "Typed usages (types) (2)", heading: true },
    { label: "car", description: "part", detail: "Rel::car", picked: false, fqn: "Rel::car" },
    { label: "taxi", description: "part", detail: "Fleet::taxi", picked: true, fqn: "Fleet::taxi" },
  ]);
  assert.deepEqual(relatedPickItems({ ...related, groups: [] }, []), []);
  const drawn = relatedPickItems(related, ["Fleet::taxi"], new Set(["Rel::car", "Fleet::taxi"]));
  assert.equal(drawn[3].description, "part · drawn", "an element the drawing shows without being included is marked drawn");
  assert.equal(drawn[4].description, "part", "an included element is drawn because it is included; no mark");
});

test("includeAfterPick keeps what was not listed, drops listed elements unchecked, adds the checked once", () => {
  const items = relatedPickItems(related, ["Fleet::taxi", "Other::thing"]);
  const picked = items.filter((item) => item.fqn === "Rel::Vehicle" || item.fqn === "Rel::car");
  assert.deepEqual(includeAfterPick(["Fleet::taxi", "Other::thing"], items, picked), ["Other::thing", "Rel::Vehicle", "Rel::car"]);
  assert.deepEqual(includeAfterPick(["Rel::car"], items, picked), ["Rel::Vehicle", "Rel::car"]);
  assert.deepEqual(includeAfterPick(["Fleet::taxi"], items, []), []);
});

test("hiddenFrom leaves one element out and nothing else", () => {
  assert.deepEqual(hiddenFrom(["A::b", "A::c", "A::b"], "A::b"), ["A::c"]);
  assert.deepEqual(hiddenFrom(["A::c"], "A::b"), ["A::c"]);
});
