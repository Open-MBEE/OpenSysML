import assert from "node:assert/strict";
import {
  copyFileSync,
  mkdirSync,
  mkdtempSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { readFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { pathToFileURL } from "node:url";
import { after, test } from "node:test";
import { Worker as NodeWorker } from "node:worker_threads";
import { Code, ConnectError, createClient } from "@connectrpc/connect";
import {
  ClosedConnectionError,
  MissingCapabilityError,
  OpenSysMLError,
  SourceDocument,
  StaleServiceError,
  connect,
  connectWasm,
} from "../src/node/index.js";
import { connectWasm as connectBrowserWasm } from "../src/browser/index.js";
import {
  connectWasmHost,
  createWasmTransport,
  instantiateInline,
  loadGoConstructor,
  serveWasmPort,
  WorkerWasmHost,
  type GoConstructor,
  type WasmHost,
  type WasmPortLike,
  type WasmWorkerEndpoint,
  type WasmWorkerRequest,
  type WasmWorkerResponse,
} from "../src/core/wasm.js";
import { WASM_PACKAGE } from "../src/core/package.js";
import { resolveWasmSources } from "../src/node/wasm-source.js";
import { SysMLService } from "../src/generated/sysml_pb.js";
import { wasmArtifacts, type WasmArtifacts } from "./support/wasm.js";
import { repoRoot, SAMPLE, useServiceBinary } from "./support/service.js";

const artifacts = await wasmArtifacts();
if (artifacts !== undefined) {
  useServiceBinary();
}
const wasmSkip =
  artifacts === undefined ? "Go is unavailable; the WASM integration test was skipped" : false;

after(() => {
  Reflect.deleteProperty(globalThis, "sysmlWasm");
});

test("the transport maps JSON envelopes, ignores unknown fields, and observes aborts", async () => {
  const unknownFields = clientFor(
    fixedHost(
      '{"jsonrpc":"2.0","id":null,"result":{"version":"transport-test","capabilities":[],"futureField":true}}',
    ),
  );
  const response = await unknownFields.getServerInfo({});
  assert.equal(response.version, "transport-test");
  assert.deepEqual(response.capabilities, []);

  const failed = clientFor(
    fixedHost(
      '{"jsonrpc":"2.0","id":null,"error":{"code":5,"message":"not found"}}',
    ),
  );
  await assert.rejects(
    () => failed.getServerInfo({}),
    (error: unknown) =>
      error instanceof ConnectError && error.code === Code.NotFound,
  );

  const controller = new AbortController();
  let closeCount = 0;
  const unanswered = clientFor({
    ...fixedHost(""),
    call: () => new Promise<string>(() => {}),
    close: () => {
      closeCount += 1;
      return Promise.resolve();
    },
  });
  const pending = unanswered.getServerInfo({}, { signal: controller.signal });
  controller.abort(new ConnectError("cancelled", Code.Canceled));
  await assert.rejects(
    pending,
    (error: unknown) => error instanceof ConnectError && error.code === Code.Canceled,
  );
  assert.equal(closeCount, 0);

  const deadline = clientFor({
    ...fixedHost(""),
    call: () => new Promise<string>(() => {}),
    close: () => {
      closeCount += 1;
      return Promise.resolve();
    },
  });
  await assert.rejects(
    () => deadline.getServerInfo({}, { timeoutMs: 20 }),
    (error: unknown) =>
      error instanceof ConnectError && error.code === Code.DeadlineExceeded,
  );
  assert.equal(closeCount, 0);
});

test("invalid WASM connection settings close the host before handshake", async () => {
  for (const options of [{ encoding: "protobuf" as const }, { timeoutMs: 0 }]) {
    let closed = false;
    const host: WasmHost = {
      ...fixedHost(""),
      close: () => {
        closed = true;
        return Promise.resolve();
      },
    };
    await assert.rejects(() => connectWasmHost(host, options), OpenSysMLError);
    assert.equal(closed, true);
  }
});

test("inline hosts capture the Go surface and reject calls after close", async () => {
  const host = await instantiateInline(emptyWasm, FakeGo);
  assert.equal("sysmlWasm" in globalThis, false);
  assert.equal(host.version, "fake-wasm");
  assert.deepEqual(JSON.parse(await host.call("Echo", "{}")), {
    jsonrpc: "2.0",
    id: null,
    result: { method: "Echo", params: {} },
  });
  await host.close();
  await assert.rejects(host.call("Echo", "{}"), ClosedConnectionError);
});

test("worker ports initialize, answer calls, and return initialization errors", async () => {
  const pair = fakePortPair();
  const received: WasmWorkerResponse[] = [];
  pair.client.addEventListener("message", (event) => received.push(event.data));
  serveWasmPort(pair.service, {
    loadWasm: () => Promise.resolve(emptyWasm),
    loadGo: () => Promise.resolve(FakeGo),
  });
  pair.client.postMessage({ type: "init", wasm: emptyWasm });
  const ready = await waitFor(received, (message) => message.type === "ready");
  assert.equal(ready.version, "fake-wasm");

  pair.client.postMessage({ type: "call", id: 1, method: "Echo", params: "{}" });
  const answer = await waitFor(
    received,
    (message): message is Extract<WasmWorkerResponse, { type: "answer" }> =>
      message.type === "answer" && message.id === 1,
  );
  assert.deepEqual(
    (JSON.parse(answer.envelope) as { result: unknown }).result,
    { method: "Echo", params: {} },
  );

  pair.client.postMessage({ type: "call", id: 2, method: "Fail", params: "{}" });
  const failedCall = await waitFor(
    received,
    (message): message is Extract<WasmWorkerResponse, { type: "answer" }> =>
      message.type === "answer" && message.id === 2,
  );
  assert.equal(
    (JSON.parse(failedCall.envelope) as { error: { code: number } }).error.code,
    Code.Internal,
  );

  const broken = fakePortPair();
  const brokenMessages: WasmWorkerResponse[] = [];
  broken.client.addEventListener("message", (event) => {
    brokenMessages.push(event.data);
  });
  serveWasmPort(broken.service, {
    loadWasm: () => Promise.reject(new Error("invalid module")),
  });
  broken.client.postMessage({ type: "init", wasm: emptyWasm });
  const initError = await waitFor(brokenMessages, (message) => message.type === "failed");
  assert.deepEqual(initError, { type: "failed", message: "invalid module" });
});

test("Go constructors are cached per runtime script and restore the previous global", async () => {
  const directory = mkdtempSync(join(tmpdir(), "opensysml-wasm-exec-"));
  const runtime = globalThis as typeof globalThis & { Go?: GoConstructor };
  const initial = runtime.Go;
  const pathA = join(directory, "go-a.mjs");
  const pathB = join(directory, "go-b.mjs");
  writeFileSync(pathA, "globalThis.Go = class GoA {};\n");
  writeFileSync(pathB, "globalThis.Go = class GoB {};\n");
  runtime.Go = FakeGo;

  try {
    const goA = await loadGoConstructor(pathToFileURL(pathA).href);
    const goB = await loadGoConstructor(pathToFileURL(pathB).href);
    const goAAgain = await loadGoConstructor(pathToFileURL(pathA).href);

    assert.equal(goA.name, "GoA");
    assert.equal(goB.name, "GoB");
    assert.strictEqual(goAAgain, goA);
    assert.strictEqual(runtime.Go, FakeGo);
  } finally {
    if (initial === undefined) {
      delete runtime.Go;
    } else {
      runtime.Go = initial;
    }
    rmSync(directory, { recursive: true, force: true });
  }
});

test("preloaded Go runtime scripts are captured from the global", async () => {
  const directory = mkdtempSync(join(tmpdir(), "opensysml-wasm-exec-"));
  const runtime = globalThis as typeof globalThis & { Go?: GoConstructor };
  const initial = runtime.Go;
  const path = join(directory, "go-preloaded.mjs");
  const specifier = `${pathToFileURL(path).href}?preloaded`;
  writeFileSync(path, "globalThis.Go = class GoPreloaded {};\n");

  try {
    await import(specifier);
    const preloaded = runtime.Go;
    assert.ok(preloaded);
    assert.equal(preloaded.name, "GoPreloaded");
    assert.strictEqual(await loadGoConstructor(specifier), preloaded);
  } finally {
    if (initial === undefined) {
      delete runtime.Go;
    } else {
      runtime.Go = initial;
    }
    rmSync(directory, { recursive: true, force: true });
  }
});

test("Go constructor lookups without a runtime wait for explicit loads", async () => {
  const directory = mkdtempSync(join(tmpdir(), "opensysml-wasm-exec-"));
  const runtime = globalThis as typeof globalThis & { Go?: GoConstructor };
  const initial = runtime.Go;
  const path = join(directory, "go-slow.mjs");
  const specifier = pathToFileURL(path).href;
  const preloaded = class PreloadedGo extends FakeGo {};
  runtime.Go = preloaded;
  writeFileSync(
    path,
    "await new Promise((resolve) => setTimeout(resolve, 50));\n" +
      "globalThis.Go = class GoSlow {};\n",
  );
  const slowLoad = loadGoConstructor(specifier);
  const queuedLookup = loadGoConstructor();

  try {
    assert.strictEqual(await queuedLookup, preloaded);
    assert.equal((await slowLoad).name, "GoSlow");
    assert.strictEqual(runtime.Go, preloaded);
  } finally {
    await Promise.allSettled([slowLoad, queuedLookup]);
    if (initial === undefined) {
      delete runtime.Go;
    } else {
      runtime.Go = initial;
    }
    rmSync(directory, { recursive: true, force: true });
  }
});

test("Go constructor load failures are not cached", async () => {
  const directory = mkdtempSync(join(tmpdir(), "opensysml-wasm-exec-"));
  const runtime = globalThis as typeof globalThis & { Go?: GoConstructor };
  const initial = runtime.Go;
  const path = join(directory, "go-retry.mjs");
  const specifier = `${pathToFileURL(path).href}?missing`;
  writeFileSync(path, "export {};\n");
  delete runtime.Go;

  try {
    const firstFailure = loadGoConstructor(specifier);
    await assert.rejects(firstFailure, /Go is unavailable/);
    const retry = loadGoConstructor(specifier);
    assert.notStrictEqual(retry, firstFailure);
    await assert.rejects(retry, /Go is unavailable/);

    writeFileSync(path, "globalThis.Go = class GoRetry {};\n");
    const GoRetry = await loadGoConstructor(`${pathToFileURL(path).href}?retry`);
    assert.equal(GoRetry.name, "GoRetry");
  } finally {
    if (initial === undefined) {
      delete runtime.Go;
    } else {
      runtime.Go = initial;
    }
    rmSync(directory, { recursive: true, force: true });
  }
});

test("Node WASM package resolution uses package-local assets", () => {
  const directory = mkdtempSync(join(tmpdir(), "opensysml-wasm-package-"));
  const packageDirectory = join(directory, "node_modules", ...WASM_PACKAGE.split("/"));
  mkdirSync(packageDirectory, { recursive: true });
  const packageJson = join(packageDirectory, "package.json");
  writeFileSync(packageJson, JSON.stringify({ name: WASM_PACKAGE, version: "0.0.0" }));
  writeFileSync(join(packageDirectory, "sysml-wasm.wasm"), "wasm");
  writeFileSync(join(packageDirectory, "wasm_exec.js"), "runtime");

  const resolver = (specifier: string): string => {
    assert.equal(specifier, `${WASM_PACKAGE}/package.json`);
    return packageJson;
  };
  try {
    assert.deepEqual(resolveWasmSources({}, resolver), {
      wasm: join(packageDirectory, "sysml-wasm.wasm"),
      wasmExec: join(packageDirectory, "wasm_exec.js"),
    });
    assert.deepEqual(resolveWasmSources({ wasmExec: "custom-runtime.js" }, resolver), {
      wasm: join(packageDirectory, "sysml-wasm.wasm"),
      wasmExec: "custom-runtime.js",
    });
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
});

test("Node WASM resolution reports the optional package installation command", () => {
  assert.throws(
    () =>
      resolveWasmSources({}, () => {
        throw new Error("not installed");
      }),
    (error: unknown) =>
      error instanceof OpenSysMLError &&
      new RegExp(
        `^connectWasm needs sysml-wasm\\.wasm: install ${WASM_PACKAGE.replace("/", "\\/")} at \\d+\\.\\d+\\.\\d+, or pass wasm and wasmExec$`,
      ).test(error.message),
  );
});

test("Node WASM resolution requires wasmExec with a supplied module", () => {
  assert.throws(
    () => resolveWasmSources({ wasm: emptyWasm }, () => "unused"),
    (error: unknown) =>
      error instanceof OpenSysMLError && error.message.includes("wasmExec"),
  );
});

test("worker calls remove aborted pending entries and ignore late answers", async () => {
  const posted: WasmWorkerRequest[] = [];
  let onMessage: ((message: WasmWorkerResponse) => void) | undefined;
  const endpoint: WasmWorkerEndpoint = {
    post(message) {
      posted.push(message);
      if (message.type === "init") {
        onMessage?.({ type: "ready", version: "fake-worker" });
      }
    },
    onMessage(listener) {
      onMessage = listener;
    },
    onFailure() {},
    terminate() {},
  };
  const host = new WorkerWasmHost(endpoint);
  await host.start({ type: "init", wasm: emptyWasm }, []);

  const controllers = Array.from({ length: 100 }, () => new AbortController());
  const calls = controllers.map((controller) =>
    host.call("NeverAnswers", "{}", controller.signal),
  );
  await new Promise<void>((resolve) => setImmediate(resolve));
  const requests = posted.filter(
    (message): message is Extract<WasmWorkerRequest, { type: "call" }> =>
      message.type === "call",
  );
  assert.equal(requests.length, 100);

  for (const controller of controllers) {
    controller.abort(new ConnectError("cancelled", Code.Canceled));
  }
  const results = await Promise.allSettled(calls);
  assert.ok(
    results.every(
      (result) =>
        result.status === "rejected" &&
        result.reason instanceof ConnectError &&
        result.reason.code === Code.Canceled,
    ),
  );
  const pending = (host as unknown as { pending: Map<number, unknown> }).pending;
  assert.equal(pending.size, 0);

  const alreadyAborted = new AbortController();
  alreadyAborted.abort(new ConnectError("cancelled", Code.Canceled));
  const postedBeforeAbortedCall = posted.length;
  await assert.rejects(host.call("AlreadyAborted", "{}", alreadyAborted.signal), {
    code: Code.Canceled,
  });
  assert.equal(posted.length, postedBeforeAbortedCall);

  const firstRequest = requests[0];
  assert.ok(firstRequest);
  onMessage?.({ type: "answer", id: firstRequest.id, envelope: "late answer" });
  assert.equal(pending.size, 0);

  const normalCall = host.call("AfterCancel", "{}");
  await new Promise<void>((resolve) => setImmediate(resolve));
  const normalRequest = posted.find(
    (message): message is Extract<WasmWorkerRequest, { type: "call" }> =>
      message.type === "call" && message.method === "AfterCancel",
  );
  assert.ok(normalRequest);
  onMessage?.({ type: "answer", id: normalRequest.id, envelope: "normal answer" });
  assert.equal(await normalCall, "normal answer");
  assert.equal(pending.size, 0);

  await host.close();
});

test("browser WASM workers connect through the shared request protocol", async () => {
  const sources = [
    emptyWasm,
    emptyWasm.buffer.slice(0),
    new WebAssembly.Module(emptyWasm),
    new Response(emptyWasm),
  ];
  for (const wasm of sources) {
    const worker = new FakeBrowserWorker();
    const connection = await connectBrowserWasm({
      wasm,
      worker: worker as unknown as Worker,
    });
    assert.equal((await connection.rpc.getServerInfo({})).version, "fake-wasm");
    await connection.close();
    assert.equal(worker.terminated, true);
  }
  assert.notEqual(emptyWasm.byteLength, 0);
});

for (const thread of ["worker", "inline"] as const) {
  test(`Node ${thread} WASM connections match the native client`, { skip: wasmSkip }, async () => {
    const wasmFiles = requireArtifacts();
    const wasmResponses: string[] = [];
    const wasmBytes =
      thread === "worker" ? new Uint8Array(await readFile(wasmFiles.wasm)) : undefined;
    await using wasm = await connectWasm({
      wasm: wasmBytes ?? wasmFiles.wasm,
      wasmExec: wasmFiles.wasmExec,
      thread,
      onResponse: ({ method }) => wasmResponses.push(method),
    });
    if (wasmBytes !== undefined) {
      assert.notEqual(wasmBytes.byteLength, 0);
    }
    await using native = await connect();
    const actualCapabilities = [...wasm.info.capabilities];
    assert.notEqual(wasm.info.version, "");
    assert.deepEqual(actualCapabilities, [
      "type_facts",
      "enum_values",
      "evaluate_subject",
      "symbol_attributes",
      "unset_value",
      "feature_values",
      "inline_language",
      "strict_conformance",
      "parse_sources",
      "complex_values",
      "structured_values",
      "measurement_refs",
      "function_values",
      "set_values",
      "tensor_values",
      "infinity_value",
      "diagnostic_codes",
      "schedule",
      "final_time",
      "metaobject_values",
      "undetermined_value",
      "performer",
      "big_int_values",
    ]);

    const wasmModel = await wasm.loads(SAMPLE);
    const nativeModel = await native.loads(SAMPLE);
    assert.equal(wasmModel.hash, nativeModel.hash);
    assert.deepEqual(wasmModel.diagnostics, nativeModel.diagnostics);
    const wasmCar = await wasmModel.symbol("Sample::Car");
    const nativeCar = await nativeModel.symbol("Sample::Car");
    assert.equal(wasmCar.id, nativeCar.id);
    assert.equal(wasmCar.kind, nativeCar.kind);
    assert.deepEqual(
      (await wasmCar.children()).map((child) => child.name),
      (await nativeCar.children()).map((child) => child.name),
    );
    assert.deepEqual(
      await wasmModel.refreshDiagnostics(),
      await nativeModel.refreshDiagnostics(),
    );
    assert.deepEqual(await wasmModel.eval("2 + 2"), await nativeModel.eval("2 + 2"));
    assert.deepEqual(
      await wasmModel.eval("Sample::Car::mass"),
      await nativeModel.eval("Sample::Car::mass"),
    );
    assert.deepEqual(
      await wasmModel.instantiate("Sample::Car"),
      await nativeModel.instantiate("Sample::Car"),
    );

    const validationText = await readFile(
      join(repoRoot, "tests", "wasm", "testdata", "core-validation.sysml"),
      "utf8",
    );
    const wasmValidation = await wasm.loads(validationText);
    const nativeValidation = await native.loads(validationText);
    assert.deepEqual(wasmValidation.diagnostics, nativeValidation.diagnostics);
    assert.ok(wasmValidation.diagnostics.some((diagnostic) => diagnostic.severity === "error"));

    const engineModelText = await readFile(
      join(repoRoot, "tests", "wasm", "testdata", "engine.sysml"),
      "utf8",
    );
    const wasmEngineModel = await wasm.loads(engineModelText);
    const nativeEngineModel = await native.loads(engineModelText);
    for (const expression of ["7", "1.5", "true", "2 ** 70"]) {
      assert.deepEqual(
        await wasmEngineModel.eval(expression),
        await nativeEngineModel.eval(expression),
        `${thread} result for ${expression}`,
      );
    }
    const wasmQuantity = await wasmEngineModel.eval("enginedemo::speed");
    assert.equal(wasmQuantity.kind, "quantity");
    assert.deepEqual(
      wasmQuantity,
      await nativeEngineModel.eval("enginedemo::speed"),
    );
    assert.deepEqual(
      await wasmEngineModel.executeAction("enginedemo::Double", { inputs: { x: 6n } }),
      await nativeEngineModel.executeAction("enginedemo::Double", { inputs: { x: 6n } }),
    );
    assert.deepEqual(
      await wasmEngineModel.executeState("enginedemo::Switch"),
      await nativeEngineModel.executeState("enginedemo::Switch"),
    );
    const sequenceModel = `package Demo {
      private import ScalarValues::*;
      metadata def Safety { attribute level : Integer = 2; }
      part def Vehicle { attribute mass : Real; }
      part seatBelt : Vehicle { @Safety { level = 4; } }
      attribute everything [*] = seatBelt.metadata;
    }`;
    const wasmSequenceModel = await wasm.loads(sequenceModel);
    const nativeSequenceModel = await native.loads(sequenceModel);
    const wasmSequence = await wasmSequenceModel.eval("Demo::everything");
    assert.equal(wasmSequence.kind, "sequence");
    assert.deepEqual(
      wasmSequence,
      await nativeSequenceModel.eval("Demo::everything"),
    );

    const multiple = await wasm.parseSources([
      SourceDocument.inline("first.sysml", "package Multi { part def Wheel; }"),
      SourceDocument.inline("second.sysml", "package Multi { part car : Wheel; }"),
    ]);
    assert.ok((await multiple.symbol("Multi::car")).id !== "");
    assert.deepEqual(await multiple.eval("1 + 1"), { kind: "int", value: 2n });

    await assert.rejects(
      () => wasmModel.query(),
      (error: unknown) =>
        error instanceof MissingCapabilityError && error.capability === "query",
    );
    await assert.rejects(
      () => wasm.rpc.convert({}),
      (error: unknown) => error instanceof ConnectError && error.code === Code.Unimplemented,
    );
    assert.ok(wasmResponses.includes("GetServerInfo"));
    assert.ok(wasmResponses.includes("ParseFile"));
    await wasm.close();
    await assert.rejects(() => wasm.loads(SAMPLE), ClosedConnectionError);
  });
}

test("Node connectWasm resolves real assets from the optional package", { skip: wasmSkip }, async () => {
  const wasmFiles = requireArtifacts();
  const directory = mkdtempSync(join(tmpdir(), "opensysml-wasm-package-"));
  const packageDirectory = join(directory, "node_modules", ...WASM_PACKAGE.split("/"));
  mkdirSync(packageDirectory, { recursive: true });
  copyFileSync(wasmFiles.wasm, join(packageDirectory, "sysml-wasm.wasm"));
  copyFileSync(wasmFiles.wasmExec, join(packageDirectory, "wasm_exec.js"));
  const packageJson = join(packageDirectory, "package.json");
  writeFileSync(packageJson, JSON.stringify({ name: WASM_PACKAGE, version: "0.0.0" }));
  const sources = resolveWasmSources({}, (specifier) => {
    assert.equal(specifier, `${WASM_PACKAGE}/package.json`);
    return packageJson;
  });
  const wasm = await connectWasm(sources);
  try {
    await using native = await connect();
    const wasmModel = await wasm.loads(SAMPLE);
    const nativeModel = await native.loads(SAMPLE);
    assert.equal(wasmModel.hash, nativeModel.hash);
    assert.deepEqual(wasmModel.diagnostics, nativeModel.diagnostics);
    assert.deepEqual(await wasmModel.eval("2 + 2"), await nativeModel.eval("2 + 2"));
  } finally {
    await wasm.close();
    rmSync(directory, { recursive: true, force: true });
  }
});

test("Node WASM workers terminate on close and stale-version refusal", { skip: wasmSkip }, async () => {
  const originalTerminate = Object.getOwnPropertyDescriptor(WorkerPrototype, "terminate")
    ?.value as ((this: NodeWorker) => Promise<number>) | undefined;
  if (originalTerminate === undefined) {
    throw new Error("Worker.prototype.terminate is unavailable");
  }
  let termination: Promise<number> | undefined;
  WorkerPrototype.terminate = function () {
    const pending = originalTerminate.call(this);
    termination = pending;
    return pending;
  };
  try {
    const wasmFiles = requireArtifacts();
    const connection = await connectWasm({
      wasm: wasmFiles.wasm,
      wasmExec: wasmFiles.wasmExec,
    });
    await connection.close();
    assert.ok(connection.isClosed);
    await awaitTermination(termination);
    await assert.rejects(() => connection.loads(SAMPLE), ClosedConnectionError);

    termination = undefined;
    await assert.rejects(
      () =>
        connectWasm({
          wasm: wasmFiles.wasm,
          wasmExec: wasmFiles.wasmExec,
          version: "nope",
        }),
      StaleServiceError,
    );
    await awaitTermination(termination);

    termination = undefined;
    await assert.rejects(
      () =>
        connectWasm({
          wasm: wasmFiles.wasm,
          wasmExec: wasmFiles.wasmExec,
          requireCapabilities: ["not-a-capability"],
        }),
      MissingCapabilityError,
    );
    await awaitTermination(termination);
  } finally {
    WorkerPrototype.terminate = originalTerminate;
  }
});

test("browser inline WASM connections answer client calls", { skip: wasmSkip }, async () => {
  const wasmFiles = requireArtifacts();
  const bytes = new Uint8Array(await readFile(wasmFiles.wasm));
  await using connection = await connectBrowserWasm({
    wasm: new Response(bytes, { headers: { "Content-Type": "application/wasm" } }),
    wasmExec: pathToFileURL(wasmFiles.wasmExec),
  });
  await using native = await connect();
  const wasmModel = await connection.loads(SAMPLE);
  const nativeModel = await native.loads(SAMPLE);
  assert.equal(wasmModel.hash, nativeModel.hash);
  assert.deepEqual(await wasmModel.eval("2 + 2"), await nativeModel.eval("2 + 2"));
  assert.equal(
    (await wasmModel.symbol("Sample::Car")).id,
    (await nativeModel.symbol("Sample::Car")).id,
  );
});

const WorkerPrototype = NodeWorker.prototype;

function requireArtifacts(): WasmArtifacts {
  if (artifacts === undefined) {
    throw new Error("WASM artifacts are required by this test");
  }
  return artifacts;
}

async function awaitTermination(pending: Promise<number> | undefined): Promise<void> {
  if (pending === undefined) {
    throw new Error("the WASM worker was not terminated");
  }
  await pending;
}

function clientFor(
  host: WasmHost,
  options: Parameters<typeof createWasmTransport>[1] = {},
) {
  return createClient(SysMLService, createWasmTransport(host, options));
}

function fixedHost(envelope: string): WasmHost {
  return {
    version: "fake",
    call: () => Promise.resolve(envelope),
    close: () => Promise.resolve(),
  };
}

const emptyWasm = new Uint8Array([0, 97, 115, 109, 1, 0, 0, 0]);

class FakeGo {
  readonly importObject = {};

  run(): Promise<void> {
    (globalThis as typeof globalThis & {
      sysmlWasm?: { version: string; call(method: string, params: string): string };
    }).sysmlWasm = {
      version: "fake-wasm",
      call(method, params) {
        if (method === "Fail") {
          throw new Error("fake failure");
        }
        const result =
          method === "GetServerInfo"
            ? { version: "fake-wasm", capabilities: [] }
            : { method, params: JSON.parse(params) as unknown };
        return JSON.stringify({
          jsonrpc: "2.0",
          id: null,
          result,
        });
      },
    };
    return Promise.resolve();
  }
}

class FakeBrowserWorker {
  terminated = false;

  private readonly listeners = new Map<string, EventListener[]>();
  private readonly serviceListeners: Array<
    (event: MessageEvent<WasmWorkerRequest>) => void
  > = [];

  constructor() {
    serveWasmPort(
      {
        postMessage: (message) => {
          this.dispatch("message", new MessageEvent("message", { data: message }));
        },
        addEventListener: (_type, listener) => this.serviceListeners.push(listener),
        start() {},
      },
      {
        loadWasm: () => Promise.resolve(emptyWasm),
        loadGo: () => Promise.resolve(FakeGo),
      },
    );
  }

  addEventListener(type: string, listener: EventListenerOrEventListenerObject): void {
    const wrapped =
      typeof listener === "function"
        ? listener
        : (event: Event) => {
            listener.handleEvent(event);
          };
    const typeListeners = this.listeners.get(type) ?? [];
    typeListeners.push(wrapped);
    this.listeners.set(type, typeListeners);
  }

  postMessage(message: unknown): void {
    queueMicrotask(() => {
      for (const listener of this.serviceListeners) {
        listener({ data: message as WasmWorkerRequest } as MessageEvent<WasmWorkerRequest>);
      }
    });
  }

  terminate(): Promise<number> {
    this.terminated = true;
    return Promise.resolve(1);
  }

  private dispatch(type: string, event: Event): void {
    for (const listener of this.listeners.get(type) ?? []) {
      listener(event);
    }
  }
}

interface FakePort {
  postMessage(message: WasmWorkerRequest): void;
  addEventListener(
    type: "message",
    listener: (event: MessageEvent<WasmWorkerResponse>) => void,
  ): void;
}

function fakePortPair(): { client: FakePort; service: WasmPortLike } {
  const clientListeners: Array<(event: MessageEvent<WasmWorkerResponse>) => void> = [];
  const serviceListeners: Array<(event: MessageEvent<WasmWorkerRequest>) => void> = [];
  const client: FakePort = {
    postMessage(message) {
      queueMicrotask(() => {
        for (const listener of serviceListeners) {
          listener({ data: message } as MessageEvent<WasmWorkerRequest>);
        }
      });
    },
    addEventListener(_type, listener) {
      clientListeners.push(listener);
    },
  };
  const service: WasmPortLike = {
    postMessage(message) {
      queueMicrotask(() => {
        for (const listener of clientListeners) {
          listener({ data: message } as MessageEvent<WasmWorkerResponse>);
        }
      });
    },
    addEventListener(_type, listener) {
      serviceListeners.push(listener);
    },
  };
  return { client, service };
}

async function waitFor<T extends WasmWorkerResponse>(
  messages: WasmWorkerResponse[],
  predicate: (message: WasmWorkerResponse) => message is T,
): Promise<T>;
async function waitFor(
  messages: WasmWorkerResponse[],
  predicate: (message: WasmWorkerResponse) => boolean,
): Promise<WasmWorkerResponse>;
async function waitFor(
  messages: WasmWorkerResponse[],
  predicate: (message: WasmWorkerResponse) => boolean,
): Promise<WasmWorkerResponse> {
  const deadline = Date.now() + 1_000;
  while (Date.now() < deadline) {
    const match = messages.find(predicate);
    if (match !== undefined) {
      return match;
    }
    await new Promise<void>((resolve) => setTimeout(resolve, 0));
  }
  throw new Error("timed out waiting for a WASM worker message");
}

void (FakeGo satisfies GoConstructor);
