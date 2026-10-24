import assert from "node:assert/strict";
import { join } from "node:path";
import { before, test } from "node:test";
import { ExecutionError, InvalidRequestError, connect } from "../src/node/index.js";
import { repoRoot, useServiceBinary } from "./support/service.js";

before(() => {
  useServiceBinary();
});

const TANK_FIXTURE = join(repoRoot, "conformance/fixtures/tank.sysml");

test("checkAction witnesses a violated property and a divergent value", async () => {
  await using connection = await connect();
  const model = await connection.load(TANK_FIXTURE);
  const result = await model.checkAction("Plant::Tank::fill", {
    performer: "Plant::tank",
    properties: ["Plant::Tank::low"],
    diverge: ["this.level"],
    witness: true,
  });
  assert.equal(result.verdict, "fails");
  assert.ok(result.lines.length > 0);
  assert.ok(result.search !== undefined);
  assert.equal(result.search.states, 11);
  assert.equal(result.search.moves, 10);
  assert.equal(result.search.maxDepth, 6);
  assert.deepEqual(result.search.boundsHit, []);
  assert.equal(result.violations.length, 1);
  const [violation] = result.violations;
  assert.equal(violation.kind, "property");
  assert.equal(violation.property, "Plant::Tank::low");
  assert.equal(violation.depth, 3);
  assert.ok(violation.witness !== undefined);
  assert.ok(violation.witness.schedule.startsWith("replay"));
  assert.notEqual(violation.witness.fileName, "");
  assert.ok(violation.witness.fileContent.includes("\n"));
  assert.equal(result.divergences.length, 1);
  assert.equal(result.divergences[0].feature, "this.level");
  assert.deepEqual(
    result.divergences[0].values.map((v) => v.value),
    ["1", "2"],
  );
  assert.equal(result.standing.engine, "check");
  assert.ok(result.standing.strength !== "");
});

test("checkAction holds a property every schedule keeps", async () => {
  await using connection = await connect();
  const model = await connection.load(TANK_FIXTURE);
  const held = await model.checkAction("Plant::Tank::fill", {
    performer: "Plant::tank",
    properties: ["Plant::Tank::capped"],
    diverge: ["this.capacity"],
  });
  assert.equal(held.verdict, "holds");
  assert.deepEqual(held.violations, []);
  assert.deepEqual(held.divergences, []);
  const bounded = await model.checkAction("Plant::Tank::fill", {
    performer: "Plant::tank",
    properties: ["Plant::Tank::capped"],
    depth: 2,
  });
  assert.equal(bounded.verdict, "undecided");
  assert.ok(bounded.search !== undefined);
  assert.ok(bounded.search.boundsHit.includes("depth"));
});

test("checkAction answers a missing behavior in band and refuses a negative bound", async () => {
  await using connection = await connect();
  const model = await connection.load(TANK_FIXTURE);
  const missing = await model.checkAction("Plant::Tank::drain").then(
    () => undefined,
    (reason: unknown) => reason,
  );
  assert.ok(missing instanceof ExecutionError);
  assert.match(missing.message, /Plant::Tank::drain/);
  const negative = await model.checkAction("Plant::Tank::fill", { depth: -1 }).then(
    () => undefined,
    (reason: unknown) => reason,
  );
  assert.ok(negative instanceof InvalidRequestError);
  assert.match(negative.message, /check_depth/);
});
