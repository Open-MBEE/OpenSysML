// The type mapping and emitter of opensysml-generate.

import assert from "node:assert/strict";
import { test } from "node:test";
import type { SpecializationFacts, TypeFacts } from "../src/core/model.js";
import {
  Definition,
  UNSTAMPED,
  classNames,
  collectDefinitions,
  elementType,
  featureType,
  isFeatureKind,
  modelStamp,
  propertyName,
  renderModule,
} from "../src/node/generate.js";
import type { SymbolTree } from "../src/node/generate.js";

function facts(over: Partial<TypeFacts> = {}): TypeFacts {
  return {
    declared: "",
    resolvedId: "",
    resolvedKind: "",
    primitive: "",
    primitiveSource: "",
    quantity: false,
    unit: "",
    ...over,
  };
}

function specialization(kind: string, targetId: string, declared = ""): SpecializationFacts {
  return { kind, declared, targetId, targetKind: "" };
}

function definition(
  fqn: string,
  kind = "partDef",
  features: Definition["features"][number][] = [],
  specializations: SpecializationFacts[] = [],
): Definition {
  return new Definition({
    id: fqn,
    name: fqn.split("::").at(-1) ?? fqn,
    kind,
    specializations,
    features,
  });
}

function feature(
  name: string,
  options: {
    kind?: string;
    type?: TypeFacts | undefined;
    multiplicity?: { lower: string; upper: string } | undefined;
    owner?: string;
  } = {},
): Definition["features"][number] {
  const owner = options.owner ?? "Demo::Vehicle";
  return {
    id: `${owner}::${name}`,
    name,
    kind: options.kind ?? "attributeUsage",
    type: options.type,
    multiplicity: options.multiplicity,
  };
}

class FakeSymbol implements SymbolTree {
  readonly type?: TypeFacts | undefined;
  readonly multiplicity?: { lower: string; upper: string } | undefined;
  readonly specializations?: readonly SpecializationFacts[] | undefined;

  constructor(
    readonly id: string,
    readonly name: string,
    readonly kind: string,
    private readonly childList: readonly SymbolTree[] = [],
  ) {}

  children(): Promise<readonly SymbolTree[]> {
    return Promise.resolve(this.childList);
  }
}

test("isFeatureKind covers structural usages and excludes behavioral ones", () => {
  for (const kind of ["attributeUsage", "partUsage", "itemUsage", "portUsage"]) {
    assert.ok(isFeatureKind(kind));
  }
  for (const kind of ["actionUsage", "stateUsage", "calcUsage", "constraintUsage", "partDef"]) {
    assert.ok(!isFeatureKind(kind));
  }
});

test("elementType maps library scalars to their counterparts", () => {
  const cases: readonly [string, string, string][] = [
    ["Boolean", "boolean", "_t.asBoolean"],
    ["String", "string", "_t.asString"],
    ["Natural", "bigint", "_t.asInt"],
    ["Integer", "bigint", "_t.asInt"],
    ["Rational", "number", "_t.asReal"],
    ["Real", "number", "_t.asReal"],
  ];
  for (const [primitive, annotation, decoder] of cases) {
    const mapped = elementType(facts({ primitive }), new Map());
    assert.equal(mapped.annotation, annotation);
    assert.equal(mapped.decoder, decoder);
    assert.equal(mapped.comment, "");
  }
});

test("elementType maps a generated definition to its class", () => {
  const mapped = elementType(
    facts({ declared: "Engine", resolvedId: "Demo::Engine" }),
    new Map([["Demo::Engine", "Engine"]]),
  );
  assert.equal(mapped.annotation, "Engine");
  assert.equal(mapped.decoder, "_t.asTyped(Engine)");
});

test("a definition reducing to a scalar maps to that scalar", () => {
  const mapped = elementType(
    facts({ declared: "Celsius", resolvedId: "Demo::Celsius", primitive: "Real" }),
    new Map([["Demo::Celsius", "Celsius"]]),
  );
  assert.equal(mapped.annotation, "number");
  assert.equal(mapped.decoder, "_t.asReal");
});

test("an enumeration-typed feature maps to a literal", () => {
  const mapped = elementType(
    facts({ declared: "Color", resolvedId: "D::Color", resolvedKind: "enumDef" }),
    new Map([["D::Color", "Color"]]),
  );
  assert.equal(mapped.annotation, "_t.EnumValue");
  assert.equal(mapped.decoder, "_t.asEnum");
});

