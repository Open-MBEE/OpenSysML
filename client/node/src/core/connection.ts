// A connection: one Connect client against one service, plus the handshake that
// tells the client what that service can do.

import {
  Code,
  ConnectError,
  createClient,
  type Client,
  type Transport,
} from "@connectrpc/connect";
import { create } from "@bufbuild/protobuf";
import {
  CAPABILITY_APPLY_EDITS,
  CAPABILITY_BIG_INT_VALUES,
  CAPABILITY_RATIONAL_VALUES,
  CAPABILITY_CONVERT,
  CAPABILITY_DOCUMENT_QUERY,
  CAPABILITY_ENGINES,
  CAPABILITY_FEATURE_VALUES,
  CAPABILITY_INLINE_LANGUAGE,
  CAPABILITY_MIGRATE,
  CAPABILITY_PARSE_SOURCES,
  CAPABILITY_PERFORMER,
  CAPABILITY_QUERY,
  CAPABILITY_RENDER_DOCUMENT,
  CAPABILITY_RENDER_DOCUMENT_HTML,
  CAPABILITY_SCHEDULE,
  CAPABILITY_SCHEDULE_EXPLORE,
  CAPABILITY_STRICT_CONFORMANCE,
  CAPABILITY_VERIFICATION,
  CAPABILITY_VERIFICATION_QUESTIONS,
  capabilityRefusal,
  mismatchReason,
  requireCapability,
  upgradeRemedy,
  ServerInfo,
} from "./capabilities.js";
import {
  ClosedConnectionError,
  ConversionError,
  ExecutionError,
  InvalidRequestError,
  MigrationError,
  ParseError,
  StaleServiceError,
  UnsupportedValueError,
} from "./errors.js";
import {
  ApplyEditsRequestSchema,
  ExecuteActionRequestSchema,
  ExecuteStateRequestSchema,
  ConvertRequestSchema,
  ListEnginesRequestSchema,
  MigrateRequestSchema,
  ParseSourcesRequestSchema,
  QueryRequestSchema,
  RenderDocumentRequestSchema,
  RunAnalysisRequestSchema,
  RunDocumentQueryRequestSchema,
  RunSweepRequestSchema,
  SweepRangeSchema,
  ValidateInstanceRequestSchema,
  EvaluateCalcRequestSchema,
  VerifyConstraintRequestSchema,
  VerifyRequirementRequestSchema,
  VerifySatisfactionRequestSchema,
  FailureReason,
  SysMLService,
  type Diagnostic as PbDiagnostic,
  type EditOperation,
  type ExecuteActionResponse,
  type ExecuteStateResponse,
  type Instance as PbInstance,
  type Outcome as PbOutcome,
  type Query,
  type RunAnalysisResponse,
  type Verdict as PbVerdict,
  type VerificationVerdict as PbVerificationVerdict,
} from "../generated/sysml_pb.js";
import { callRpc, fromHandshakeError, fromRpcError } from "./status.js";
import { requireString } from "./arguments.js";
import { decodeDiagnostic, Instance, Model, strictConformanceOf } from "./model.js";
import type { ModelDiagnostic } from "./errors.js";
import type { ParseOptions } from "./model.js";
import { sourceDocuments, type Source } from "./sources.js";
import {
  Conversion,
  EXPERIMENTAL_NOTICE,
  isExperimental,
  isV1,
  MIGRATED_NOT_CONVERTED,
  Migration,
  MIGRATION_NOTICE,
  migrationReportOf,
  pathIsV1,
} from "./conversion.js";
import type { MigrateOptions } from "./conversion.js";
import {
  buildQuery,
  elementsOf,
  type QueryElement,
  type QueryForm,
  type QueryPayload,
} from "./query.js";
import {
  bindingHoldsBigInt,
  bindingHoldsRational,
  bindingRationalsAsReals,
  buildBindings,
  documentResult,
  type BindingValues,
  type DocumentQueryResult,
} from "./document.js";
import {
  engineInfoOf,
  standingOf,
  ENGINE_AUTO,
  type EngineInfo,
} from "./engines.js";
import { Exploration, Outcome } from "./exploration.js";
import {
  analysisResultOf,
  decodeVerificationVerdict,
  raiseFailure,
  raiseWrongKind,
  sweepTableOf,
  validationOf,
  verdictOf,
  valuesMap,
  valueOrError,
  QUESTION_EVALUATE,
  CalcResult,
  type AnalysisResult,
  type SweepTable,
  type Validation,
  type Verdict,
} from "./verdict.js";
import { toValue, type SysMLValue, type ValueInput } from "./values.js";
import {
  editErrorOf,
  editResultOf,
  EditRequestBuilder,
  Editor,
  type EditOperationData,
  type EditResult,
} from "./edit.js";

/** Wire encoding of the request and response bodies. Protobuf is the default. */
export type Encoding = "protobuf" | "json";

/** Observability hook: called with every response the service returns. */
export type ResponseTap = (event: {
  method: string;
  response: unknown;
}) => void;

/** Options shared by every way of opening a connection. */
export interface TransportOptions {
  /** `host:port`, `http://host:port` or `https://host:port` of a running service. */
  address?: string;
  /** Body encoding. Protobuf by default; JSON costs ~6x the server CPU on large answers. */
  encoding?: Encoding;
  /** Deadline applied to every call this connection makes. */
  timeoutMs?: number;
  /** Extra headers sent with every call. */
  headers?: Record<string, string>;
  /** Called with each response, for logging, metrics or a conformance runner. */
  onResponse?: ResponseTap;
}

/**
 * What a connection talks to, and how it lets go. A private child releases its
 * reference on close; an external service is only disconnected from.
 */
