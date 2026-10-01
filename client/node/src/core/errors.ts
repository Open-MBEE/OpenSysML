// Errors this client raises. Everything derives from OpenSysMLError, so a caller
// can catch the family without knowing the members.

import type { ServerInfo } from "./capabilities.js";
import type { AnalysisResult } from "./verdict.js";

/** Base class of every error this client raises. */
export class OpenSysMLError extends Error {
  constructor(message: string, options?: { cause?: unknown }) {
    super(message, options);
    this.name = new.target.name;
  }
}

/**
 * The service could not be reached, started, or answered nothing usable. Also
 * what a failed call becomes: the subclasses name the statuses a caller acts on
 * differently, and the ConnectError behind one is always its `cause`.
 */
export class ServiceError extends OpenSysMLError {
  /** Status the call failed with, when the failure came from a call. */
  readonly code: string | undefined;

  constructor(message: string, options: { cause?: unknown; code?: string } = {}) {
    super(message, options.cause === undefined ? {} : { cause: options.cause });
    this.code = options.code;
  }
}

/** A private child service failed to start, or died while it was needed. */
export class ServiceStartError extends ServiceError {}

/** The service already listening is not the one asked for, reported rather than stopped. */
export class StaleServiceError extends ServiceError {
  /** Address the mismatched service is listening on. */
  readonly address: string;
  /** How it differs from the service that was asked for. */
  readonly reason: string;
  /** What to do about it. */
  readonly remedy: string;
  /** What it reported about itself, when it could be asked. */
  readonly info: ServerInfo | undefined;

  constructor(
    address: string,
    reason: string,
    remedy: string,
    options: { info?: ServerInfo; cause?: unknown } = {},
  ) {
    super(
      `the sysml-grpc service already listening on ${address} is not the one this client ` +
        `asked for: ${reason}.\n  service: ${options.info?.describe() ?? address}\n  fix:     ${remedy}`,
      options.cause === undefined ? {} : { cause: options.cause },
    );
    this.address = address;
    this.reason = reason;
    this.remedy = remedy;
    this.info = options.info;
  }
}

/** The connection was closed and cannot be used again. */
export class ClosedConnectionError extends OpenSysMLError {
  constructor() {
    super("this connection is closed; open another with connect()");
  }
}

/** The service no longer holds the model a call named; load it again. */
export class ModelNotFoundError extends ServiceError {}

/** The service could not read the source file a call named. */
export class ModelFileNotFoundError extends ServiceError {}

/** The service rejected the request as malformed or unsupported. */
export class InvalidRequestError extends ServiceError {}

/** A call exceeded its deadline, or was cancelled. */
export class ServiceTimeoutError extends ServiceError {}

/** The connected service does not implement the call at all. */
export class UnsupportedOperationError extends ServiceError {}

/** A value contradicts itself on the wire, such as an array whose elements do not fill its dimensions. */
export class MalformedValueError extends OpenSysMLError {}

/** A release binary could not be downloaded, or could not be installed once it was. */
export class DownloadError extends ServiceError {}

/** A download's digest contradicts the one expected of it, so it is never used. */
export class ChecksumMismatchError extends DownloadError {}

/** Nothing pins or signs a digest for the release, leaving only its origin's word. */
export class UnpinnedReleaseError extends ChecksumMismatchError {}

/**
 * No signature on the checksum manifest could be checked at all: none published,
 * unreadable, or no verifier installed. Refused exactly as an unpinned release is.
 */
export class UnsignedReleaseError extends UnpinnedReleaseError {}

/** A signature was checked and does not verify: another signer, or a changed manifest. */
export class ManifestSignatureError extends ChecksumMismatchError {}

/** A model file could not be read, or its content did not parse. */
export class ParseError extends OpenSysMLError {
  /** Diagnostics the service reported, in the order it reported them. */
  readonly diagnostics: readonly ModelDiagnostic[];
  /** The model the errors belong to, when one was loaded, so it stays inspectable. */
  readonly model: ModelLike | undefined;

