// The ergonomic surface: a loaded model, its symbols, and the objects it
// instantiates. Everything here is built on Connection.rpc.

import {
  CAPABILITY_EVALUATE_SUBJECT,
  CAPABILITY_INLINE_LANGUAGE,
  CAPABILITY_STRICT_CONFORMANCE,
  CAPABILITY_SYMBOL_ANNOTATING_METADATA,
  CAPABILITY_SYMBOL_MEMBERS,
  requireCapability,
  upgradeRemedy,
} from "./capabilities.js";
import type { Connection, VerifyOptions } from "./connection.js";
import { requireString, requireSourceText } from "./arguments.js";
import { EvaluationError, OpenSysMLError, ParseError, SymbolNotFoundError } from "./errors.js";
import { requireLanguage } from "./sources.js";
import type { ModelDiagnostic } from "./errors.js";
import type { Conversion, ConvertOptions } from "./conversion.js";
import type { ElementProperties } from "./element-properties.js";
import type { InstancePaths } from "./instance-paths.js";
import type { QueryForm, QueryElement, QueryPayload } from "./query.js";
import type { BindingValues, DocumentQueryResult } from "./document.js";
import type { RenderedView } from "./render-view.js";
import type { CheckOptions, CheckResult } from "./check.js";
import type { Graphs } from "./graphs.js";
import type { Editor } from "./edit.js";
import type { Exploration } from "./exploration.js";
import type {
  AnalysisResult,
  CalcResult,
  SweepTable,
  Validation,
  Verdict,
  RunsResult,
  SelfCheckResult,
  Recording,
  ImportResult,
  Comparison,
} from "./verdict.js";
import { fromJson, type JsonValue } from "@bufbuild/protobuf";
import {
  InstanceSchema,
  type Diagnostic,
  type FeatureValue as PbFeatureValue,
  type Instance as PbInstance,
  type InstanceReach as PbInstanceReach,
  type ParseFileRequest,
  type SymbolInfo,
} from "../generated/sysml_pb.js";
import { callRpc } from "./status.js";
import { decodeValue, type SysMLValue, type ValueInput } from "./values.js";

/** How alike two names must be for one to be suggested for the other. */
const NEAR_ENOUGH = 0.6;

/** Symbols a search for near names reads before giving up, so a big model is cheap. */
const NEAR_SEARCH_LIMIT = 500;

/** How alike two names are, from 0 to 1, by edit distance over the longer one. */
function similarity(left: string, right: string): number {
  const longest = Math.max(left.length, right.length);
  if (longest === 0) {
    return 1;
  }
  return 1 - editDistance(left, right) / longest;
}

/** Levenshtein distance, one row of the matrix at a time. */
function editDistance(left: string, right: string): number {
  let previous = Array.from({ length: right.length + 1 }, (_, index) => index);
  for (let row = 1; row <= left.length; row += 1) {
    const current = [row];
    for (let column = 1; column <= right.length; column += 1) {
      const substitution = (previous[column - 1] ?? 0) + (left[row - 1] === right[column - 1] ? 0 : 1);
      const deletion = (previous[column] ?? 0) + 1;
      const insertion = (current[column - 1] ?? 0) + 1;
      current.push(Math.min(substitution, deletion, insertion));
    }
    previous = current;
  }
  return previous[right.length] ?? 0;
}

/** What a symbol lookup asks the service for beyond the symbol itself. */
export interface SymbolOptions {
  /**
   * List the members inherited from standard-library content too, which the
   * service otherwise leaves out and counts in `withheldLibraryMembers`.
   * Requires `symbol_members`.
   */
  includeLibraryMembers?: boolean;
}

/** How a source is parsed. */
export interface ParseOptions {
  /** Language of inline content ("sysml" or "kerml"); requires `inline_language`. */
  language?: string;
  /** Reject the OpenSysML notation extensions; requires `strict_conformance`. */
  strictConformance?: boolean;
}

/** How several documents are parsed as one model. */
export interface ParseSourcesOptions extends ParseOptions {
  /**
   * The `hash` of a model of the same documents the caller holds results of,
   * typically the previous `parseSources` answer: the new model's `affected`
   * then names the documents whose results may differ from it. Requires
   * `parse_sources_affected`.
   */
  baseModelHash?: string;
}

/** Where an expression is evaluated. */
export interface EvalOptions {
  /** FQN of the symbol whose scope the expression's names resolve in. */
  context?: string;
  /** FQN of a usage to instantiate and evaluate against; requires `evaluate_subject`. */
  subject?: string;
}

/** A declared multiplicity range. A bound the service could not evaluate is empty. */
export interface Multiplicity {
  readonly lower: string;
  readonly upper: string;
  /** Whether the range admits no value; undefined when the lower bound is unknown. */
  readonly isOptional: boolean | undefined;
  /** Whether the range admits more than one value; undefined when the upper bound is unknown. */
  readonly isCollection: boolean | undefined;
}

/** The integer one bound spells, or undefined when it spells none. */
function multiplicityBound(bound: string): number | undefined {
  const trimmed = bound.trim();
  if (!/^[+-]?\d+$/.test(trimmed)) {
    return undefined;
  }
  return Number.parseInt(trimmed, 10);
}