export interface ConnectionBackend {
  /** Human-readable provenance, used to name the service in an error message. */
  readonly origin: string;
  /** Releases this connection's hold. Never stops a service the client did not start. */
  release(): Promise<void>;
  /** Reports a warning the runtime raises: Node emits it, the browser logs it. */
  warn?(message: string, type: string): void;
  /**
   * Makes a path the caller handed this client absolute, so that what a
   * `Migration` remembers as its source outlives a later change of working
   * directory. Absent where paths have no working directory, as in a browser.
   */
  resolvePath?(path: string): string;
}

/** A connection to a sysml-grpc service. Close it, or use `await using`. */
export class Connection {
  /**
   * The generated Connect client. The ergonomic API covers what this version
   * supports; this is the escape hatch to the RPCs it does not.
   */
  readonly rpc: Client<typeof SysMLService>;
  readonly info: ServerInfo;
  readonly encoding: Encoding;

  private readonly backend: ConnectionBackend;
  private readonly timeoutMs: number | undefined;
  private closed = false;

  private constructor(init: {
    rpc: Client<typeof SysMLService>;
    info: ServerInfo;
    encoding: Encoding;
    backend: ConnectionBackend;
    timeoutMs?: number | undefined;
  }) {
    this.rpc = init.rpc;
    this.info = init.info;
    this.encoding = init.encoding;
    this.backend = init.backend;
    this.timeoutMs = init.timeoutMs;
  }

  /**
   * Opens a connection over `transport` and performs the capability handshake.
   * A refused handshake — the service is not the release asked for, or lacks a
   * required capability — releases the backend and is never returned.
   */
  static async open(init: {
    transport: Transport;
    backend: ConnectionBackend;
    encoding: Encoding;
    timeoutMs?: number | undefined;
    /** Release tag the service must report. */
    requiredVersion?: string;
    /** Capabilities the service must report. */
    requiredCapabilities?: readonly string[];
    /** How a stale-service error addresses the service, and its remedy. */
    stale?: { address: string; remedy: string };
  }): Promise<Connection> {
    const rpc = createClient(SysMLService, init.transport);
    let info: ServerInfo;
    try {
      info = await handshake(rpc, init.backend.origin, init.timeoutMs);
      const reason = mismatchReason(info, {
        ...(init.requiredVersion === undefined
          ? {}
          : { version: init.requiredVersion }),
      });
      if (reason !== undefined) {
        throw new StaleServiceError(
          init.stale?.address ?? init.backend.origin,
          reason,
          init.stale?.remedy ??
            "pass a release the service can report, or none",
          { info },
        );
      }
      for (const capability of init.requiredCapabilities ?? []) {
        requireCapability(info, capability, upgradeRemedy(capability));
      }
    } catch (error) {
      // A refused connection is never returned, so nothing else can release
      // the hold the open took on the service.
      await init.backend.release();
      throw error;
    }
    return new Connection({
      rpc,
      info,
      encoding: init.encoding,
      backend: init.backend,
      ...(init.timeoutMs === undefined ? {} : { timeoutMs: init.timeoutMs }),
    });
  }

  /** Whether this connection has been closed. */
  get isClosed(): boolean {
    return this.closed;
  }

  /** Call options every request of this connection carries. */
  callOptions(): { timeoutMs?: number } {
    if (this.closed) {
      throw new ClosedConnectionError();
    }
    return this.timeoutMs === undefined ? {} : { timeoutMs: this.timeoutMs };
  }

  /** Parses a file the service can read, and returns the model it loaded. */
  load(path: string, options: ParseOptions = {}): Promise<Model> {
    requireString("path", path);
    return Model.parse(this, { source: { case: "filePath", value: path } }, options);
  }

  /** Parses inline source text, and returns the model it loaded. */
  loads(source: string, options: ParseOptions = {}): Promise<Model> {
    return Model.parse(
      this,
      { source: { case: "content", value: source } },
      options,
    );
  }

  /**
   * Adopts a model the service already holds, by hash. Lets a hash pass between
   * processes, and answers NOT_FOUND once the service has evicted the model.
   */
  model(hash: string): Model {
    return Model.adopt(this, hash);
  }

  /** Parses several documents together as one model, one root per document. */
  async parseSources(documents: readonly Source[], options: ParseOptions = {}): Promise<Model> {
    const sources = sourceDocuments(documents);
    requireCapability(
      this.info,
      CAPABILITY_PARSE_SOURCES,
      upgradeRemedy(CAPABILITY_PARSE_SOURCES),
    );
    const capabilities = [CAPABILITY_PARSE_SOURCES];
    if (sources.some((source) => source.language !== undefined)) {
      requireCapability(
        this.info,
        CAPABILITY_INLINE_LANGUAGE,
        upgradeRemedy(CAPABILITY_INLINE_LANGUAGE),
      );
      capabilities.push(CAPABILITY_INLINE_LANGUAGE);
    }
    const strictConformance = strictConformanceOf(options);
    if (strictConformance === true) {
      this.requireStrictConformance();
      capabilities.push(CAPABILITY_STRICT_CONFORMANCE);
    }
    const response = await callRpc(
      this.rpc.parseSources(
        create(ParseSourcesRequestSchema, {
          documents: sources.map((source) => source.toPb()),
          strictConformance: strictConformance === true,
        }),
        this.callOptions(),
      ),
      "file",
      capabilityRefusal(this.info, capabilities),
    );
    const diagnostics = response.diagnostics.map(decodeDiagnostic);
    if (response.error !== "") {
      throw new ParseError(response.error, diagnostics);
    }
    return Model.fromRoots(this, response.modelHash, response.roots, diagnostics, {
      documents: sources.map((source) => source.documentName),
    });
  }