  constructor(
    message: string,
    diagnostics: readonly ModelDiagnostic[] = [],
    options: { model?: ModelLike } = {},
  ) {
    super(message);
    this.diagnostics = diagnostics;
    this.model = options.model;
  }
}

/** The surface of a model a ParseError carries, kept structural so model.ts stays a client of this file. */
export interface ModelLike {
  readonly hash: string;
  readonly diagnostics: readonly ModelDiagnostic[];
}

/** One diagnostic about a model, at a source position when the service gave one. */
export interface ModelDiagnostic {
  severity: string;
  message: string;
  /** What was found, stable across message wording (`"syntax"`, a validation code,
   * `"choice-point"`, `"guard-unevaluable"`); `""` when the service assigned none. */
  code: string;
  file?: string;
  startLine?: number;
  startColumn?: number;
  endLine?: number;
  endColumn?: number;
}

/** No symbol of that name is declared in the model. */
export class SymbolNotFoundError extends OpenSysMLError {
  /** Name that was looked up. `name` is the error's class, as on every Error. */
  readonly symbolName: string;
  /** Names the model declares that are close enough to be typos of it. */
  readonly suggestions: readonly string[];

  constructor(name: string, near: readonly string[] = []) {
    const hint = near.length > 0 ? `; did you mean ${near.join(", ")}?` : "";
    super(`the model declares no symbol named ${JSON.stringify(name)}${hint}`);
    this.symbolName = name;
    this.suggestions = [...near];
  }
}

/** An expression could not be evaluated, or a symbol could not be instantiated. */
export class EvaluationError extends OpenSysMLError {
  /** Why the service could not answer, when it classified the failure. */
  readonly reason: FailureCause;
  readonly diagnostics: readonly ModelDiagnostic[];

  constructor(
    message: string,
    reason: FailureCause = "unspecified",
    diagnostics: readonly ModelDiagnostic[] = [],
  ) {
    super(message);
    this.reason = reason;
    this.diagnostics = diagnostics;
  }
}

/** A run — an execution, verification, calculation, analysis or sweep — failed. */
export class ExecutionError extends EvaluationError {}

/** The element a verification named is of another kind, a wrong request rather than a verdict. */
export class WrongKindError extends ExecutionError {}

/** An analysis case could not run to its end but left something to inspect, carried as `result`. */
export class AnalysisRunError extends ExecutionError {
  /** The partial result the failed run left: the evaluations made, the verdicts left undecided. */
  readonly result: AnalysisResult;

  constructor(
    message: string,
    result: AnalysisResult,
    options: { diagnostics?: readonly ModelDiagnostic[]; reason?: FailureCause } = {},
  ) {
    super(message, options.reason ?? "unspecified", options.diagnostics ?? []);
    this.result = result;
  }
}

/** A model could not be written in the format asked for. */
export class ConversionError extends OpenSysMLError {
  readonly diagnostics: readonly ModelDiagnostic[];

  constructor(message: string, diagnostics: readonly ModelDiagnostic[] = []) {
    super(message);
    this.diagnostics = diagnostics;
  }
}

/** The service sent a value the wire format cannot represent, or a caller sent one it cannot carry. */
export class UnsupportedValueError extends OpenSysMLError {}

/** A feature value could not be evaluated or was never materialized. */
export class FeatureValueError extends OpenSysMLError {
  /** Name of the feature. */
  readonly featureName: string;
  /** Error description the service reported. */
  readonly message: string;

  constructor(featureName: string, message: string) {
    super(`feature value ${JSON.stringify(featureName)}: ${message}`);
    this.featureName = featureName;
    this.message = message;
  }
}

/** A feature holds a value of another type than its generated view declares. */
export class TypeMismatchError extends OpenSysMLError {
  /** Name of the feature. */
  readonly featureName: string;
  /** Type the generated class declares. */
  readonly expected: string;
  /** The value actually decoded. */
  readonly value: unknown;

  constructor(featureName: string, expected: string, value: unknown) {
    super(`feature value ${JSON.stringify(featureName)}: expected ${expected}, got ${describeValue(value)}`);
    this.featureName = featureName;
    this.expected = expected;
    this.value = value;
  }
}