/** Builds a Multiplicity whose predicates mirror Python's. */
function multiplicityOf(lower: string, upper: string): Multiplicity {
  const lowerBound = lower === "" ? undefined : multiplicityBound(lower);
  return {
    lower,
    upper,
    isOptional: lowerBound === undefined ? undefined : lowerBound === 0,
    isCollection: isCollectionUpper(upper),
  };
}

/** Whether an upper bound admits more than one: unknown for none, or one that is not a number. */
function isCollectionUpper(upper: string): boolean | undefined {
  if (upper === "") {
    return undefined;
  }
  if (upper === "*") {
    return true;
  }
  const bound = multiplicityBound(upper);
  return bound === undefined ? undefined : bound > 1;
}

/** The symbols of one level of a model, then those of the levels below it. */
async function* walkLevels(level: ModelSymbol[]): AsyncGenerator<ModelSymbol> {
  if (level.length === 0) {
    return;
  }
  yield* level;
  const children = await Promise.all(level.map((symbol) => symbol.children()));
  yield* walkLevels(children.flat());
}

/** A model the service has parsed and holds under its hash. */
export class Model {
  readonly connection: Connection;
  /** The hash the service holds this model under; every later call names it. */
  readonly hash: string;
  /** Diagnostics the parse reported, in the order the service reported them. */
  readonly diagnostics: readonly ModelDiagnostic[];
  /** The name of each document this model was parsed from, empty for an adopted model. */
  readonly documents: readonly string[];
  /**
   * The documents whose analysis, diagnostics or conversion may differ from
   * the base model's, in document order, when `parseSources` was given a
   * `baseModelHash`; every document when the service could not compare the
   * two, and empty for a parse without a base.
   */
  readonly affected: readonly string[];

  private readonly rootSymbols: readonly ModelSymbol[];
  private readonly ownsConnection: boolean;

  private constructor(init: {
    connection: Connection;
    hash: string;
    roots?: readonly ModelSymbol[];
    diagnostics: readonly ModelDiagnostic[];
    ownsConnection: boolean;
    documents?: readonly string[];
    affected?: readonly string[];
    sourcePath?: string;
  }) {
    this.connection = init.connection;
    this.hash = init.hash;
    this.rootSymbols = init.roots ?? [];
    this.diagnostics = init.diagnostics;
    this.ownsConnection = init.ownsConnection;
    this.documents = init.documents ?? [];
    this.affected = init.affected ?? [];
    this.sourcePath = init.sourcePath;
  }

  /** Adopts a model the service already holds, by the hash it holds it under. */
  static adopt(connection: Connection, hash: string): Model {
    return new Model({ connection, hash, diagnostics: [], ownsConnection: false });
  }

  /** Builds a model of several roots, from a ParseSources response. */
  static fromRoots(
    connection: Connection,
    hash: string,
    roots: readonly SymbolInfo[],
    diagnostics: readonly ModelDiagnostic[],
    options: { ownsConnection?: boolean; documents?: readonly string[]; affected?: readonly string[] } = {},
  ): Model {
    return new Model({
      connection,
      hash,
      roots: roots.map((info) => new ModelSymbol(connection, hash, info)),
      diagnostics,
      ownsConnection: options.ownsConnection ?? false,
      ...(options.documents === undefined ? {} : { documents: options.documents }),
      ...(options.affected === undefined ? {} : { affected: options.affected }),
    });
  }

  /** Whether this model was parsed here, and so knows its root symbol. */
  get parsed(): boolean {
    return this.rootSymbols.length > 0;
  }

  /** The model's root namespace. An adopted model has none: look symbols up by id. */
  get root(): ModelSymbol {
    if (this.rootSymbols.length === 0) {
      throw new OpenSysMLError(
        `model ${this.hash} was adopted by hash, not parsed here, so its root is unknown; ` +
          "look a symbol up by its qualified name with symbolById()",
      );
    }
    return this.rootSymbols[0];
  }

  /** One root per document parsed; the one symbol {@link root} names is the first. */
  get roots(): readonly ModelSymbol[] {
    return this.rootSymbols;
  }

  /** The path this model was loaded from, when it was parsed from a file;
   * inline content and parseSources models have none, whatever they are named. */
  readonly sourcePath: string | undefined;

  /** The error-severity diagnostics the parse reported. */
  get errors(): ModelDiagnostic[] {
    return this.diagnostics.filter((diagnostic) => diagnostic.severity === "error");
  }

  /** Whether the parse reported no error-severity diagnostic. */
  get ok(): boolean {
    return this.errors.length === 0;
  }

  /** Raises a {@link ParseError} carrying this model when its parse reported errors. */
  raiseForErrors(): this {
    if (this.hasErrors) {
      throw new ParseError(this.errors.map((diagnostic) => diagnostic.message).join("\n"), this.errors, {
        model: this,
      });
    }
    return this;
  }