test("a valued enumeration maps to its scalar", () => {
  const mapped = elementType(
    facts({
      declared: "Code",
      resolvedId: "D::Code",
      resolvedKind: "enumDef",
      primitive: "Integer",
    }),
    new Map([["D::Code", "Code"]]),
  );
  assert.equal(mapped.annotation, "bigint");
  assert.equal(mapped.decoder, "_t.asInt");
});

test("Complex maps to the complex value arm", () => {
  const mapped = elementType(facts({ primitive: "Complex" }), new Map());
  assert.equal(mapped.annotation, "_t.ComplexValue");
  assert.equal(mapped.decoder, "_t.asComplex");
});

test("an unmapped primitive maps to a raw value", () => {
  const mapped = elementType(facts({ primitive: "Number" }), new Map());
  assert.equal(mapped.annotation, "_t.SysMLValue");
  assert.ok(mapped.comment.includes("Number"));
});

test("a quantity maps to the quantity arm naming its unit", () => {
  const mapped = elementType(facts({ primitive: "Real", quantity: true, unit: "kg" }), new Map());
  assert.equal(mapped.annotation, "_t.Quantity");
  assert.equal(mapped.decoder, "_t.asQuantity");
  assert.ok(mapped.comment.includes("kg"));

  const noUnit = elementType(facts({ primitive: "Real", quantity: true }), new Map());
  assert.equal(noUnit.annotation, "_t.SysMLValue");
  assert.equal(noUnit.decoder, "_t.asObject");
});

test("an unresolved or untyped feature maps to a raw value", () => {
  const unresolved = elementType(facts({ declared: "Missing" }), new Map());
  assert.equal(unresolved.annotation, "_t.SysMLValue");
  assert.ok(unresolved.comment.includes("Missing"));

  const untyped = elementType(undefined, new Map());
  assert.equal(untyped.annotation, "_t.SysMLValue");
  assert.ok(untyped.comment !== "");
});

test("a type without a generated class maps to a raw value naming the FQN", () => {
  const mapped = elementType(facts({ declared: "Anything", resolvedId: "Base::Anything" }), new Map());
  assert.equal(mapped.annotation, "_t.SysMLValue");
  assert.ok(mapped.comment.includes("Base::Anything"));
});

test("multiplicity decides between a bare value, an option and a list", () => {
  const cases: readonly [{ lower: string; upper: string } | undefined, string][] = [
    [undefined, "number"],
    [{ lower: "1", upper: "1" }, "number"],
    [{ lower: "0", upper: "1" }, "number | undefined"],
    [{ lower: "0", upper: "*" }, "readonly number[]"],
    [{ lower: "2", upper: "4" }, "readonly number[]"],
    [{ lower: "1", upper: "" }, "number"],
  ];
  for (const [multiplicity, expected] of cases) {
    const mapped = featureType(
      feature("mass", { type: facts({ primitive: "Real" }), multiplicity }),
      new Map(),
    );
    assert.equal(mapped.annotation, expected);
  }
});

test("a feature named like a TypedObject member is renamed", () => {
  const source = renderModule([
    definition("Demo::Vehicle", "partDef", [
      feature("instance", { type: facts({ primitive: "Real" }) }),
    ]),
  ]);
  assert.ok(source.includes("get instance_(): number"));
  assert.ok(source.includes('_t.featureValue(this, "instance", _t.asReal)'));
});

test("a feature named unchecked does not shadow the escape hatch", () => {
  const source = renderModule([
    definition("Demo::Vehicle", "partDef", [
      feature("unchecked", { type: facts({ primitive: "Real" }) }),
    ]),
  ]);
  assert.ok(source.includes("get unchecked_(): number"));
});

test("propertyName renames reserved members and words", () => {
  assert.equal(propertyName("mass"), "mass");
  assert.equal(propertyName("instance"), "instance_");
  assert.equal(propertyName("unchecked"), "unchecked_");
  assert.equal(propertyName("resolver"), "resolver_");
  assert.equal(propertyName("class"), "class_");
  assert.equal(propertyName("my part"), "my_part");
});

