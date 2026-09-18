// Runs the conformance suite in conformance/ through this client, over each
// protocol asked for, and writes the report tools/cmd/conformance writes.

import { execFileSync } from "node:child_process";
import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { parseArgs } from "node:util";

import { BINARY_ENV, SERVICE_ENV, connect } from "../src/node/index.js";
import type { Connection, Encoding } from "../src/core/connection.js";
import type { ConnectOptions } from "../src/node/index.js";
import { MUTATIONS, type MutationName } from "./mutations.js";
import { Runner, accumulate, type Report } from "./runner.js";
import { loadScenarios } from "./scenarios.js";

/** The protocols the client can be asked to speak, and how each is spoken. */
const PROTOCOLS = new Map<string, { protocol: "connect" | "grpc"; encoding: Encoding }>([
  ["grpc", { protocol: "grpc", encoding: "protobuf" }],
  ["connect", { protocol: "connect", encoding: "protobuf" }],
  ["connect-json", { protocol: "connect", encoding: "json" }],
]);

/** How one run of the suite is configured. */
export interface RunOptions {
  dir: string;
  binary?: string;
  service?: string;
  repo: string;
  protocols: string[];
  report?: string;
  run?: string;
  verbose: boolean;
  allowSkips: boolean;
  mutate?: MutationName;
  log?: (line: string) => void;
}

/** Runs the suite and returns its report; throws when a scenario fails. */
export async function runSuite(options: RunOptions): Promise<Report> {
  const scenarios = loadScenarios(join(options.dir, "scenarios"));
  const filter = options.run === undefined ? undefined : new RegExp(options.run);
  const log = options.log ?? ((line: string) => {
    console.log(line);
  });

  let workDir: string | undefined;
  let binary = options.binary;
  if (options.service === undefined && binary === undefined) {
    workDir = mkdtempSync(join(tmpdir(), "conformance-node-"));
    binary = buildService(options.repo, workDir);
  }
  const previousBinary = process.env[BINARY_ENV];
  const previousService = process.env[SERVICE_ENV];
  if (binary !== undefined) {
    process.env[BINARY_ENV] = binary;
    Reflect.deleteProperty(process.env, SERVICE_ENV);
  }
  if (options.service !== undefined) {
    process.env[SERVICE_ENV] = options.service;
  }

  const service = options.service ?? binary ?? "";
  const report: Report = { service, total: 0, passed: 0, failed: 0, skipped: 0, errored: 0, protocols: [] };
  // One connection held open for the run, so every protocol tests one service
  // process and one parse cache, as tools/cmd/conformance does.
  const held = await connect({ timeoutMs: 60_000 });
  try {
    for (const name of options.protocols) {
      const wire = PROTOCOLS.get(name);
      if (wire === undefined) {
        throw new Error(`unknown protocol ${JSON.stringify(name)}; want ${[...PROTOCOLS.keys()].join(", ")}`);
      }
      const { runner, connection } = await open(name, wire, options, service, log);
      try {
        accumulate(report, await runner.runAll(scenarios, filter));
      } finally {
        await connection.close();
      }
    }
  } finally {
    await held.close();
    restore(BINARY_ENV, previousBinary);
    restore(SERVICE_ENV, previousService);
    if (workDir !== undefined) {
      rmSync(workDir, { recursive: true, force: true });
    }
  }

  if (options.report !== undefined) {
    writeReport(options.report, report);
  }
  if (report.failed > 0 || report.errored > 0) {
    throw new Error(`${report.failed + report.errored} of ${report.total} scenarios failed`);
  }
  if (report.skipped > 0 && !options.allowSkips) {
    throw new Error(
      `${report.skipped} scenarios were skipped because v1 does not cover their RPC; ` +
        "pass --allow-skips to accept that",
    );
  }
  return report;
}

/** One protocol's connection and the runner over it. */
async function open(
  name: string,
  wire: { protocol: "connect" | "grpc"; encoding: Encoding },
  options: RunOptions,
  service: string,
  log: (line: string) => void,
): Promise<{ runner: Runner; connection: Connection }> {
  const runnerOptions = {
    fixtures: join(options.dir, "fixtures"),
    service,
    protocol: name,
    verbose: options.verbose,
    log,
    ...(options.mutate === undefined ? {} : { mutate: MUTATIONS[options.mutate] }),
  };
  // The runner's tap needs the connection and the connection needs the tap, so
  // the tap is installed indirectly.
  const state: { tap?: (event: { method: string; response: unknown }) => void } = {};
  const connectOptions: ConnectOptions = {
    protocol: wire.protocol,
    encoding: wire.encoding,
    timeoutMs: 60_000,
    onResponse: (event) => {
      state.tap?.(event);
    },
  };
  const connection = await connect(connectOptions);
  const runner = new Runner(connection, runnerOptions);
  state.tap = runner.tap();
  await runner.readCapabilities();
  return { runner, connection };
}

/** Builds the service the suite tests, the way tools/cmd/conformance does. */
function buildService(repo: string, workDir: string): string {
  const output = join(workDir, process.platform === "win32" ? "sysml-grpc.exe" : "sysml-grpc");
  execFileSync("go", ["build", "-o", output, "./cmd/sysml-grpc"], { cwd: resolve(repo), stdio: "inherit" });
  return output;
}

function writeReport(path: string, report: Report): void {
  const data = `${JSON.stringify(report, undefined, 2)}\n`;
  if (path === "-") {
    process.stdout.write(data);
    return;
  }
  writeFileSync(path, data);
}

function restore(name: string, value: string | undefined): void {
  if (value === undefined) {
    Reflect.deleteProperty(process.env, name);
  } else {
    process.env[name] = value;
  }
}

/** Parses the command line, mirroring tools/cmd/conformance's flags. */
export function parseOptions(argv: string[]): RunOptions {
  const { values } = parseArgs({
    args: argv,
    options: {
      dir: { type: "string", default: "conformance" },
      binary: { type: "string" },
      service: { type: "string" },
      repo: { type: "string", default: "." },
      protocols: { type: "string", default: "grpc,connect,connect-json" },
      report: { type: "string" },
      run: { type: "string" },
      verbose: { type: "boolean", short: "v", default: false },
      "allow-skips": { type: "boolean", default: false },
      mutate: { type: "string" },
    },
  });
  const protocols = values.protocols.split(",").map((item) => item.trim()).filter((item) => item !== "");
  if (protocols.length === 0) {
    throw new Error("--protocols must name at least one protocol");
  }
  if (new Set(protocols).size !== protocols.length) {
    throw new Error("--protocols names a protocol more than once");
  }
  const mutate: MutationName | undefined = values.mutate;
  if (mutate !== undefined && !(mutate in MUTATIONS)) {
    throw new Error(`unknown --mutate ${JSON.stringify(mutate)}; want ${Object.keys(MUTATIONS).join(", ")}`);
  }
  return {
    dir: values.dir,
    ...(values.binary === undefined ? {} : { binary: values.binary }),
    ...(values.service === undefined ? {} : { service: values.service }),
    repo: values.repo,
    protocols,
    ...(values.report === undefined ? {} : { report: values.report }),
    ...(values.run === undefined ? {} : { run: values.run }),
    verbose: values.verbose,
    allowSkips: values["allow-skips"],
    ...(mutate === undefined ? {} : { mutate }),
  };
}

if (import.meta.url === `file://${process.argv[1]}`) {
  try {
    await runSuite(parseOptions(process.argv.slice(2)));
  } catch (error) {
    process.stderr.write(`conformance: ${error instanceof Error ? error.message : String(error)}\n`);
    process.exitCode = 1;
  }
}