  /** Parses a source over `connection`. Used by Connection.load / Connection.loads. */
  static async parse(
    connection: Connection,
    source: Pick<ParseFileRequest, "source">,
    options: ParseOptions = {},
    ownsConnection = false,
  ): Promise<Model> {
    if (source.source.case === "content") {
      requireSourceText("source", source.source.value);
    }
    if (options.language !== undefined) {
      requireLanguage(options.language);
      requireCapability(connection.info, CAPABILITY_INLINE_LANGUAGE, upgradeRemedy(CAPABILITY_INLINE_LANGUAGE));
    }
    const { strictConformance } = options;
    if (strictConformance === true) {
      requireCapability(
        connection.info,
        CAPABILITY_STRICT_CONFORMANCE,
        upgradeRemedy(CAPABILITY_STRICT_CONFORMANCE),
      );
    }
    const response = await callRpc(
      connection.rpc.parseFile(
        {
          source: source.source,
          ...(options.language === undefined ? {} : { language: options.language }),
          ...(strictConformance === undefined ? {} : { strictConformance }),
        },
        connection.callOptions(),
      ),
      source.source.case === "filePath" ? "file" : "model",
    );
    const diagnostics = response.diagnostics.map(decodeDiagnostic);
    if (response.error !== "") {
      throw new ParseError(response.error, diagnostics);
    }
    if (response.root === undefined) {
      throw new ParseError("the service parsed the source but returned no root symbol", diagnostics);
    }
    return new Model({
      connection,
      hash: response.modelHash,
      roots: [new ModelSymbol(connection, response.modelHash, response.root)],
      diagnostics,
      ownsConnection,
      ...(source.source.case === "filePath"
        ? { documents: [source.source.value], sourcePath: source.source.value }
        : {}),
    });
  }

  /** Whether the parse reported an error-severity diagnostic. */
  get hasErrors(): boolean {
    return this.diagnostics.some((diagnostic) => diagnostic.severity === "error");
  }

  /** Evaluates a SysML expression against this model. */
  async eval(expression: string, options: EvalOptions = {}): Promise<SysMLValue> {
    requireString("expression", expression);
    if (options.subject !== undefined) {
      requireCapability(
        this.connection.info,
        CAPABILITY_EVALUATE_SUBJECT,
        upgradeRemedy(CAPABILITY_EVALUATE_SUBJECT),
      );
    }
    const response = await callRpc(
      this.connection.rpc.evaluate(
        {
          modelHash: this.hash,
          expression,
          ...(options.context === undefined ? {} : { contextSymbolId: options.context }),
          ...(options.subject === undefined ? {} : { subjectSymbolId: options.subject }),
        },
        this.connection.callOptions(),
      ),
    );
    if (response.error !== "") {
      throw new EvaluationError(response.error, "unspecified", response.diagnostics.map(decodeDiagnostic));
    }
    return decodeValue(response.result);
  }

  /** Looks a symbol up by short name, FQN or id; throws when the model declares none. */
  async symbol(name: string): Promise<ModelSymbol> {
    requireString("name", name);
    // The empty name names the model itself, as Python's model.get("") does.
    if (name === "") {
      return this.parsed ? this.root : this.symbolById(name);
    }
    if (this.looksQualified(name) || !this.parsed) {
      // A qualified name or a name on an adopted model is resolved by the
      // service; one it cannot resolve is searched for, as Python's find does.
      try {
        return await this.symbolById(name);
      } catch (error) {
        if (!(error instanceof SymbolNotFoundError)) {
          throw error;
        }
      }
      const qualified = await this.find(name);
      if (qualified !== undefined) {
        return qualified;
      }
      throw new SymbolNotFoundError(name, await this.nearNames(name));
    }
    const found = await this.find(name);
    if (found === undefined) {
      // A short name may still be one the service resolves directly, as an id.
      try {
        return await this.symbolById(name);
      } catch (error) {
        if (!(error instanceof SymbolNotFoundError)) {
          throw error;
        }
      }
      throw new SymbolNotFoundError(name, await this.nearNames(name));
    }
    return found;
  }

  /** Writes this model out in `toFormat`. */
  convert(
    toFormat: string,
    options: ConvertOptions = {},
  ): Promise<Conversion> {
    return this.connection.convert(toFormat, { modelHash: this.hash }, options);
  }

  /** This model written in SysML notation. */
  toSysml(): Promise<Conversion> {
    return this.convert("sysml");
  }

  /** This model written as RDF Turtle. */
  toTurtle(): Promise<Conversion> {
    return this.convert("ttl");
  }

  /** This model written as API JSON. */
  toApiJson(): Promise<Conversion> {
    return this.convert("api-json");
  }

  /** Runs a SysML v2 API & Services Query over this model. */
  query(options: { payload?: QueryPayload } & QueryForm = {}): Promise<QueryElement[]> {
    return this.connection.query(this.hash, options);
  }

  /** Reads the metamodel properties of one element by their specification names. */
  elementProperties(element: string, ...properties: string[]): Promise<ElementProperties> {
    return this.connection.elementProperties(this.hash, element, properties);
  }

  /**
   * Every root-relative path that reaches one object of this model's
   * population, aliases through connector ends and reference usages included:
   * what `Instance.reachedBy` gives one of. The object is an `Instance` the
   * model instantiated, its id, or a path — the qualified name it was
   * instantiated as (`Garage::car`), its id (`#2`), or a path through feature
   * values from either (`Garage::car.wheels[2]`; indexes count from 1).
   * Refused before asking a service without `instance_paths`.
   */
  instancePaths(object: Instance | bigint | string): Promise<InstancePaths> {
    if (object instanceof Instance) {
      return this.connection.instancePaths(this.hash, { instanceId: object.id });
    }
    if (typeof object === "bigint") {
      return this.connection.instancePaths(this.hash, { instanceId: object });
    }
    requireString("object", object);
    return this.connection.instancePaths(this.hash, { path: object });
  }

