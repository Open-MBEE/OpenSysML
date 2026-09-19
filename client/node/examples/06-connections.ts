// The ways a program can be connected to the service: a private child of this
// process, several connections sharing one, an already-running service over
// Connect or gRPC in either body encoding, and many calls in flight at once.
//
//   npm run example -- 06-connections

import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { once } from "node:events";
import { connect, formatValue, loads, resolveBinary } from "../src/node/index.js";
import { ROVER, section, show } from "./model.js";

/** Starts a service this program owns, the way a deployment would run one. */
async function startService(): Promise<{ address: string; child: ReturnType<typeof spawn> }> {
  const binary = await resolveBinary();
  const child = spawn(binary.path, ["-port", "0", "-health-port", "0", "-report-address"], {
    stdio: ["pipe", "pipe", "inherit"],
  });
  const address = await new Promise<string>((resolve, reject) => {
    let buffered = "";
    child.stdout.on("data", (chunk: Buffer) => {
      buffered += chunk.toString();
      const newline = buffered.indexOf("\n");
      if (newline !== -1) {
        resolve(buffered.slice(0, newline).trim());
      }
    });
    child.once("error", reject);
    child.once("exit", (code) => {
      reject(new Error(`the service exited with ${String(code)} before reporting an address`));
    });
  });
  return { address, child };
}

async function main(): Promise<void> {
  section("One private service, shared by two connections");
  const first = await connect();
  const second = await connect();
  show("origin", first.info.origin);
  show("same service", first.info.origin === second.info.origin);
  assert.equal(first.info.origin, second.info.origin);
  // The child outlives the first close and stops when the last connection goes.
  await first.close();
  show("second still works", !(await second.loads(ROVER)).hasErrors);
  await second.close();

  section("A one-shot model, which owns the connection it opened");
  {
    await using model = await loads(ROVER);
    show("wheelCount", formatValue(await model.eval("wheelCount", { context: "Rover::Rover" })));
  }

  section("A service this program started itself");
  const service = await startService();
  try {
    show("address", service.address);
    // gRPC carries protobuf bodies only, so that pair is refused before it opens.
    await assert.rejects(() =>
      connect({ address: service.address, protocol: "grpc", encoding: "json" }),
    );
    for (const [protocol, encoding] of [
      ["connect", "protobuf"],
      ["connect", "json"],
      ["grpc", "protobuf"],
    ] as const) {
      const connection = await connect({ address: service.address, protocol, encoding });
      try {
        const model = await connection.loads(ROVER);
        const value = await model.eval("wheelCount", { context: "Rover::Rover" });
        show(`${protocol} + ${encoding}`, `wheelCount = ${formatValue(value)}`);
      } finally {
        // Closing leaves a service this client did not start running.
        await connection.close();
      }
    }

    section("Many calls at once, on one connection");
    const connection = await connect({ address: service.address, timeoutMs: 30_000 });
    try {
      const model = await connection.loads(ROVER);
      const expressions = ["1 + 1", "2 * 21", "6 / 4", '"a" + "b"', "true and false", "(1, 2, 3)"];
      const values = await Promise.all(expressions.map((expression) => model.eval(expression)));
      show("evaluated", values.map((value) => formatValue(value)).join(", "));
      const parsed = await Promise.all(
        Array.from({ length: 4 }, (_, index) =>
          connection.loads(`package P${index} { part def Widget${index} {} }`),
        ),
      );
      show("parsed at once", parsed.map((one) => one.hash.slice(0, 8)).join(", "));
      assert.equal(new Set(parsed.map((one) => one.hash)).size, 4);
      const [wheel, battery, tree] = await Promise.all([
        model.symbol("Rover::Wheel"),
        model.symbol("Rover::Battery"),
        model.instantiate("Rover::Battery"),
      ]);
      show("read at once", `${wheel.id}, ${battery.id}, ${tree.root.typeId}`);
    } finally {
      await connection.close();
    }
  } finally {
    // This program started the service, so this program stops it, unless it
    // already exited — then there is no exit left to wait for.
    if (service.child.exitCode === null && service.child.signalCode === null) {
      const exited = once(service.child, "exit");
      service.child.kill("SIGTERM");
      await exited;
    }
  }

  section("The service is gone");
  await assert.rejects(() => connect({ address: service.address, timeoutMs: 1000 }));
  show("connecting again", "fails, as it should");
}

await main();
