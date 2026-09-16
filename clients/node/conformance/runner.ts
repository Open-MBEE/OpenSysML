// The conformance runner: it executes each scenario through the client's public
// API and compares the answer by the rules in conformance/README.md.

import { ConnectError } from "@connectrpc/connect";
import { readFileSync } from "node:fs";
import { isAbsolute, join, normalize, resolve as resolvePath, sep } from "node:path";
import type { DescMessage, Message } from "@bufbuild/protobuf";

import { SysMLService } from "../src/generated/sysml_pb.js";
import type { Connection } from "../src/core/connection.js";
import { Model } from "../src/core/model.js";
import { statusName } from "../src/core/status.js";
import { check, render } from "./compare.js";
import { byCodeUnit } from "./order.js";
import { MODEL_HASH_PLACEHOLDER, Normalizer } from "./normalize.js";
import { Literal, methodOf, type Expect, type Scenario, type ScenarioModel } from "./scenarios.js";

/** The RPCs v1 of this client covers. Everything else is a stated skip. */
export const COVERED_RPCS = ["GetServerInfo", "ParseFile", "GetSymbol", "Evaluate", "Instantiate"] as const;

/** One scenario's outcome. The shape cmd/conformance writes. */
export interface Result {
  id: string;
  outcome: "pass" | "fail" | "skip" | "error";
  rpc: string;
  reason?: string;
  failures?: string[];
  status: string;
  duration_ms: number;
}

/** One protocol's results. */
export interface Summary {
  protocol: string;
  service: string;
  capabilities: string[];
  total: number;
  passed: number;
  failed: number;
  skipped: number;
  errored: number;
  results: Result[];
}

/** The whole run, across protocols. */
export interface Report {
  service: string;
  total: number;
  passed: number;
  failed: number;
  skipped: number;
  errored: number;
  protocols: Summary[];
}

/** A deliberate corruption of a response, used to prove the runner is not vacuous. */
export type Mutation = (method: string, response: Message) => void;

/** How a runner reports and what it is allowed to do to the answers. */
export interface RunnerOptions {
  /** Directory holding the suite's fixtures. */
  fixtures: string;
  /** How the report names the service under test. */
  service: string;
  /** Protocol name this runner's connection speaks, for the report. */
  protocol: string;
  verbose?: boolean;
  log?: (line: string) => void;
  mutate?: Mutation;
}

const FIXTURE_REFERENCE = /^\$\{fixture:([^}]+)\}$/;

/** A call the client made: the response message and the schema to read it by. */
interface Answer {
  schema: DescMessage;
  message: Message;
}

/** What a scenario's call did, or why this client cannot make it. */
type Attempt =
  | { kind: "skip"; reason: string }
  | { kind: "done"; status: string; message: string; answer?: Answer };

/** Runs the suite over one connection. */
export class Runner {
  private readonly captured = new Map<string, Message>();
  private readonly hashes = new Map<string, string>();
  private readonly parsed = new Map<string, Model>();
  private capabilities: string[] = [];

  constructor(
    private readonly connection: Connection,
    private readonly options: RunnerOptions,
  ) {}

  /**
   * The response tap the runner's connection must install: comparison needs the
   * message on the wire, which the ergonomic API deliberately does not return.
   */
  tap(): (event: { method: string; response: unknown }) => void {
    return ({ method, response }) => {
      if (isMessage(response)) {
        this.options.mutate?.(method, response);
        this.captured.set(method, response);
      }
    };
  }

  /** Reads the capabilities the scenarios gate on. */
  async readCapabilities(): Promise<void> {
    const info = await this.connection.serverInfo();
    this.capabilities = [...info.capabilities].sort(byCodeUnit);
  }