  /** Runs a named document query against this model. */
  runDocumentQuery(
    queryId: string,
    bindings?: Readonly<Record<string, BindingValues>>,
  ): Promise<DocumentQueryResult> {
    return this.connection.runDocumentQuery(this.hash, queryId, bindings);
  }

  /** Runs a query expression written in place of a named query against this model. */
  runInlineDocumentQuery(
    expression: string,
    bindings?: Readonly<Record<string, BindingValues>>,
  ): Promise<DocumentQueryResult> {
    return this.connection.runInlineDocumentQuery(this.hash, expression, bindings);
  }

  /** Renders a named document of this model to Markdown, HTML or AsciiDoc. */
  renderDocument(
    documentId: string,
    options: { form?: "markdown" | "html" | "asciidoc" } = {},
  ): Promise<string> {
    return this.connection.renderDocument(this.hash, documentId, options);
  }

  /** Renders a named view or targeted pseudo-view as diagram data. */
  renderView(
    viewName: string,
    options: {
      ports?: "minimal" | "full";
      show?: readonly string[];
      compartments?: "positioned" | "most" | "ports";
    } = {},
  ): Promise<RenderedView> {
    return this.connection.renderView(this.hash, viewName, options);
  }

  /** Searches every schedule of an action of this model, as `sysml -check-property` does. */
  checkAction(
    actionSymbolId: string,
    options: CheckOptions & { inputs?: Readonly<Record<string, ValueInput>> } = {},
  ): Promise<CheckResult> {
    return this.connection.checkAction(this.hash, actionSymbolId, options);
  }

  /** Searches every schedule of a state machine of this model. */
  checkStateMachine(stateMachineSymbolId: string, options: CheckOptions = {}): Promise<CheckResult> {
    return this.connection.checkStateMachine(this.hash, stateMachineSymbolId, options);
  }

  /** Exports the lowered graph of an action or state machine of this model as `graphs:1` JSON. */
  exportGraphs(subject: string): Promise<Graphs> {
    return this.connection.exportGraphs(this.hash, subject);
  }

  /** Sets the feature values data files assign into this model. */
  importData(
    imports: Parameters<Connection["importData"]>[1],
    options: Parameters<Connection["importData"]>[2] = {},
  ): Promise<ImportResult> {
    return this.connection.importData(this.hash, imports, options);
  }

  /** Runs the configurations a results sidecar indexes against this model and compares the runs with the tool's. */
  compareResults(results: string, options: Parameters<Connection["compareResults"]>[2] = {}): Promise<Comparison> {
    return this.connection.compareResults(this.hash, results, options);
  }

  /** Starts an edit of this model, to be applied in one call. */
  edit(): Editor {
    return this.connection.edit(this.hash);
  }

  /** Executes an action definition of this model. */
  executeAction(
    actionSymbolId: string,
    options: {
      inputs?: Readonly<Record<string, ValueInput>>;
      schedule?: string;
      performer?: string;
      /** Simulated seconds to run for (`sysml -advance`); unset runs to completion. */
      advance?: number;
      /** The seed the model's own draws are taken from (`sysml -seed`). */
      seed?: bigint;
      /** How RandomFunctions draws resolve: `random`, `min`, `max` or `average`. */
      draws?: string;
      /** The step the run's clock ticks by, in seconds; 0 is a continuous clock. */
      clockStep?: number;
      /** Return the run's typed trace; needs the `action_trace` capability. */
      trace?: boolean;
      /** Signals to post to the performer in order as the run waits; needs `performer`. */
      events?: readonly string[];
    } = {},
  ) {
    return this.connection.executeAction(this.hash, actionSymbolId, options);
  }

  /** Runs an action once per valid order of its choice points. */
  exploreAction(
    actionSymbolId: string,
    options: {
      inputs?: Readonly<Record<string, ValueInput>>;
      schedule?: string;
      performer?: string;
    } = {},
  ): Promise<Exploration> {
    return this.connection.exploreAction(this.hash, actionSymbolId, options);
  }

  /** Executes a state machine of this model. */
  executeState(
    stateMachineSymbolId: string,
    options: {
      events?: readonly string[];
      schedule?: string;
      performer?: string;
      trace?: boolean;
      advance?: number;
      seed?: bigint;
      draws?: string;
      clockStep?: number;
    } = {},
  ) {
    return this.connection.executeState(this.hash, stateMachineSymbolId, options);
  }

  /** Runs a state machine once per valid order of its choice points. */
  exploreState(
    stateMachineSymbolId: string,
    options: { events?: readonly string[]; schedule?: string; performer?: string } = {},
  ): Promise<Exploration> {
    return this.connection.exploreState(this.hash, stateMachineSymbolId, options);
  }

  /** Asks whether a constraint of this model holds. */
  verifyConstraint(
    symbolId: string,
    options: VerifyOptions = {},
  ): Promise<Verdict> {
    return this.connection.verifyConstraint(this.hash, symbolId, options);
  }

  /** Asks whether a requirement of this model is satisfied. */
  verifyRequirement(
    symbolId: string,
    options: VerifyOptions = {},
  ): Promise<Verdict> {
    return this.connection.verifyRequirement(this.hash, symbolId, options);
  }

