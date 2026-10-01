// Verification: constraints, requirements, satisfaction and validation.

import assert from "node:assert/strict";
import { before, test } from "node:test";
import {
  MissingCapabilityError,
  WrongKindError,
  connect,
  explainVerdict,
  verdictEvaluated,
  verdictHolds,
} from "../src/node/index.js";
import { fakeConnection } from "./support/fake.js";
import { useServiceBinary } from "./support/service.js";

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
