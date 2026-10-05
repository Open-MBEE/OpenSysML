// What a connected service can do. A client asks by capability name, never by
// version: the service does not answer UNIMPLEMENTED for a capability it lacks,
// so the advertised list is the only reliable answer.

import { OpenSysMLError } from "./errors.js";
import { PLATFORM_PACKAGE_PREFIX } from "./package.js";

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
/** The `Convert` RPC. */
export const CAPABILITY_CONVERT = "convert";
/** The `Migrate` RPC, which migrates a SysML v1 model. */
export const CAPABILITY_MIGRATE = "migrate";
/** The verification RPCs. */
export const CAPABILITY_VERIFICATION = "verification";
/** The `question` field of the verification RPCs. */
export const CAPABILITY_VERIFICATION_QUESTIONS = "verification_questions";
/** The `Query` RPC. */
export const CAPABILITY_QUERY = "query";
/** The `RunDocumentQuery` RPC. */
export const CAPABILITY_DOCUMENT_QUERY = "document_query";
/** The `RenderDocument` RPC. */
export const CAPABILITY_RENDER_DOCUMENT = "render_document";
/** `form` on `RenderDocumentRequest` asking for HTML rather than Markdown. */
export const CAPABILITY_RENDER_DOCUMENT_HTML = "render_document_html";
/** The `RenderView` RPC, returning typed diagram data. */
export const CAPABILITY_RENDER_VIEW = "render_view";
/** What the body of a verification case answered, as `verification_verdicts`. */
export const CAPABILITY_VERIFICATION_VERDICTS = "verification_verdicts";
/** `RunAnalysisResponse.evaluations`: each call the run made to a calc held as a value, such as a trade study's evaluation of every alternative. */
export const CAPABILITY_CASE_EVALUATIONS = "case_evaluations";
/** The `ApplyEdits` RPC. */
export const CAPABILITY_APPLY_EDITS = "apply_edits";
/** Source-preserving add-member and delete authoring operations. */
export const CAPABILITY_AUTHORING = "authoring";
/** The `ApplyEdits` `add_connection` operation. */
export const CAPABILITY_CONNECTION_AUTHORING = "connection_authoring";
/** The `ApplyEdits` `add_satisfy` operation. */
export const CAPABILITY_SATISFY_AUTHORING = "satisfy_authoring";
/** The `ApplyEdits` `add_requirement_constraint` operation. */
export const CAPABILITY_REQUIREMENT_CONSTRAINT_AUTHORING = "requirement_constraint_authoring";
/** The `ApplyEdits` `add_transition` operation. */
export const CAPABILITY_TRANSITION_AUTHORING = "transition_authoring";
/** The `ApplyEdits` `add_verify` operation and anonymous objectives. */
export const CAPABILITY_VERIFICATION_OBJECTIVE_AUTHORING = "verification_objective_authoring";
/** The `ApplyEdits` `add_metadata` operation. */
export const CAPABILITY_METADATA_AUTHORING = "metadata_authoring";
/** `ApplyEdits` metadata prefixes. */
export const CAPABILITY_METADATA_PREFIX_AUTHORING = "metadata_prefix_authoring";
/** `ApplyEdits` can add `first`/`then` action sequencing members. */
export const CAPABILITY_SEQUENCE_AUTHORING = "sequence_authoring";
/** `ApplyEdits` can add action-body statements and succession source multiplicities. */
export const CAPABILITY_ACTION_BODY_STATEMENT_AUTHORING = "action_body_statement_authoring";
/** The `ApplyEdits` `add_import` operation. */
export const CAPABILITY_IMPORT_AUTHORING = "import_authoring";
/** The `ApplyEdits` `add_documentation` operation and `AddMemberEdit.doc`. */
export const CAPABILITY_DOCUMENTATION_AUTHORING = "documentation_authoring";
/** The `ApplyEdits` `add_comment` and `add_note` operations. */
export const CAPABILITY_COMMENT_AUTHORING = "comment_authoring";
/** The additional `AddMemberEdit` modifiers and `ref`/`return` kinds. */
export const CAPABILITY_MEMBER_MODIFIERS = "member_modifiers";
/** `ApplyEdits` adds a directed usage with no kind keyword (`in x : T;`). */
export const CAPABILITY_IMPLICIT_PARAMETERS = "implicit_parameters";
/** Constraint body expressions and asserted constraints in `ApplyEdits`. */
export const CAPABILITY_CONSTRAINT_BODY_AUTHORING = "constraint_body_authoring";
/** State behavior member kinds in `ApplyEdits`. */
export const CAPABILITY_STATE_ACTION_AUTHORING = "state_action_authoring";
/** `ApplyEdits` edits a model of several documents as one batch and answers each edited document by name in `documents`. Not used by this version; see the README. */
export const CAPABILITY_EDIT_DOCUMENTS = "edit_documents";
/** `ParseSources`, parsing several named documents as one model. */
export const CAPABILITY_PARSE_SOURCES = "parse_sources";
/** The `schedule` field of the execution requests, naming the scheduling policy. */
export const CAPABILITY_SCHEDULE = "schedule";
/** The `explore` scheduling policy, answering with every `outcomes` entry and an `exploration` status. */
export const CAPABILITY_SCHEDULE_EXPLORE = "schedule_explore";
/** `performerSymbolId` on the action and state requests: the object the behavior runs on, a declaration or a path from one into its parts. */
export const CAPABILITY_PERFORMER = "performer";
/** `trace` on `ExecuteStateRequest`, returning documented events from one run. */
export const CAPABILITY_STATE_TRACE = "state_trace";
/** The `ListEngines` RPC, the `engine` field selecting an analysis engine, and `engine`, `strength` and `bounds` on the answers. */
export const CAPABILITY_ENGINES = "engines";
/** A model-level result the model leaves open as `Value.undetermined`, read as an `undetermined` value. */
export const CAPABILITY_UNDETERMINED_VALUE = "undetermined_value";
/** An Integer beyond int64 as `Value.bigIntValue`, `Quantity.bigIntMagnitude` and `DocumentValue.bigIntValue`; a service without it reads one sent to it as null. */
export const CAPABILITY_BIG_INT_VALUES = "big_int_values";

