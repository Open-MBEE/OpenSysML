import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";

import type { RenderResult } from "../protocol";
import { landingModel } from "./model";
import type { EngineInstance, LandingModel } from "./model";
import { presented } from "./present";

interface LandingFixture {
  hash: string;
  render: Omit<RenderResult, "form" | "artifact" | "version">;
  instances: EngineInstance[];
}

const fixture = JSON.parse(readFileSync("src/landing/stack.json", "utf8")) as LandingFixture;
const fixtureRender = fixture.render as RenderResult;

function fixtureModel(): LandingModel {
  return landingModel(fixture.hash, fixtureRender, fixture.instances);
}

function withPartAttrs(model: LandingModel, feature: string, attrs: Record<string, string>): LandingModel {
  const part = model.parts.get(feature);
  assert.ok(part);
  const parts = new Map(model.parts);
  parts.set(feature, { ...part, attrs });
  return { ...model, parts };
}

test("presented uses fixture attributes and preserves edge and port data", () => {
  const model = fixtureModel();
  const flexo = model.parts.get("flexo");
  assert.ok(flexo);
  const result = presented(model);
  const flexoNode = result.nodes.find(({ id }) => id === flexo.id);
  assert.ok(flexoNode);

  assert.equal(flexo.id, "n19");
  assert.equal(flexoNode.ports?.[0]?.id, "n19.0");
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

test("presented falls back to the feature name and omits a missing language", () => {
  const model = fixtureModel();
  const unlabeled = model.parts.get("opensysml");
  assert.ok(unlabeled);
  const unlabeledAttrs = { ...unlabeled.attrs };
  delete unlabeledAttrs.label;
  const unlabeledModel = withPartAttrs(model, "opensysml", unlabeledAttrs);
  const unlabeledNode = presented(unlabeledModel).nodes.find(({ id }) => id === unlabeled.id);
  assert.equal(unlabeledNode?.name, unlabeled.feature);

  const flexo = model.parts.get("flexo");
  assert.ok(flexo);
  const roleOnlyAttrs = { ...flexo.attrs };
  delete roleOnlyAttrs.lang;
  const roleOnlyModel = withPartAttrs(model, "flexo", roleOnlyAttrs);
  const roleOnlyNode = presented(roleOnlyModel).nodes.find(({ id }) => id === flexo.id);
  assert.equal(roleOnlyNode?.detail, roleOnlyAttrs.role);
  assert.ok(!roleOnlyNode?.detail?.includes(" · "));
});

test("presented returns non-part nodes unchanged without mutating the input render", () => {
  const model = fixtureModel();
  const unmodeledNode = { ...model.render.nodes[0], id: "not-a-part" };
  const render = { ...model.render, nodes: [...model.render.nodes, unmodeledNode] };
  const input = { ...model, render };
  const originalRender = JSON.stringify(input.render);

  const result = presented(input);

  assert.strictEqual(result.nodes[result.nodes.length - 1], unmodeledNode);
  assert.equal(JSON.stringify(input.render), originalRender);
});
