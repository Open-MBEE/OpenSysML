// The connect-time requirements: a release the service cannot report, a
// capability it does not have, and the model's own error surface.

import type { Transport } from "@connectrpc/connect";
import { Code, ConnectError } from "@connectrpc/connect";
import assert from "node:assert/strict";
import { before, test } from "node:test";
import { Connection } from "../src/core/connection.js";
import {
  MissingCapabilityError,
  ServiceUnavailableError,
  ParseError,
  StaleServiceError,
  connect,
  currentPrivateService,
} from "../src/node/index.js";
import { fakeTransport } from "./support/fake.js";
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

test("serverInfo answers the degraded handshake of a service without GetServerInfo", async () => {
  const transport = {
    unary() {
      return Promise.reject(
        new ConnectError("unknown method GetServerInfo for service sysml.SysMLService", Code.Unimplemented),
      );
    },
    stream() {
      return Promise.reject(
        new ConnectError("unknown method for service sysml.SysMLService", Code.Unimplemented),
      );
    },
  } as unknown as Transport;
  const connection = await Connection.open({
    transport,
    encoding: "protobuf",
    backend: { origin: "the fake transport", release: () => Promise.resolve() },
  });
  assert.equal(connection.info.answered, false);
  const info = await connection.serverInfo();
  assert.equal(info.answered, false);
  assert.equal(info.version, "");
  assert.deepEqual([...info.capabilities], []);
});

test("a refused private connection does not keep the child", async () => {
  const before = currentPrivateService()?.refs ?? 0;
  await assert.rejects(() => connect({ version: "no-such-release-tag" }), StaleServiceError);
  assert.equal(currentPrivateService()?.refs ?? 0, before);
});

function countingBackend(): { backend: { origin: string; release(): Promise<void> }; releases: () => number } {
  let releases = 0;
  return {
    backend: {
      origin: "the counting fake backend",
      release: () => {
        releases += 1;
        return Promise.resolve();
      },
    },
    releases: () => releases,
  };
}

test("a handshake that never answers releases the backend exactly once", async () => {
  const counting = countingBackend();
  const transport = {
    unary: () => Promise.reject(new Error("the service is away")),
    stream: () => Promise.reject(new Error("no streams")),
  } as unknown as Transport;
  await assert.rejects(
    Connection.open({ transport, backend: counting.backend, encoding: "protobuf" }),
  );
  assert.equal(counting.releases(), 1);
});

test("a refused capability releases the backend exactly once", async () => {
  const counting = countingBackend();
  await assert.rejects(
    Connection.open({
      transport: fakeTransport({ version: "test", capabilities: [] }, () => {
        throw new Error("the fake service answers nothing for this method");
      }),
      backend: counting.backend,
      encoding: "protobuf",
      requiredCapabilities: ["never_a_capability"],
    }),
    MissingCapabilityError,
  );
  assert.equal(counting.releases(), 1);
});

test("a release the service does not report releases the backend exactly once", async () => {
  const counting = countingBackend();
  await assert.rejects(
    Connection.open({
      transport: fakeTransport({ version: "test", capabilities: [] }, () => {
        throw new Error("the fake service answers nothing for this method");
      }),
      backend: counting.backend,
      encoding: "protobuf",
      requiredVersion: "no-such-release-tag",
    }),
    StaleServiceError,
  );
  assert.equal(counting.releases(), 1);
});

test("a refused private connection keeps an earlier hold on the shared child", async () => {
  await using keepAlive = await connect();
  const refs = currentPrivateService()?.refs;
  assert.ok(refs !== undefined && refs > 0);
  await assert.rejects(
    () => connect({ requireCapabilities: ["no_such_capability"] }),
    MissingCapabilityError,
  );
  const info = await keepAlive.serverInfo();
  assert.ok(info.answered);
  const model = await keepAlive.loads(SAMPLE);
  assert.equal(model.ok, true);
  assert.equal(currentPrivateService()?.refs, refs);
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

test("a handshake the service cannot serve is a ServiceUnavailableError", async () => {
  const transport = {
    unary() {
      return Promise.reject(new ConnectError("connect ECONNREFUSED 127.0.0.1:1", Code.Unavailable));
    },
    stream() {
      return Promise.reject(new ConnectError("connect ECONNREFUSED 127.0.0.1:1", Code.Unavailable));
    },
  } as unknown as Transport;
  const counting = countingBackend();
  await assert.rejects(
    Connection.open({ transport, backend: counting.backend, encoding: "protobuf" }),
    (error: unknown) => {
      assert.ok(error instanceof ServiceUnavailableError);
      assert.equal(error.code, "UNAVAILABLE");
      return true;
    },
  );
});
