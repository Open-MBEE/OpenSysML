// The conformance runner: it executes each scenario through the client's public
// API and compares the answer by the rules in conformance/README.md.

import { ConnectError } from "@connectrpc/connect";
import { readFileSync } from "node:fs";
import { isAbsolute, join, normalize, resolve as resolvePath, sep } from "node:path";
import type { DescMessage, Message } from "@bufbuild/protobuf";

import { fromJson } from "@bufbuild/protobuf";
import type { JsonValue } from "@bufbuild/protobuf";
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
import { statusName } from "../src/core/status.js";
import { check, render } from "./compare.js";
import { byCodeUnit } from "./order.js";
import { MODEL_HASH_PLACEHOLDER, Normalizer } from "./normalize.js";
import { Literal, methodOf, type Expect, type Scenario, type ScenarioModel } from "./scenarios.js";

/** The RPCs this client covers. Everything else is a stated skip. */
export const COVERED_RPCS = [
  "ApplyEdits",
  "Convert",
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
      return { kind: "skip", reason: `this client does not cover ${rpc}` };
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
      const names = documents.map((entry) =>
        String((entry as Record<string, unknown>)["filePath"] ?? (entry as Record<string, unknown>)["name"] ?? ""),
      );
      if (new Set(names).size !== names.length) {
        return "the client's parseSources() refuses two documents of one name before asking the service";
      }
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
      case "ParseSources": {
        const documents = (request["documents"] as Record<string, unknown>[] | undefined) ?? [];
        await this.connection.parseSources(
          documents.map((entry) => {
            if (typeof entry["content"] === "string") {
              return SourceDocument.inline(String(entry["name"] ?? ""), entry["content"]);
            }
            return SourceDocument.file(String(entry["filePath"] ?? entry["file_path"] ?? ""));
          }),
          { strictConformance: request["strict_conformance"] === true },
        );
        return;
      }
      case "GetDiagnostics":
        await this.model(request, modelHash, scenario).refreshDiagnostics();
        return;
      case "ExecuteAction":
        await this.model(request, modelHash, scenario).executeAction(String(request["action_symbol_id"]), {
          inputs: valueMap(request["inputs"]),
          ...(typeof request["schedule"] === "string" ? { schedule: request["schedule"] } : {}),
        });
        return;
      case "ExecuteState":
        await this.model(request, modelHash, scenario).executeState(String(request["state_machine_symbol_id"]), {
          ...(typeof request["schedule"] === "string" ? { schedule: request["schedule"] } : {}),
        });
        return;
      case "Convert": {
        const toFormat = String(request["to_format"] ?? "");
        const source =
          typeof request["content"] === "string"
            ? { content: request["content"] as string }
            : typeof request["file_path"] === "string"
              ? { path: request["file_path"] as string }
              : { modelHash };
        await this.connection.convert(toFormat, source, {
          ...(typeof request["from_format"] === "string"
            ? { fromFormat: request["from_format"] }
            : {}),
        });
        return;
      }
      case "ApplyEdits": {
        const operations = (request["operations"] as unknown[] | undefined) ?? [];
        await this.connection.applyEdits(
          modelHash === "" ? String(request["model_hash"] ?? "") : modelHash,
          operations.map((entry) => fromJson(EditOperationSchema, entry as Record<string, JsonValue>)),
          {
            acceptDocuments: request["accept_documents"] === true,
            ...(typeof request["document"] === "string" ? { document: request["document"] } : {}),
          },
        );
        return;
      }
      case "VerifyConstraint":
        await this.model(request, modelHash, scenario).verifyConstraint(String(request["symbol_id"]), {
          ...(typeof request["subject_symbol_id"] === "string" ? { subject: request["subject_symbol_id"] } : {}),
          ...(typeof request["question"] === "string" ? { question: request["question"] } : {}),
        });
        return;
      case "VerifyRequirement":
        await this.model(request, modelHash, scenario).verifyRequirement(String(request["symbol_id"]), {
          ...(typeof request["subject_symbol_id"] === "string" ? { subject: request["subject_symbol_id"] } : {}),
          ...(typeof request["question"] === "string" ? { question: request["question"] } : {}),
        });
        return;
      case "VerifySatisfaction":
        await this.model(request, modelHash, scenario).verifySatisfaction({
          ...(typeof request["symbol_id"] === "string" ? { symbolId: request["symbol_id"] } : {}),
        });
        return;
      case "ValidateInstance":
        await this.model(request, modelHash, scenario).validateInstance(String(request["symbol_id"]));
        return;
      case "EvaluateCalc":
        await this.model(request, modelHash, scenario).calc(String(request["symbol_id"]), {
          arguments: valueList(request["arguments"]),
        });
        return;
      case "Query":
        await this.model(request, modelHash, scenario).query({
          ...(request["query"] !== undefined
            ? { query: fromJson(QuerySchema, request["query"] as Record<string, JsonValue>) }
            : {}),
          ...(typeof request["oslc_query"] === "string" ? { oslc: request["oslc_query"] } : {}),
        });
        return;
      case "RunDocumentQuery": {
        const bindings: Record<string, DocumentValue[]> = {};
        for (const binding of (request["bindings"] as Record<string, unknown>[] | undefined) ?? []) {
          bindings[String(binding["parameter"])] = (
            (binding["values"] as Record<string, JsonValue>[] | undefined) ?? []
          ).map((value) => bindingValue(fromJson(DocumentValueSchema, value)));
        }
        await this.model(request, modelHash, scenario).runDocumentQuery(
          String(request["query_id"]),
          bindings,
        );
        return;
      }
      case "RenderDocument":
        await this.model(request, modelHash, scenario).renderDocument(String(request["document_id"]));
        return;
      case "RunAnalysis":
        await this.model(request, modelHash, scenario).runAnalysis(String(request["symbol_id"]), {
          ...(typeof request["subject_symbol_id"] === "string" ? { subject: request["subject_symbol_id"] } : {}),
          arguments: valueList(request["arguments"]),
          namedArguments: valueMap(request["named_arguments"]),
        });
        return;
      case "RunSweep": {
        const ranges: Record<string, [Value, Value] | [Value, Value, Value]> = {};
        for (const range of (request["ranges"] as Record<string, JsonValue>[] | undefined) ?? []) {
          const start = fromJson(ValueSchema, range["start"] as Record<string, JsonValue>);
          const end = fromJson(ValueSchema, range["end"] as Record<string, JsonValue>);
          ranges[String(range["parameter"])] =
            range["step"] === undefined
              ? [start, end]
              : [start, end, fromJson(ValueSchema, range["step"] as Record<string, JsonValue>)];
        }
        await this.model(request, modelHash, scenario).runSweep(String(request["symbol_id"]), ranges, {
          ...(typeof request["subject_symbol_id"] === "string" ? { subject: request["subject_symbol_id"] } : {}),
          arguments: valueList(request["arguments"]),
          ...(typeof request["samples"] === "number" ? { samples: request["samples"] } : {}),
          ...(typeof request["seed"] === "number" ? { seed: request["seed"] } : {}),
        });
        return;
      }
      case "ListEngines":
        await this.connection.listEngines();
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
    }
    if (tree instanceof Literal) {
      return tree.toJSON();
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
