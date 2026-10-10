// Verification: constraints, requirements, satisfaction and validation.

import { create } from "@bufbuild/protobuf";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { before, test } from "node:test";
import {
  VerifyRequirementResponseSchema,
  VerifySatisfactionResponseSchema,
} from "../src/generated/sysml_pb.js";
import {
  MissingCapabilityError,
  WrongKindError,
  connect,
  explainVerdict,
  verdictEvaluated,
  verdictHolds,
} from "../src/node/index.js";
import { fakeConnection } from "./support/fake.js";
import { repoRoot, useServiceBinary } from "./support/service.js";

const CASES_FIXTURE = join(repoRoot, "conformance", "fixtures", "verification_cases.sysml");

before(() => {
  useServiceBinary();
});

const QUESTIONS = `package P {
    private import ScalarValues::*;
    part hg { attribute power : Real; }
    attribute d : Real;
    assert constraint lemma { hg.power * hg.power >= 0.0 }
    assert constraint bad { hg.power * d <= hg.power }
    assert constraint never { hg.power > 1.0 and hg.power < 0.0 }
    assert constraint sq { hg.power ** 2.0 >= 0.0 }
}
`;

const SATISFACTION = `package P {
    private import ScalarValues::*;
    part def Inner { attribute power : Real; }
    part def Sub { attribute power : Real; part inner : Inner; }
    part def Thing { part sub : Sub; }
    requirement def R2 {
        subject vehicle : Thing;
        require constraint { vehicle.sub.power > 0.0 }
    }
    requirement def R3 {
        subject vehicle : Thing;
        require constraint { vehicle.sub.inner.power > 0.0 }
    }
    requirement r2 : R2 { subject vehicle = craft; }
    requirement r3 : R3 { subject vehicle = craft; }
    part craft : Thing {
        part :>> sub {
            attribute :>> power = 5.0;
            part :>> inner { attribute :>> power = 7.0; }
        }
    }
    part analysis {
        assert satisfy r2 by craft;
        assert satisfy r3 by craft;
    }
}
`;

test("the service advertises verification", async () => {
  await using connection = await connect();
  assert.ok(connection.info.has("verification"));
});

test("verifySatisfaction answers one verdict per assertion", async () => {
  await using connection = await connect();
  const model = await connection.loads(SATISFACTION);
  const verdicts = await model.verifySatisfaction({ question: "holds" });
  assert.equal(verdicts.length, 2);
  assert.ok(verdicts.every((verdict) => verdictHolds(verdict.verdict)));
  assert.ok(verdicts.every((verdict) => /satisfy r[23] by craft/.test(verdict.verdict.subject.element)));
});

test("satisfied is true only when every assertion holds", async () => {
  await using connection = await connect();
  const model = await connection.loads(SATISFACTION);
  assert.equal(await model.satisfied({ question: "holds" }), true);
  const failing = await connection.loads(`package F {
    private import ScalarValues::*;
    part def Thing { attribute power : Real; }
    requirement def Positive {
        subject t : Thing;
        require constraint { t.power > 0.0 }
    }
    requirement positive : Positive;
    part craft : Thing { attribute :>> power = -1.0; }
    part analysis { assert satisfy positive by craft; }
}`);
  assert.equal(await failing.satisfied({ question: "holds" }), false);
});

test("verifyConstraint answers a verdict about the named constraint", async () => {
  await using connection = await connect();
  const model = await connection.loads(QUESTIONS);
  const verdict = await model.verifyConstraint("P::lemma", { question: "holds" });
  assert.ok(verdictHolds(verdict.verdict));
  assert.match(explainVerdict(verdict.verdict), /✓/);
});

test("verifyRequirement answers whether the requirement holds", async () => {
  await using connection = await connect();
  const model = await connection.loads(SATISFACTION);
  const verdict = await model.verifyRequirement("P::r2");
  assert.ok(verdictEvaluated(verdict.verdict));
  assert.ok(verdictHolds(verdict.verdict));
});

test("validateInstance answers every assertion by path", async () => {
  await using connection = await connect();
  const model = await connection.loads(`package Fleet {
    private import ScalarValues::*;
    part def Wheel {
        attribute pressure default = 32.0;
        assert constraint pressureOk { pressure >= 30.0 }
    }
    part def Car {
        attribute mass = 1500.0;
        part wheels : Wheel[2] { attribute :>> pressure = 20.0; }
        assert constraint massOk { mass < 2000.0 }
    }
    part car : Car;
}`);
  const validation = await model.validateInstance("Fleet::car");
  assert.ok(validation.verdicts.length > 0);
  assert.equal(validation.bounded, false);
});

test("a wrong kind raises WrongKindError", async () => {
  await using connection = await connect();
  const model = await connection.loads(QUESTIONS);
  await assert.rejects(() => model.verifyConstraint("P::hg"), WrongKindError);
});