  /** Asks whether this model's satisfaction assertions hold. */
  verifySatisfaction(
    options: { symbolId?: string; engine?: string; question?: string } = {},
  ): Promise<Verdict[]> {
    return this.connection.verifySatisfaction(this.hash, options);
  }

  /** Whether every verdict {@link verifySatisfaction} returns holds. */
  async satisfied(
    options: { symbolId?: string; engine?: string; question?: string } = {},
  ): Promise<boolean> {
    const verdicts = await this.verifySatisfaction(options);
    return verdicts.every((verdict) => verdict.verdict.kind === "holds");
  }

  /** Checks every assertion about an object of this model's parts. */
  validateInstance(symbolId: string, options: { engine?: string } = {}): Promise<Validation> {
    return this.connection.validateInstance(this.hash, symbolId, options);
  }

  /** Invokes a calculation of this model. */
  calc(
    symbolId: string,
    options: { arguments?: readonly ValueInput[]; engine?: string } = {},
  ): Promise<CalcResult> {
    return this.connection.calc(this.hash, symbolId, options);
  }

  /** Runs an analysis case of this model. */
  runAnalysis(
    symbolId: string,
    options: {
      subject?: string;
      arguments?: readonly ValueInput[];
      namedArguments?: Readonly<Record<string, ValueInput>>;
      schedule?: string;
      engine?: string;
      /** Preview the external tool call the case would make instead of running it. */
      toolDryRun?: boolean;
    } = {},
  ): Promise<AnalysisResult> {
    return this.connection.runAnalysis(this.hash, symbolId, options);
  }

  /** Runs an analysis case once per valid order of its choice points. */
  exploreAnalysis(
    symbolId: string,
    options: {
      subject?: string;
      arguments?: readonly ValueInput[];
      namedArguments?: Readonly<Record<string, ValueInput>>;
      schedule?: string;
    } = {},
  ): Promise<Exploration> {
    return this.connection.exploreAnalysis(this.hash, symbolId, options);
  }

  /** Runs an action, or a Monte Carlo case, of this model `runs` times. */
  runRuns(options: Parameters<Connection["runRuns"]>[1]): Promise<RunsResult> {
    return this.connection.runRuns(this.hash, options);
  }

  /** Applies the reflective SysMLValidation constraints to every element of this model. */
  selfCheck(packages: readonly string[] = []): Promise<SelfCheckResult> {
    return this.connection.selfCheck(this.hash, packages);
  }

  /** Runs an analysis case of this model and records the runs into it. */
  recordRun(symbolId: string, options: Parameters<Connection["recordRun"]>[2] = {}): Promise<Recording> {
    return this.connection.recordRun(this.hash, symbolId, options);
  }

  /** Runs an analysis case or calc of this model once per row of a sweep. */
  runSweep(
    symbolId: string,
    ranges: Readonly<
      Record<string, readonly [ValueInput, ValueInput] | readonly [ValueInput, ValueInput, ValueInput]>
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
    return this.connection.runSweep(this.hash, symbolId, ranges, options);
  }

  /** The diagnostics the service reports for this model now — for a model of
   * several documents, or one parsed before later edits changed it. `disableLint`
   * leaves those lints out and `enableLint` keeps opt-in ones in, as
   * `sysml -disable-lint`/`-enable-lint` select them. */
  async refreshDiagnostics(
    options: { disableLint?: readonly string[]; enableLint?: readonly string[] } = {},
  ): Promise<ModelDiagnostic[]> {
    const response = await callRpc(
      this.connection.rpc.getDiagnostics(
        {
          modelHash: this.hash,
          disableLint: [...(options.disableLint ?? [])],
          enableLint: [...(options.enableLint ?? [])],
        },
        this.connection.callOptions(),
      ),
    );
    return response.diagnostics.map(decodeDiagnostic);
  }

  /** Looks a symbol up by its qualified name, in one call. */
  async symbolById(id: string, options: SymbolOptions = {}): Promise<ModelSymbol> {
    requireString("id", id);
    if (options.includeLibraryMembers === true) {
      requireCapability(this.connection.info, CAPABILITY_SYMBOL_MEMBERS, upgradeRemedy(CAPABILITY_SYMBOL_MEMBERS));
    }
    const response = await callRpc(
      this.connection.rpc.getSymbol(
        { modelHash: this.hash, symbolId: id, includeLibraryMembers: options.includeLibraryMembers === true },
        this.connection.callOptions(),
      ),
    );
    if (response.symbol === undefined) {
      // A single call: near names are symbol()'s to compute.
      throw new SymbolNotFoundError(id);
    }
    return new ModelSymbol(this.connection, this.hash, response.symbol);
  }

  /** Looks a symbol up by short name, FQN or id, breadth-first from the root. */
  async find(name: string): Promise<ModelSymbol | undefined> {
    requireString("name", name);
    if (name === "") {
      return this.parsed ? this.root : undefined;
    }
    for await (const symbol of this.walk()) {
      if (symbol.name === name || symbol.id === name) {
        return symbol;
      }
    }
    return undefined;
  }

  /** Every symbol of the model, breadth-first from the root. */
  async *walk(): AsyncGenerator<ModelSymbol> {
    yield* walkLevels([...this.rootSymbols]);
  }

