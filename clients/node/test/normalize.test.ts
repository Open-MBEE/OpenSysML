// Normalization: the values a call cannot repeat, replaced the way
// conformance/README.md says.

import assert from "node:assert/strict";
import { test } from "node:test";
import { create } from "@bufbuild/protobuf";
import {
  DiagnosticSchema,
  InstanceSchema,
  InstantiateResponseSchema,
  ParseFileResponseSchema,
  ServerInfoResponseSchema,
  SpanSchema,
  SymbolInfoSchema,
  ValueSchema,
  FeatureValueSchema,
  FunctionSchema,
} from "../src/generated/sysml_pb.js";
import { Integer, Normalizer, MODEL_HASH_PLACEHOLDER, PATH_PLACEHOLDER, VERSION_PLACEHOLDER } from "../conformance/normalize.js";

const HASH = "3f1a2b3c4d5e6f70";

test("the model hash of this call, the version and absolute paths are replaced", () => {
  const response = create(ParseFileResponseSchema, {
    modelHash: HASH,
    root: create(SymbolInfoSchema, { id: "Sample", name: "Sample", kind: "package" }),
    diagnostics: [
      create(DiagnosticSchema, {
        severity: "error",
        message: "unexpected token",
        span: create(SpanSchema, { file: "/tmp/whatever/sample.sysml", startLine: 3 }),
      }),
    ],
  });
  const tree = new Normalizer(HASH).normalize(ParseFileResponseSchema, response);
  assert.equal(tree["model_hash"], MODEL_HASH_PLACEHOLDER);
  const diagnostics = tree["diagnostics"] as Record<string, unknown>[];
  const span = diagnostics[0]?.["span"] as Record<string, unknown>;
  assert.equal(span["file"], PATH_PLACEHOLDER);
  // A relative path is the scenario's own, so it stays as written.
  assert.equal(
    (
      (
        new Normalizer(HASH).normalize(
          ParseFileResponseSchema,
          create(ParseFileResponseSchema, {
            diagnostics: [
              create(DiagnosticSchema, { span: create(SpanSchema, { file: "fixtures/sample.sysml" }) }),
            ],
          }),
        )["diagnostics"] as Record<string, unknown>[]
      )[0]?.["span"] as Record<string, unknown>
    )["file"],
    "fixtures/sample.sysml",
  );
});

test("the service version is replaced, and capabilities are not", () => {
  const tree = new Normalizer(HASH).normalize(
    ServerInfoResponseSchema,
    create(ServerInfoResponseSchema, { version: "0.9.3", capabilities: ["query", "convert"] }),
  );
  assert.equal(tree["version"], VERSION_PLACEHOLDER);
  assert.deepEqual(tree["capabilities"], ["query", "convert"]);
});

test("instance ids are relabelled in the order they appear, consistently", () => {
  const response = create(InstantiateResponseSchema, {
    instance: create(InstanceSchema, {
      id: 41n,
      typeSymbolId: "Sample::Car",
      featureValues: {
        wheels: create(FeatureValueSchema, {
          featureName: "wheels",
          values: [
            create(ValueSchema, { kind: { case: "instanceId", value: 42n } }),
            create(ValueSchema, { kind: { case: "instanceId", value: 41n } }),
          ],
        }),
      },
    }),
    instances: [create(InstanceSchema, { id: 42n, typeSymbolId: "Sample::Wheel" })],
  });
  const tree = new Normalizer(HASH).normalize(InstantiateResponseSchema, response);
  const instance = tree["instance"] as Record<string, unknown>;
  assert.equal(instance["id"], "@1");
  const values = (
    (instance["feature_values"] as Record<string, Record<string, unknown>>)["wheels"]["values"] as
      | Record<string, unknown>[]
      | undefined
  )?.map((value) => value["instance_id"]);
  // The same id is the same label; a new one gets the next.
  assert.deepEqual(values, ["@2", "@1"]);
  assert.equal((tree["instances"] as Record<string, unknown>[])[0]?.["id"], "@2");
});

test("the object a function was read off is labelled with the instance it names", () => {
  const response = create(InstantiateResponseSchema, {
    instance: create(InstanceSchema, {
      id: 41n,
      typeSymbolId: "Sample::Scaler",
      featureValues: {
        scale: create(FeatureValueSchema, {
          featureName: "scale",
          values: [
            create(ValueSchema, {
              kind: { case: "function", value: create(FunctionSchema, { calcId: "Sample::Scaler::scale", selfId: 41n }) },
            }),
            create(ValueSchema, {
              kind: { case: "function", value: create(FunctionSchema, { calcId: "Sample::Sq" }) },
            }),
          ],
        }),
      },
    }),
  });
  const tree = new Normalizer(HASH).normalize(InstantiateResponseSchema, response);
  const instance = tree["instance"] as Record<string, unknown>;
  const values = (instance["feature_values"] as Record<string, Record<string, unknown>>)["scale"]["values"] as Record<
    string,
    Record<string, unknown>
  >[];
  assert.deepEqual(values[0]?.["function"], { calc_id: "Sample::Scaler::scale", self_id: "@1" });
  // A function bound to no object leaves self_id unset, so it does not appear.
  assert.deepEqual(values[1]?.["function"], { calc_id: "Sample::Sq" });
});

test("an integral field is an Integer, so it is never compared as a float", () => {
  const tree = new Normalizer(HASH).normalize(
    ParseFileResponseSchema,
    create(ParseFileResponseSchema, {
      diagnostics: [create(DiagnosticSchema, { span: create(SpanSchema, { startLine: 3 }) })],
    }),
  );
  const span = (tree["diagnostics"] as Record<string, unknown>[])[0]?.["span"] as Record<string, unknown>;
  assert.ok(span["start_line"] instanceof Integer);
  assert.equal(String(span["start_line"]), "3");
});
