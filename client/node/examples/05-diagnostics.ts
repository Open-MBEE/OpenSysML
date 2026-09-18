// How the client reports what went wrong: diagnostics on a model that parsed
// with errors, and the typed error each failing call raises.
//
//   npm run example -- 05-diagnostics

import assert from "node:assert/strict";
import { mkdtemp, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import {
  ClosedConnectionError,
  EvaluationError,
  ModelFileNotFoundError,
  ModelNotFoundError,
  OpenSysMLError,
  SymbolNotFoundError,
  connect,
} from "../src/node/index.js";
import { ROVER, section, show } from "./model.js";

/** Runs a call that should fail, and prints the error it raised. */
async function failing(label: string, call: () => Promise<unknown>): Promise<unknown> {
  try {
    await call();
  } catch (error) {
    assert.ok(error instanceof OpenSysMLError, `${label} raised ${String(error)}`);
    show(label, `${error.constructor.name}: ${error.message}`);
    return error;
  }
  throw new Error(`${label} was expected to fail and did not`);
}

async function main(): Promise<void> {
  const connection = await connect();
  try {
    section("A model that parses with errors");
    // Parsing answers with the model and its diagnostics; it does not throw.
    const broken = await connection.loads(`package Broken {
  part def Wheel {
    attribute radius : ScalarValues::Real =
  }
}
`);
    show("has errors", broken.hasErrors);
    for (const diagnostic of broken.diagnostics.slice(0, 4)) {
      const at = `${diagnostic.startLine ?? "?"}:${diagnostic.startColumn ?? "?"}`;
      show(`${diagnostic.severity} at ${at}`, diagnostic.message);
    }
    assert.ok(broken.hasErrors);

    section("Errors the model raises");
    const model = await connection.loads(ROVER);
    const notFound = await failing("a name that is a typo", () => model.symbol("Wheeel"));
    assert.ok(notFound instanceof SymbolNotFoundError);
    show("looked up", notFound.symbolName);
    show("suggested", notFound.suggestions.join(", "));
    assert.equal(notFound.suggestions[0], "Rover::Wheel");
    await failing("a name nothing resembles", () => model.symbol("Sprocket"));
    await failing("an unknown qualified symbol", () => model.symbol("Rover::Sprocket"));
    const bad = await failing("an expression that cannot run", () => model.eval("mass + "));
    assert.ok(bad instanceof EvaluationError);
    await failing("an expression out of scope", () => model.eval("mass"));
    await failing("instantiating what is not there", () => model.instantiate("Rover::Sprocket"));

    section("Errors the connection raises");
    const missing = await failing("a hash the service has not got", () =>
      connection.model("0".repeat(64)).eval("2 + 2"),
    );
    assert.ok(missing instanceof ModelNotFoundError);
    show("status", missing.code);
    const absent = await failing("a file that does not exist", () =>
      connection.load(join(tmpdir(), "no-such-model.sysml")),
    );
    assert.ok(absent instanceof ModelFileNotFoundError);

    section("A file that does exist");
    const directory = await mkdtemp(join(tmpdir(), "opensysml-example-"));
    const path = join(directory, "rover.sysml");
    await writeFile(path, ROVER);
    const fromFile = await connection.load(path);
    show("root", fromFile.root.childIds.join(", "));
    show("errors", fromFile.hasErrors);
    // The hash covers the document's name as well as its text, so a file and the
    // same source inline are two models in the service's cache.
    show("hash", `${fromFile.hash.slice(0, 12)} vs inline ${model.hash.slice(0, 12)}`);

    section("Options that are not options");
    await failing("an encoding there is none of", () => connect({ encoding: "yaml" as "json" }));
    await failing("a timeout that is not a duration", () => connect({ timeoutMs: -1 }));
    await failing("an address nothing answers on", () =>
      connect({ address: "127.0.0.1:1", timeoutMs: 500 }),
    );
  } finally {
    await connection.close();
  }

  section("A closed connection");
  const closed = await connect();
  const model = await closed.loads(ROVER);
  await closed.close();
  const after = await failing("evaluating on it", () => model.eval("2 + 2"));
  assert.ok(after instanceof ClosedConnectionError);
}

await main();