  /** Runs every scenario whose id matches `filter`, reporting as it goes. */
  async runAll(scenarios: Scenario[], filter?: RegExp): Promise<Summary> {
    const summary: Summary = {
      protocol: this.options.protocol,
      service: this.options.service,
      capabilities: this.capabilities,
      total: 0,
      passed: 0,
      failed: 0,
      skipped: 0,
      errored: 0,
      results: [],
    };
    for (const scenario of scenarios) {
      if (filter !== undefined && !filter.test(scenario.id)) {
        continue;
      }
      const result = await this.run(scenario);
      summary.results.push(result);
      summary.total += 1;
      switch (result.outcome) {
        case "pass":
          summary.passed += 1;
          break;
        case "fail":
          summary.failed += 1;
          break;
        case "skip":
          summary.skipped += 1;
          break;
        default:
          summary.errored += 1;
      }
      this.report(result);
    }
    this.log(
      `\n[${summary.protocol}] ${summary.total} scenarios: ${summary.passed} passed, ` +
        `${summary.failed} failed, ${summary.skipped} skipped, ${summary.errored} in error`,
    );
    return summary;
  }

  /** Runs one scenario: parse the model it names, make the call, compare. */
  async run(scenario: Scenario): Promise<Result> {
    const started = performance.now();
    const rpc = methodOf(scenario);
    const result: Result = { id: scenario.id, outcome: "pass", rpc, status: "OK", duration_ms: 0 };
    const finish = (): Result => {
      result.duration_ms = Math.round((performance.now() - started) * 1000) / 1000;
      return result;
    };

    let expect: Expect = scenario.expect ?? {};
    const missing = (scenario.requires_capabilities ?? []).filter(
      (capability) => !this.capabilities.includes(capability),
    );
    if (missing.length > 0) {
      if (scenario.expect_without_capability === undefined) {
        result.outcome = "skip";
        result.status = "-";
        result.reason = `the service does not report ${missing.join(", ")}`;
        return finish();
      }
      expect = scenario.expect_without_capability;
      result.reason = `the service does not report ${missing.join(", ")}, so the without-capability expectation applies`;
    }

    if (scenario.model?.fixtures !== undefined) {
      result.outcome = "skip";
      result.status = "-";
      result.reason = "v1 of this client parses one document at a time, not a model of several";
      return finish();
    }

    let modelHash = "";
    let request: Record<string, unknown>;
    try {
      if (scenario.model !== undefined) {
        modelHash = await this.modelHash(scenario.model);
      }
      request = this.resolve(scenario.request ?? {}, modelHash) as Record<string, unknown>;
    } catch (error) {
      finish();
      return errored(result, error);
    }

    this.captured.clear();
    let attempt: Attempt;
    try {
      attempt = await this.attempt(rpc, request, modelHash, scenario);
    } catch (error) {
      finish();
      return errored(result, error);
    }
    if (attempt.kind === "skip") {
      result.outcome = "skip";
      result.status = "-";
      result.reason = attempt.reason;
      return finish();
    }

    const wantStatus = expect.status ?? "OK";
    result.status = attempt.status;
    if (attempt.status !== "OK") {
      if (attempt.status.toLowerCase() !== wantStatus.toLowerCase()) {
        result.outcome = "fail";
        result.failures = [`status: ${attempt.status} (${attempt.message}), want ${wantStatus}`];
        return finish();
      }
      const wantMessage = expect.status_message_contains;
      if (wantMessage !== undefined && !attempt.message.includes(wantMessage)) {
        result.outcome = "fail";
        result.failures = [
          `status message ${JSON.stringify(attempt.message)} does not contain ${JSON.stringify(wantMessage)}`,
        ];
      }
      return finish();
    }
    if (wantStatus !== "OK") {
      result.outcome = "fail";
      result.failures = [`the call succeeded, want status ${wantStatus}`];
      return finish();
    }
    if (attempt.answer === undefined) {
      finish();
      return errored(result, new Error(`the client made no ${rpc} call the runner could compare`));
    }

    const normalized = new Normalizer(modelHash).normalize(attempt.answer.schema, attempt.answer.message);
    if (this.options.verbose === true) {
      this.log(`       ${render(normalized)}`);
    }
    const failures = check(expect, normalized);
    if (failures.length > 0) {
      result.outcome = "fail";
      result.failures = failures;
    }
    return finish();
  }

