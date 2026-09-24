import assert from "node:assert/strict";
import { test } from "node:test";

import { ActiveEditor, AutoOpenContext, Dismissals, DISMISSED_KEY, Lifecycle, renamedUri, shouldAutoOpen, Store } from "./autoopen";

const car: ActiveEditor = { uri: "file:///ws/car.sysml", languageId: "sysml", scheme: "file", inGroup: true, tab: "text" };
const ready: AutoOpenContext = { enabled: true, available: true, hasPanel: false, dismissed: false, editor: car };

test("a model file in an ordinary editor opens its diagram", () => {
  assert.equal(shouldAutoOpen(ready), true);
  assert.equal(shouldAutoOpen({ ...ready, editor: { ...car, uri: "file:///ws/core.kerml", languageId: "kerml" } }), true);
});

test("nothing opens with the setting off, without a server, with a panel already, or after a dismissal", () => {
  assert.equal(shouldAutoOpen({ ...ready, enabled: false }), false);
  assert.equal(shouldAutoOpen({ ...ready, available: false }), false);
  assert.equal(shouldAutoOpen({ ...ready, hasPanel: true }), false);
  assert.equal(shouldAutoOpen({ ...ready, dismissed: true }), false);
  assert.equal(shouldAutoOpen({ ...ready, editor: undefined }), false);
});

test("only a model file from disk, in a text tab of an editor group, qualifies", () => {
  assert.equal(shouldAutoOpen({ ...ready, editor: { ...car, languageId: "markdown", uri: "file:///ws/notes.md" } }), false);
  assert.equal(shouldAutoOpen({ ...ready, editor: { ...car, scheme: "untitled", uri: "untitled:Untitled-1" } }), false);
  assert.equal(shouldAutoOpen({ ...ready, editor: { ...car, scheme: "git" } }), false);
  assert.equal(shouldAutoOpen({ ...ready, editor: { ...car, scheme: "sysml-stdlib" } }), false);
  assert.equal(shouldAutoOpen({ ...ready, editor: { ...car, tab: "diff" } }), false);
  assert.equal(shouldAutoOpen({ ...ready, editor: { ...car, tab: "other" } }), false);
  assert.equal(shouldAutoOpen({ ...ready, editor: { ...car, tab: "none" } }), false);
  assert.equal(shouldAutoOpen({ ...ready, editor: { ...car, inGroup: false } }), false);
});

// A Store over a map, standing in for workspaceState.
function memory(initial: string[] | undefined = undefined): Store & { writes: (string[] | undefined)[] } {
  let value = initial;
  const writes: (string[] | undefined)[] = [];
  return {
    writes,
    get: (key) => (key === DISMISSED_KEY ? value : undefined),
    update: async (key, next) => {
      assert.equal(key, DISMISSED_KEY);
      value = next;
      writes.push(next);
    },
  };
}

test("a dismissal is recorded once, read back, and cleared", async () => {
  const store = memory();
  const dismissals = new Dismissals(store);
  assert.equal(dismissals.has(car.uri), false);
  await dismissals.record(car.uri);
  await dismissals.record(car.uri);
  assert.equal(dismissals.has(car.uri), true);
  assert.deepEqual(store.writes, [[car.uri]]);
  await dismissals.clear(car.uri);
  assert.equal(dismissals.has(car.uri), false);
  assert.deepEqual(store.writes.at(-1), undefined, "an empty list is removed from the store");
});

test("clearing a document never dismissed writes nothing, and leaves the others", async () => {
  const store = memory(["file:///ws/a.sysml", "file:///ws/b.sysml"]);
  const dismissals = new Dismissals(store);
  await dismissals.clear("file:///ws/c.sysml");
  assert.deepEqual(store.writes, []);
  await dismissals.clear("file:///ws/a.sysml");
  assert.deepEqual(store.writes, [["file:///ws/b.sysml"]]);
});

test("a rename carries the dismissal to the new name", async () => {
  const store = memory(["file:///ws/a.sysml", "file:///ws/b.sysml"]);
  const dismissals = new Dismissals(store);
  await dismissals.rename("file:///ws/c.sysml", "file:///ws/d.sysml");
  assert.deepEqual(store.writes, [], "a document never dismissed has nothing to carry");
  await dismissals.rename("file:///ws/a.sysml", "file:///ws/b.sysml");
  assert.deepEqual(store.get(DISMISSED_KEY), ["file:///ws/b.sysml"], "renaming onto a dismissed name keeps one entry");
  await dismissals.rename("file:///ws/b.sysml", "file:///ws/e.sysml");
  assert.equal(dismissals.has("file:///ws/b.sysml"), false);
  assert.equal(dismissals.has("file:///ws/e.sysml"), true);
});

