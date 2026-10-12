// The typed-view runtime: decoders, accessors and the generated-class check,
// exercised over pb instances the way a generated module reads them.

import assert from "node:assert/strict";
import { create } from "@bufbuild/protobuf";
import { test } from "node:test";
import {
  FeatureValueError,
  InstanceTypeError,
  TypeMismatchError,
} from "../src/core/errors.js";
import { Instance } from "../src/core/model.js";
import {
  TypedObject,
  asEnum,
  asQuantity,
  asRational,
  asReal,
  asInt,
  asString,
  asTyped,
  featureValue,
  listFeatureValue,
  optionalFeatureValue,
  registerTyped,
} from "../src/core/typed.js";
import { FeatureValueSchema, InstanceSchema, ValueSchema } from "../src/generated/sysml_pb.js";
import type { FeatureValue, Value } from "../src/generated/sysml_pb.js";

function scalarFeature(name: string, kind: Value["kind"], materialized = true) {
  return create(FeatureValueSchema, {
    featureName: name,
    value: create(ValueSchema, { kind }),
    materialized,
  });
}

function vehicleInstance(extraFeatures: Record<string, FeatureValue> = {}) {
  const engine = create(InstanceSchema, {
    id: 2n,
    typeSymbolId: "Demo::Engine",
    featureValues: { power: scalarFeature("power", { case: "realValue", value: 300.0 }) },
  });
  const vehicle = create(InstanceSchema, {
    id: 1n,
    typeSymbolId: "Demo::Vehicle",
    featureValues: {
      mass: scalarFeature("mass", { case: "realValue", value: 1500.0 }),
      engine: scalarFeature("engine", { case: "instanceId", value: 2n }),
      broken: create(FeatureValueSchema, { featureName: "broken", error: "evaluation failed" }),
      label: scalarFeature("label", { case: "intValue", value: 7n }),
      ratios: create(FeatureValueSchema, {
        featureName: "ratios",
        values: [
          create(ValueSchema, { kind: { case: "realValue", value: 1.5 } }),
          create(ValueSchema, { kind: { case: "intValue", value: 2n } }),
        ],
        materialized: true,
      }),
      ...extraFeatures,
    },
  });
  const instances = [new Instance(vehicle), new Instance(engine)];
  return {
    root: instances[0],
    resolve: (id: bigint) => instances.find((instance) => instance.id === id),
  };
}

class Engine extends TypedObject {
  static readonly sysmlId: string = "Demo::Engine";
  static {
    registerTyped(this);
  }

  get power(): number {
    return featureValue(this, "power", asReal);
  }
}

class Vehicle extends TypedObject {
  static readonly sysmlId: string = "Demo::Vehicle";
  static {
    registerTyped(this);
  }

  get mass(): number {
    return featureValue(this, "mass", asReal);
  }

  get engine(): Engine {
    return featureValue(this, "engine", asTyped(Engine));
  }

  get broken(): number {
    return featureValue(this, "broken", asReal);
  }

  get label(): string {
    return featureValue(this, "label", asString);
  }

  get spare(): Engine | undefined {
    return optionalFeatureValue(this, "spare", asTyped(Engine));
  }

  get ratios(): readonly number[] {
    return listFeatureValue(this, "ratios", asReal);
  }
}

class SportsCar extends Vehicle {
  static readonly sysmlId: string = "Demo::SportsCar";
  static {
    registerTyped(this);
  }

  get topSpeed(): number {
    return featureValue(this, "topSpeed", asReal);
  }
}

test("fromInstance reads scalar and nested slots", () => {
  const { root, resolve } = vehicleInstance();
  const vehicle = Vehicle.fromInstance(root, resolve);
  assert.equal(vehicle.mass, 1500.0);
  assert.ok(vehicle.engine instanceof Engine);
  assert.equal(vehicle.engine.power, 300.0);
  assert.equal(vehicle.instance.id, 1n);
});