  /** Makes the scenario's call through the public API, or says why it cannot. */
  private async attempt(
    rpc: string,
    request: Record<string, unknown>,
    modelHash: string,
    scenario: Scenario,
  ): Promise<Attempt> {
    if (!COVERED_RPCS.includes(rpc as (typeof COVERED_RPCS)[number])) {
      return { kind: "skip", reason: `v1 of this client does not cover ${rpc}` };
    }
    const unsupported = this.unsupported(rpc, request);
    if (unsupported !== undefined) {
      return { kind: "skip", reason: unsupported };
    }
    try {
      await this.call(rpc, request, modelHash, scenario);
    } catch (error) {
      const connectError = asConnectError(error);
      if (connectError !== undefined) {
        return { kind: "done", status: statusName(connectError.code), message: connectError.rawMessage };
      }
      // The API raises a failure the service reported in a successful answer;
      // the answer itself is what the scenario compares.
      if (!this.captured.has(rpc)) {
        throw error;
      }
    }
    const message = this.captured.get(rpc);
    return {
      kind: "done",
      status: "OK",
      message: "",
      ...(message === undefined ? {} : { answer: { schema: outputSchema(rpc), message } }),
    };
  }

  /** Why the public API cannot express this request, when it cannot. */
  private unsupported(rpc: string, request: Record<string, unknown>): string | undefined {
    if (rpc !== "ParseFile") {
      return undefined;
    }
    if (typeof request["content"] !== "string" && typeof request["file_path"] !== "string") {
      return "the client's load()/loads() always name a source, so a request naming neither cannot be made through it";
    }
    return undefined;
  }

  private async call(
    rpc: string,
    request: Record<string, unknown>,
    modelHash: string,
    scenario: Scenario,
  ): Promise<void> {
    switch (rpc) {
      case "GetServerInfo":
        await this.connection.serverInfo();
        return;
      case "ParseFile": {
        const options = {
          ...(typeof request["language"] === "string" ? { language: request["language"] } : {}),
          ...(request["strict_conformance"] === true ? { strict: true } : {}),
        };
        const content = request["content"];
        if (typeof content === "string") {
          await this.connection.loads(content, options);
          return;
        }
        await this.connection.load(String(request["file_path"]), options);
        return;
      }
      case "GetSymbol":
        await this.model(request, modelHash, scenario).symbolById(String(request["symbol_id"]));
        return;
      case "Evaluate": {
        const model = this.model(request, modelHash, scenario);
        await model.eval(String(request["expression"]), {
          ...(typeof request["context_symbol_id"] === "string" ? { context: request["context_symbol_id"] } : {}),
          ...(typeof request["subject_symbol_id"] === "string" ? { subject: request["subject_symbol_id"] } : {}),
        });
        return;
      }
      case "Instantiate":
        await this.model(request, modelHash, scenario).instantiate(String(request["symbol_id"]));
        return;
      default:
        throw new Error(`the runner covers ${COVERED_RPCS.join(", ")}, not ${rpc}`);
    }
  }

  /**
   * The model a scenario's call addresses: the one parsed from its fixture, or a
   * hash adopted as written, which is how "no-such-model" reaches the service.
   */
  private model(request: Record<string, unknown>, modelHash: string, scenario: Scenario): Model {
    const named = request["model_hash"];
    const hash = typeof named === "string" ? named : modelHash;
    if (hash === "") {
      throw new Error(`${scenario.id} names no model hash`);
    }
    return this.parsed.get(hash) ?? this.connection.model(hash);
  }

