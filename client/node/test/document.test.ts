// Named document queries and rendering, against the real service and the
// renderer's own fixtures.

import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { before, test } from "node:test";
import {
  DocumentEvent,
  DocumentState,
  DocumentVerdict,
  ElementRef,
  ObjectRef,
  connect,
} from "../src/node/index.js";
import { repoRoot, useServiceBinary } from "./support/service.js";

before(() => {
  useServiceBinary();
});

const VERDICT_FIXTURE = join(
  repoRoot,
  "internal/doc/docrender/testdata/verdict_report.sysml",
);
const OBJECT_FIXTURE = join(
  repoRoot,
  "tests/grpc/testdata/conformance/document_query_object_by_path.sysml",
);
const STATE_FIXTURE = join(repoRoot, "tests/grpc/testdata/conformance/document_query_states.sysml");
const REPORT_FIXTURE = join(repoRoot, "internal/doc/docrender/testdata/telescope_report.sysml");
const REPORT_MD = join(repoRoot, "internal/doc/docrender/testdata/telescope_report.golden.md");
const REPORT_HTML = join(
  repoRoot,
  "internal/doc/docrender/testdata/telescope_report.golden.html",
);

test("states and events over the object instantiate built", async () => {
  await using connection = await connect();
  const model = await connection.loads(readFileSync(STATE_FIXTURE, "utf8"));
  await model.instantiate("Lamps::lamp");
  const lamp = { root: new ObjectRef({ path: "Lamps::lamp" }) };
  const states = await model.runDocumentQuery("Lamps::CurrentStates", lamp);
  const off = await model.runDocumentQuery("Lamps::Off");
  const steps = await model.runDocumentQuery("Lamps::Steps", lamp);
  assert.deepEqual(states.columns, ["machine", "statePath", "region"]);
  assert.equal(states.length, 1);
  const state = states.rows[0].state;
  assert.ok(state instanceof DocumentState);
  assert.equal(state.machine, "lp");
  assert.equal(state.name, "off");
  assert.equal(state.path, "off");
  assert.equal(state.object.path, "Lamps::lamp");
  assert.deepEqual(
    off.rows.map((row) => row.object?.toString()),
    ["Lamps::lamp"],
  );
  const entered = steps.rows[0].event;
  assert.ok(entered instanceof DocumentEvent);
  assert.equal(entered.kind, "entry");
  assert.equal(entered.state, "off");
});

test("a verdicts query checks the element as declared", async () => {
  await using connection = await connect();
  const model = await connection.loads(readFileSync(VERDICT_FIXTURE, "utf8"));
  const result = await model.runDocumentQuery("Garage::Checks", {
    root: new ElementRef("Garage::car"),
  });
  assert.deepEqual(result.columns, ["path", "name", "verdict", "reason"]);
  const byText = new Map(
    result.rows.map((row) => [`${row.verdict?.text ?? ""} on ${row.verdict?.path ?? ""}`, row]),
  );
  const massOk = byText.get("assert constraint massOk on Garage::car");
  assert.ok(massOk?.verdict instanceof DocumentVerdict);
  assert.equal(massOk.verdict.status, "holds");
  assert.equal(massOk.verdict.kind, "constraint");
  assert.deepEqual(massOk.cell(0), ["Garage::car"]);
  const powerLow = byText.get("assert constraint powerLow on Garage::car.engine")?.verdict;
  assert.equal(powerLow?.status, "violated");
  const fits = byText.get("assert constraint fits on Garage::car")?.verdict;
  assert.equal(fits?.status, "undecided");
  const satisfied = byText.get("satisfy strongEngine by car.engine on Garage::car.engine")?.verdict;
  assert.ok(satisfied instanceof DocumentVerdict);
  assert.equal(satisfied.kind, "satisfaction");
  assert.equal(satisfied.status, "holds");
});

test("a query binds the object instantiate built by path", async () => {
  await using connection = await connect();
  const model = await connection.loads(readFileSync(OBJECT_FIXTURE, "utf8"));
  await model.instantiate("Garage::car");
  const result = await model.runDocumentQuery("Garage::Parts", {
    root: new ObjectRef({ path: "car" }),
  });
  assert.deepEqual(result.columns, ["name", "pressure"]);
  assert.equal(result.rows.length, 2);
  assert.deepEqual(
    result.rows.map((row) => row.cell(0)),
    [["wheels[1]"], ["wheels[2]"]],
  );
  assert.deepEqual(
    result.rows.map((row) => row.cell(1)),
    [[30n], [30n]],
  );
  const first = result.rows[0].object;
  assert.ok(first instanceof ObjectRef);
  assert.equal(first.element?.id, "Garage::Car::wheels");
});

test("a query binds the object instantiate built by id", async () => {
  await using connection = await connect();
  const model = await connection.loads(readFileSync(OBJECT_FIXTURE, "utf8"));
  const car = await model.instantiate("Garage::car");
  const result = await model.runDocumentQuery("Garage::Parts", {
    root: new ObjectRef({ id: car.root.id }),
  });
  assert.equal(result.rows.length, 2);
  assert.ok(result.rows.every((row) => row.object instanceof ObjectRef));
});

test("renderDocument answers the golden markdown", async () => {
  await using connection = await connect();
  const model = await connection.loads(readFileSync(REPORT_FIXTURE, "utf8"));
  const markdown = await model.renderDocument("Observatory::MassReport");
  assert.equal(markdown, readFileSync(REPORT_MD, "utf8"));
});

test("renderDocument answers the golden html", async () => {
  await using connection = await connect();
  const model = await connection.loads(readFileSync(REPORT_FIXTURE, "utf8"));
  const html = await model.renderDocument("Observatory::MassReport", { form: "html" });
  assert.equal(html, readFileSync(REPORT_HTML, "utf8"));
});