/** A generated typed view was asked to wrap an instance of another type. */
export class InstanceTypeError extends OpenSysMLError {
  /** FQN of the definition the generated class views. */
  readonly expected: string;
  /** FQN the instance reports as its type. */
  readonly actual: string;

  constructor(expected: string, actual: string) {
    super(
      `instance of ${JSON.stringify(actual)} is not a ${JSON.stringify(expected)}; ` +
        "call the generated class's unchecked(instance) to view it without this check",
    );
    this.expected = expected;
    this.actual = actual;
  }
}

function describeValue(value: unknown): string {
  if (value === null) {
    return "null";
  }
  if (typeof value === "object") {
    try {
      return JSON.stringify(value);
    } catch {
      return "[unprintable]";
    }
  }
  switch (typeof value) {
    case "string":
      return JSON.stringify(value);
    case "number":
      return String(value);
    case "bigint":
      return `${value.toString()}n`;
    case "boolean":
    case "undefined":
      return String(value);
    default:
      return "[unprintable]";
  }
}

/** A query payload is not one the SysML v2 API & Services query model describes. */
export class QueryError extends OpenSysMLError {}

/** A document query binding cannot be written before anything is sent. */
export class DocumentQueryError extends OpenSysMLError {}

/** One declaration referring to the target of a refused rename, delete or move. */
export class Referrer {
  /** The referring declaration, as the notation names it. */
  readonly name: string;
  /** The document declaring it, as the parse named it. */
  readonly document: string;

  constructor(name: string, document: string) {
    this.name = name;
    this.document = document;
  }
}

/** An edit to a model was refused, and nothing was changed. */
export class EditError extends OpenSysMLError {
  /** Refusal kind, as the wire enum names it, e.g. `EDIT_FAILURE_UNKNOWN_TARGET`. */
  readonly failure: string;
  /** Diagnostics behind the refusal. */
  readonly diagnostics: readonly ModelDiagnostic[];
  /** Where references a refused rename, delete or move would have broken are made. */
  readonly referringElements: readonly string[];
  /** The same referrers, each with the document declaring it. */
  readonly referrers: readonly Referrer[];

  constructor(
    message: string,
    options: {
      failure?: string;
      diagnostics?: readonly ModelDiagnostic[];
      referringElements?: readonly string[];
      referrers?: readonly Referrer[];
    } = {},
  ) {
    super(message);
    this.failure = options.failure ?? "";
    this.diagnostics = options.diagnostics ?? [];
    this.referringElements = [...(options.referringElements ?? [])];
    this.referrers = [...(options.referrers ?? [])];
  }
}

/** An editor with no operations was applied. */
export class NoEditsError extends EditError {}

/** The element an edit names cannot carry that edit. */
export class EditTargetError extends EditError {}

/** The new value or name itself cannot be read. */
export class InvalidEditError extends EditError {}

/** A rename would break references to the renamed element. */
export class RenameReferencedError extends EditError {}

/** Two operations would edit the same bytes of the source. */
export class OverlappingEditsError extends EditError {}

/** The edited notation could not be read back. */
export class EditResultError extends EditError {}

/** An add-member owner is not declared in the model. */
export class OwnerNotFoundError extends EditError {}

/** An add-member owner cannot contain members. */
export class OwnerNotNamespaceError extends EditError {}

/** A declaration kind is invalid for the source language. */
export class IllegalMemberKindError extends InvalidEditError {}

/** An owner already declares the requested member name. */
export class MemberNameTakenError extends EditError {}

/** A referenced declaration is deleted without cascade. */
export class DeleteReferencedError extends EditError {}

/** A move's new owner is the moved declaration or inside it. */
export class OwnerInsideTargetError extends EditError {}

/** A move would leave a reference no spelling can restore. */
export class MoveReferencedError extends EditError {}

/** A rename, delete or move is referred to from a document the edit cannot rewrite. */
export class ReferencedElsewhereError extends EditError {}

/** The service's classification of a failure it reported in a successful answer. */
export type FailureCause =
  | "unspecified"
  | "evaluation"
  | "wrong_kind"
  | "ambiguous_subject"
  | "undecided";
