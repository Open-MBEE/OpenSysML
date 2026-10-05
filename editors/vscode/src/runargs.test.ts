import assert from "node:assert/strict";
import { test } from "node:test";

import { asRunElementArgs, runArguments, runPaths, runTitle } from "./runargs";

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
    assert.deepEqual(runArguments({ uri: "file:///ws/demo.sysml", element: "Demo::X", kind }, [file]), [flag, "Demo::X", file]);
  }
});

test("every path given is loaded, so a sibling file's declarations resolve", () => {
  assert.deepEqual(runArguments({ uri: "file:///ws/a.sysml", element: "Demo::job", kind: "action" }, ["/ws", "/lib"]), [
    "-action",
    "Demo::job",
    "/ws",
    "/lib",
  ]);
});

test("a kind no check takes runs nothing, nor does an empty path list", () => {
  assert.equal(runArguments({ uri: "file:///ws/demo.sysml", element: "Demo::Rover", kind: "part" }, ["/ws/demo.sysml"]), undefined);
  assert.equal(runArguments({ uri: "file:///ws/demo.sysml", element: "Demo::go", kind: "action" }, []), undefined);
});

test("a file in a workspace folder runs over every folder, one outside runs alone", () => {
  assert.deepEqual(runPaths("/ws/models/demo.sysml", ["/ws", "/other"]), ["/ws", "/other"]);
  assert.deepEqual(runPaths("/ws", ["/ws"]), ["/ws"]);
  assert.deepEqual(runPaths("/elsewhere/demo.sysml", ["/ws"]), ["/elsewhere/demo.sysml"]);
  assert.deepEqual(runPaths("/wsx/demo.sysml", ["/ws"]), ["/wsx/demo.sysml"]);
  assert.deepEqual(runPaths("/scratch/demo.sysml", []), ["/scratch/demo.sysml"]);
  assert.deepEqual(runPaths("C:\\ws\\demo.sysml", ["C:\\ws"]), ["C:\\ws"]);
});

test("the title says Run for behaviors and Evaluate for the rest", () => {
  assert.equal(runTitle({ uri: "u", element: "Demo::Charge", kind: "action" }), "Run Demo::Charge");
  assert.equal(runTitle({ uri: "u", element: "Demo::Mission", kind: "state" }), "Run Demo::Mission");
  assert.equal(runTitle({ uri: "u", element: "Demo::Fall", kind: "calc" }), "Evaluate Demo::Fall");
  assert.equal(runTitle({ uri: "u", element: "Demo::wagonSpec", kind: "requirement" }), "Evaluate Demo::wagonSpec");
});
