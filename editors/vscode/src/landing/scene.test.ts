import assert from "node:assert/strict";
import { test } from "node:test";

import { MASS_ROLLUP_SCENE, SCENES, STACK_SCENE, chooseScene, forcedSceneId, requiredVariant, variantChain } from "./scene";

test("the scenes declare distinct ids, controls, programs and snapshots", () => {
  assert.deepEqual(SCENES.map(({ id }) => id), ["stack", "mass"]);
  assert.equal(STACK_SCENE.program.kind, "journey");
  assert.deepEqual(STACK_SCENE.controls, ["run", "edit", "debug", "download"]);
  assert.equal(MASS_ROLLUP_SCENE.program.kind, "rollup");
  assert.deepEqual(MASS_ROLLUP_SCENE.controls, ["run", "edit", "download"]);
  assert.equal(MASS_ROLLUP_SCENE.variants[0]?.rootSymbol, "MassRollup::saturnVWithIU");
  for (const scene of SCENES) {
    assert.ok(scene.snapshot.hash.length > 0, `${scene.id} has a snapshot hash`);
    assert.ok(Array.isArray(scene.snapshot.instances) && scene.snapshot.instances.length > 0, `${scene.id} has instances`);
    assert.ok(scene.pngName.endsWith(".png"));
  }
});

test("the roll-up's second variant builds on the first, and the chain unwinds through it", () => {
  const [iu, valued] = MASS_ROLLUP_SCENE.variants;
  assert.equal(iu?.rootSymbol, "MassRollup::saturnVWithIU");
  assert.equal(valued?.rootSymbol, "MassRollup::saturnVComplete");
  assert.equal(valued?.requires, iu?.id);
  assert.equal(requiredVariant(MASS_ROLLUP_SCENE, iu!), undefined);
  assert.equal(requiredVariant(MASS_ROLLUP_SCENE, valued!), iu);
  assert.deepEqual(variantChain(MASS_ROLLUP_SCENE, undefined), []);
  assert.deepEqual(variantChain(MASS_ROLLUP_SCENE, iu), [iu]);
  assert.deepEqual(variantChain(MASS_ROLLUP_SCENE, valued), [valued, iu]);
  const scene = { ...MASS_ROLLUP_SCENE, variants: [{ ...iu!, requires: "missing" }, { ...valued!, requires: "iu" }] };
  assert.equal(requiredVariant(scene, scene.variants[0]), undefined, "a requirement that names no variant stands alone");
  assert.deepEqual(variantChain(scene, scene.variants[1]).map(({ id }) => id), ["iu-mass", "iu"]);
});

test("forcedSceneId reads the scene from the query or the fragment", () => {
  assert.equal(forcedSceneId(new URL("https://example.test/")), undefined);
  assert.equal(forcedSceneId(new URL("https://example.test/?scene=mass")), "mass");
  assert.equal(forcedSceneId(new URL("https://example.test/#scene=stack")), "stack");
  assert.equal(forcedSceneId(new URL("https://example.test/?scene=mass#scene=stack")), "mass");
  assert.equal(forcedSceneId(new URL("https://example.test/#refereed")), undefined);
  assert.equal(forcedSceneId(new URL("https://example.test/?scene=")), undefined);
});

test("chooseScene honours a forced scene and falls back to the random draw", () => {
  const never = (): number => {
    throw new Error("the draw is not consulted when the URL forces a scene");
  };
  assert.equal(chooseScene(SCENES, new URL("https://example.test/?scene=mass"), never), MASS_ROLLUP_SCENE);
  assert.equal(chooseScene(SCENES, new URL("https://example.test/#scene=stack"), never), STACK_SCENE);
  assert.equal(chooseScene(SCENES, new URL("https://example.test/?scene=nonesuch"), () => 0), STACK_SCENE);
  assert.equal(chooseScene(SCENES, new URL("https://example.test/"), () => 0), STACK_SCENE);
  assert.equal(chooseScene(SCENES, new URL("https://example.test/"), () => 0.5), MASS_ROLLUP_SCENE);
  assert.equal(chooseScene(SCENES, new URL("https://example.test/"), () => 0.999999), MASS_ROLLUP_SCENE);
  assert.equal(chooseScene(SCENES, new URL("https://example.test/"), () => 1), MASS_ROLLUP_SCENE);
  assert.throws(() => chooseScene([], new URL("https://example.test/")), /no scene/u);
});

test("chooseScene draws each scene in proportion to the draw", () => {
  const counts = new Map<string, number>();
  const draws = 1000;
  for (let i = 0; i < draws; i++) {
    const scene = chooseScene(SCENES, new URL("https://example.test/"), () => i / draws);
    counts.set(scene.id, (counts.get(scene.id) ?? 0) + 1);
  }
  assert.deepEqual([...counts.entries()], [["stack", draws / 2], ["mass", draws / 2]]);
});