test("an unrestricted name with quotes is escaped", () => {
  const source = renderModule([
    definition('Demo::say "hi"\\x', "partDef", [
      feature('mass "kg"\\x', {
        type: facts({ primitive: "Real" }),
        owner: 'Demo::say "hi"\\x',
      }),
    ]),
  ]);
  assert.ok(source.includes('sysmlId: string = "Demo::say \\"hi\\"\\\\x"'));
  assert.ok(source.includes('_t.featureValue(this, "mass \\"kg\\"\\\\x", _t.asReal)'));
  assert.ok(source.includes("class say__hi__x"));
  assert.ok(source.includes("get mass__kg__x()"));
});

test("classNames disambiguate collisions", () => {
  const names = classNames([
    definition("A::Thing"),
    definition("B::Thing"),
    definition("A::Other"),
  ]);
  assert.equal(names.get("A::Thing"), "A_Thing");
  assert.equal(names.get("B::Thing"), "B_Thing");
  assert.equal(names.get("A::Other"), "Other");
});

test("classNames sanitize identifiers", () => {
  const names = classNames([definition("P::my part"), definition("P::class")]);
  assert.equal(names.get("P::my part"), "my_part");
  assert.equal(names.get("P::class"), "class_");
});

test("collectDefinitions walks the tree and sorts", async () => {
  const engine = new FakeSymbol("Demo::Engine", "Engine", "partDef");
  const mass = new FakeSymbol("Demo::Vehicle::mass", "mass", "attributeUsage");
  const constraint = new FakeSymbol("Demo::Vehicle::ok", "ok", "constraintUsage");
  const vehicle = new FakeSymbol("Demo::Vehicle", "Vehicle", "partDef", [mass, constraint]);
  const root = new FakeSymbol("Demo", "Demo", "package", [vehicle, engine]);

  const definitions = await collectDefinitions(root);
  assert.deepEqual(
    definitions.map((definition) => definition.id),
    ["Demo::Engine", "Demo::Vehicle"],
  );
  assert.deepEqual(
    definitions[1]?.features.map((item) => item.name),
    ["mass"],
  );
});

test("renderModule emits the header and stamps, deterministically", () => {
  const definitions = [
    definition("Demo::Engine", "partDef", [
      feature("power", { type: facts({ primitive: "Real" }), owner: "Demo::Engine" }),
    ]),
    definition("Demo::Vehicle", "partDef", [
      feature("engine", {
        kind: "partUsage",
        type: facts({ declared: "Engine", resolvedId: "Demo::Engine" }),
      }),
      feature("mass", { type: facts({ primitive: "Real" }) }),
    ]),
  ];
  const source = renderModule(definitions, "abc");
  assert.equal(source, renderModule(definitions, "abc"));
  assert.ok(source.startsWith("// Generated by opensysml-generate. Do not edit."));
  assert.ok(source.includes('SYSML_GENERATOR_VERSION = "4"'));
  assert.ok(source.includes('SYSML_MODEL_HASH = "sha256:abc"'));
  assert.ok(source.includes('import * as _t from "@openmbee/opensysml"'));
  assert.ok(source.includes("export class Engine extends _t.TypedObject"));
  assert.ok(source.includes('sysmlId: string = "Demo::Vehicle"'));
  assert.ok(source.includes("_t.registerTyped(this)"));
  assert.ok(source.includes("get engine(): Engine"));
});

test("an unstamped module says unstamped", () => {
  assert.ok(renderModule([]).includes(`SYSML_MODEL_HASH = "${UNSTAMPED}"`));
});

test("modelStamp normalizes newlines", () => {
  assert.equal(modelStamp("a\nb"), modelStamp("a\r\nb"));
  assert.equal(modelStamp("a\rb"), modelStamp("a\nb"));
  assert.notEqual(modelStamp("a\nb"), modelStamp("a\nc"));
});

test("renderModule emits bases before subclasses", () => {
  const car = definition("Demo::Car", "partDef", [], [
    specialization("specializes", "Demo::Vehicle", "Vehicle"),
  ]);
  const vehicle = definition("Demo::Vehicle");
  const source = renderModule([car, vehicle]);
  assert.ok(source.indexOf("class Vehicle") < source.indexOf("class Car"));
  assert.ok(source.includes("class Car extends Vehicle"));
});

test("every generalization edge becomes a base", () => {
  for (const kind of ["subsets", "redefines"]) {
    const car = definition("Demo::Car", "partDef", [], [
      specialization(kind, "Demo::Vehicle", "Vehicle"),
    ]);
    const source = renderModule([car, definition("Demo::Vehicle")]);
    assert.ok(source.includes("class Car extends Vehicle"));
  }
});

