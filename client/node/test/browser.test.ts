// The browser entry point, exercised over the same fetch transport a page uses:
// an explicit address only, no ownership, and CORS as the browser would need it.

import assert from "node:assert/strict";
import { spawn, type ChildProcess } from "node:child_process";
import { after, before, test } from "node:test";
import { connect } from "../src/browser/index.js";
import { OpenSysMLError } from "../src/core/errors.js";
import { SAMPLE, serviceBinary } from "./support/service.js";

const ORIGIN = "https://app.example.test";
let service: ChildProcess | undefined;
let address = "";

before(async () => {
  service = spawn(
    serviceBinary(),
    ["-port", "0", "-health-port", "0", "-report-address", "-cors-allowed-origins", ORIGIN],
    { stdio: ["pipe", "pipe", "ignore"] },
  );
  address = await firstLine(service);
});

after(() => {
  service?.kill("SIGKILL");
});

test("a page reaches a running service with fetch, and closing leaves it running", async () => {
  const connection = await connect({ address });
  assert.ok(connection.info.answered);
  const model = await connection.loads(SAMPLE);
  assert.deepEqual(await model.eval("2 + 2"), { kind: "int", value: 4n });
  assert.equal((await model.symbol("Sample::Car")).id, "Sample::Car");

  await connection.close();
  assert.equal(service?.exitCode, null);
});

test("the browser entry point cannot start a service, so it insists on an address", async () => {
  await assert.rejects(() => connect({ address: "  " }), OpenSysMLError);
});

test("the allowed origin is answered on the preflight, and another origin is not", async () => {
  const allowed = await preflight(ORIGIN);
  assert.equal(allowed.headers.get("access-control-allow-origin"), ORIGIN);
  const denied = await preflight("https://elsewhere.example.test");
  // Exact origins only: the service never answers with a wildcard.
  assert.equal(denied.headers.get("access-control-allow-origin"), null);
});

function preflight(origin: string): Promise<Response> {
  return fetch(`http://${address}/sysml.SysMLService/GetServerInfo`, {
    method: "OPTIONS",
    headers: {
      origin,
      "access-control-request-method": "POST",
      "access-control-request-headers": "content-type,connect-protocol-version",
    },
  });
}

function firstLine(child: ChildProcess): Promise<string> {
  return new Promise<string>((resolve, reject) => {
    let buffered = "";
    child.stdout?.on("data", (chunk: Buffer) => {
      buffered += chunk.toString();
      const newline = buffered.indexOf("\n");
      if (newline !== -1) {
        resolve(buffered.slice(0, newline).trim());
      }
    });
    child.once("error", reject);
    child.once("exit", () => {
      reject(new Error("the service exited before reporting an address"));
    });
  });
}
