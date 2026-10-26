// Standard queries: the SysML v2 API & Services Query over a model.

import assert from "node:assert/strict";
import { before, test } from "node:test";
import { QueryElement, connect } from "../src/node/index.js";
import { buildQuery } from "../src/core/query.js";
import { PrimitiveOperator } from "../src/generated/sysml_pb.js";
import { useServiceBinary } from "./support/service.js";
import { fakeConnection } from "./support/fake.js";

before(() => {
  useServiceBinary();
});

const MODEL = `package Demo {
    abstract part def Vehicle {
        attribute mass;
    }
    part def Wheel;
    part vehicle : Vehicle {
        part wheels : Wheel[4];
    }
}
`;

const COOKBOOK = {
  "@type": "Query",
  where: {
    "@type": "PrimitiveConstraint",
    operator: "=",
    property: "@type",
    value: ["PartUsage"],
  },
};

test("the cookbook payload selects the part usages", async () => {
  await using connection = await connect();
  const model = await connection.loads(MODEL);
  const elements = await model.query({ payload: COOKBOOK });
  assert.ok(elements.length >= 1);
  assert.ok(elements.every((element) => element instanceof QueryElement));
  assert.ok(elements.some((element) => element.get("name") === "vehicle"));
});

test("scope and select narrow the answer", async () => {
  await using connection = await connect();
  const model = await connection.loads(MODEL);
  const elements = await model.query({
    scope: [{ "@id": "Demo::vehicle" }],
    where: {
      "@type": "PrimitiveConstraint",
      operator: "=",
      property: "@type",
      value: ["PartUsage"],
    },
  });
  assert.ok(elements.length >= 1);
});

test("composite and inverse constraints evaluate", async () => {
  await using connection = await connect();
  const model = await connection.loads(MODEL);
  const elements = await model.query({
    where: {
      "@type": "CompositeConstraint",
      operator: "or",
      constraint: [
        { "@type": "PrimitiveConstraint", operator: "=", property: "@type", value: ["PartUsage"] },
        { "@type": "PrimitiveConstraint", operator: "=", property: "@type", value: ["PartDefinition"] },
      ],
    },
  });
  assert.ok(elements.length >= 2);
});

test("reported elements become records", async () => {
  await using connection = await connect();
  const model = await connection.loads(MODEL);
  const elements = await model.query({ payload: COOKBOOK });
  const vehicle = elements.find((element) => element.get("name") === "vehicle");
  assert.ok(vehicle !== undefined);
  assert.equal(vehicle.get("@type"), "PartUsage");
  assert.equal(typeof vehicle.asDict(), "object");
  assert.match(vehicle.toString(), /vehicle|PartUsage/);
});

test("a query that asks for nothing answers every element", async () => {
  await using connection = await connect();
  const model = await connection.loads(MODEL);
  const everything = await model.query();
  const vehicle = await model.symbol("Vehicle");
  assert.ok(everything.some((element) => element.id === vehicle.id));
});

test("an OSLC-only query still sends just the OSLC text", async () => {
  await using connection = await connect();
  const model = await connection.loads(MODEL);
  const elements = await model.query({ oslc: 'sysml:name="vehicle"' });
  assert.ok(elements.length >= 1);
  assert.ok(elements.some((element) => element.get("name") === "vehicle"));
});

// An empty oslc is no OSLC at all: a structured ask beside it sends its Query.
test("an empty oslc still sends the structured query", async () => {
  let seen: { oslcQuery?: string; query?: unknown } | undefined;
  const connection = await fakeConnection(["query"], (method, input) => {
    assert.equal(method, "Query");
    seen = input as { oslcQuery?: string; query?: unknown };
    return { elements: [] };
  });
  await connection.query("hash", {
    oslc: "",
    where: { "@type": "PrimitiveConstraint", operator: "=", property: "@type", value: ["PartUsage"] },
  });
  assert.ok(seen !== undefined);
  assert.equal(seen.oslcQuery, "");
  assert.ok(seen.query !== undefined);
  await connection.close();

  let onlyOslc: { oslcQuery?: string; query?: unknown } | undefined;
  const again = await fakeConnection(["query"], (method, input) => {
    assert.equal(method, "Query");
    onlyOslc = input as { oslcQuery?: string; query?: unknown };
    return { elements: [] };
  });
  await again.query("hash", { oslc: 'sysml:name="vehicle"' });
  assert.ok(onlyOslc !== undefined);
  assert.equal(onlyOslc.oslcQuery, 'sysml:name="vehicle"');
  assert.equal(onlyOslc.query, undefined);
  await again.close();
});