  /** Instantiates a part or usage, by short name, FQN or id. */
  async instantiate(name: string): Promise<InstanceTree> {
    requireString("name", name);
    const id = this.looksQualified(name) ? name : (await this.symbol(name)).id;
    const response = await callRpc(
      this.connection.rpc.instantiate(
        { modelHash: this.hash, symbolId: id },
        this.connection.callOptions(),
      ),
    );
    if (response.error !== "") {
      throw new EvaluationError(response.error, "unspecified", response.diagnostics.map(decodeDiagnostic));
    }
    if (response.instance === undefined) {
      throw new EvaluationError(`the service instantiated ${id} but returned no object`);
    }
    return new InstanceTree(
      new Instance(response.instance),
      response.instances.map((instance) => new Instance(instance)),
      response.diagnostics.map(decodeDiagnostic),
    );
  }

  /** Releases the model. Closes the connection only when this model opened it. */
  async close(): Promise<void> {
    if (this.ownsConnection) {
      await this.connection.close();
    }
  }

  async [Symbol.asyncDispose](): Promise<void> {
    await this.close();
  }

  /** Whether a name is one the service can resolve directly, without a search. */
  private looksQualified(name: string): boolean {
    return name.includes("::") || name === this.rootSymbols[0]?.id;
  }

  /** The names of the model closest to one it has not got, best first. */
  private async nearNames(name: string): Promise<string[]> {
    const scored: { id: string; score: number }[] = [];
    let seen = 0;
    for await (const symbol of this.walk()) {
      // An unnamed symbol, the root among them, is near nothing.
      if (symbol.name === "") {
        continue;
      }
      // A short name and a qualified one are both candidates, either being
      // what a mistyped lookup may have meant.
      for (const candidate of [symbol.name, symbol.id]) {
        if (candidate === "") {
          continue;
        }
        const score = similarity(name.toLowerCase(), candidate.toLowerCase());
        if (score >= NEAR_ENOUGH) {
          scored.push({ id: candidate, score });
        }
      }
      seen += 1;
      if (seen === NEAR_SEARCH_LIMIT) {
        break;
      }
    }
    scored.sort((left, right) => right.score - left.score);
    return scored.slice(0, 3).map((one) => one.id);
  }
}

/** One derived type of a feature. */
export interface TypeRef {
  /** FQN of the type. */
  id: string;
  /** Symbol kind of the type (`partDef`, `attributeDef`, …). */
  kind: string;
}

/** Static type facts about a symbol, as the service reports them. `declared`,
 * `resolvedId` and `resolvedKind` are the typing as written (`: T`); `types` is the
 * derived KerML `Feature::type` — the `type` property of `elementProperties` — which
 * the typing of every feature it subsets (`:>`), references (`::>`) or redefines
 * (`:>>`) contributes to. */
export interface TypeFacts {
  declared: string;
  resolvedId: string;
  resolvedKind: string;
  primitive: string;
  primitiveSource: string;
  quantity: boolean;
  unit: string;
  /** Derived types, most specific first with redundant supertypes removed; empty for
   * a definition or when none derives. */
  types: readonly TypeRef[];
}

/** One attribute of a symbol, with its declared default when it has one. */
export interface AttributeFacts {
  name: string;
  type: string;
  unit: string;
  value: SysMLValue;
}

/** One generalization edge written on a symbol: `kind` is `specializes`, `subsets`,
 * `redefines`, `typing` or `references` (reference subsetting, `::>` and `include`).
 * `elementProperties` reflects these under `ownedSpecialization`, with the implied
 * ones the metamodel adds. */
export interface SpecializationFacts {
  kind: string;
  declared: string;
  targetId: string;
  targetKind: string;
}

/** A symbol of a loaded model. */
export class ModelSymbol {
  readonly id: string;
  readonly name: string;
  /** Declaration keyword kind (`partDef`, `actionUsage`, …): the written syntax, not
   * the metaclass `elementProperties` reports. */
  readonly kind: string;
  /** Summary of what is written: `type` and `specializes` are the first written edge
   * of each (`specializations` has them all), `direction` the direction written on the
   * feature (the derived `Feature::direction` is `elementProperties`' `direction`). */
  readonly metadata: Readonly<Record<string, string>>;
  readonly childIds: readonly string[];
  /**
   * Every member, own then inherited, as a service with `symbol_members`
   * enumerates them; empty from one without. The standard-library members are
   * left out and counted in `withheldLibraryMembers` unless the symbol was
   * fetched with `includeLibraryMembers`.
   */
  readonly members: readonly MemberFacts[];
  /** Members inherited from standard-library content the service did not send. */
  readonly withheldLibraryMembers: number;
  /**
   * The metadata usages annotating this symbol, the inverse of KerML
   * `AnnotatingElement::annotatedElement`: its own `#M` prefixes and body
   * `@M` usages, then the ones naming it in an `about` clause elsewhere, which
   * `childIds` and `members` leave out. Empty from a service without
   * `symbol_annotating_metadata`.
   */
  readonly annotatingMetadataIds: readonly string[];
  readonly attributes: readonly AttributeFacts[];
  /** Written typing (`declared`) and derived `Feature::type` (`types`). */
  readonly type: TypeFacts | undefined;
  readonly multiplicity: Multiplicity | undefined;
  /** Every generalization edge written, in declaration order. */
  readonly specializations: readonly SpecializationFacts[];
  /** Library attributes the service did not send, when it withheld any. */
  readonly withheldLibraryAttributes: number;