test("several supertypes extend the first and redeclare the others' features", () => {
  const hybrid = definition("Demo::Hybrid", "partDef", [], [
    specialization("specializes", "Demo::Vehicle", "Vehicle"),
    specialization("subsets", "Demo::Electric", "Electric"),
    specialization("redefines", "Demo::Vehicle", "Vehicle"),
  ]);
  const source = renderModule([
    hybrid,
    definition("Demo::Vehicle", "partDef", [
      feature("mass", { type: facts({ primitive: "Real" }) }),
    ]),
    definition("Demo::Electric", "partDef", [
      feature("charge", { type: facts({ primitive: "Real" }), owner: "Demo::Electric" }),
    ]),
  ]);
  assert.ok(source.includes("class Hybrid extends Vehicle"));
  // The second base's features are reachable as re-declared getters.
  const hybridAt = source.indexOf("class Hybrid");
  assert.ok(hybridAt < source.indexOf("get charge()", hybridAt));
});

test("a base implied by a sibling base is left implicit", () => {
  const vehicle = definition("Demo::Vehicle");
  const electric = definition("Demo::Electric", "partDef", [], [
    specialization("specializes", "Demo::Vehicle", "Vehicle"),
  ]);
  const hybrid = definition("Demo::Hybrid", "partDef", [], [
    specialization("specializes", "Demo::Vehicle", "Vehicle"),
    specialization("specializes", "Demo::Electric", "Electric"),
  ]);
  const source = renderModule([hybrid, electric, vehicle]);
  assert.ok(source.includes("class Hybrid extends Electric"));
  assert.ok(!source.includes("left out"));
});

test("an unlinearizable base is reported and its features still reachable", () => {
  const left = definition("Demo::Left");
  const right = definition("Demo::Right");
  const one = definition("Demo::One", "partDef", [], [
    specialization("specializes", "Demo::Left", "Left"),
    specialization("specializes", "Demo::Right", "Right"),
  ]);
  const two = definition("Demo::Two", "partDef", [
    feature("extra", { type: facts({ primitive: "Real" }), owner: "Demo::Two" }),
  ], [
    specialization("specializes", "Demo::Right", "Right"),
    specialization("specializes", "Demo::Left", "Left"),
  ]);
  const both = definition("Demo::Both", "partDef", [], [
    specialization("specializes", "Demo::One", "One"),
    specialization("specializes", "Demo::Two", "Two"),
  ]);
  const source = renderModule([both, one, two, left, right]);
  assert.ok(source.includes("class Both extends One"));
  assert.ok(source.includes("// specializes Demo::Two, left out:"));
  // Its features still surface as re-declared getters.
  const bothAt = source.indexOf("class Both");
  assert.ok(bothAt < source.indexOf("get extra()", bothAt));
});

test("a base only a dropped base implied is reported", () => {
  const left = definition("Demo::Left");
  const right = definition("Demo::Right");
  const extra = definition("Demo::Extra");
  const one = definition("Demo::One", "partDef", [], [
    specialization("specializes", "Demo::Left", "Left"),
    specialization("specializes", "Demo::Right", "Right"),
  ]);
  const two = definition("Demo::Two", "partDef", [], [
    specialization("specializes", "Demo::Right", "Right"),
    specialization("specializes", "Demo::Left", "Left"),
    specialization("specializes", "Demo::Extra", "Extra"),
  ]);
  const both = definition("Demo::Both", "partDef", [], [
    specialization("specializes", "Demo::One", "One"),
    specialization("subsets", "Demo::Extra", "Extra"),
    specialization("specializes", "Demo::Two", "Two"),
  ]);
  const source = renderModule([both, one, two, left, right, extra]);
  assert.ok(source.includes("class Both extends One"));
  assert.ok(source.includes("// subsets Demo::Extra, left out"));
  assert.ok(source.includes("// specializes Demo::Two, left out"));
});

test("an unmapped base is reported", () => {
  const car = definition("Demo::Car", "partDef", [], [
    specialization("specializes", "Base::Vehicle", "Vehicle"),
  ]);
  const source = renderModule([car]);
  assert.ok(source.includes("// specializes Base::Vehicle, which has no generated class"));
});
