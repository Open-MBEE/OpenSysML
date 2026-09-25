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
import { EvaluationError, OpenSysMLError, ParseError, SymbolNotFoundError } from "./errors.js";
import type { ModelDiagnostic } from "./errors.js";
import type {
  Diagnostic,
  FeatureValue as PbFeatureValue,
  Instance as PbInstance,
  ParseFileRequest,
  SymbolInfo,
} from "../generated/sysml_pb.js";
import { callRpc } from "./status.js";
import { decodeValue, type SysMLValue } from "./values.js";

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
  strict?: boolean;
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

  private readonly rootSymbol: ModelSymbol | undefined;
  private readonly ownsConnection: boolean;

  private constructor(init: {
    connection: Connection;
    hash: string;
    root?: ModelSymbol;
    diagnostics: readonly ModelDiagnostic[];
    ownsConnection: boolean;
  }) {
    this.connection = init.connection;
    this.hash = init.hash;
    this.rootSymbol = init.root;
    this.diagnostics = init.diagnostics;
    this.ownsConnection = init.ownsConnection;
  }

  /** Adopts a model the service already holds, by the hash it holds it under. */
  static adopt(connection: Connection, hash: string): Model {
    return new Model({ connection, hash, diagnostics: [], ownsConnection: false });
  }

  /** Whether this model was parsed here, and so knows its root symbol. */
  get parsed(): boolean {
    return this.rootSymbol !== undefined;
  }

  /** The model's root namespace. An adopted model has none: look symbols up by id. */
  get root(): ModelSymbol {
    if (this.rootSymbol === undefined) {
      throw new OpenSysMLError(
        `model ${this.hash} was adopted by hash, not parsed here, so its root is unknown; ` +
          "look a symbol up by its qualified name with symbolById()",
      );
    }
    return this.rootSymbol;
  }

  /** Parses a source over `connection`. Used by Connection.load / Connection.loads. */
  static async parse(
    connection: Connection,
    source: Pick<ParseFileRequest, "source">,
    options: ParseOptions = {},
    ownsConnection = false,
  ): Promise<Model> {
    if (options.language !== undefined) {
      requireCapability(connection.info, CAPABILITY_INLINE_LANGUAGE, upgradeRemedy(CAPABILITY_INLINE_LANGUAGE));
    }
    if (options.strict === true) {
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
          ...(options.strict === undefined ? {} : { strictConformance: options.strict }),
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
      root: new ModelSymbol(connection, response.modelHash, response.root),
      diagnostics,
      ownsConnection,
    });
  }

  /** Whether the parse reported an error-severity diagnostic. */
  get hasErrors(): boolean {
    return this.diagnostics.some((diagnostic) => diagnostic.severity === "error");
  }

  /** Evaluates a SysML expression against this model. */
  async eval(expression: string, options: EvalOptions = {}): Promise<SysMLValue> {
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
    if (this.looksQualified(name)) {
      return this.symbolById(name);
    }
    // A short name is searched for from the root, which an adopted model has not
    // got; the service resolves a name the model declares at its top level.
    if (this.rootSymbol === undefined) {
      return this.symbolById(name);
    }
    const found = await this.find(name);
    if (found === undefined) {
      throw new SymbolNotFoundError(name, await this.nearNames(name));
    }
    return found;
  }

  /** Looks a symbol up by its qualified name, in one call. */
  async symbolById(id: string): Promise<ModelSymbol> {
    const response = await callRpc(
      this.connection.rpc.getSymbol(
        { modelHash: this.hash, symbolId: id },
        this.connection.callOptions(),
      ),
    );
    if (response.symbol === undefined) {
      throw new SymbolNotFoundError(id);
    }
    return new ModelSymbol(this.connection, this.hash, response.symbol);
  }

  /** Looks a symbol up by short name, FQN or id, breadth-first from the root. */
  async find(name: string): Promise<ModelSymbol | undefined> {
    for await (const symbol of this.walk()) {
      if (symbol.name === name || symbol.id === name) {
        return symbol;
      }
    }
    return undefined;
  }

  /** Every symbol of the model, breadth-first from the root. */
  async *walk(): AsyncGenerator<ModelSymbol> {
    const queue: ModelSymbol[] = [this.root];
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
    return name.includes("::") || name === this.rootSymbol?.id;
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
      const score = similarity(name.toLowerCase(), symbol.name.toLowerCase());
      if (score >= NEAR_ENOUGH) {
        scored.push({ id: symbol.id, score });
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

function decodeDiagnostic(diagnostic: Diagnostic): ModelDiagnostic {
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