  /**
   * Writes a model out in another of the formats OpenSysML writes. Exactly one
   * of `source.path`, `source.content` and `source.modelHash` names the source.
   */
  async convert(
    toFormat: string,
    source: { path: string } | { content: string } | { modelHash: string },
    options: { fromFormat?: string; tolerateSyntaxErrors?: boolean } = {},
  ): Promise<Conversion> {
    const given = ["path", "content", "modelHash"].filter(
      (name) => (source as Record<string, unknown>)[name] !== undefined,
    );
    if (given.length !== 1) {
      throw new RangeError(
        "provide exactly one of path, content or modelHash; got " +
          (given.length > 0 ? given.join(", ") : "none"),
      );
    }
    const fromFormat = options.fromFormat ?? "";
    if (
      isV1(fromFormat) ||
      (fromFormat === "" && "path" in source && pathIsV1(source.path))
    ) {
      const name = "path" in source ? source.path : "the source";
      throw new InvalidRequestError(
        `${name} ${MIGRATED_NOT_CONVERTED}; call migrate() with the same source`,
        {
          code: "INVALID_ARGUMENT",
        },
      );
    }
    requireCapability(
      this.info,
      CAPABILITY_CONVERT,
      upgradeRemedy(CAPABILITY_CONVERT),
    );
    const request = create(ConvertRequestSchema, {
      toFormat,
      fromFormat: options.fromFormat ?? "",
      tolerateSyntaxErrors: options.tolerateSyntaxErrors === true,
    });
    if ("path" in source) {
      request.source = { case: "filePath", value: source.path };
    } else if ("content" in source) {
      request.source = { case: "content", value: source.content };
    } else {
      request.source = { case: "modelHash", value: source.modelHash };
    }
    const response = await callRpc(
      this.rpc.convert(request, this.callOptions()),
      "path" in source ? "file" : "model",
      capabilityRefusal(this.info, [CAPABILITY_CONVERT]),
    );
    // Judged from the response, so an inferred format counts and a service too
    // old to mark the conversion is still read as experimental.
    const experimental =
      response.experimental ||
      isExperimental(response.fromFormat, response.toFormat);
    let notice = response.experimentalNotice;
    if (notice === "" && experimental) {
      notice = EXPERIMENTAL_NOTICE;
    }
    if (experimental) {
      // Warned before the error is raised: a refusal is the mapping's
      // experimental behavior, not a reason to say nothing about it.
      this.warn(notice, "ExperimentalFeatureWarning");
    }
    const diagnostics = response.diagnostics.map(decodeDiagnostic);
    if (response.error !== "") {
      throw new ConversionError(response.error, diagnostics);
    }
    return new Conversion({
      content: response.content,
      fromFormat: response.fromFormat,
      toFormat: response.toFormat,
      diagnostics,
      experimental,
      experimentalNotice: notice,
    });
  }

  /**
   * Migrates a SysML v1 model — a Cameo/MagicDraw `.mdzip`, a UML XMI `.xmi` or
   * an Eclipse UML2 `.uml` export — to one of the formats OpenSysML writes. A
   * migration is ledgered, not lossless: every v1 element lands in the
   * `Migration.report` as mapped, approximated, unmapped or skipped. Exactly one
   * of `source.path` and `source.content` names the source; inline content is
   * the file's bytes and needs `fromFormat` to say which form they are.
   */
  async migrate(
    toFormat: string,
    source: { path: string } | { content: Uint8Array },
    options: MigrateOptions = {},
  ): Promise<Migration> {
    const given = ["path", "content"].filter(
      (name) => (source as Record<string, unknown>)[name] !== undefined,
    );
    if (given.length !== 1) {
      throw new RangeError(
        "provide exactly one of path or content; got " +
          (given.length > 0 ? given.join(", ") : "none"),
      );
    }
    if (
      options.layoutPath !== undefined &&
      options.layoutContent !== undefined
    ) {
      throw new RangeError(
        "provide at most one of layoutPath and layoutContent",
      );
    }
    const fromFormat = options.fromFormat ?? "";
    if (fromFormat !== "" && !isV1(fromFormat)) {
      const name = "path" in source ? source.path : "the source";
      throw new InvalidRequestError(
        `${name} is ${fromFormat} input, which is converted, not migrated: only a SysML v1 model ` +
          "(xmi, uml or mdzip) is migrated; call convert() with the same source",
        { code: "INVALID_ARGUMENT" },
      );
    }
    requireCapability(
      this.info,
      CAPABILITY_MIGRATE,
      upgradeRemedy(CAPABILITY_MIGRATE),
    );
    // Remembered absolute before the call, against the working directory of
    // the moment the source was named rather than of the moment it answers.
    const sourcePath =
      "path" in source
        ? (this.backend.resolvePath?.(source.path) ?? source.path)
        : "";
    const request = create(MigrateRequestSchema, {
      toFormat,
      fromFormat,
      report: options.report === true,
      results: options.results === true,
      imageBaseUrl: options.imageBaseUrl ?? "",
      strict: options.strict === true,
    });
    if ("path" in source) {
      request.source = { case: "filePath", value: source.path };
    } else {
      request.source = { case: "content", value: source.content };
    }
    if (options.layoutPath !== undefined) {
      request.layout = { case: "layoutPath", value: options.layoutPath };
    } else if (options.layoutContent !== undefined) {
      request.layout = { case: "layoutContent", value: options.layoutContent };
    }
    const response = await callRpc(
      this.rpc.migrate(request, this.callOptions()),
      "path" in source ? "file" : "model",
      capabilityRefusal(this.info, [CAPABILITY_MIGRATE]),
    );
    const notice =
      response.experimentalNotice !== ""
        ? response.experimentalNotice
        : MIGRATION_NOTICE;
    // Warned before the error is raised: a refusal is the mapping's
    // experimental behavior, not a reason to say nothing about it.
    this.warn(notice, "ExperimentalFeatureWarning");
    if (response.error !== "") {
      throw new MigrationError(response.error);
    }
    return new Migration({
      content: response.content,
      fromFormat: response.fromFormat,
      toFormat: response.toFormat,
      report: migrationReportOf(response.report),
      results: response.results,
      files: new Map(response.files.map((file) => [file.path, file.content])),
      sourcePath,
      experimentalNotice: notice,
    });
  }