// The seven operator literals of API & Services §7.1.4 reach the wire as their enums.
test("every standard operator translates", () => {
  const cases: [string, string, unknown, PrimitiveOperator, string[]][] = [
    ["<=", "multiplicityLower", 2, PrimitiveOperator.LESS_EQUAL, ["2"]],
    [">=", "multiplicityLower", 4, PrimitiveOperator.GREATER_EQUAL, ["4"]],
    ["<", "multiplicityUpper", "*", PrimitiveOperator.LESS, ["*"]],
    ["<", "multiplicityUpper", ["3"], PrimitiveOperator.LESS, ["3"]],
    [">", "multiplicityUpper", 1.5, PrimitiveOperator.GREATER, ["1.5"]],
    ["in", "@type", ["PartUsage", "AttributeUsage"], PrimitiveOperator.IN, ["PartUsage", "AttributeUsage"]],
    ["instanceOf", "@type", "Usage", PrimitiveOperator.INSTANCE_OF, ["Usage"]],
    ["instanceOf", "@type", ["Usage", "Definition"], PrimitiveOperator.INSTANCE_OF, ["Usage", "Definition"]],
  ];
  for (const [operator, property, value, wire, values] of cases) {
    const query = buildQuery({ where: { operator, property, value } });
    const where = query.where;
    assert.ok(where);
    assert.equal(where.constraint.case, "primitive");
    const primitive = where.constraint.value as { operator: PrimitiveOperator; value: string[] };
    assert.equal(primitive.operator, wire, operator);
    assert.deepEqual(primitive.value, values, operator);
  }
});

// Each operator's operand shape is checked here, not sent to be refused.
test("an operand of the wrong shape is refused", () => {
  const cases: [Record<string, unknown>, RegExp][] = [
    [{ operator: "<=", property: "multiplicityLower", value: [1, 2] }, /<= on "multiplicityLower" compares against one value, not a list of 2/],
    [{ operator: ">=", property: "multiplicityLower" }, />= on "multiplicityLower" has no value/],
    [{ operator: "<", property: "multiplicityLower", value: [] }, /compares against one value, not a list of 0/],
    [{ operator: "in", property: "@type", value: "PartUsage" }, /in on "@type" compares against a list of values/],
    [{ operator: "in", property: "@type", value: [] }, /in on "@type" has no values/],
    [{ operator: "instanceOf", property: "@type", value: 7 }, /instanceOf on "@type" compares against a metaclass name, not 7/],
    [{ operator: "instanceOf", property: "@type", value: ["Usage", ""] }, /compares against a metaclass name, not ""/],
    [{ operator: "=<", property: "multiplicityLower", value: 1 }, /unknown primitive operator "=<"; expected one of <, <=, =, >, >=, in, instanceOf/],
    [{ "@type": "CompositeConstraint", operator: "and", constraint: [{ operator: "=", property: "name", value: "x" }] }, /combines at least two constraints, got one/],
  ];
  for (const [where, message] of cases) {
    assert.throws(() => buildQuery({ where }), message);
  }
});

test("orderBy translates from the payload and the keyword", () => {
  const query = buildQuery({ payload: { "@type": "Query", orderBy: ["@type", "name"] } });
  assert.deepEqual(query.orderBy, ["@type", "name"]);
  assert.deepEqual(buildQuery({ orderBy: ["@type", "name"] }).orderBy, ["@type", "name"]);
  assert.deepEqual(buildQuery({ payload: { scope: ["Demo"] } }).orderBy, []);
  assert.throws(() => buildQuery({ payload: { orderBy: 7 } }), /orderBy is a list/);
  assert.throws(() => buildQuery({ payload: { orderBy: [42] } }), /a selected property is a name/);
  assert.throws(() => buildQuery({ payload: { orderBy: ["name"] }, orderBy: ["name"] }), /not both/);
});

test("the new operators evaluate", async () => {
  await using connection = await connect();
  const model = await connection.loads(MODEL);
  const atLeastFour = await model.query({ where: { operator: ">=", property: "multiplicityLower", value: 4 } });
  assert.deepEqual(atLeastFour.map((e) => e.id), ["Demo::vehicle::wheels"]);
  const usages = await model.query({ where: { operator: "in", property: "@type", value: ["PartUsage", "AttributeUsage"] } });
  assert.deepEqual(usages.map((e) => e.id), ["Demo::Vehicle::mass", "Demo::vehicle", "Demo::vehicle::wheels"]);
  const features = await model.query({ where: { operator: "instanceOf", property: "@type", value: "Feature" } });
  assert.deepEqual(features.map((e) => e.id), usages.map((e) => e.id));
  const none = await model.query({ where: { operator: "<=", property: "multiplicityUpper", value: 3 } });
  assert.deepEqual(none, []);
});

test("orderBy sorts the answer", async () => {
  await using connection = await connect();
  const model = await connection.loads(MODEL);
  const byName = await model.query({ select: ["name"], orderBy: ["name"] });
  assert.deepEqual(byName.map((e) => e.id), [
    "Demo", "Demo::Vehicle", "Demo::Wheel", "Demo::Vehicle::mass", "Demo::vehicle", "Demo::vehicle::wheels",
  ]);
  const byTypeThenName = await model.query({ payload: { "@type": "Query", select: ["name"], orderBy: ["@type", "name"] } });
  assert.deepEqual(byTypeThenName.map((e) => e.type), [
    "AttributeUsage", "Package", "PartDefinition", "PartDefinition", "PartUsage", "PartUsage",
  ]);
  await assert.rejects(model.query({ orderBy: ["colour"] }), /unknown query property/);
});
