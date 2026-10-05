import assert from "node:assert/strict";
import { test } from "node:test";

import { asRunElementArgs, runArguments, runTitle } from "./runargs";

test("a lens argument with the three fields is accepted", () => {
  const args = asRunElementArgs({ uri: "file:///ws/demo.sysml", element: "Demo::Fall", kind: "calc" });
  assert.deepEqual(args, { uri: "file:///ws/demo.sysml", element: "Demo::Fall", kind: "calc" });
});

test("a lens argument missing a field, or not an object, is refused", () => {
  assert.equal(asRunElementArgs({ uri: "file:///ws/demo.sysml", kind: "calc" }), undefined);
  assert.equal(asRunElementArgs({ uri: "", element: "Demo::Fall", kind: "calc" }), undefined);
  assert.equal(asRunElementArgs("Demo::Fall"), undefined);
  assert.equal(asRunElementArgs(undefined), undefined);
});

test("each kind maps to the sysml check that takes the element by name", () => {
  const file = "/ws/demo.sysml";
  for (const [kind, flag] of [
    ["action", "-action"],
    ["state", "-state"],
    ["calc", "-calc"],
    ["constraint", "-constraint"],
    ["requirement", "-requirement"],
  ]) {
    assert.deepEqual(runArguments({ uri: "file:///ws/demo.sysml", element: "Demo::X", kind }, file), [flag, "Demo::X", file]);
  }
});

test("a kind no check takes runs nothing", () => {
  assert.equal(runArguments({ uri: "file:///ws/demo.sysml", element: "Demo::Rover", kind: "part" }, "/ws/demo.sysml"), undefined);
});

test("the title says Run for behaviors and Evaluate for the rest", () => {
  assert.equal(runTitle({ uri: "u", element: "Demo::Charge", kind: "action" }), "Run Demo::Charge");
  assert.equal(runTitle({ uri: "u", element: "Demo::Mission", kind: "state" }), "Run Demo::Mission");
  assert.equal(runTitle({ uri: "u", element: "Demo::Fall", kind: "calc" }), "Evaluate Demo::Fall");
  assert.equal(runTitle({ uri: "u", element: "Demo::wagonSpec", kind: "requirement" }), "Evaluate Demo::wagonSpec");
});