  /** Runs a SysML v2 API & Services Query over a loaded model. */
  async query(
    modelHash: string,
    options: { payload?: QueryPayload } & QueryForm = {},
  ): Promise<QueryElement[]> {
    requireCapability(
      this.info,
      CAPABILITY_QUERY,
      upgradeRemedy(CAPABILITY_QUERY),
    );
    const response = await callRpc(
      this.rpc.query(
        create(QueryRequestSchema, {
          modelHash,
          oslcQuery: options.oslc ?? "",
          ...queryField(options),
        }),
        this.callOptions(),
      ),
      "model",
      capabilityRefusal(this.info, [CAPABILITY_QUERY]),
    );
    return elementsOf(response);
  }

  /** Runs a named document query and answers its typed rows. */
  async runDocumentQuery(
    modelHash: string,
    queryId: string,
    bindings?: Readonly<Record<string, BindingValues>>,
  ): Promise<DocumentQueryResult> {
    requireCapability(
      this.info,
      CAPABILITY_DOCUMENT_QUERY,
      upgradeRemedy(CAPABILITY_DOCUMENT_QUERY),
    );
    const wire = buildBindings(bindings);
    if (wire.some(bindingHoldsBigInt)) {
      requireCapability(this.info, CAPABILITY_BIG_INT_VALUES, upgradeRemedy(CAPABILITY_BIG_INT_VALUES));
    }
    if (!this.info.has(CAPABILITY_RATIONAL_VALUES)) {
      wire.forEach(bindingRationalsAsReals);
    }
    if (wire.some(bindingHoldsRational)) {
      requireCapability(this.info, CAPABILITY_RATIONAL_VALUES, upgradeRemedy(CAPABILITY_RATIONAL_VALUES));
    }
    const response = await callRpc(
      this.rpc.runDocumentQuery(
        create(RunDocumentQueryRequestSchema, {
          modelHash,
          queryId,
          bindings: wire,
        }),
        this.callOptions(),
      ),
      "model",
      capabilityRefusal(this.info, [CAPABILITY_DOCUMENT_QUERY]),
    );
    return documentResult(response);
  }

  /** Applies source-preserving edits to a loaded model. Operations are the
   * tuple form the {@link Editor} builds, or wire `EditOperation` messages,
   * which pass through as written. */
  async applyEdits(
    modelHash: string,
    operations: readonly (EditOperationData | EditOperation)[],
    options: { acceptDocuments?: boolean; document?: string } = {},
  ): Promise<EditResult> {
    requireCapability(
      this.info,
      CAPABILITY_APPLY_EDITS,
      upgradeRemedy(CAPABILITY_APPLY_EDITS),
    );
    const { operations: built, capabilities } = new EditRequestBuilder(
      this.info,
    ).build(operations);
    const response = await callRpc(
      this.rpc.applyEdits(
        create(ApplyEditsRequestSchema, {
          modelHash,
          operations: built,
          acceptDocuments: options.acceptDocuments ?? true,
          document: options.document ?? "",
        }),
        this.callOptions(),
      ),
      "model",
      capabilityRefusal(this.info, capabilities),
    );
    const refusal = editErrorOf(response);
    if (refusal !== undefined) {
      throw refusal;
    }
    return editResultOf(response);
  }

  /** Starts an edit of a loaded model, applied with {@link Editor.apply}. */
  edit(modelHash: string): Editor {
    return new Editor(modelHash, (hash, operations) =>
      this.applyEdits(hash, operations),
    );
  }

  /** Renders a named document to Markdown or HTML. */
  async renderDocument(
    modelHash: string,
    documentId: string,
    options: { form?: string } = {},
  ): Promise<string> {
    const form = options.form ?? "markdown";
    if (form !== "markdown" && form !== "html") {
      throw new RangeError("form must be 'markdown' or 'html'");
    }
    const capabilities = [CAPABILITY_RENDER_DOCUMENT];
    if (form === "html") {
      capabilities.push(CAPABILITY_RENDER_DOCUMENT_HTML);
    }
    for (const capability of capabilities) {
      requireCapability(this.info, capability, upgradeRemedy(capability));
    }
    const response = await callRpc(
      this.rpc.renderDocument(
        create(RenderDocumentRequestSchema, {
          modelHash,
          documentId,
          form: form === "markdown" ? "" : form,
        }),
        this.callOptions(),
      ),
      "model",
      capabilityRefusal(this.info, capabilities),
    );
    return form === "html" ? response.html : response.markdown;
  }

  /** Executes an action definition. */
  async executeAction(
    modelHash: string,
    actionSymbolId: string,
    options: {
      inputs?: Readonly<Record<string, ValueInput>>;
      schedule?: string;
      performer?: string;
    } = {},
  ): Promise<{
    outputs: ReadonlyMap<string, SysMLValue | UnsupportedValueError>;
    performer: ReadonlyMap<string, SysMLValue | UnsupportedValueError>;
    finalTime: number;
  }> {
    refuseExploring(options.schedule, "exploreAction");
    const response = await this.sendExecuteAction(
      modelHash,
      actionSymbolId,
      options,
    );
    if (response.error !== "") {
      throw new ExecutionError(
        response.error,
        "unspecified",
        response.diagnostics.map(decodeDiagnostic),
      );
    }
    return {
      outputs: valuesMap(Object.entries(response.outputs)),
      performer: valuesMap(Object.entries(response.performerAttributes)),
      finalTime: response.finalTime,
    };
  }

  /** Runs an action once per valid order of its choice points, within a budget. */
  async exploreAction(
    modelHash: string,
    actionSymbolId: string,
    options: {
      inputs?: Readonly<Record<string, ValueInput>>;
      schedule?: string;
      performer?: string;
    } = {},
  ): Promise<Exploration> {
    requireExploring(options.schedule ?? "explore");
    const response = await this.sendExecuteAction(modelHash, actionSymbolId, {
      ...options,
      schedule: options.schedule ?? "explore",
    });
    return this.explorationOf(response);
  }

