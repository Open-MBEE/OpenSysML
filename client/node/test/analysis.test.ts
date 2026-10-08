// Analysis cases and parameter sweeps against the real service.

import assert from "node:assert/strict";
import { before, test } from "node:test";
import { InvalidRequestError, WrongKindError, connect } from "../src/node/index.js";
import { useServiceBinary } from "./support/service.js";

before(() => {
  useServiceBinary();
});

const MODEL = `package An {
    private import ScalarValues::*;

    part def Ship {
        attribute cost : Real default = 5.0;
        attribute other : Real default = 7.0;
    }

    calc def Sum {
        in a : Real;
        in b : Real;
        return : Real = a + b;
    }

    analysis def CostAnalysis {
        subject s : Ship;
        in limit : Real = 20.0;
        out total : Real = Sum(s.cost, s.other);
        objective affordable {
            require constraint { total <= limit }
        }
    }

    part ship : Ship;
    part barge : Ship {
        attribute :>> cost = 30.0;
    }

    analysis shipCost : CostAnalysis {
        subject s = ship;
    }

    analysis plain {
        out x : Real = 1.0 + 2.0;
    }
}
`;

test("a usage binding its subject computes its outputs", async () => {
  await using connection = await connect();
  const model = await connection.loads(MODEL);
  const result = await model.runAnalysis("An::shipCost");
  assert.deepEqual(result.outputs.get("total"), { kind: "real", value: 12.0 });
});

test("a case without objective has no verdict", async () => {
  await using connection = await connect();
  const model = await connection.loads(MODEL);
  const result = await model.runAnalysis("An::plain");
  assert.deepEqual(result.outputs.get("x"), { kind: "real", value: 3.0 });
  assert.equal(result.verdicts.length, 0);
  assert.equal(result.satisfied, true);
});

test("a definition runs on the subject named", async () => {
  await using connection = await connect();
  const model = await connection.loads(MODEL);
  const result = await model.runAnalysis("An::CostAnalysis", { subject: "An::barge" });
  assert.deepEqual(result.outputs.get("total"), { kind: "real", value: 37.0 });
});

test("arguments bind the inputs", async () => {
  await using connection = await connect();
  const model = await connection.loads(MODEL);
  const result = await model.runAnalysis("An::shipCost", { arguments: [10.0] });
  assert.deepEqual(result.outputs.get("total"), { kind: "real", value: 12.0 });
});

test("a wrong kind raises WrongKindError", async () => {
  await using connection = await connect();
  const model = await connection.loads(MODEL);
  await assert.rejects(() => model.runAnalysis("An::Ship"), WrongKindError);
  await assert.rejects(() => model.calc("An::shipCost"), WrongKindError);
  await assert.rejects(() => model.verifyConstraint("An::Sum"), WrongKindError);
});

test("calc invokes the calculation", async () => {
  await using connection = await connect();
  const model = await connection.loads(MODEL);
  const result = await model.calc("An::Sum", { arguments: [2.0, 3.0] });
  assert.deepEqual(result.value, { kind: "real", value: 5.0 });
});

const SWEEP = `package Sw {
    private import ScalarValues::*;

    part def Ship {
        attribute cost : Real default = 5.0;
    }

    calc def Twice {
        in n : Integer;
        return : Integer = n * 2;
    }

    calc def Ratio {
        in a : Real;
        in b : Real;
        return : Real = a / b;
    }

    analysis def CostAnalysis {
        subject s : Ship;
        in limit : Real = 20.0;
        out total : Real = s.cost * limit;
        objective affordable {
            require constraint { total <= 100.0 }
        }
    }

    part ship : Ship;
}
`;

test("an integer range steps by one", async () => {
  await using connection = await connect();
  const model = await connection.loads(SWEEP);
  const table = await model.runSweep("Sw::Twice", { n: [1n, 3n] });
  assert.equal(table.length, 3);
  assert.deepEqual(
    table.rows.map((row) => row.outputs.get("result")),
    [2n, 4n, 6n].map((n) => ({ kind: "int" as const, value: n })),
  );
  assert.equal(table.sampled, false);
});

test("a stated step advances by it", async () => {
  await using connection = await connect();
  const model = await connection.loads(SWEEP);
  const table = await model.runSweep("Sw::Twice", { n: [1n, 5n, 2n] });
  assert.equal(table.length, 3);
  assert.deepEqual(
    table.rows.map((row) => row.inputs.get("n")),
    [1n, 3n, 5n].map((n) => ({ kind: "int" as const, value: n })),
  );
});

test("several ranges run their cartesian product", async () => {
  await using connection = await connect();
  const plus = `package Plus { private import ScalarValues::*; calc def Sum { in a : Integer; in b : Integer; return : Integer = a + b; } }`;
  const m = await connection.loads(plus);
  const table = await m.runSweep("Plus::Sum", { a: [1n, 2n], b: [10n, 11n] });
  assert.equal(table.length, 4);
});

test("a failing run is a row and the table goes on", async () => {
  await using connection = await connect();
  const model = await connection.loads(SWEEP);
  const table = await model.runSweep("Sw::Ratio", { b: [-1.0, 1.0] }, { arguments: [6.0] });
  assert.equal(table.length, 3);
  assert.equal(table.rows[1].failed, true);
  assert.ok(table.rows[1].error !== "");
  assert.equal(table.rows[0].failed, false);
});

test("an analysis case row carries its verdict", async () => {
  await using connection = await connect();
  const model = await connection.loads(SWEEP);
  const table = await model.runSweep("Sw::CostAnalysis", { limit: [1.0, 3.0] }, { subject: "Sw::ship" });
  assert.ok(table.rows.every((row) => row.verdicts.length > 0));
});

test("a range of the wrong arity is refused before it is sent", async () => {
  await using connection = await connect();
  const model = await connection.loads(SWEEP);
  await assert.rejects(() => model.runSweep("Sw::Twice", { n: [1n] as never }), InvalidRequestError);
});