test("a non-question engine refusal names the engine call", async () => {
  const conn = await fakeConnection(["verification"]);
  await assert.rejects(
    () => conn.verifyConstraint("hash", "c", { engine: "explore" }),
    MissingCapabilityError,
  );
});

test("runAnalysis engine explore is refused as an answer-about-outcomes", async () => {
  await using connection = await connect();
  const model = await connection.loads(QUESTIONS);
  await assert.rejects(
    () => model.runAnalysis("P::lemma", { engine: "explore" }),
    /exploreAnalysis/,
  );
});

test("a direct verdict attaches every verification the response carried", async () => {
  const conn = await fakeConnection(["verification"], () =>
    create(VerifyRequirementResponseSchema, {
      verdict: { kind: "requirement", element: "satisfy Demo::r", holds: true },
      verificationVerdicts: [
        { caseId: "Demo::checkMass", kind: "fail" },
        { caseId: "Demo::checkMass::inner", kind: "inconclusive", subcase: true },
      ],
    }),
  );
  const verdict = await conn.verifyRequirement("hash1", "Demo::r");
  assert.equal(verdict.verdict.kind, "holds");
  assert.deepEqual(
    verdict.verdict.verifications.map((v) => v.caseId),
    ["Demo::checkMass", "Demo::checkMass::inner"],
  );
});

test("a satisfaction verdict takes only the verifications of its own requirement", async () => {
  const conn = await fakeConnection(["verification"], () =>
    create(VerifySatisfactionResponseSchema, {
      verdicts: [
        { kind: "satisfy", elementId: "Demo::s1", requirementId: "Demo::r1", holds: true },
        { kind: "satisfy", elementId: "Demo::s2", requirementId: "Demo::r2", holds: true },
        { kind: "satisfy", elementId: "Demo::s3", holds: true },
      ],
      verificationVerdicts: [
        { caseId: "c1", kind: "pass", requirementId: "Demo::r1" },
        { caseId: "c2", kind: "fail", requirementId: "Demo::r2" },
        { caseId: "c3", kind: "pass", requirementId: "Demo::r1" },
      ],
    }),
  );
  const verdicts = await conn.verifySatisfaction("hash1");
  assert.deepEqual(
    verdicts[0]?.verdict.verifications.map((v) => v.caseId),
    ["c1", "c3"],
  );
  assert.deepEqual(
    verdicts[1]?.verdict.verifications.map((v) => v.caseId),
    ["c2"],
  );
  assert.deepEqual(verdicts[2]?.verdict.verifications, []);
});

test("verifyRequirement reports the verdicts of the case verifying it", async () => {
  await using connection = await connect();
  const model = await connection.loads(readFileSync(CASES_FIXTURE, "utf8"));
  const verdict = await model.verifyRequirement("Demo::zeroed");
  assert.ok(verdict.verdict.kind === "holds" || verdict.verdict.kind === "undecided");
  assert.ok(verdict.verdict.verifications.length > 0);
  assert.ok(verdict.verdict.verifications.some((v) => /checkZero/.test(v.caseId)));
});

const BOUND = `package Demo {
    private import ScalarValues::*;
    part def Thing { attribute v : Integer = 2; }
    part t : Thing;
    requirement def Under {
        subject s : Thing;
        in limit : Integer;
        in slack : Integer = 0;
        require constraint { s.v + slack < limit }
    }
    constraint def Between {
        in low : Integer;
        in high : Integer = 10;
        low < high
    }
}`;

test("arguments bind a requirement's and a constraint's in parameters", async () => {
  await using connection = await connect();
  const model = await connection.loads(BOUND);
  const holds = await model.verifyRequirement("Demo::Under", {
    subject: "Demo::t",
    namedArguments: { limit: 5n },
  });
  assert.equal(holds.verdict.kind, "holds", holds.verdict.kind === "undecided" ? holds.verdict.error : "");
  const fails = await model.verifyRequirement("Demo::Under", {
    subject: "Demo::t",
    arguments: [5n, 4n],
  });
  assert.equal(fails.verdict.kind, "fails", fails.verdict.kind);
  const between = await model.verifyConstraint("Demo::Between", { arguments: [3n] });
  assert.equal(between.verdict.kind, "holds", between.verdict.kind);
  const unknown = await model.verifyRequirement("Demo::Under", {
    subject: "Demo::t",
    namedArguments: { limit: 5n, bound: 1n },
  });
  assert.equal(unknown.verdict.kind, "undecided");
  assert.match(unknown.verdict.error, /bound/);
});

test("arguments are refused before the call without verification_arguments", async () => {
  const conn = await fakeConnection(["verification"]);
  await assert.rejects(
    () => conn.verifyRequirement("hash", "r", { namedArguments: { limit: 5n } }),
    MissingCapabilityError,
  );
  await assert.rejects(
    () => conn.verifyConstraint("hash", "c", { arguments: [3] }),
    MissingCapabilityError,
  );
});