  private async sendExecuteAction(
    modelHash: string,
    actionSymbolId: string,
    options: {
      inputs?: Readonly<Record<string, ValueInput>>;
      schedule?: string;
      performer?: string;
    },
  ): Promise<ExecuteActionResponse> {
    const inputs: Record<string, ReturnType<typeof toValue>> = {};
    for (const [name, value] of Object.entries(options.inputs ?? {})) {
      inputs[name] = toValue(value, this.info);
    }
    const capabilities = this.runCapabilities(
      options.schedule,
      options.performer,
    );
    const response = await callRpc(
      this.rpc.executeAction(
        create(ExecuteActionRequestSchema, {
          modelHash,
          actionSymbolId,
          inputs,
          schedule: options.schedule ?? "",
          performerSymbolId: options.performer ?? "",
        }),
        this.callOptions(),
      ),
      "model",
      capabilityRefusal(this.info, capabilities),
    );
    return response;
  }

  /** Executes a state machine. */
  async executeState(
    modelHash: string,
    stateMachineSymbolId: string,
    options: {
      events?: readonly string[];
      schedule?: string;
      performer?: string;
    } = {},
  ): Promise<{
    statesVisited: string[];
    finalContext: ReadonlyMap<string, SysMLValue | UnsupportedValueError>;
    finalTime: number;
  }> {
    refuseExploring(options.schedule, "exploreState");
    const response = await this.sendExecuteState(
      modelHash,
      stateMachineSymbolId,
      options,
    );
    if (response.error !== "") {
      throw new ExecutionError(
        response.error,
        "unspecified",
        response.diagnostics.map(decodeDiagnostic),
      );
    }
    return {
      statesVisited: [...response.statesVisited],
      finalContext: valuesMap(Object.entries(response.finalContext)),
      finalTime: response.finalTime,
    };
  }

  /** Runs a state machine over the events once per valid order of its choice points. */
  async exploreState(
    modelHash: string,
    stateMachineSymbolId: string,
    options: {
      events?: readonly string[];
      schedule?: string;
      performer?: string;
    } = {},
  ): Promise<Exploration> {
    requireExploring(options.schedule ?? "explore");
    const response = await this.sendExecuteState(
      modelHash,
      stateMachineSymbolId,
      {
        ...options,
        schedule: options.schedule ?? "explore",
      },
    );
    return this.explorationOf(response);
  }

  private async sendExecuteState(
    modelHash: string,
    stateMachineSymbolId: string,
    options: {
      events?: readonly string[];
      schedule?: string;
      performer?: string;
    },
  ): Promise<ExecuteStateResponse> {
    const capabilities = this.runCapabilities(
      options.schedule,
      options.performer,
    );
    const response = await callRpc(
      this.rpc.executeState(
        create(ExecuteStateRequestSchema, {
          modelHash,
          stateMachineSymbolId,
          events: [...(options.events ?? [])],
          schedule: options.schedule ?? "",
          performerSymbolId: options.performer ?? "",
        }),
        this.callOptions(),
      ),
      "model",
      capabilityRefusal(this.info, capabilities),
    );
    return response;
  }

  private explorationOf(
    response:
      ExecuteActionResponse | ExecuteStateResponse | RunAnalysisResponse,
  ): Exploration {
    if (response.error !== "") {
      const reason =
        "failureReason" in response
          ? response.failureReason
          : FailureReason.UNSPECIFIED;
      raiseFailure(
        response.error,
        reason,
        response.diagnostics.map(decodeDiagnostic),
      );
    }
    const status = response.exploration;
    return new Exploration({
      outcomes: response.outcomes.map((pb) => outcomeOf(pb)),
      complete: status?.complete ?? false,
      runs: status?.runs ?? 0,
      budgetsHit: status?.budgetsHit ?? [],
      runsBudget: status?.runsBudget ?? 0,
      depthBudget: status?.depthBudget ?? 0,
      probabilitiesLowerBound: status?.probabilitiesLowerBound ?? false,
    });
  }

  /** Lists the analysis engines the service answers with, as `sysml -engines` does. */
  async listEngines(): Promise<EngineInfo[]> {
    this.requireEngines();
    const response = await callRpc(
      this.rpc.listEngines(
        create(ListEnginesRequestSchema, {}),
        this.callOptions(),
      ),
      "model",
      capabilityRefusal(this.info, [CAPABILITY_ENGINES]),
    );
    return response.engines.map(engineInfoOf);
  }

  /** Asks whether a constraint holds. */
  async verifyConstraint(
    modelHash: string,
    symbolId: string,
    options: { subject?: string; engine?: string; question?: string } = {},
  ): Promise<Verdict> {
    this.requireVerification();
    this.requireEngine(options.engine);
    this.requireQuestion(options.question);
    const response = await callRpc(
      this.rpc.verifyConstraint(
        create(VerifyConstraintRequestSchema, {
          modelHash,
          symbolId,
          subjectSymbolId: options.subject ?? "",
          engine: engineField(options.engine),
          question: questionField(options.question),
        }),
        this.callOptions(),
      ),
      "model",
      capabilityRefusal(this.info, [
        CAPABILITY_VERIFICATION,
        ...engineCapabilities(options.engine),
        ...questionCapabilities(options.question),
      ]),
    );
    return this.verdictOf(response);
  }

  /** Asks whether a requirement is satisfied. */
  async verifyRequirement(
    modelHash: string,
    symbolId: string,
    options: { subject?: string; engine?: string; question?: string } = {},
  ): Promise<Verdict> {
    this.requireVerification();
    this.requireEngine(options.engine);
    this.requireQuestion(options.question);
    const response = await callRpc(
      this.rpc.verifyRequirement(
        create(VerifyRequirementRequestSchema, {
          modelHash,
          symbolId,
          subjectSymbolId: options.subject ?? "",
          engine: engineField(options.engine),
          question: questionField(options.question),
        }),
        this.callOptions(),
      ),
      "model",
      capabilityRefusal(this.info, [
        CAPABILITY_VERIFICATION,
        ...engineCapabilities(options.engine),
        ...questionCapabilities(options.question),
      ]),
    );
    return this.verdictOf(response);
  }

