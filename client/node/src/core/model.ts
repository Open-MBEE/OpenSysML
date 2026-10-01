// The ergonomic surface: a loaded model, its symbols, and the objects it
// instantiates. Everything here is built on Connection.rpc.

import {
  CAPABILITY_EVALUATE_SUBJECT,
  CAPABILITY_INLINE_LANGUAGE,
  CAPABILITY_STRICT_CONFORMANCE,
  requireCapability,
  upgradeRemedy,
} from "./capabilities.js";
import type { Connection } from "./connection.js";
import { requireString, requireSourceText } from "./arguments.js";
import { EvaluationError, OpenSysMLError, ParseError, SymbolNotFoundError } from "./errors.js";
import { requireLanguage } from "./sources.js";
import type { ModelDiagnostic } from "./errors.js";
import type { Conversion } from "./conversion.js";
import type { QueryForm, QueryElement, QueryPayload } from "./query.js";
import type { BindingValues, DocumentQueryResult } from "./document.js";
import type { Editor } from "./edit.js";
import type { Exploration } from "./exploration.js";
import type {
  AnalysisResult,
  CalcResult,
  SweepTable,
  Validation,
  Verdict,
} from "./verdict.js";
import type {
  Diagnostic,
  FeatureValue as PbFeatureValue,
  Instance as PbInstance,
  ParseFileRequest,
  SymbolInfo,
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

/** How a source is parsed. */
export interface ParseOptions {
  /** Language of inline content ("sysml" or "kerml"); requires `inline_language`. */
  language?: string;
  /** Reject the OpenSysML notation extensions; requires `strict_conformance`. */
  strictConformance?: boolean;
  /**
   * @deprecated Use `strictConformance`; `strict` is its alias. Passing both
   * with different values is refused.
   */
  strict?: boolean;
}

/** Resolves `strictConformance` and its `strict` alias; differing values refuse. */
export function strictConformanceOf(options: {
  strict?: boolean;
  strictConformance?: boolean;
}): boolean | undefined {
  if (
    options.strict !== undefined &&
    options.strictConformance !== undefined &&
    options.strict !== options.strictConformance
  ) {
    throw new RangeError(
      `strict (${options.strict}) and strictConformance (${options.strictConformance}) disagree; ` +
        "pass only strictConformance",
    );
  }
  return options.strictConformance ?? options.strict;
}

/** Where an expression is evaluated. */
export interface EvalOptions {
  /** FQN of the symbol whose scope the expression's names resolve in. */
  context?: string;
  /** FQN of a usage to instantiate and evaluate against; requires `evaluate_subject`. */
  subject?: string;
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

  private readonly rootSymbols: readonly ModelSymbol[];
  private readonly ownsConnection: boolean;

  private constructor(init: {
    connection: Connection;
    hash: string;
    roots?: readonly ModelSymbol[];
    diagnostics: readonly ModelDiagnostic[];
    ownsConnection: boolean;
    documents?: readonly string[];
    sourcePath?: string;
  }) {
    this.connection = init.connection;
    this.hash = init.hash;
    this.rootSymbols = init.roots ?? [];
    this.diagnostics = init.diagnostics;
    this.ownsConnection = init.ownsConnection;
    this.documents = init.documents ?? [];
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
    options: { ownsConnection?: boolean; documents?: readonly string[] } = {},
  ): Model {
    return new Model({
      connection,
      hash,
      roots: roots.map((info) => new ModelSymbol(connection, hash, info)),
      diagnostics,
      ownsConnection: options.ownsConnection ?? false,
      ...(options.documents === undefined ? {} : { documents: options.documents }),
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
    const strictConformance = strictConformanceOf(options);
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
    options: { fromFormat?: string; tolerateSyntaxErrors?: boolean } = {},
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

  /** Runs a named document query against this model. */
  runDocumentQuery(
    queryId: string,
    bindings?: Readonly<Record<string, BindingValues>>,
  ): Promise<DocumentQueryResult> {
    return this.connection.runDocumentQuery(this.hash, queryId, bindings);
  }

  /** Renders a named document of this model to Markdown or HTML. */
  renderDocument(
    documentId: string,
    options: { form?: "markdown" | "html" } = {},
  ): Promise<string> {
    return this.connection.renderDocument(this.hash, documentId, options);
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
    options: { events?: readonly string[]; schedule?: string; performer?: string } = {},
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
    options: { subject?: string; engine?: string; question?: string } = {},
  ): Promise<Verdict> {
    return this.connection.verifyConstraint(this.hash, symbolId, options);
  }

  /** Asks whether a requirement of this model is satisfied. */
  verifyRequirement(
    symbolId: string,
    options: { subject?: string; engine?: string; question?: string } = {},
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
   * several documents, or one parsed before later edits changed it. */
  async refreshDiagnostics(): Promise<ModelDiagnostic[]> {
    const response = await callRpc(
      this.connection.rpc.getDiagnostics(
        { modelHash: this.hash },
        this.connection.callOptions(),
      ),
    );
    return response.diagnostics.map(decodeDiagnostic);
  }

  /** Looks a symbol up by its qualified name, in one call. */
  async symbolById(id: string): Promise<ModelSymbol> {
    requireString("id", id);
    const response = await callRpc(
      this.connection.rpc.getSymbol(
        { modelHash: this.hash, symbolId: id },
        this.connection.callOptions(),
      ),
    );
    if (response.symbol === undefined) {
      throw new SymbolNotFoundError(id, await this.nearNames(id));
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
    const queue: ModelSymbol[] = [...this.rootSymbols];
    while (queue.length > 0) {
      const current = queue.shift();
      if (current === undefined) {
        break;
      }
      yield current;
      queue.push(...(await current.children()));
    }
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

/** Static type facts about a symbol, as the service reports them. */
export interface TypeFacts {
  declared: string;
  resolvedId: string;
  resolvedKind: string;
  primitive: string;
  primitiveSource: string;
  quantity: boolean;
  unit: string;
}

/** One attribute of a symbol, with its declared default when it has one. */
export interface AttributeFacts {
  name: string;
  type: string;
  unit: string;
  value: SysMLValue;
}

/** One specialization relationship a symbol declares or inherits. */
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
  readonly kind: string;
  readonly metadata: Readonly<Record<string, string>>;
  readonly childIds: readonly string[];
  readonly attributes: readonly AttributeFacts[];
  readonly type: TypeFacts | undefined;
  readonly multiplicity: { lower: string; upper: string } | undefined;
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
          };
    this.multiplicity =
      info.multiplicity === undefined
        ? undefined
        : { lower: info.multiplicity.lower, upper: info.multiplicity.upper };
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
    const children: ModelSymbol[] = [];
    for (const id of this.childIds) {
      const response = await callRpc(
        this.connection.rpc.getSymbol(
          { modelHash: this.modelHash, symbolId: id },
          this.connection.callOptions(),
        ),
      );
      if (response.symbol !== undefined) {
        children.push(new ModelSymbol(this.connection, this.modelHash, response.symbol));
      }
    }
    return children;
  }

  /** The value of one attribute, or undefined when the symbol declares no such attribute. */
  attribute(name: string): AttributeFacts | undefined {
    return this.attributes.find((attribute) => attribute.name === name);
  }
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

  constructor(instance: PbInstance) {
    this.id = instance.id;
    this.typeId = instance.typeSymbolId;
    const features = new Map<string, FeatureValue>();
    for (const [name, value] of Object.entries(instance.featureValues)) {
      features.set(name, decodeFeatureValue(value));
    }
    this.features = features;
  }

  /** The value of one feature, or undefined when the object has no such feature. */
  get(name: string): FeatureValue | undefined {
    return this.features.get(name);
  }
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