  /** Parses a scenario's fixture once per run and remembers the hash. */
  private async modelHash(model: ScenarioModel): Promise<string> {
    if (model.fixture === undefined) {
      throw new Error("a model names no fixture");
    }
    const key = `${model.fixture}|${model.language ?? ""}|${String(model.strict_conformance ?? false)}`;
    const known = this.hashes.get(key);
    if (known !== undefined) {
      return known;
    }
    const source = this.fixture(model.fixture);
    const parsed = await this.connection.loads(source, {
      ...(model.language === undefined ? {} : { language: model.language }),
      ...(model.strict_conformance === undefined ? {} : { strict: model.strict_conformance }),
    });
    const failure = parsed.diagnostics.find((diagnostic) => diagnostic.severity === "error");
    if (failure !== undefined) {
      throw new Error(`fixture ${JSON.stringify(model.fixture)} does not parse clean: ${failure.message}`);
    }
    if (parsed.hash === "") {
      throw new Error(`parsing fixture ${JSON.stringify(model.fixture)} returned no model hash`);
    }
    this.hashes.set(key, parsed.hash);
    this.parsed.set(parsed.hash, parsed);
    return parsed.hash;
  }

  /** Replaces "${model_hash}" and "${fixture:<name>}" in a request. */
  private resolve(tree: unknown, modelHash: string): unknown {
    if (Array.isArray(tree)) {
      return tree.map((item) => this.resolve(item, modelHash));
    }
    if (typeof tree === "object" && tree !== null && !(tree instanceof Literal)) {
      const out: Record<string, unknown> = {};
      for (const [key, value] of Object.entries(tree)) {
        out[key] = this.resolve(value, modelHash);
      }
      return out;
    }
    if (typeof tree === "string") {
      if (tree === MODEL_HASH_PLACEHOLDER) {
        if (modelHash === "") {
          throw new Error(`the request names ${MODEL_HASH_PLACEHOLDER} but the scenario declares no model`);
        }
        return modelHash;
      }
      const reference = FIXTURE_REFERENCE.exec(tree);
      if (reference !== null) {
        return this.fixture(reference[1]);
      }
    }
    return tree;
  }

  /** Reads a fixture's source, refusing a name that leaves the fixtures directory. */
  private fixture(name: string): string {
    const fixtures = resolvePath(this.options.fixtures);
    const path = isAbsolute(name) ? normalize(name) : resolvePath(join(fixtures, normalize(name)));
    if (!path.startsWith(fixtures + sep)) {
      throw new Error(`fixture ${JSON.stringify(name)} is outside ${fixtures}`);
    }
    return readFileSync(path, "utf8");
  }

  private report(result: Result): void {
    const marks = { pass: "PASS", fail: "FAIL", skip: "SKIP", error: "ERR " };
    this.log(`${marks[result.outcome]} ${result.id.padEnd(46)} ${result.status}`);
    if (result.reason !== undefined) {
      this.log(`       ${result.reason}`);
    }
    for (const failure of result.failures ?? []) {
      this.log(`       ${failure}`);
    }
  }

  private log(line: string): void {
    (this.options.log ?? console.log)(line);
  }
}

/** Adds one protocol's results to a report. */
export function accumulate(report: Report, summary: Summary): void {
  report.protocols.push(summary);
  report.total += summary.total;
  report.passed += summary.passed;
  report.failed += summary.failed;
  report.skipped += summary.skipped;
  report.errored += summary.errored;
}

/** The response schema of an RPC, which the normalizer reads the answer by. */
function outputSchema(rpc: string): DescMessage {
  const method = Object.values(SysMLService.method).find((candidate) => candidate.name === rpc);
  if (method === undefined) {
    throw new Error(`${SysMLService.typeName} has no RPC ${JSON.stringify(rpc)}`);
  }
  return method.output;
}

/** The Connect error an exception carries, whether raised directly or wrapped. */
function asConnectError(error: unknown): ConnectError | undefined {
  if (error instanceof ConnectError) {
    return error;
  }
  if (error instanceof Error && error.cause instanceof ConnectError) {
    return error.cause;
  }
  return undefined;
}

function errored(result: Result, error: unknown): Result {
  result.outcome = "error";
  result.status = "-";
  result.reason = error instanceof Error ? error.message : String(error);
  return result;
}

function isMessage(value: unknown): value is Message {
  return typeof value === "object" && value !== null && "$typeName" in value;
}