  /** Asks whether the model's satisfaction assertions hold. */
  async verifySatisfaction(
    modelHash: string,
    options: { symbolId?: string; engine?: string; question?: string } = {},
  ): Promise<Verdict[]> {
    this.requireVerification();
    this.requireEngine(options.engine);
    this.requireQuestion(options.question);
    const response = await callRpc(
      this.rpc.verifySatisfaction(
        create(VerifySatisfactionRequestSchema, {
          modelHash,
          symbolId: options.symbolId ?? "",
          engine: engineField(options.engine),
          question: questionField(options.question),
        }),
        this.callOptions(),
      ),
      "model",
      capabilityRefusal(this.info, [
        CAPABILITY_VERIFICATION,
        ...engineCapabilities(options.engine),
        ...questionCapabilities(options.question),
      ]),
    );
    const diagnostics = response.diagnostics.map(decodeDiagnostic);
    if (response.error !== "") {
      raiseFailure(response.error, response.failureReason, diagnostics);
    }
    for (const pbVerdict of response.verdicts) {
      raiseWrongKind(pbVerdict, diagnostics);
    }
    const instances = this.instancesOf(response.instances);
    const verifications = response.verificationVerdicts.map(
      decodeVerificationVerdict,
    );
    return response.verdicts.map((pbVerdict) =>
      verdictOf(pbVerdict, instances, diagnostics, verifications),
    );
  }

  /** Checks every assertion about an object of one of this model's parts. */
  async validateInstance(
    modelHash: string,
    symbolId: string,
    options: { engine?: string } = {},
  ): Promise<Validation> {
    this.requireVerification();
    this.requireEngine(options.engine);
    const response = await callRpc(
      this.rpc.validateInstance(
        create(ValidateInstanceRequestSchema, {
          modelHash,
          symbolId,
          engine: engineField(options.engine),
        }),
        this.callOptions(),
      ),
      "model",
      capabilityRefusal(this.info, [
        CAPABILITY_VERIFICATION,
        ...engineCapabilities(options.engine),
      ]),
    );
    const diagnostics = response.diagnostics.map(decodeDiagnostic);
    if (response.error !== "") {
      raiseFailure(response.error, response.failureReason, diagnostics);
    }
    return validationOf(
      response,
      this.instancesOf(response.instances),
      diagnostics,
    );
  }

  /** Invokes a calculation. */
  async calc(
    modelHash: string,
    symbolId: string,
    options: { arguments?: readonly ValueInput[]; engine?: string } = {},
  ): Promise<CalcResult> {
    this.requireVerification();
    this.requireEngine(options.engine);
    const response = await callRpc(
      this.rpc.evaluateCalc(
        create(EvaluateCalcRequestSchema, {
          modelHash,
          symbolId,
          arguments: (options.arguments ?? []).map((arg) =>
            toValue(arg, this.info),
          ),
          engine: engineField(options.engine),
        }),
        this.callOptions(),
      ),
      "model",
      capabilityRefusal(this.info, [
        CAPABILITY_VERIFICATION,
        ...engineCapabilities(options.engine),
      ]),
    );
    const diagnostics = response.diagnostics.map(decodeDiagnostic);
    if (response.error !== "") {
      raiseFailure(response.error, response.failureReason, diagnostics);
    }
    const outputs = new Map<string, SysMLValue | UnsupportedValueError>();
    for (const output of response.outputs) {
      outputs.set(output.name, valueOrError(output.value));
    }
    const result =
      outputs.size === 0 && response.result !== undefined
        ? valueOrError(response.result)
        : undefined;
    return new CalcResult({
      ...(result === undefined ? {} : { value: result }),
      outputs,
      diagnostics,
      standing: standingOf(response),
    });
  }

  /** Runs an analysis case. */
  async runAnalysis(
    modelHash: string,
    symbolId: string,
    options: {
      subject?: string;
      arguments?: readonly ValueInput[];
      namedArguments?: Readonly<Record<string, ValueInput>>;
      schedule?: string;
      engine?: string;
    } = {},
  ): Promise<AnalysisResult> {
    refuseExploring(options.schedule, "exploreAnalysis");
    if (options.engine === "explore") {
      throw new RangeError(
        "engine 'explore' answers with every outcome, not one run's result: use exploreAnalysis",
      );
    }
    const response = await this.sendRunAnalysis(modelHash, symbolId, options);
    const diagnostics = response.diagnostics.map(decodeDiagnostic);
    // A request refused before the run, or a failure leaving nothing to
    // report, has no partial result.
    if (
      response.error !== "" &&
      response.outputs.length === 0 &&
      response.verdicts.length === 0 &&
      response.evaluations.length === 0 &&
      response.instances.length === 0
    ) {
      raiseFailure(response.error, response.failureReason, diagnostics);
    }
    return analysisResultOf(
      response,
      this.instancesOf(response.instances),
      diagnostics,
    );
  }

  /** Runs an analysis case once per valid order of its choice points. */
  async exploreAnalysis(
    modelHash: string,
    symbolId: string,
    options: {
      subject?: string;
      arguments?: readonly ValueInput[];
      namedArguments?: Readonly<Record<string, ValueInput>>;
      schedule?: string;
    } = {},
  ): Promise<Exploration> {
    requireExploring(options.schedule ?? "explore");
    const response = await this.sendRunAnalysis(modelHash, symbolId, {
      ...options,
      schedule: options.schedule ?? "explore",
    });
    return this.explorationOf(response);
  }

