import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";

import { landingModel, renderResult, sceneTarget } from "./model";
import { renderedViewFromJson } from "@openmbee/opensysml/browser";
import type { EngineInstance, LandingModel } from "./model";
import { STACK_SCENE } from "./scene";
import { presentedStack, stackTone } from "./present";

interface LandingFixture {
  hash: string;
  /** The protojson RenderViewResponse the module answered. */
  render: unknown;
  instances: EngineInstance[];
}

const fixture = JSON.parse(readFileSync("src/landing/stack.json", "utf8")) as LandingFixture;
const fixtureRender = renderResult(renderedViewFromJson(fixture.render));

function fixtureModel(): LandingModel {
  return landingModel(sceneTarget(STACK_SCENE), fixture.hash, fixtureRender, fixture.instances);
}

function withPartAttrs(model: LandingModel, feature: string, attrs: Record<string, string>): LandingModel {
  const part = model.parts.get(feature);
  assert.ok(part);
  const parts = new Map(model.parts);
  parts.set(feature, { ...part, attrs });
  return { ...model, parts };
}

test("presentedStack uses fixture attributes and preserves edge and port data", () => {
  const model = fixtureModel();
  const flexo = model.parts.get("flexo");
  assert.ok(flexo);
  const result = presentedStack(model);
  const flexoNode = result.nodes.find(({ id }) => id === flexo.id);
  assert.ok(flexoNode);

  assert.equal(flexo.id, "n109");
  assert.equal(flexoNode.ports?.[0]?.id, "n109.0");
  assert.deepEqual(
    {
      name: flexoNode.name,
      kind: flexoNode.kind,
      type: flexoNode.type,
      detail: flexoNode.detail,
    },
    {
      name: "Flexo MMS",
      kind: "model store",
      type: "",
      detail: "Kotlin · the versioned store",
    },
  );

  for (const node of model.render.nodes) {
    assert.deepEqual(
      result.nodes.find(({ id }) => id === node.id)?.ports,
      node.ports,
    );
  }
  assert.deepEqual(
    result.edges,
    model.render.edges.map((edge) => ({ ...edge, label: "" })),
  );
});

test("presentedStack falls back to the feature name and omits a missing language", () => {
  const model = fixtureModel();
  const unlabeled = model.parts.get("opensysml");
  assert.ok(unlabeled);
  const unlabeledAttrs = { ...unlabeled.attrs };
  delete unlabeledAttrs.label;
  const unlabeledModel = withPartAttrs(model, "opensysml", unlabeledAttrs);
  const unlabeledNode = presentedStack(unlabeledModel).nodes.find(({ id }) => id === unlabeled.id);
  assert.equal(unlabeledNode?.name, unlabeled.feature);

  const flexo = model.parts.get("flexo");
  assert.ok(flexo);
  const roleOnlyAttrs = { ...flexo.attrs };
  delete roleOnlyAttrs.lang;
  const roleOnlyModel = withPartAttrs(model, "flexo", roleOnlyAttrs);
  const roleOnlyNode = presentedStack(roleOnlyModel).nodes.find(({ id }) => id === flexo.id);
  assert.equal(roleOnlyNode?.detail, roleOnlyAttrs.role);
  assert.ok(!roleOnlyNode?.detail?.includes(" · "));
});

test("presentedStack returns non-part nodes unchanged without mutating the input render", () => {
  const model = fixtureModel();
  const unmodeledNode = { ...model.render.nodes[0], id: "not-a-part" };
  const render = { ...model.render, nodes: [...model.render.nodes, unmodeledNode] };
  const input = { ...model, render };
  const originalRender = JSON.stringify(input.render);

  const result = presentedStack(input);

  assert.strictEqual(result.nodes[result.nodes.length - 1], unmodeledNode);
  assert.equal(JSON.stringify(input.render), originalRender);
});

test("stackTone maps the model's kinds to the hero's tones and leaves a component plain", () => {
  const model = fixtureModel();
  const tones = Object.fromEntries([...model.parts.values()].map((part) => [part.feature, stackTone(part)]));
  assert.equal(tones.flexo, "accent");
  assert.equal(tones.opensysml, "primary");
  assert.equal(tones.pilot, "secondary");
  for (const part of model.parts.values()) {
    if (part.owner !== undefined) {
      assert.equal(stackTone(part), undefined, part.feature);
    }
  }
  assert.equal(stackTone({ ...model.parts.get("flexo")!, attrs: {} }), undefined);
});