  private readonly connection: Connection;
  private readonly modelHash: string;

  constructor(connection: Connection, modelHash: string, info: SymbolInfo) {
    this.connection = connection;
    this.modelHash = modelHash;
    this.id = info.id;
    this.name = info.name;
    this.kind = info.kind;
    this.metadata = { ...info.metadata };
    this.childIds = [...info.childIds];
    this.members = info.members.map((member) => ({
      id: member.id,
      name: member.name,
      kind: member.kind,
      inherited: member.inherited,
      inheritedFrom: member.inheritedFrom,
      library: member.library,
    }));
    this.withheldLibraryMembers = info.withheldLibraryMembers;
    this.annotatingMetadataIds = [...info.annotatingMetadataIds];
    this.attributes = info.attributes.map((attribute) => ({
      name: attribute.name,
      type: attribute.type,
      unit: attribute.unit,
      value: decodeValue(attribute.value),
    }));
    this.type =
      info.typeInfo === undefined
        ? undefined
        : {
            declared: info.typeInfo.declared,
            resolvedId: info.typeInfo.resolvedId,
            resolvedKind: info.typeInfo.resolvedKind,
            primitive: info.typeInfo.primitive,
            primitiveSource: info.typeInfo.primitiveSource,
            quantity: info.typeInfo.quantity,
            unit: info.typeInfo.unit,
            types: info.typeInfo.types.map((ref) => ({ id: ref.id, kind: ref.kind })),
          };
    this.multiplicity =
      info.multiplicity === undefined
        ? undefined
        : multiplicityOf(info.multiplicity.lower, info.multiplicity.upper);
    this.specializations = info.specializations.map((specialization) => ({
      kind: specialization.kind,
      declared: specialization.declared,
      targetId: specialization.targetId,
      targetKind: specialization.targetKind,
    }));
    this.withheldLibraryAttributes = info.withheldLibraryAttributes;
  }

  /** The symbols this one owns, fetched one call each. */
  async children(): Promise<ModelSymbol[]> {
    return this.fetchSymbols(this.childIds);
  }

  /**
   * The metadata usages annotating this symbol (`annotatingMetadataIds`), fetched
   * one call each: `type` names the metadata definition and `attributes` reads the
   * values the annotation binds. Refused by a service without
   * `symbol_annotating_metadata`, which reports none.
   */
  async annotatingMetadata(): Promise<ModelSymbol[]> {
    requireCapability(
      this.connection.info,
      CAPABILITY_SYMBOL_ANNOTATING_METADATA,
      upgradeRemedy(CAPABILITY_SYMBOL_ANNOTATING_METADATA),
    );
    return this.fetchSymbols(this.annotatingMetadataIds);
  }

  private async fetchSymbols(ids: readonly string[]): Promise<ModelSymbol[]> {
    const responses = await Promise.all(
      ids.map((id) =>
        callRpc(
          this.connection.rpc.getSymbol(
            { modelHash: this.modelHash, symbolId: id },
            this.connection.callOptions(),
          ),
        ),
      ),
    );
    const symbols: ModelSymbol[] = [];
    for (const response of responses) {
      if (response.symbol !== undefined) {
        symbols.push(new ModelSymbol(this.connection, this.modelHash, response.symbol));
      }
    }
    return symbols;
  }

  /** The value of one attribute, or undefined when the symbol declares no such attribute. */
  attribute(name: string): AttributeFacts | undefined {
    return this.attributes.find((attribute) => attribute.name === name);
  }

  /** The members this symbol declares itself: `members` without the inherited ones. */
  ownMembers(): MemberFacts[] {
    return this.members.filter((member) => !member.inherited);
  }

  /**
   * The members inherited through specialization and typing, in inheritance
   * order, as KerML `Type::member` lists them: a redefining member hides the one
   * it redefines, and a private member of a supertype is not inherited.
   */
  inheritedMembers(): MemberFacts[] {
    return this.members.filter((member) => member.inherited);
  }

  /**
   * This symbol fetched again with the standard-library members listed rather
   * than counted, each marked `library` with its provenance. Refused before
   * asking a service without `symbol_members`, which would ignore the request.
   */
  async withLibraryMembers(): Promise<ModelSymbol> {
    requireCapability(this.connection.info, CAPABILITY_SYMBOL_MEMBERS, upgradeRemedy(CAPABILITY_SYMBOL_MEMBERS));
    const response = await callRpc(
      this.connection.rpc.getSymbol(
        { modelHash: this.modelHash, symbolId: this.id, includeLibraryMembers: true },
        this.connection.callOptions(),
      ),
    );
    if (response.symbol === undefined) {
      throw new SymbolNotFoundError(this.id);
    }
    return new ModelSymbol(this.connection, this.modelHash, response.symbol);
  }
}

/** One member of a type, own or inherited. */
export interface MemberFacts {
  /** The member's qualified name, which `symbolById` accepts. */
  id: string;
  /** The declared name, empty for an anonymous member. */
  name: string;
  /** The element kind (`partUsage`, `attributeUsage`, ...). */
  kind: string;
  /** Whether the member is inherited rather than declared by the symbol. */
  inherited: boolean;
  /** The qualified name of the type declaring an inherited member; empty for an own one. */
  inheritedFrom: string;
  /** Whether the member is standard-library content. */
  library: boolean;
}

