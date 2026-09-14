import assert from "node:assert/strict";
import { test } from "node:test";

import { declaredViewEntries, DEFAULT_PSEUDO_VIEW, impliedView, pseudoViewEntries } from "./views";

test("declaredViewEntries lists the views the server declares, with why one cannot be drawn", () => {
  assert.deepEqual(declaredViewEntries(undefined), []);
  assert.deepEqual(
    declaredViewEntries({
      views: [
        { name: "M::Parts", kind: "interconnection", supported: true },
        { name: "M::Shape", kind: "geometry", supported: false, reason: "no geometry renderer" },
      ],
    }),
    [
      { value: "M::Parts", label: "M::Parts — interconnection", supported: true, reason: undefined },
      { value: "M::Shape", label: "M::Shape — geometry", supported: false, reason: "no geometry renderer" },
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