test("a renamed URI is the document itself or one below a renamed folder", () => {
  assert.equal(renamedUri("file:///ws/a.sysml", "file:///ws/a.sysml", "file:///ws/b.sysml"), "file:///ws/b.sysml");
  assert.equal(renamedUri("file:///ws/models/car.sysml", "file:///ws/models", "file:///ws/vehicles"), "file:///ws/vehicles/car.sysml");
  assert.equal(renamedUri("file:///ws/models/x/car.sysml", "file:///ws/models/", "file:///ws/v/"), "file:///ws/v/x/car.sysml");
  assert.equal(renamedUri("file:///ws/models-old/car.sysml", "file:///ws/models", "file:///ws/v"), undefined, "a sibling sharing a prefix is not inside");
  assert.equal(renamedUri("file:///ws/other.sysml", "file:///ws/a.sysml", "file:///ws/b.sysml"), undefined);
});

test("renaming or deleting a folder carries or drops every dismissal inside it", async () => {
  const store = memory(["file:///ws/models/a.sysml", "file:///ws/models/sub/b.sysml", "file:///ws/other.sysml"]);
  const dismissals = new Dismissals(store);
  await dismissals.rename("file:///ws/models", "file:///ws/vehicles");
  assert.deepEqual(store.get(DISMISSED_KEY), ["file:///ws/other.sysml", "file:///ws/vehicles/a.sysml", "file:///ws/vehicles/sub/b.sysml"]);
  await dismissals.clear("file:///ws/vehicles");
  assert.deepEqual(store.get(DISMISSED_KEY), ["file:///ws/other.sysml"]);
});

test("a write that fails does not hold up the ones after it", async () => {
  const store = memory();
  let failing = true;
  const update = store.update;
  store.update = (key, next) => (failing ? Promise.reject(new Error("disk full")) : update(key, next));
  const dismissals = new Dismissals(store);
  await dismissals.record("file:///ws/a.sysml").then(
    () => assert.fail("the failing write rejects"),
    () => undefined,
  );
  failing = false;
  await dismissals.record("file:///ws/b.sysml");
  assert.deepEqual(store.get(DISMISSED_KEY), ["file:///ws/a.sysml", "file:///ws/b.sysml"]);
});

test("mutations fired together land in order, none overwriting another", async () => {
  const store = memory(["file:///ws/a.sysml", "file:///ws/b.sysml"]);
  const dismissals = new Dismissals(store);
  // A folder rename moves both files at once; nothing awaits between them.
  const first = dismissals.rename("file:///ws/a.sysml", "file:///ws/x/a.sysml");
  const second = dismissals.rename("file:///ws/b.sysml", "file:///ws/x/b.sysml");
  const third = dismissals.record("file:///ws/c.sysml");
  assert.equal(dismissals.has("file:///ws/x/a.sysml"), true, "the change shows before the store is written");
  assert.equal(dismissals.has("file:///ws/a.sysml"), false);
  await Promise.all([first, second, third]);
  assert.deepEqual(store.get(DISMISSED_KEY), ["file:///ws/x/a.sysml", "file:///ws/x/b.sysml", "file:///ws/c.sysml"]);
  assert.deepEqual(store.writes.at(-1), store.get(DISMISSED_KEY));
});

test("a panel the extension disposes is not a dismissal; one the user closes is", () => {
  const ours = new Lifecycle();
  assert.equal(ours.disposed, false);
  assert.equal(ours.dispose(), true, "the first dispose does the work");
  assert.equal(ours.disposed, true);
  assert.equal(ours.dispose(), false, "a second dispose is a no-op");
  assert.equal(ours.closed(), false, "the webview's onDidDispose that follows is ours");

  const theirs = new Lifecycle();
  assert.equal(theirs.closed(), true, "onDidDispose with no dispose before it is the user's");
  assert.equal(theirs.disposed, true);
  assert.equal(theirs.dispose(), false, "disposing after the user closed does nothing");
});
