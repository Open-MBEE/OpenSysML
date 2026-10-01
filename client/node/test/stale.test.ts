// The connect-time requirements: a release the service cannot report, a
// capability it does not have, and the model's own error surface.

import assert from "node:assert/strict";
import { before, test } from "node:test";
import {
  MissingCapabilityError,
  ParseError,
  StaleServiceError,
  connect,
  currentPrivateService,
} from "../src/node/index.js";
import { SAMPLE, useServiceBinary } from "./support/service.js";

before(() => {
  useServiceBinary();
});

const BROKEN = `package Broken {
    part def Boat {
        part motor : Missing;
    }
}
`;

test("a version the service does not report is refused", async () => {
  await using connection = await connect();
  const service = currentPrivateService();
  assert.ok(service !== undefined);
  const version = connection.info.version;
  await assert.rejects(
    () => connect({ address: service.address, version: `${version}-not` }),
    (error: unknown) => {
      assert.ok(error instanceof StaleServiceError);
      assert.match(error.reason, /was asked for|did not answer/);
      return true;
    },
  );
});

test("a capability the service does not report is refused at connect", async () => {
  await using keepAlive = await connect();
  assert.ok(keepAlive.info.answered);
  const service = currentPrivateService();
  assert.ok(service !== undefined);
  await assert.rejects(
    () => connect({ address: service.address, requireCapabilities: ["never_a_capability"] }),
    MissingCapabilityError,
  );
});

test("a refused private connection does not keep the child", async () => {
  const before = currentPrivateService()?.refs ?? 0;
  await assert.rejects(() => connect({ version: "no-such-release-tag" }), StaleServiceError);
  assert.equal(currentPrivateService()?.refs ?? 0, before);
});

test("raiseForErrors throws a ParseError carrying the model", async () => {
  await using connection = await connect();
  const model = await connection.loads(BROKEN);
  assert.equal(model.ok, false);
  assert.ok(model.errors.length > 0);
  assert.equal(model.hasErrors, true);
  assert.throws(
    () => model.raiseForErrors(),
    (error: unknown) => {
      assert.ok(error instanceof ParseError);
      assert.equal(error.model, model);
      return true;
    },
  );
});

test("a clean model is ok and raiseForErrors returns it", async () => {
  await using connection = await connect();
  const model = await connection.loads(SAMPLE);
  assert.equal(model.ok, true);
  assert.equal(model.errors.length, 0);
  assert.equal(model.raiseForErrors(), model);
});

test("an adopted model has no documents and no root", async () => {
  await using connection = await connect();
  const model = await connection.loads(SAMPLE);
  const adopted = connection.model(model.hash);
  assert.equal(adopted.parsed, false);
  assert.equal(adopted.documents.length, 0);
  assert.equal(adopted.sourcePath, undefined);
  assert.throws(() => adopted.root, /adopted/);
});