test("a slot error is preserved", () => {
  const vehicle = Vehicle.fromInstance(vehicleInstance().root);
  assert.throws(
    () => vehicle.broken,
    (error: unknown) => error instanceof FeatureValueError && error.featureName === "broken",
  );
});

test("a type mismatch is reported", () => {
  const vehicle = Vehicle.fromInstance(vehicleInstance().root);
  assert.throws(
    () => vehicle.label,
    (error: unknown) => error instanceof TypeMismatchError && error.expected === "str",
  );
});

test("a missing required slot raises", () => {
  const root = new Instance(create(InstanceSchema, { id: 3n, typeSymbolId: "Demo::Vehicle" }));
  const vehicle = Vehicle.fromInstance(root);
  assert.throws(() => vehicle.mass, TypeMismatchError);
});

test("an integer widens to float but a boolean does not", () => {
  assert.equal(asReal("x", { kind: "int", value: 3n }), 3.0);
  assert.throws(() => asReal("x", { kind: "boolean", value: true }), TypeMismatchError);
  assert.throws(() => asInt("x", { kind: "boolean", value: true }), TypeMismatchError);
});

test("a Rational decodes exactly whichever arm carried it; a Real rounds one once", () => {
  const third = { kind: "rational", numerator: 1n, denominator: 3n } as const;
  assert.deepEqual(asRational("x", third), { numerator: 1n, denominator: 3n });
  assert.deepEqual(asRational("x", { kind: "real", value: 0.1 }), {
    numerator: 3602879701896397n,
    denominator: 36028797018963968n,
  });
  assert.deepEqual(asRational("x", { kind: "real", value: -0.25 }), { numerator: -1n, denominator: 4n });
  assert.deepEqual(asRational("x", { kind: "int", value: 7n }), { numerator: 7n, denominator: 1n });
  assert.equal(asReal("x", third), 1 / 3);
  for (const bad of [Infinity, NaN]) {
    assert.throws(() => asRational("x", { kind: "real", value: bad }), TypeMismatchError);
  }
  assert.throws(() => asRational("x", { kind: "boolean", value: true }), TypeMismatchError);
});

test("optionalFeatureValue returns undefined when absent, present when held", () => {
  const { root, resolve } = vehicleInstance();
  assert.equal(Vehicle.fromInstance(root, resolve).spare, undefined);

  const withSpare = vehicleInstance({
    spare: scalarFeature("spare", { case: "instanceId", value: 2n }),
  });
  assert.ok(Vehicle.fromInstance(withSpare.root, withSpare.resolve).spare instanceof Engine);
});

test("optionalFeatureValue returns undefined when unset", () => {
  const { root } = vehicleInstance({
    spare: scalarFeature("spare", { case: "unset", value: true }),
  });
  assert.equal(Vehicle.fromInstance(root).spare, undefined);
});


test("listFeatureValue decodes every element", () => {
  const vehicle = Vehicle.fromInstance(vehicleInstance().root);
  assert.deepEqual(vehicle.ratios, [1.5, 2.0]);
});

test("listFeatureValue is empty when unset", () => {
  const { root } = vehicleInstance({
    ratios: scalarFeature("ratios", { case: "unset", value: true }),
  });
  assert.deepEqual(Vehicle.fromInstance(root).ratios, []);
});


test("listFeatureValue is empty when absent or null", () => {
  const root = new Instance(create(InstanceSchema, { id: 4n, typeSymbolId: "Demo::Vehicle" }));
  assert.deepEqual(Vehicle.fromInstance(root).ratios, []);

  const { root: withNull } = vehicleInstance({
    ratios: scalarFeature("ratios", { case: "null", value: "" }),
  });
  assert.deepEqual(Vehicle.fromInstance(withNull).ratios, []);
});

