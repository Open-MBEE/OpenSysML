// What a connected service can do. A client asks by capability name, never by
// version: the service does not answer UNIMPLEMENTED for a capability it lacks,
// so the advertised list is the only reliable answer.

import { OpenSysMLError } from "./errors.js";

/** Static type facts on a symbol: `typeInfo`, `multiplicity`, `specializations`. */
export const CAPABILITY_TYPE_FACTS = "type_facts";
/** Populated `SymbolInfo.attributes`. */
export const CAPABILITY_SYMBOL_ATTRIBUTES = "symbol_attributes";
/** Evaluating an expression against an instantiated subject. */
export const CAPABILITY_EVALUATE_SUBJECT = "evaluate_subject";
/** An object's values as `Instance.feature_values`. */
export const CAPABILITY_FEATURE_VALUES = "feature_values";
/** An enumeration literal as `Value.enum_literal`. */
export const CAPABILITY_ENUM_VALUES = "enum_values";
/** A valueless feature of a value type as `Value.unset`. */
export const CAPABILITY_UNSET_VALUE = "unset_value";
/** A complex number as `Value.complex`, rather than an unsupported null. */
export const CAPABILITY_COMPLEX_VALUES = "complex_values";
/** An array, a vector and a vector quantity as their own arms, rather than unsupported nulls. */
export const CAPABILITY_STRUCTURED_VALUES = "structured_values";
/** A bare measurement unit (`SI::m`, `m / s`) as `Value.measurement_ref`, rather than an unsupported null. */
export const CAPABILITY_MEASUREMENT_REFS = "measurement_refs";
/** A calc held as a value as `Value.function`, named by its declaration, rather than an unsupported null. */
export const CAPABILITY_FUNCTION_VALUES = "function_values";
/** The unbounded value `*` as `Value.infinity`, rather than an unsupported null. */
export const CAPABILITY_INFINITY_VALUE = "infinity_value";
/** `Diagnostic.code` is populated, so an empty code is a finding none was assigned. */
export const CAPABILITY_DIAGNOSTIC_CODES = "diagnostic_codes";
/** A unique, unordered collection (a `Collections::Set`'s elements) as `Value.set`, rather than an unsupported null. */
export const CAPABILITY_SET_VALUES = "set_values";
/** A tensor quantity of any rank as `Value.tensor_quantity`, rather than an unsupported null. */
export const CAPABILITY_TENSOR_VALUES = "tensor_values";
/** An element reflected on (`x meta T`, the last of `x.metadata`) as `Value.metaobject`, rather than an unsupported null. */
export const CAPABILITY_METAOBJECT_VALUES = "metaobject_values";
/** `finalTime` on an execution response: the run's simulation clock when it ended, in seconds. */
export const CAPABILITY_FINAL_TIME = "final_time";
/** `ParseFileRequest.language`, which declares the language of inline content. */
export const CAPABILITY_INLINE_LANGUAGE = "inline_language";
/** `ParseFileRequest.strict_conformance`. */
export const CAPABILITY_STRICT_CONFORMANCE = "strict_conformance";
/** The `Convert` RPC. Not used by this version; see the README. */
export const CAPABILITY_CONVERT = "convert";
/** The verification RPCs. Not used by this version; see the README. */
export const CAPABILITY_VERIFICATION = "verification";
/** What the body of a verification case answered, as `verification_verdicts`. */
export const CAPABILITY_VERIFICATION_VERDICTS = "verification_verdicts";
/** `RunAnalysisResponse.evaluations`: each call the run made to a calc held as a value, such as a trade study's evaluation of every alternative. */
export const CAPABILITY_CASE_EVALUATIONS = "case_evaluations";
/** The `Query` RPC. Not used by this version; see the README. */
export const CAPABILITY_QUERY = "query";
/** The `ApplyEdits` RPC. Not used by this version; see the README. */
export const CAPABILITY_APPLY_EDITS = "apply_edits";
/** The `schedule` field of the execution requests, naming the scheduling policy. Not used by this version; see the README. */
export const CAPABILITY_SCHEDULE = "schedule";
/** The `explore` scheduling policy, answering with every `outcomes` entry and an `exploration` status. Not used by this version; see the README. */
export const CAPABILITY_SCHEDULE_EXPLORE = "schedule_explore";
/** The `ListEngines` RPC, the `engine` field selecting an analysis engine, and `engine`, `strength` and `bounds` on the answers. Not used by this version; see the README. */
export const CAPABILITY_ENGINES = "engines";

/**
 * Orders capability names by code unit, the order the service reports them in.
 * Locale-aware collation ignores the underscores that separate their words.
 */
function byCodeUnit(a: string, b: string): number {
  if (a < b) {
    return -1;
  }
  return a > b ? 1 : 0;
}

/** Self-description of the service a connection talks to. */
export class ServerInfo {
  /** Version the service reports. Informational only; empty when unanswered. */
  readonly version: string;
  readonly capabilities: ReadonlySet<string>;
  /** Whether the service answered the handshake; false means it predates `GetServerInfo`. */
  readonly answered: boolean;
  /** Where the service came from: the binary this client started, or the address it dialled. */
  readonly origin: string;

  constructor(init: {
    version: string;
    capabilities: Iterable<string>;
    answered: boolean;
    origin: string;
  }) {
    this.version = init.version;
    this.capabilities = new Set(init.capabilities);
    this.answered = init.answered;
    this.origin = init.origin;
  }

  has(capability: string): boolean {
    return this.capabilities.has(capability);
  }

  /** One-line description of the service, for an error message. */
  describe(): string {
    if (!this.answered) {
      return `${this.origin} (version unknown: too old to answer GetServerInfo, so it predates every capability)`;
    }
    const reported =
      [...this.capabilities].sort(byCodeUnit).join(", ") || "none";
    const version = this.version === "" ? "unknown" : this.version;
    return `${this.origin} (version ${version}, capabilities: ${reported})`;
  }
}

/** The connected service does not report a capability the operation requires. */
export class MissingCapabilityError extends OpenSysMLError {
  readonly capability: string;
  readonly info: ServerInfo;

  constructor(capability: string, info: ServerInfo, remedy: string) {
    super(
      `the sysml-grpc service does not support the ${JSON.stringify(capability)} capability, ` +
        `which this operation requires.\n  service: ${info.describe()}\n  fix:     ${remedy}`,
    );
    this.capability = capability;
    this.info = info;
  }
}

/** Throws unless the service reports `capability`. */
export function requireCapability(
  info: ServerInfo,
  capability: string,
  remedy: string,
): void {
  if (!info.has(capability)) {
    throw new MissingCapabilityError(capability, info, remedy);
  }
}

/** Remedy text for a service that lacks `capability`, naming both routes to one that has it. */
export function upgradeRemedy(capability: string): string {
  return (
    `run a sysml-grpc whose GetServerInfo reports ${JSON.stringify(capability)}: install a ` +
    `newer @opensysml/sysml-grpc-<platform> package, point $OPENSYSML_BINARY at a build that ` +
    `has it, or start one yourself and connect to its address`
  );
}