/**
 * How a traversal of an object graph reached one object. The traversal is
 * depth-first from the root, over the features of an object in name order and
 * the values of a feature in their order, reaching an object once.
 */
export interface InstanceReach {
  /** The object holding this one. */
  instanceId: bigint;
  /** The holder's feature, a key of its `features`. */
  featureName: string;
  /** The qualified name of the feature as declared. */
  featureSymbolId: string;
  /** The position among the values of a multi-valued feature, counted from 1 as an object path indexes them; 0 for a single-valued one. */
  index: number;
  /** The path from the root, the feature names joined by dots and indexed within a multi-valued feature (`wheels[2].hub`). */
  path: string;
}

/** One feature's value on an instantiated object. */
export type FeatureValue =
  | { kind: "single"; value: SysMLValue; materialized: boolean }
  | { kind: "many"; values: SysMLValue[] }
  | { kind: "error"; error: string };

/** An instantiated object. */
export class Instance {
  readonly id: bigint;
  readonly typeId: string;
  readonly features: ReadonlyMap<string, FeatureValue>;
  /**
   * How the traversal that built the object graph reached this object;
   * undefined for the root, and from a service without `instance_paths`.
   */
  readonly reachedBy: InstanceReach | undefined;

  constructor(instance: PbInstance) {
    this.id = instance.id;
    this.typeId = instance.typeSymbolId;
    const features = new Map<string, FeatureValue>();
    for (const [name, value] of Object.entries(instance.featureValues)) {
      features.set(name, decodeFeatureValue(value));
    }
    this.features = features;
    this.reachedBy = instance.reachedBy === undefined ? undefined : decodeReach(instance.reachedBy);
  }

  /**
   * The id of the object holding this one along the path the traversal reached
   * it by, which `InstanceTree.byId` and `InstanceTree.owner` resolve; undefined
   * for the root. One object is often a value of several features (a port and
   * the connector end referencing it are one object); this is the holder the
   * traversal came through, and `Model.instancePaths` lists every path to it.
   */
  get ownerId(): bigint | undefined {
    return this.reachedBy?.instanceId;
  }

  /**
   * The path from the root the traversal reached this object by
   * (`powertrain.engine.fuelIn`, `wheels[2].hub`); empty for the root.
   */
  get path(): string {
    return this.reachedBy?.path ?? "";
  }

  /** The value of one feature, or undefined when the object has no such feature. */
  get(name: string): FeatureValue | undefined {
    return this.features.get(name);
  }
}

/**
 * instanceFromJson decodes one instance from the protojson form of an `Instance`
 * message, as a snapshot of an Instantiate answer stores it, into the typed object.
 */
export function instanceFromJson(json: unknown): Instance {
  return new Instance(fromJson(InstanceSchema, json as JsonValue));
}

/** What one instantiation produced: the root object and every object it owns. */
export class InstanceTree {
  readonly root: Instance;
  /** Every object the instantiation produced, the root included when the service sent it. */
  readonly all: readonly Instance[];
  readonly diagnostics: readonly ModelDiagnostic[];

  constructor(root: Instance, all: readonly Instance[], diagnostics: readonly ModelDiagnostic[]) {
    this.root = root;
    this.all = all;
    this.diagnostics = diagnostics;
  }

  /** The value of one feature of the root object. */
  get(name: string): FeatureValue | undefined {
    return this.root.get(name);
  }

  /** The object with this id, or undefined when the instantiation produced none. */
  byId(id: bigint): Instance | undefined {
    return this.all.find((instance) => instance.id === id) ?? (this.root.id === id ? this.root : undefined);
  }

  /**
   * The object holding another along the path the traversal reached it by;
   * undefined for the root, and for a holder the instantiation did not produce.
   */
  owner(instance: Instance): Instance | undefined {
    const id = instance.ownerId;
    return id === undefined ? undefined : this.byId(id);
  }
}

function decodeReach(reach: PbInstanceReach): InstanceReach {
  return {
    instanceId: reach.instanceId,
    featureName: reach.featureName,
    featureSymbolId: reach.featureSymbolId,
    index: reach.index,
    path: reach.path,
  };
}

function decodeFeatureValue(value: PbFeatureValue): FeatureValue {
  if (value.error !== "") {
    return { kind: "error", error: value.error };
  }
  if (value.values.length > 0) {
    return { kind: "many", values: value.values.map((element) => decodeValue(element)) };
  }
  return { kind: "single", value: decodeValue(value.value), materialized: value.materialized };
}

/** Reads a wire `Diagnostic` into this client's own {@link ModelDiagnostic}. */
export function decodeDiagnostic(diagnostic: Diagnostic): ModelDiagnostic {
  const span = diagnostic.span;
  return {
    severity: diagnostic.severity,
    message: diagnostic.message,
    code: diagnostic.code,
    orderDependent: [...diagnostic.orderDependent],
    independent: diagnostic.independent,
    ...(span === undefined
      ? {}
      : {
          file: span.file,
          startLine: span.startLine,
          startColumn: span.startCol,
          endLine: span.endLine,
          endColumn: span.endCol,
        }),
  };
}