test("a required slot still rejects unset", () => {
  const { root } = vehicleInstance({
    mass: scalarFeature("mass", { case: "unset", value: true }),
  });
  assert.throws(() => Vehicle.fromInstance(root).mass, TypeMismatchError);
});

test("typed objects compare by instance identity", () => {
  const { root } = vehicleInstance();
  const view = Vehicle.fromInstance(root);
  const same = Vehicle.fromInstance(root);
  assert.ok(view.equals(same));
  assert.ok(!view.equals(Engine.unchecked(root)));
});

test("asQuantity decodes a quantity and rejects a bare number", () => {
  const quantity = {
    kind: "quantity" as const,
    magnitude: { kind: "real" as const, value: 5.0 },
    unit: "SI::kg",
  };
  assert.equal(asQuantity("mass", quantity), quantity);
  assert.throws(
    () => asQuantity("mass", { kind: "real", value: 5.0 }),
    (error: unknown) => error instanceof TypeMismatchError && error.expected === "Quantity",
  );
});

test("the type errors are reachable from the package", async () => {
  const pkg = await import("../src/core/index.js");
  assert.equal(pkg.InstanceTypeError, InstanceTypeError);
  assert.ok(pkg.MissingCapabilityError.prototype instanceof pkg.OpenSysMLError);
});

test("fromInstance rejects an instance of another type", () => {
  const { root } = vehicleInstance();
  assert.throws(
    () => Engine.fromInstance(root),
    (error: unknown) =>
      error instanceof InstanceTypeError &&
      error.expected === "Demo::Engine" &&
      error.actual === "Demo::Vehicle" &&
      error.message.includes("Demo::Engine") &&
      error.message.includes("Demo::Vehicle"),
  );
});

test("fromInstance accepts a subtype instance", () => {
  const root = new Instance(
    create(InstanceSchema, {
      id: 5n,
      typeSymbolId: "Demo::SportsCar",
      featureValues: {
        mass: scalarFeature("mass", { case: "realValue", value: 1200.0 }),
        topSpeed: scalarFeature("topSpeed", { case: "realValue", value: 250.0 }),
      },
    }),
  );
  const asBase = Vehicle.fromInstance(root);
  assert.equal(asBase.mass, 1200.0);
  assert.equal(SportsCar.fromInstance(root).topSpeed, 250.0);
});

test("fromInstance accepts a type no generated class describes", () => {
  const root = new Instance(
    create(InstanceSchema, {
      id: 6n,
      typeSymbolId: "Demo::myCar",
      featureValues: { mass: scalarFeature("mass", { case: "realValue", value: 900.0 }) },
    }),
  );
  assert.equal(Vehicle.fromInstance(root).mass, 900.0);
});

test("fromInstance accepts an instance with no reported type", () => {
  const root = new Instance(
    create(InstanceSchema, {
      id: 7n,
      featureValues: { mass: scalarFeature("mass", { case: "realValue", value: 1.0 }) },
    }),
  );
  assert.equal(Vehicle.fromInstance(root).mass, 1.0);
});

test("unchecked bypasses the type check", () => {
  const { root } = vehicleInstance();
  const view = Engine.unchecked(root);
  assert.equal(view.instance.typeId, "Demo::Vehicle");
  assert.throws(() => view.power, TypeMismatchError);
});

test("a nested slot of the wrong type is rejected", () => {
  const { root, resolve } = vehicleInstance({
    engine: scalarFeature("engine", { case: "instanceId", value: 1n }),
  });
  assert.throws(() => Vehicle.fromInstance(root, resolve).engine, InstanceTypeError);
});

test("asEnum decodes a literal and rejects a plain value", () => {
  const literal = {
    name: "Color::red",
    literalId: "D::Color::red",
    enumerationId: "D::Color",
  };
  assert.equal(asEnum("c", { kind: "enum", value: literal }), literal);
  assert.throws(() => asEnum("c", { kind: "string", value: "Color::red" }), TypeMismatchError);
});
