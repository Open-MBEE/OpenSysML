// Standard queries: the SysML v2 API & Services Query over a model.

import assert from "node:assert/strict";
import { before, test } from "node:test";
import { QueryElement, connect } from "../src/node/index.js";
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
  assert.equal(seen?.oslcQuery, "");
  assert.ok(seen?.query !== undefined);
  await connection.close();

  let onlyOslc: { oslcQuery?: string; query?: unknown } | undefined;
  const again = await fakeConnection(["query"], (method, input) => {
    assert.equal(method, "Query");
    onlyOslc = input as { oslcQuery?: string; query?: unknown };
    return { elements: [] };
  });
  await again.query("hash", { oslc: 'sysml:name="vehicle"' });
  assert.equal(onlyOslc?.oslcQuery, 'sysml:name="vehicle"');
  assert.equal(onlyOslc?.query, undefined);
  await again.close();
});
