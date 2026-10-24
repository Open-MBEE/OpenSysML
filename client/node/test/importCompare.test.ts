// The data import and the results comparison against the real service.

import assert from "node:assert/strict";
import { before, test } from "node:test";
import { ExecutionError, InvalidRequestError, connect } from "../src/node/index.js";
import { useServiceBinary } from "./support/service.js";

before(() => {
  useServiceBinary();
});

const VEHICLES = `package V {
    private import ScalarValues::*;
    part def Vehicle {
        attribute count : Integer default 1;
        attribute label : String default "plain";
    }
    part vehicle : Vehicle;
    part car : Vehicle {
        attribute :>> count = 2;
    }
}`;

const PROBE = `package Cfg {
    private import ScalarValues::*;
    part def Probe {
        attribute total : Real = 1.0;
        action steady { first start; then assign total := 4.0; then done; }
    }
    action def 'Group 1' { part target : Probe; perform action run ::> target.steady; }
}`;

const RESULTS = JSON.stringify({
  source: "probe.xmi",
  configurations: [
    {
      id: "_c1",
      name: "Cfg::'Group 1'",
      runs: 2,
      target: "target",
      behavior: "run",
      observables: ["total"],
      snapshots: [
        { id: "_r1", values: { total: 4.0 } },
        { id: "_r2", values: { total: 6.0 } },
      ],
    },
    { id: "_cn", name: "Cfg::Nothing", observables: [], snapshots: [] },
  ],
});

test("importData sets each row's values and answers the edited model", async () => {
  await using connection = await connect();
  const model = await connection.loads(VEHICLES);
  const result = await model.importData([
    { name: "fleet.csv", content: "element,count,label\nV::vehicle,7,fast\nV::car,9,\n" },
    { name: "labels.txt", content: '[{"element":"V::car","label":"slow"}]', format: "json" },
  ]);
  assert.equal(result.files.length, 2);
  assert.equal(result.files[0]?.name, "fleet.csv");
  assert.equal(result.files[0]?.values, 3n);
  assert.equal(result.files[0]?.elements, 2n);
  assert.deepEqual(
    result.files.flatMap((file) => file.assignments.map((a) => a.description)),
    [
      "V::vehicle::count = 7 (redefines V::Vehicle::count)",
      'V::vehicle::label = "fast" (redefines V::Vehicle::label)',
      "V::car::count = 9",
      'V::car::label = "slow" (redefines V::Vehicle::label)',
    ],
  );
  assert.equal(result.files[0]?.assignments[0]?.redefines, "V::Vehicle::count");
  assert.notEqual(result.files[0]?.assignments[0]?.where, "");
  assert.match(result.content, /attribute :>> count = 7;/);
  assert.match(result.content, /attribute :>> label = "slow";/);
  assert.equal(result.documents.length, 1);
  assert.notEqual(result.modelHash, "");
  assert.notEqual(result.modelHash, model.hash);
  const imported = connection.model(result.modelHash);
  const car = await imported.symbol("V::car");
  assert.equal(car.name, "car");
});

test("importData dry run reports the assignments and leaves the model as it is", async () => {
  await using connection = await connect();
  const model = await connection.loads(VEHICLES);
  const result = await model.importData([{ name: "fleet.tsv", content: "element\tcount\nV::vehicle\t7\n" }], {
    dryRun: true,
  });
  assert.deepEqual(
    result.files[0]?.assignments.map((a) => a.description),
    ["V::vehicle::count = 7 (redefines V::Vehicle::count)"],
  );
  assert.equal(result.modelHash, "");
  assert.equal(result.content, "");
  assert.deepEqual(result.documents, []);
});

test("importData refuses a row naming no element and refuses an empty request", async () => {
  await using connection = await connect();
  const model = await connection.loads(VEHICLES);
  await assert.rejects(
    model.importData([{ name: "fleet.csv", content: "element,count\nV::car,9\nV::Nowhere,1\n" }]),
    (error: unknown) => error instanceof ExecutionError && /V::Nowhere/.test(error.message),
  );
  await assert.rejects(model.importData([]), InvalidRequestError);
});

test("compareResults tables the tool's distributions beside the runs'", async () => {
  await using connection = await connect();
  const model = await connection.loads(PROBE);
  const comparison = await model.compareResults(RESULTS, { seed: 7 });
  assert.equal(comparison.source, "probe.xmi");
  assert.equal(comparison.holds, false);
  assert.equal(comparison.configurations.length, 2);
  const [group, nothing] = comparison.configurations;
  assert.equal(group.name, "Cfg::'Group 1'");
  assert.equal(group.status, "holds");
  assert.equal(group.runs, 2n);
  assert.equal(group.completed, 2n);
  assert.equal(group.storedRuns, 2n);
  assert.equal(group.seed, 7n);
  assert.equal(group.rows.length, 2);
  assert.equal(group.observables.length, 1);
  const total = group.observables[0];
  assert.equal(total.stored, "total");
  assert.equal(total.feature, "target.total");
  assert.equal(total.compared, true);
  assert.ok(total.tool && total.run && total.difference);
  assert.equal(total.tool.count, 2n);
  assert.equal(total.tool.mean, 5.0);
  assert.equal(total.run.mean, 4.0);
  assert.equal(total.difference.mean, "-20.0%");
  assert.ok(group.lines.length > 0);
  assert.equal(nothing.status, "unresolved");
  assert.notEqual(nothing.unresolved, "");
  assert.ok(nothing.lines.some((line) => line.includes("Cfg::Nothing")));
});

test("compareResults selects configurations by name and refuses bad options", async () => {
  await using connection = await connect();
  const model = await connection.loads(PROBE);
  const comparison = await model.compareResults(RESULTS, { actions: ["Group 1"], runs: 1, seed: 3 });
  assert.deepEqual(
    comparison.configurations.map((c) => [c.name, c.status, c.runs]),
    [["Cfg::'Group 1'", "holds", 1n]],
  );
  await assert.rejects(model.compareResults(RESULTS, { runs: -1 }), InvalidRequestError);
  await assert.rejects(model.compareResults("not json"), InvalidRequestError);
});
