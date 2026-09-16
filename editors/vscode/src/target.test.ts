import assert from "node:assert/strict";
import { test } from "node:test";

import { isModelLanguage, isModelPath, resolveTarget } from "./target";

const car = { uri: "file:///ws/car.sysml", languageId: "sysml" };
const core = { uri: "file:///ws/core.kerml", languageId: "kerml" };
const notes = { uri: "file:///ws/notes.md", languageId: "markdown" };

test("a menu's resource wins over everything else", () => {
  const target = resolveTarget(
    { argument: "file:///ws/other.kerml", focusedPanel: car.uri, activeEditor: notes, visibleEditors: [car, notes] },
    "draw",
  );
  assert.deepEqual(target, { kind: "document", uri: "file:///ws/other.kerml", fromPanel: false });
});

test("a menu's resource that is not a model file is refused by name", () => {
  const target = resolveTarget({ argument: "file:///ws/read%20me.md", visibleEditors: [car] }, "draw");
  assert.deepEqual(target, { kind: "none", message: "read me.md is not a .sysml or .kerml file." });
});

test("the focused diagram panel names its own document and says so", () => {
  const target = resolveTarget({ focusedPanel: car.uri, activeEditor: notes, visibleEditors: [notes] }, "draw");
  assert.deepEqual(target, { kind: "document", uri: car.uri, fromPanel: true });
});

test("the active model editor is the document", () => {
  const target = resolveTarget({ activeEditor: core, visibleEditors: [car, core] }, "draw");
  assert.deepEqual(target, { kind: "document", uri: core.uri, fromPanel: false });
});

test("an active editor of another language yields to the one visible model editor", () => {
  const target = resolveTarget({ activeEditor: notes, visibleEditors: [notes, car] }, "draw a diagram of it");
  assert.deepEqual(target, { kind: "document", uri: car.uri, fromPanel: false });
});

test("no active editor yields to the one visible model editor, even split twice", () => {
  const target = resolveTarget({ visibleEditors: [car, car, notes] }, "draw");
  assert.deepEqual(target, { kind: "document", uri: car.uri, fromPanel: false });
});

test("several visible model files without focus is ambiguous", () => {
  const target = resolveTarget({ activeEditor: notes, visibleEditors: [car, core] }, "draw a diagram of it");
  assert.deepEqual(target, {
    kind: "none",
    message: "Several .sysml or .kerml files are open; focus the one to draw a diagram of it.",
  });
});

test("nothing to draw asks for a model file, naming the verb", () => {
  assert.deepEqual(resolveTarget({ visibleEditors: [notes] }, "export a diagram of it"), {
    kind: "none",
    message: "Open a .sysml or .kerml file to export a diagram of it.",
  });
  assert.deepEqual(resolveTarget({ visibleEditors: [] }, "draw a diagram of it"), {
    kind: "none",
    message: "Open a .sysml or .kerml file to draw a diagram of it.",
  });
});

test("model languages and paths", () => {
  assert.equal(isModelLanguage("sysml"), true);
  assert.equal(isModelLanguage("kerml"), true);
  assert.equal(isModelLanguage("markdown"), false);
  assert.equal(isModelPath("file:///ws/a.sysml"), true);
  assert.equal(isModelPath("/ws/A.KERML"), true);
  assert.equal(isModelPath("file:///ws/a.sysmlx"), false);
  assert.equal(isModelPath("file:///ws/sysml"), false);
});
