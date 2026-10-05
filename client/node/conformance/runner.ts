// The conformance runner: it executes each scenario through the client's public
// API and compares the answer by the rules in conformance/README.md.

import { ConnectError } from "@connectrpc/connect";
import { readFileSync } from "node:fs";
import { isAbsolute, join, normalize, resolve as resolvePath, sep } from "node:path";
import { fromJson, type DescMessage, type JsonValue, type Message } from "@bufbuild/protobuf";
import {
  DocumentValueSchema,
  EditOperationSchema,
  QuerySchema,
  SysMLService,
  ValueSchema,
} from "../src/generated/sysml_pb.js";
import type { DocumentValue as PbDocumentValue, Value } from "../src/generated/sysml_pb.js";
import type { Connection } from "../src/core/connection.js";
import { Model } from "../src/core/model.js";
import {
  ElementRef,
  ObjectRef,
  type DocumentValue,
} from "../src/core/document.js";
import { SourceDocument } from "../src/core/sources.js";
import type { SysMLValue } from "../src/core/values.js";
import { ServiceError } from "../src/core/errors.js";
import { statusName } from "../src/core/status.js";
import { check, render } from "./compare.js";
import { byCodeUnit } from "./order.js";
import { MODEL_HASH_PLACEHOLDER, Normalizer } from "./normalize.js";
import { Literal, methodOf, type Expect, type Scenario, type ScenarioModel } from "./scenarios.js";

/** The RPCs this client covers. Everything else is a stated skip. */
export const COVERED_RPCS = [
  "ApplyEdits",
  "Convert",
  "Migrate",
  "Evaluate",
  "EvaluateCalc",
  "ExecuteAction",
  "ExecuteState",
  "GetDiagnostics",
  "GetServerInfo",
  "GetSymbol",
  "Instantiate",
  "ListEngines",
  "ParseFile",
  "ParseSources",
  "Query",
  "RenderDocument",
  "RunAnalysis",
  "RunDocumentQuery",
  "RunSweep",
  "ValidateInstance",
  "VerifyConstraint",
  "VerifyRequirement",
  "VerifySatisfaction",
] as const;

type CoveredRpc = (typeof COVERED_RPCS)[number];

function isCovered(rpc: string): rpc is CoveredRpc {
  return (COVERED_RPCS as readonly string[]).includes(rpc);
}

/** One scenario's outcome. The shape tools/cmd/conformance writes. */
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
const FIXTURE_BASE64_REFERENCE = /^\$\{fixture_base64:([^}]+)\}$/;

/** A call the client made: the response message and the schema to read it by. */
interface Answer {
  schema: DescMessage;
  message: Message;
}

