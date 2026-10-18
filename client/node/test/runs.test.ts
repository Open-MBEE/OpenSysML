// Repeated runs, the reflective self-check and run recording against the real service.

import assert from "node:assert/strict";
import { before, test } from "node:test";
import { ExecutionError, InvalidRequestError, connect } from "../src/node/index.js";
import { useServiceBinary } from "./support/service.js";

before(() => {
  useServiceBinary();
});

const MODEL = `package MC {
    private import ScalarValues::*;
    private import RandomFunctions::*;
    action def Settle { out t : Real; first start; then assign t := uniform(1.0, 5.0); then done; }
    part def Probe { attribute t : Real; action settle { first start; then assign t := uniform(1.0, 5.0); then done; } }
    individual def probe :> Probe;
    analysis def Mc :> Simulation::MonteCarlo {
        subject analysed : Probe;
        perform action run ::> analysed.settle;
        attribute :>> observed : Real = analysed.t;
        return Mean : Real = mean;
    }
    part def P { attribute m : Real = 2.0; }
    part p : P;
    analysis def Cost { subject s : P; return total : Real = s.m * 3.0; }
}`;

test("runRuns tables an action's runs and their distribution", async () => {
  await using connection = await connect();
  const model = await connection.loads(MODEL);
  const runs = await model.runRuns({ action: "MC::Settle", runs: 4, seed: 7, observe: ["t"] });
  assert.equal(runs.length, 4);
  assert.deepEqual(runs.observables, ["t"]);
  assert.equal(runs.distributions.length, 1);
  assert.equal(runs.distributions[0]?.count, 4n);
  assert.equal(runs.distributions[0]?.numeric, true);
  assert.equal(runs.seed, 7n);
  assert.match(runs.toString(), /^t: 4 run\(s\)/);
});

test("runRuns concludes a Monte Carlo case over the sample", async () => {
  await using connection = await connect();
  const model = await connection.loads(MODEL);
  const sample = await model.runRuns({ analysis: "MC::Mc", subject: "MC::probe", runs: 3, seed: 1 });
  assert.equal(sample.length, 3);
  assert.equal(sample.unconcluded, "");
  assert.ok(sample.outputs.has("Mean"));
});

test("runRuns refuses random runs without a seed and an unknown action in band", async () => {
  await using connection = await connect();
  const model = await connection.loads(MODEL);
  await assert.rejects(model.runRuns({ action: "MC::Settle", runs: 1 }), InvalidRequestError);
  await assert.rejects(model.runRuns({ action: "MC::Nope", runs: 1, draws: "min" }), ExecutionError);
});

test("selfCheck applies the reflective constraints and refuses an unknown rule package", async () => {
  await using connection = await connect();
  const model = await connection.loads(MODEL);
  const check = await model.selfCheck();
  assert.ok(check.elements > 0n && check.checks > 0n);
  assert.match(check.summary, /^Self-model check: /);
  assert.ok(check.applications.size > 0);
  await assert.rejects(model.selfCheck(["Nope"]), ExecutionError);
});

test("recordRun records a run and a sample into the model", async () => {
  await using connection = await connect();
  const model = await connection.loads(MODEL);
  const recording = await model.recordRun("MC::Cost", { subject: "MC::p" });
  assert.ok(recording.run !== undefined);
  assert.equal(recording.package, "Records");
  assert.deepEqual(recording.records, ["Records::Cost_run1"]);
  assert.equal(recording.documents.length, 1);
  assert.notEqual(recording.modelHash, "");
  const sample = await model.recordRun("MC::Mc", { subject: "MC::probe", runs: 2, seed: 3 });
  assert.ok(sample.sample !== undefined);
  assert.equal(sample.records.length, 3);
  await assert.rejects(model.recordRun("MC::Nope"), ExecutionError);
});