  private async sendRunAnalysis(
    modelHash: string,
    symbolId: string,
    options: {
      subject?: string;
      arguments?: readonly ValueInput[];
      namedArguments?: Readonly<Record<string, ValueInput>>;
      schedule?: string;
      engine?: string;
    },
  ): Promise<RunAnalysisResponse> {
    this.requireVerification();
    this.requireSchedule(options.schedule);
    this.requireEngine(options.engine);
    const namedArguments: Record<string, ReturnType<typeof toValue>> = {};
    for (const [name, arg] of Object.entries(options.namedArguments ?? {})) {
      namedArguments[name] = toValue(arg, this.info);
    }
    const response = await callRpc(
      this.rpc.runAnalysis(
        create(RunAnalysisRequestSchema, {
          modelHash,
          symbolId,
          subjectSymbolId: options.subject ?? "",
          arguments: (options.arguments ?? []).map((arg) =>
            toValue(arg, this.info),
          ),
          namedArguments,
          schedule: options.schedule ?? "",
          engine: engineField(options.engine),
        }),
        this.callOptions(),
      ),
      "model",
      capabilityRefusal(this.info, [
        CAPABILITY_VERIFICATION,
        ...scheduleCapabilities(options.schedule),
        ...engineCapabilities(options.engine),
      ]),
    );
    return response;
  }

  /** Runs an analysis case or calc once per row of a parameter sweep. */
  async runSweep(
    modelHash: string,
    symbolId: string,
    ranges: Readonly<
      Record<
        string,
        | readonly [ValueInput, ValueInput]
        | readonly [ValueInput, ValueInput, ValueInput]
      >
    >,
    options: {
      subject?: string;
      arguments?: readonly ValueInput[];
      namedArguments?: Readonly<Record<string, ValueInput>>;
      samples?: bigint | number;
      seed?: bigint | number;
      engine?: string;
    } = {},
  ): Promise<SweepTable> {
    this.requireVerification();
    this.requireEngine(options.engine);
    const namedArguments: Record<string, ReturnType<typeof toValue>> = {};
    for (const [name, arg] of Object.entries(options.namedArguments ?? {})) {
      namedArguments[name] = toValue(arg, this.info);
    }
    const request = create(RunSweepRequestSchema, {
      modelHash,
      symbolId,
      subjectSymbolId: options.subject ?? "",
      arguments: (options.arguments ?? []).map((arg) =>
        toValue(arg, this.info),
      ),
      namedArguments,
      samples: BigInt(options.samples ?? 0),
      seed: BigInt(options.seed ?? 0),
      engine: engineField(options.engine),
    });
    for (const [name, bounds] of Object.entries(ranges)) {
      request.ranges.push(this.sweepRange(name, bounds));
    }
    const response = await callRpc(
      this.rpc.runSweep(request, this.callOptions()),
      "model",
      capabilityRefusal(this.info, [
        CAPABILITY_VERIFICATION,
        ...engineCapabilities(options.engine),
      ]),
    );
    const diagnostics = response.diagnostics.map(decodeDiagnostic);
    if (response.error !== "") {
      raiseFailure(response.error, response.failureReason, diagnostics);
    }
    return sweepTableOf(
      response,
      this.instancesOf(response.instances),
      diagnostics,
    );
  }

  private sweepRange(
    name: string,
    bounds: readonly ValueInput[],
  ): ReturnType<typeof create<typeof SweepRangeSchema>> {
    if (bounds.length !== 2 && bounds.length !== 3) {
      throw new InvalidRequestError(
        `range of ${name} takes (from, to) or (from, to, step), not ${bounds.length} value(s)`,
      );
    }
    return create(SweepRangeSchema, {
      parameter: name,
      start: toValue(bounds[0], this.info),
      end: toValue(bounds[1], this.info),
      ...(bounds.length === 3 ? { step: toValue(bounds[2], this.info) } : {}),
    });
  }

  private verdictOf(response: {
    verdict?: PbVerdict | undefined;
    instances: PbInstance[];
    diagnostics: PbDiagnostic[];
    error: string;
    verificationVerdicts?: PbVerificationVerdict[] | undefined;
  }): Verdict {
    const diagnostics: readonly ModelDiagnostic[] =
      response.diagnostics.map(decodeDiagnostic);
    if (response.error !== "") {
      throw new ExecutionError(response.error, "unspecified", diagnostics);
    }
    const pbVerdict = response.verdict;
    if (pbVerdict === undefined) {
      throw new ExecutionError(
        "the service answered the verification without a verdict",
      );
    }
    raiseWrongKind(pbVerdict, diagnostics);
    return verdictOf(
      pbVerdict,
      this.instancesOf(response.instances),
      diagnostics,
      (response.verificationVerdicts ?? []).map(decodeVerificationVerdict),
      true,
    );
  }

  private instancesOf(pbInstances: readonly PbInstance[]): Instance[] {
    if (pbInstances.length > 0) {
      requireCapability(
        this.info,
        CAPABILITY_FEATURE_VALUES,
        upgradeRemedy(CAPABILITY_FEATURE_VALUES),
      );
    }
    return pbInstances.map((pb) => new Instance(pb));
  }

  private requireVerification(): void {
    requireCapability(
      this.info,
      CAPABILITY_VERIFICATION,
      upgradeRemedy(CAPABILITY_VERIFICATION),
    );
  }

  private requireEngines(): void {
    requireCapability(
      this.info,
      CAPABILITY_ENGINES,
      upgradeRemedy(CAPABILITY_ENGINES),
    );
  }

  private requireEngine(engine: string | undefined): void {
    for (const capability of engineCapabilities(engine)) {
      requireCapability(this.info, capability, upgradeRemedy(capability));
    }
  }

  private requireQuestion(question: string | undefined): void {
    for (const capability of questionCapabilities(question)) {
      requireCapability(this.info, capability, upgradeRemedy(capability));
    }
  }

  private requireSchedule(schedule: string | undefined): void {
    for (const capability of scheduleCapabilities(schedule)) {
      requireCapability(this.info, capability, upgradeRemedy(capability));
    }
  }

  private requireStrictConformance(): void {
    requireCapability(
      this.info,
      CAPABILITY_STRICT_CONFORMANCE,
      upgradeRemedy(CAPABILITY_STRICT_CONFORMANCE),
    );
  }