/**
 * Orders capability names by code unit, the order the service reports them in.
 * Locale-aware collation ignores the underscores that separate their words.
 */
export function byCodeUnit(a: string, b: string): number {
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
    `newer ${PLATFORM_PACKAGE_PREFIX}<platform> package, point $OPENSYSML_BINARY at a build that ` +
    `has it, or start one yourself and connect to its address`
  );
}

/**
 * Why `info` is not the service that was asked for, or undefined when it is. A
 * release is compared as an exact tag: a build that cannot be shown to be the
 * one asked for is a mismatch.
 */
export function mismatchReason(
  info: ServerInfo,
  options: { version?: string; capabilities?: Iterable<string> } = {},
): string | undefined {
  const reasons: string[] = [];
  if (options.version !== undefined) {
    if (!info.answered) {
      reasons.push(
        `it did not answer GetServerInfo, so it cannot be shown to be the ${options.version} ` +
          `that was asked for`,
      );
    } else if (info.version !== options.version) {
      reasons.push(
        `it reports version ${info.version === "" ? "unknown" : info.version}, but ` +
          `${options.version} was asked for`,
      );
    }
  }
  const missing = [...(options.capabilities ?? [])].filter((capability) => !info.has(capability));
  missing.sort(byCodeUnit);
  if (missing.length > 0) {
    const named = missing.map((capability) => JSON.stringify(capability)).join(", ");
    const noun = missing.length > 1 ? "capabilities" : "capability";
    reasons.push(`it does not report the ${named} ${noun} this client requires`);
  }
  return reasons.length === 0 ? undefined : reasons.join("; ");
}

/**
 * The UNIMPLEMENTED translator for a call whose capabilities were checked
 * before it was sent: a refusal the service still answers with is mapped back
 * to the capability it names — the first one present in its message — rather
 * than arriving as a generic unsupported-operation error.
 */
export function capabilityRefusal(
  info: ServerInfo,
  capabilities: readonly string[],
): ((details: string) => MissingCapabilityError) | undefined {
  if (capabilities.length === 0) {
    return undefined;
  }
  return (details) => {
    const capability = capabilities.find((name) => details.includes(name)) ?? capabilities[0];
    return new MissingCapabilityError(capability, info, upgradeRemedy(capability));
  };
}