/** What one scenario's call is made with: its resolved request and the model it addresses. */
interface Call {
  request: Record<string, unknown>;
  modelHash: string;
  scenario: Scenario;
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
        // One scenario makes one top-level RPC; the answer it compares is the
        // first of that method, since the API may ask the service more (a
        // not-found lookup still names near misses, which re-queries).
        if (!this.captured.has(method)) {
          this.captured.set(method, response);
        }
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
    for await (const result of this.results(scenarios, filter)) {
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

  /** The results of the scenarios matching `filter`, each run as the one before it ends. */
  private async *results(scenarios: Scenario[], filter?: RegExp): AsyncGenerator<Result> {
    for (const scenario of scenarios) {
      if (filter === undefined || filter.test(scenario.id)) {
        yield this.run(scenario);
      }
    }
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
    if (!isCovered(rpc)) {
      return { kind: "skip", reason: `this client does not cover ${rpc}` };
    }
    const unsupported = this.unsupported(rpc, request);
    if (unsupported !== undefined) {
      return { kind: "skip", reason: unsupported };
    }
    try {
      await this.calls[rpc]({ request, modelHash, scenario });
    } catch (error) {
      const connectError = asConnectError(error);
      if (connectError !== undefined) {
        return { kind: "done", status: statusName(connectError.code), message: connectError.rawMessage };
      }
      // A refusal the client makes before asking, in the service's own words
      // and with the status the service would answer: a v1 model offered to
      // convert(), or a v2 one to migrate().
      if (error instanceof ServiceError && error.code !== undefined && !this.captured.has(rpc)) {
        return { kind: "done", status: error.code, message: error.message };
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
    if (rpc === "ParseFile") {
      if (typeof request["content"] !== "string" && typeof request["file_path"] !== "string") {
        return "the client's load()/loads() always name a source, so a request naming neither cannot be made through it";
      }
      return undefined;
    }
    if (rpc === "ParseSources") {
      const documents = request["documents"];
      if (!Array.isArray(documents) || documents.length === 0) {
        return "the client's parseSources() refuses an empty document list before asking the service";
      }
      const names = documents.map((entry) => {
        const record = entry as Record<string, unknown>;
        return stringOf(record["filePath"] ?? record["name"]);
      });
      if (new Set(names).size !== names.length) {
        return "the client's parseSources() refuses two documents of one name before asking the service";
      }
    }
    return undefined;
  }

  /** How each covered RPC is made through the public API. */
  private readonly calls: Record<CoveredRpc, (call: Call) => Promise<unknown>> = {
    GetServerInfo: () => this.connection.serverInfo(),
    ParseFile: ({ request }) => this.parseFile(request),
    GetSymbol: (call) => this.model(call).symbolById(String(call.request["symbol_id"])),
    Evaluate: (call) =>
      this.model(call).eval(String(call.request["expression"]), {
        ...stringOption(call.request, "context_symbol_id", "context"),
        ...stringOption(call.request, "subject_symbol_id", "subject"),
      }),
    Instantiate: (call) => this.model(call).instantiate(String(call.request["symbol_id"])),
    ParseSources: ({ request }) => this.parseSources(request),
    GetDiagnostics: (call) => this.model(call).refreshDiagnostics(),
    ExecuteAction: (call) =>
      this.model(call).executeAction(String(call.request["action_symbol_id"]), {
        inputs: valueMap(call.request["inputs"]),
        ...stringOption(call.request, "schedule", "schedule"),
      }),
    ExecuteState: (call) =>
      this.model(call).executeState(String(call.request["state_machine_symbol_id"]), {
        ...stringOption(call.request, "schedule", "schedule"),
        ...flagOption(call.request, "trace", "trace"),
      }),
    Convert: ({ request, modelHash }) =>
      this.connection.convert(stringOf(request["to_format"]), convertSource(request, modelHash), {
        ...stringOption(request, "from_format", "fromFormat"),
      }),
    Migrate: ({ request }) =>
      this.connection.migrate(stringOf(request["to_format"]), migrateSource(request), {
        ...stringOption(request, "from_format", "fromFormat"),
        ...flagOption(request, "report", "report"),
        ...flagOption(request, "results", "results"),
        ...stringOption(request, "layout_path", "layoutPath"),
        ...stringOption(request, "layout_content", "layoutContent"),
        ...stringOption(request, "image_base_url", "imageBaseUrl"),
        ...flagOption(request, "strict", "strict"),
      }),
    ApplyEdits: ({ request, modelHash }) => this.applyEdits(request, modelHash),
    VerifyConstraint: (call) =>
      this.model(call).verifyConstraint(String(call.request["symbol_id"]), subjectAndQuestion(call.request)),
    VerifyRequirement: (call) =>
      this.model(call).verifyRequirement(String(call.request["symbol_id"]), subjectAndQuestion(call.request)),
    VerifySatisfaction: (call) =>
      this.model(call).verifySatisfaction(stringOption(call.request, "symbol_id", "symbolId")),
    ValidateInstance: (call) => this.model(call).validateInstance(String(call.request["symbol_id"])),
    EvaluateCalc: (call) =>
      this.model(call).calc(String(call.request["symbol_id"]), {
        arguments: valueList(call.request["arguments"]),
      }),
    Query: (call) =>
      this.model(call).query({
        ...(call.request["query"] !== undefined
          ? { query: fromJson(QuerySchema, call.request["query"] as Record<string, JsonValue>) }
          : {}),
        ...stringOption(call.request, "oslc_query", "oslc"),
      }),
    RunDocumentQuery: (call) =>
      this.model(call).runDocumentQuery(
        String(call.request["query_id"]),
        documentBindings(call.request["bindings"]),
      ),
    RenderDocument: (call) => this.model(call).renderDocument(String(call.request["document_id"])),
    RunAnalysis: (call) =>
      this.model(call).runAnalysis(String(call.request["symbol_id"]), {
        ...stringOption(call.request, "subject_symbol_id", "subject"),
        arguments: valueList(call.request["arguments"]),
        namedArguments: valueMap(call.request["named_arguments"]),
      }),
    RunSweep: (call) =>
      this.model(call).runSweep(String(call.request["symbol_id"]), sweepRanges(call.request["ranges"]), {
        ...stringOption(call.request, "subject_symbol_id", "subject"),
        arguments: valueList(call.request["arguments"]),
        ...numberOption(call.request, "samples", "samples"),
        ...numberOption(call.request, "seed", "seed"),
      }),
    ListEngines: () => this.connection.listEngines(),
  };

  private async parseFile(request: Record<string, unknown>): Promise<void> {
    const options = {
      ...stringOption(request, "language", "language"),
      ...flagOption(request, "strict_conformance", "strict"),
    };
    const content = request["content"];
    if (typeof content === "string") {
      await this.connection.loads(content, options);
      return;
    }
    await this.connection.load(String(request["file_path"]), options);
  }

  private async parseSources(request: Record<string, unknown>): Promise<void> {
    const documents = (request["documents"] as Record<string, unknown>[] | undefined) ?? [];
    await this.connection.parseSources(
      documents.map((entry) => {
        if (typeof entry["content"] === "string") {
          return SourceDocument.inline(stringOf(entry["name"]), entry["content"]);
        }
        return SourceDocument.file(stringOf(entry["filePath"] ?? entry["file_path"]));
      }),
      { strictConformance: request["strict_conformance"] === true },
    );
  }

  private async applyEdits(request: Record<string, unknown>, modelHash: string): Promise<void> {
    const operations = (request["operations"] as unknown[] | undefined) ?? [];
    await this.connection.applyEdits(
      modelHash === "" ? stringOf(request["model_hash"]) : modelHash,
      operations.map((entry) => fromJson(EditOperationSchema, entry as Record<string, JsonValue>)),
      {
        acceptDocuments: request["accept_documents"] === true,
        ...stringOption(request, "document", "document"),
      },
    );
  }

  /**
   * The model a scenario's call addresses: the one parsed from its fixture, or a
   * hash adopted as written, which is how "no-such-model" reaches the service.
   */
  private model({ request, modelHash, scenario }: Call): Model {
    const named = request["model_hash"];
    const hash = typeof named === "string" ? named : modelHash;
    if (hash === "") {
      throw new Error(`${scenario.id} names no model hash`);
    }
    return this.parsed.get(hash) ?? this.connection.model(hash);
  }

  /** Parses a scenario's fixture once per run and remembers the hash. */
  private async modelHash(model: ScenarioModel): Promise<string> {
    if (model.fixture === undefined && model.fixtures === undefined) {
      throw new Error("a model names no fixture");
    }
    const key =
      `${model.fixture ?? ""}|${(model.fixtures ?? []).join(",")}|${model.language ?? ""}|` +
      String(model.strict_conformance ?? false);
    const known = this.hashes.get(key);
    if (known !== undefined) {
      return known;
    }
    const parsed =
      model.fixtures === undefined
        ? await this.connection.loads(this.fixture(model.fixture ?? ""), {
            ...(model.language === undefined ? {} : { language: model.language }),
            ...(model.strict_conformance === undefined ? {} : { strict: model.strict_conformance }),
          })
        : await this.connection.parseSources(
            model.fixtures.map((name) => SourceDocument.inline(name, this.fixture(name))),
            {
              ...(model.strict_conformance === undefined
                ? {}
                : { strictConformance: model.strict_conformance }),
            },
          );
    const failure = parsed.diagnostics.find((diagnostic) => diagnostic.severity === "error");
    if (failure !== undefined) {
      throw new Error(`fixture ${JSON.stringify(model.fixture ?? model.fixtures)} does not parse clean: ${failure.message}`);
    }
    if (parsed.hash === "") {
      throw new Error(`parsing fixture ${JSON.stringify(model.fixture ?? model.fixtures)} returned no model hash`);
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
      const bytes = FIXTURE_BASE64_REFERENCE.exec(tree);
      if (bytes !== null) {
        return this.fixtureBytes(bytes[1]);
      }
    }
    if (tree instanceof Literal) {
      return tree.toJSON();
    }
    return tree;
  }

  /** Reads a fixture's source, refusing a name that leaves the fixtures directory. */
  private fixture(name: string): string {
    return readFileSync(this.fixturePath(name), "utf8");
  }

  /** Reads a fixture's bytes, as `${fixture_base64:<name>}` carries them to a bytes field. */
  private fixtureBytes(name: string): Uint8Array {
    return new Uint8Array(readFileSync(this.fixturePath(name)));
  }

  private fixturePath(name: string): string {
    const fixtures = resolvePath(this.options.fixtures);
    const path = isAbsolute(name) ? normalize(name) : resolvePath(join(fixtures, normalize(name)));
    if (!path.startsWith(fixtures + sep)) {
      throw new Error(`fixture ${JSON.stringify(name)} is outside ${fixtures}`);
    }
    return path;
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

/** A request field read as a string; a field of another form reads as none. */
function stringOf(value: unknown): string {
  return typeof value === "string" ? value : "";
}

/** The option `name` taken from a request's string field `key`; nothing when the field is not a string. */
function stringOption<K extends string>(
  request: Record<string, unknown>,
  key: string,
  name: K,
): { [P in K]?: string } {
  const value = request[key];
  return typeof value === "string" ? ({ [name]: value } as { [P in K]?: string }) : {};
}

/** The option `name` taken from a request's number field `key`; nothing when the field is not a number. */
function numberOption<K extends string>(
  request: Record<string, unknown>,
  key: string,
  name: K,
): { [P in K]?: number } {
  const value = request[key];
  return typeof value === "number" ? ({ [name]: value } as { [P in K]?: number }) : {};
}

/** The flag `name` set when a request's field `key` is `true`; nothing otherwise. */
function flagOption<K extends string>(
  request: Record<string, unknown>,
  key: string,
  name: K,
): { [P in K]?: true } {
  return request[key] === true ? ({ [name]: true } as { [P in K]?: true }) : {};
}

/** The subject and question options of a verification request. */
function subjectAndQuestion(request: Record<string, unknown>): { subject?: string; question?: string } {
  return {
    ...stringOption(request, "subject_symbol_id", "subject"),
    ...stringOption(request, "question", "question"),
  };
}

/** What a Convert request converts: inline content, a file, or the model it names. */
function convertSource(
  request: Record<string, unknown>,
  modelHash: string,
): { content: string } | { path: string } | { modelHash: string } {
  if (typeof request["content"] === "string") {
    return { content: request["content"] };
  }
  if (typeof request["file_path"] === "string") {
    return { path: request["file_path"] };
  }
  return { modelHash };
}

/** What a Migrate request migrates: bytes, base64 text decoded to bytes, or a file. */
function migrateSource(request: Record<string, unknown>): { content: Uint8Array } | { path: string } {
  const content = request["content"];
  if (content instanceof Uint8Array) {
    return { content };
  }
  if (typeof content === "string") {
    return { content: Buffer.from(content, "base64") };
  }
  return { path: stringOf(request["file_path"]) };
}

/** A RunDocumentQuery request's bindings, each parameter's values decoded. */
function documentBindings(json: unknown): Record<string, DocumentValue[]> {
  const bindings: Record<string, DocumentValue[]> = {};
  for (const binding of (json as Record<string, unknown>[] | undefined) ?? []) {
    bindings[String(binding["parameter"])] = (
      (binding["values"] as Record<string, JsonValue>[] | undefined) ?? []
    ).map((value) => bindingValue(fromJson(DocumentValueSchema, value)));
  }
  return bindings;
}

/** A RunSweep request's ranges, each a start and end with an optional step. */
function sweepRanges(json: unknown): Record<string, [Value, Value] | [Value, Value, Value]> {
  const ranges: Record<string, [Value, Value] | [Value, Value, Value]> = {};
  for (const range of (json as Record<string, JsonValue>[] | undefined) ?? []) {
    const start = fromJson(ValueSchema, range["start"] as Record<string, JsonValue>);
    const end = fromJson(ValueSchema, range["end"] as Record<string, JsonValue>);
    ranges[stringOf(range["parameter"])] =
      "step" in range
        ? [start, end, fromJson(ValueSchema, range["step"] as Record<string, JsonValue>)]
        : [start, end];
  }
  return ranges;
}

/** A scenario's `inputs` or `named_arguments` object, each value a wire Value JSON. */
function valueMap(json: unknown): Record<string, Value> {
  const out: Record<string, Value> = {};
  for (const [name, entry] of Object.entries((json as Record<string, JsonValue> | undefined) ?? {})) {
    out[name] = fromJson(ValueSchema, entry as Record<string, JsonValue>);
  }
  return out;
}

/** A scenario's `arguments` list, each element a wire Value JSON. */
function valueList(json: unknown): Value[] {
  return ((json as Record<string, JsonValue>[] | undefined) ?? []).map((entry) =>
    fromJson(ValueSchema, entry),
  );
}

/** A wire DocumentValue as the document API's binding value type. */
function bindingValue(pb: PbDocumentValue): DocumentValue {
  switch (pb.kind.case) {
    case "elementId":
      return new ElementRef(pb.kind.value);
    case "object":
      return new ObjectRef({ id: pb.kind.value.instanceId, path: pb.kind.value.path });
    case "boolValue":
      return pb.kind.value;
    case "stringValue":
      return pb.kind.value;
    case "intValue":
      return pb.kind.value;
    case "realValue":
      return pb.kind.value;
    case "infinity":
      return { kind: "infinity" };
    case "quantity": {
      const quantity = pb.kind.value;
      const value: SysMLValue = {
        kind: "quantity",
        magnitude:
          quantity.magnitude.case === "intMagnitude"
            ? { kind: "int", value: quantity.magnitude.value }
            : { kind: "real", value: quantity.magnitude.case === "realMagnitude" ? quantity.magnitude.value : 0 },
        unit: quantity.unit,
        ...(quantity.unitTerm === undefined
          ? {}
          : {
              unitTerm: {
                scaleNum: quantity.unitTerm.scaleNum,
                scaleDen: quantity.unitTerm.scaleDen,
                factors: quantity.unitTerm.factors.map((factor) => ({
                  unitId: factor.unitId,
                  exponent: factor.exponent,
                })),
              },
            }),
      };
      return value;
    }
    default:
      throw new Error(`a binding cannot carry ${String(pb.kind.case)}`);
  }
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