  private runCapabilities(
    schedule: string | undefined,
    performer: string | undefined,
  ): string[] {
    const capabilities = scheduleCapabilities(schedule);
    if (performer !== undefined && performer !== "") {
      capabilities.push(CAPABILITY_PERFORMER);
    }
    for (const capability of capabilities) {
      requireCapability(this.info, capability, upgradeRemedy(capability));
    }
    return capabilities;
  }

  private warn(message: string, type: string): void {
    this.backend.warn?.(message, type);
  }

  /** Asks the service what it is and what it can do, again. */
  async serverInfo(): Promise<ServerInfo> {
    try {
      const response = await this.rpc.getServerInfo({}, this.callOptions());
      return new ServerInfo({
        version: response.version,
        capabilities: response.capabilities,
        answered: true,
        origin: this.backend.origin,
      });
    } catch (error) {
      const connectError = ConnectError.from(error);
      // A service too old to answer is reported the same way the handshake
      // reports it: no version, no capabilities, unanswered.
      if (connectError.code === Code.Unimplemented) {
        return new ServerInfo({ version: "", capabilities: [], answered: false, origin: this.backend.origin });
      }
      throw fromRpcError(error);
    }
  }

  /**
   * Releases this connection. A private child loses a reference and stops when
   * the last one goes; a service this client did not start keeps running.
   */
  async close(): Promise<void> {
    if (this.closed) {
      return;
    }
    this.closed = true;
    await this.backend.release();
  }

  async [Symbol.asyncDispose](): Promise<void> {
    await this.close();
  }
}

async function handshake(
  rpc: Client<typeof SysMLService>,
  origin: string,
  timeoutMs: number | undefined,
): Promise<ServerInfo> {
  const options = timeoutMs === undefined ? {} : { timeoutMs };
  try {
    const info = await rpc.getServerInfo({}, options);
    return new ServerInfo({
      version: info.version,
      capabilities: info.capabilities,
      answered: true,
      origin,
    });
  } catch (error) {
    const connectError = ConnectError.from(error);
    // A service too old to answer the handshake is still usable; anything else is not.
    if (connectError.code === Code.Unimplemented) {
      return new ServerInfo({
        version: "",
        capabilities: [],
        answered: false,
        origin,
      });
    }
    throw fromHandshakeError(connectError, origin);
  }
}

// Whether a schedule spelling names the exploring policy, options or not.
function explores(schedule: string | undefined): boolean {
  return (
    schedule !== undefined &&
    schedule !== "" &&
    (schedule === "explore" || schedule.startsWith("explore:"))
  );
}

// The engine field as sent: empty for auto, which every service reads as such.
function engineField(engine: string | undefined): string {
  return engine === undefined || engine === "" || engine === ENGINE_AUTO
    ? ""
    : engine;
}

// The question field as sent: empty for evaluate, which every service reads as such.
function questionField(question: string | undefined): string {
  return question === undefined ||
    question === "" ||
    question === QUESTION_EVALUATE
    ? ""
    : question;
}

// The capabilities a schedule spelling needs of the service: none for the default.
function scheduleCapabilities(schedule: string | undefined): string[] {
  const capabilities: string[] = [];
  if (schedule !== undefined && schedule !== "") {
    capabilities.push(CAPABILITY_SCHEDULE);
  }
  if (explores(schedule)) {
    capabilities.push(CAPABILITY_SCHEDULE_EXPLORE);
  }
  return capabilities;
}

// The capabilities an engine selection needs of the service: none for auto.
function engineCapabilities(engine: string | undefined): string[] {
  const capabilities: string[] = [];
  if (engineField(engine) !== "") {
    capabilities.push(CAPABILITY_ENGINES);
  }
  if (engine === "explore") {
    capabilities.push(CAPABILITY_SCHEDULE_EXPLORE);
  }
  return capabilities;
}

// The capabilities a question needs of the service: none for evaluate.
function questionCapabilities(question: string | undefined): string[] {
  return questionField(question) !== ""
    ? [CAPABILITY_VERIFICATION_QUESTIONS]
    : [];
}

// Refuse an exploring schedule on a method answering one run's result.
function refuseExploring(schedule: string | undefined, method: string): void {
  if (explores(schedule)) {
    throw new RangeError(
      `schedule ${JSON.stringify(schedule)} answers with every outcome, not one run's ` +
        `result: use ${method}`,
    );
  }
}

// Refuse a schedule that would answer one run's result on a method reading outcomes.
function requireExploring(schedule: string): void {
  if (!explores(schedule)) {
    throw new RangeError(
      `schedule ${JSON.stringify(schedule)} answers one run's result, not every outcome: ` +
        `spell it 'explore' or 'explore:runs=<n>,depth=<d>'`,
    );
  }
}

/** One wire outcome read, an undecodable value kept as its error. */
function outcomeOf(pb: PbOutcome): Outcome {
  const outputs = new Map<string, SysMLValue | UnsupportedValueError>();
  for (const [name, value] of Object.entries(pb.outputs)) {
    outputs.set(name, valueOrError(value));
  }
  return new Outcome({
    outputs,
    finalState: pb.finalState,
    statesVisited: pb.statesVisited,
    error: pb.error,
    linearizations: pb.linearizations,
    probability: pb.probability,
    witness: pb.witness,
    diagnostics: pb.diagnostics.map(decodeDiagnostic),
  });
}

/**
 * The Query a request sends. An OSLC-only request sends just oslcQuery (the
 * two are mutually exclusive); every other request sends a Query, empty when
 * nothing was asked for, which answers every element. An empty oslc is no
 * OSLC at all, so a structured ask beside it still sends its Query.
 */
function queryField(options: { payload?: QueryPayload } & QueryForm): { query?: Query } {
  if (options.query !== undefined) {
    return { query: options.query };
  }
  if (options.oslc === undefined || options.oslc === "") {
    return { query: buildQuery(options) };
  }
  return {};
}
