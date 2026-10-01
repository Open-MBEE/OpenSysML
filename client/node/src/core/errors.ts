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

/** A query payload is not one the SysML v2 API & Services query model describes. */
export class QueryError extends OpenSysMLError {}

/** A document query binding cannot be written before anything is sent. */
export class DocumentQueryError extends OpenSysMLError {}

/** The service's classification of a failure it reported in a successful answer. */
export type FailureCause =
  | "unspecified"
  | "evaluation"
  | "wrong_kind"
  | "ambiguous_subject"
  | "undecided";
