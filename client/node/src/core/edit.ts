// Changing a loaded model and writing it back, with its layout intact.
//
// An edit is described, not typed out: an Editor collects operations naming
// elements the way a read result names them, and `apply()` has the service
// perform them on the source it parsed. The service edits the bytes the
// operations reach and nothing else, so comments, blank lines and indentation
// outside an edited span come back unchanged, and it re-parses what it edited
// before returning it.

import { create } from "@bufbuild/protobuf";
import {
  AddCommentEditSchema,
  AddConnectionEditSchema,
  AddDocumentationEditSchema,
  AddImportEditSchema,
  AddMemberEditSchema,
  AddMetadataEditSchema,
  AddMetadataPrefixEditSchema,
  AddNoteEditSchema,
  AddRequirementConstraintEditSchema,
  AddSatisfyEditSchema,
  AddSequenceEditSchema,
  AddTransitionEditSchema,
  AddVerifyEditSchema,
  DeleteEditSchema,
  EditFailure,
  EditOperationSchema,
  MetadataFeatureValueSchema,
  MoveEditSchema,
  RenameEditSchema,
  SetValueEditSchema,
  type AddSequenceEdit,
  type ApplyEditsResponse,
  type EditOperation,
} from "../generated/sysml_pb.js";
import {
  CAPABILITY_ACTION_BODY_STATEMENT_AUTHORING,
  CAPABILITY_APPLY_EDITS,
  CAPABILITY_AUTHORING,
  CAPABILITY_COMMENT_AUTHORING,
  CAPABILITY_CONNECTION_AUTHORING,
  CAPABILITY_CONSTRAINT_BODY_AUTHORING,
  CAPABILITY_DOCUMENTATION_AUTHORING,
  CAPABILITY_IMPORT_AUTHORING,
  CAPABILITY_IMPLICIT_PARAMETERS,
  CAPABILITY_MEMBER_MODIFIERS,
  CAPABILITY_METADATA_AUTHORING,
  CAPABILITY_METADATA_PREFIX_AUTHORING,
  CAPABILITY_REQUIREMENT_CONSTRAINT_AUTHORING,
  CAPABILITY_SATISFY_AUTHORING,
  CAPABILITY_SEQUENCE_AUTHORING,
  CAPABILITY_STATE_ACTION_AUTHORING,
  CAPABILITY_TRANSITION_AUTHORING,
  CAPABILITY_VERIFICATION_OBJECTIVE_AUTHORING,
  requireCapability,
  upgradeRemedy,
  type ServerInfo,
} from "./capabilities.js";
import { Conversion, FORMAT_SYSML } from "./conversion.js";
import {
  DeleteReferencedError,
  EditError,
  EditResultError,
  EditTargetError,
  IllegalMemberKindError,
  InvalidEditError,
  MemberNameTakenError,
  MoveReferencedError,
  NoEditsError,
  OverlappingEditsError,
  OwnerInsideTargetError,
  OwnerNotFoundError,
  OwnerNotNamespaceError,
  ReferencedElsewhereError,
  RenameReferencedError,
  Referrer,
  type ModelDiagnostic,
} from "./errors.js";
import type { ModelSymbol } from "./model.js";
import { decodeDiagnostic } from "./model.js";

/** One element an operation targets: a symbol id (FQN) or a ModelSymbol. */
export type EditTarget = string | ModelSymbol;

/** A low-level edit operation tuple, as {@link Connection.applyEdits} accepts. */
export type EditOperationData = readonly unknown[];

/** One byte range an operation replaced in the source it saw. */
export class AppliedEdit {
  /** Position of the operation in the editor. */
  readonly operationIndex: number;
  /** Element edited, by the id it was named with. */
  readonly target: string;
  /** Byte offset where the replacement starts. */
  readonly offset: number;
  /** Number of bytes replaced; zero when text was only inserted. */
  readonly length: number;
  /** The bytes that were there. */
  readonly oldText: string;
  /** What replaced them. */
  readonly newText: string;
  /** The document the bytes belong to, named as the parse named it. */
  readonly document: string;

  constructor(init: {
    operationIndex: number;
    target: string;
    offset: number;
    length: number;
    oldText: string;
    newText: string;
    document: string;
  }) {
    this.operationIndex = init.operationIndex;
    this.target = init.target;
    this.offset = init.offset;
    this.length = init.length;
    this.oldText = init.oldText;
    this.newText = init.newText;
    this.document = init.document;
  }

  toString(): string {
    return `${this.target}: ${JSON.stringify(this.oldText)} -> ${JSON.stringify(this.newText)}`;
  }
}

/** The edited notation of one document of the model. */
export class EditedDocument {
  /** The document's name as the parse named it. */
  readonly name: string;
  /** The edited notation, byte-identical to the source outside the edited spans. */
  readonly content: string;

  constructor(init: { name: string; content: string }) {
    this.name = init.name;
    this.content = init.content;
  }

  toString(): string {
    return this.content;
  }
}

/** The edited notation, as a {@link Conversion}. */
export class EditResult extends Conversion {
  /** What each operation changed, grouped by document, in source order within one. */
  readonly applied: readonly AppliedEdit[];
  /** The edited notation of every document the edits rewrote, edited document first. */
  readonly documents: readonly EditedDocument[];

  constructor(init: {
    content: string;
    fromFormat: string;
    toFormat: string;
    diagnostics: readonly ModelDiagnostic[];
    experimental: boolean;
    experimentalNotice: string;
    applied: readonly AppliedEdit[];
    documents: readonly EditedDocument[];
  }) {
    super(init);
    this.applied = init.applied;
    this.documents = init.documents;
  }
}

/** Refusal kinds, as the wire enum names them, and the error each raises. */
const FAILURE_ERRORS = new Map<string, new (message: string, options?: { failure?: string; diagnostics?: readonly ModelDiagnostic[]; referringElements?: readonly string[]; referrers?: readonly Referrer[] }) => EditError>([
  ["EDIT_FAILURE_NO_OPERATIONS", NoEditsError],
  ["EDIT_FAILURE_UNKNOWN_TARGET", EditTargetError],
  ["EDIT_FAILURE_AMBIGUOUS_TARGET", EditTargetError],
  ["EDIT_FAILURE_NOT_VALUED", EditTargetError],
  ["EDIT_FAILURE_NOT_NAMED", EditTargetError],
  ["EDIT_FAILURE_INVALID_VALUE", InvalidEditError],
  ["EDIT_FAILURE_INVALID_NAME", InvalidEditError],
  ["EDIT_FAILURE_RENAME_REFERENCED", RenameReferencedError],
  ["EDIT_FAILURE_OVERLAPPING_EDITS", OverlappingEditsError],
  ["EDIT_FAILURE_RESULT_INVALID", EditResultError],
  ["EDIT_FAILURE_OWNER_UNKNOWN", OwnerNotFoundError],
  ["EDIT_FAILURE_OWNER_NOT_NAMESPACE", OwnerNotNamespaceError],
  ["EDIT_FAILURE_ILLEGAL_KIND", IllegalMemberKindError],
  ["EDIT_FAILURE_MEMBER_NAME_TAKEN", MemberNameTakenError],
  ["EDIT_FAILURE_DELETE_REFERENCED", DeleteReferencedError],
  ["EDIT_FAILURE_OWNER_INSIDE_TARGET", OwnerInsideTargetError],
  ["EDIT_FAILURE_MOVE_REFERENCED", MoveReferencedError],
  ["EDIT_FAILURE_REFERENCED_ELSEWHERE", ReferencedElsewhereError],
]);

const FAILURE_NAMES = new Map<EditFailure, string>([
  [EditFailure.UNSPECIFIED, "EDIT_FAILURE_UNSPECIFIED"],
  [EditFailure.NO_OPERATIONS, "EDIT_FAILURE_NO_OPERATIONS"],
  [EditFailure.UNKNOWN_TARGET, "EDIT_FAILURE_UNKNOWN_TARGET"],
  [EditFailure.AMBIGUOUS_TARGET, "EDIT_FAILURE_AMBIGUOUS_TARGET"],
  [EditFailure.NOT_VALUED, "EDIT_FAILURE_NOT_VALUED"],
  [EditFailure.INVALID_VALUE, "EDIT_FAILURE_INVALID_VALUE"],
  [EditFailure.INVALID_NAME, "EDIT_FAILURE_INVALID_NAME"],
  [EditFailure.NOT_NAMED, "EDIT_FAILURE_NOT_NAMED"],
  [EditFailure.RENAME_REFERENCED, "EDIT_FAILURE_RENAME_REFERENCED"],
  [EditFailure.OVERLAPPING_EDITS, "EDIT_FAILURE_OVERLAPPING_EDITS"],
  [EditFailure.RESULT_INVALID, "EDIT_FAILURE_RESULT_INVALID"],
  [EditFailure.OWNER_UNKNOWN, "EDIT_FAILURE_OWNER_UNKNOWN"],
  [EditFailure.OWNER_NOT_NAMESPACE, "EDIT_FAILURE_OWNER_NOT_NAMESPACE"],
  [EditFailure.ILLEGAL_KIND, "EDIT_FAILURE_ILLEGAL_KIND"],
  [EditFailure.MEMBER_NAME_TAKEN, "EDIT_FAILURE_MEMBER_NAME_TAKEN"],
  [EditFailure.DELETE_REFERENCED, "EDIT_FAILURE_DELETE_REFERENCED"],
  [EditFailure.OWNER_INSIDE_TARGET, "EDIT_FAILURE_OWNER_INSIDE_TARGET"],
  [EditFailure.MOVE_REFERENCED, "EDIT_FAILURE_MOVE_REFERENCED"],
  [EditFailure.REFERENCED_ELSEWHERE, "EDIT_FAILURE_REFERENCED_ELSEWHERE"],
]);

/** Name a refusal kind, including one this client's enum has no name for. */
export function failureName(failure: number): string {
  return FAILURE_NAMES.get(failure) ?? `EDIT_FAILURE_${failure}`;
}

/** Build the error a refusal kind names. */
export function errorForFailure(
  failure: string,
  message: string,
  options: {
    diagnostics?: readonly ModelDiagnostic[];
    referringElements?: readonly string[];
    referrers?: readonly Referrer[];
  } = {},
): EditError {
  const ctor = FAILURE_ERRORS.get(failure) ?? EditError;
  return new ctor(message, { failure, ...options });
}

/** The referrers of an `ApplyEditsResponse`, each with the document declaring it. */
export function referrersOf(response: ApplyEditsResponse): Referrer[] {
  return response.referrers.map((referrer) => new Referrer(referrer.name, referrer.document));
}

/** Read an `ApplyEditsResponse` as an {@link EditResult}. */
export function editResultOf(response: ApplyEditsResponse, appliedSource = FORMAT_SYSML): EditResult {
  return new EditResult({
    content: response.content,
    fromFormat: appliedSource,
    toFormat: appliedSource,
    diagnostics: [],
    experimental: false,
    experimentalNotice: "",
    applied: response.applied.map(
      (edit) =>
        new AppliedEdit({
          operationIndex: edit.operationIndex,
          target: edit.target,
          offset: edit.offset,
          length: edit.length,
          oldText: edit.oldText,
          newText: edit.newText,
          document: edit.document,
        }),
    ),
    documents: response.documents.map(
      (document) => new EditedDocument({ name: document.name, content: document.content }),
    ),
  });
}

/** The refusal an `ApplyEditsResponse` carries, as a typed error, or undefined. */
export function editErrorOf(response: ApplyEditsResponse): EditError | undefined {
  if (response.error === "") {
    return undefined;
  }
  return errorForFailure(failureName(response.failure), response.error, {
    diagnostics: response.diagnostics.map(decodeDiagnostic),
    referringElements: response.referringElements,
    referrers: referrersOf(response),
  });
}

type TextField = string | null | undefined;

function nameOf(value: unknown): string {
  if (value === null) {
    return "null";
  }
  return typeof value;
}

function sequenceText(label: string, value: TextField, optional = false): void {
  if (value === undefined && optional) {
    return;
  }
  if (typeof value !== "string") {
    throw new TypeError(`${label} must be notation text, not ${nameOf(value)}`);
  }
}

function sequenceTuple(
  owner: string,
  keyword: string,
  ref = "",
  memberKind = "",
  memberName = "",
  typeName = "",
  after = "",
  fields?: Record<string, unknown>,
): unknown[] {
  const operation: unknown[] = [
    "add_sequence", owner, keyword, ref, memberKind, memberName, typeName, after,
  ];
  if (fields !== undefined && Object.keys(fields).length > 0) {
    operation.push(fields);
  }
  return operation;
}

function sequenceOptions(after?: string, multiplicity?: string): [string, string | undefined] {
  sequenceText("multiplicity", multiplicity, true);
  sequenceText("after", after, true);
  return [after ?? "", multiplicity];
}

function sequenceKeywordOptions(keyword: string, options: [string, string | undefined]): void {
  if (options[1] !== undefined && keyword !== "then") {
    throw new RangeError("multiplicity requires then=True");
  }
}

function sequenceStatement(
  owner: string,
  keyword: string,
  memberKind = "",
  fields: Record<string, unknown> = {},
  options?: {
    ref?: string | undefined;
    memberName?: string | undefined;
    typeName?: string | undefined;
    options?: [string, string | undefined];
    after?: string;
    multiplicity?: string;
  },
): unknown[] {
  const rest = { ...fields };
  const opts =
    options?.options ??
    sequenceOptions(
      (options?.after ?? rest["after"]) as string | undefined,
      (options?.multiplicity ?? rest["multiplicity"]) as string | undefined,
    );
  delete rest["after"];
  delete rest["multiplicity"];
  const [after, multiplicity] = opts;
  if (multiplicity !== undefined) {
    rest["multiplicity"] = multiplicity;
  }
  return sequenceTuple(
    owner,
    keyword,
    options?.ref ?? "",
    memberKind,
    options?.memberName ?? "",
    options?.typeName ?? "",
    after,
    rest,
  );
}

/** Chainable action-body items for nested `if` and loop statements. */
export class Body {
  #operations: unknown[][] = [];

  /** The body items collected so far. */
  get operations(): readonly unknown[][] {
    return [...this.#operations];
  }

  #keyword(then: boolean | undefined): string {
    if (then === undefined) {
      return this.#operations.length === 0 ? "" : "then";
    }
    if (typeof then !== "boolean") {
      throw new TypeError(`then must be bool or None, not ${nameOf(then)}`);
    }
    return then ? "then" : "";
  }

  #addStatement(
    kind: string,
    options: { then?: boolean | undefined; typeName?: string | undefined } & Record<string, unknown>,
  ): this {
    const fields = { ...options };
    delete fields.then;
    delete fields.typeName;
    const opts = sequenceOptions(
      fields["after"] as string | undefined,
      fields["multiplicity"] as string | undefined,
    );
    delete fields.after;
    delete fields.multiplicity;
    const keyword = this.#keyword(options.then);
    sequenceKeywordOptions(keyword, opts);
    this.#operations.push(
      sequenceStatement("", keyword, kind, fields, {
        typeName: options.typeName ?? "",
        options: opts,
      }),
    );
    return this;
  }

  /** Emit `first <ref>;`. */
  addFirst(ref: string): this {
    sequenceText("ref", ref);
    this.#operations.push(sequenceStatement("", "first", "", {}, { ref }));
    return this;
  }

  /** Emit `then [m] <ref>;` or `then [m] <kind> <action> : <type>;`. */
  addThen(
    ref?: string,
    action?: string,
    options: { type?: string; kind?: string; multiplicity?: string } = {},
  ): this {
    const { type, kind = "action", multiplicity } = options;
    for (const [label, text] of [
      ["ref", ref],
      ["action", action],
      ["type", type],
      ["kind", kind],
      ["multiplicity", multiplicity],
    ] as const) {
      sequenceText(label, text, true);
    }
    const opts: [string, string | undefined] = ["", multiplicity];
    if ((ref === undefined) === (action === undefined)) {
      throw new RangeError("exactly one of ref and action is required");
    }
    if (ref !== undefined) {
      if (type !== undefined || kind !== "action") {
        throw new RangeError("a then reference takes no type or kind");
      }
      this.#operations.push(sequenceStatement("", "then", "", {}, { ref, options: opts }));
    } else {
      this.#operations.push(
        sequenceStatement("", "then", kind, {}, {
          memberName: action,
          typeName: type ?? "",
          options: opts,
        }),
      );
    }
    return this;
  }

  /** Emit `<kind> <name> : <type>;`. */
  addAction(name?: string, options: { type?: string; kind?: string } = {}): this {
    const { type, kind = "action" } = options;
    for (const [label, text] of [
      ["name", name],
      ["type", type],
      ["kind", kind],
    ] as const) {
      sequenceText(label, text, true);
    }
    this.#operations.push(
      sequenceStatement("", "", kind, {}, {
        memberName: name ?? "",
        typeName: type ?? "",
      }),
    );
    return this;
  }

  /** Emit `accept <payload> [: <type>] [via <via>];` or `then [m] accept ...;`. */
  addAccept(
    payload: string,
    options: { type?: string; via?: string; then?: boolean; multiplicity?: string } = {},
  ): this {
    const { type, via, then, multiplicity } = options;
    sequenceText("payload", payload);
    sequenceText("type", type, true);
    sequenceText("via", via, true);
    return this.#addStatement("accept", {
      then,
      typeName: type ?? "",
      parameter: payload,
      via: via ?? "",
      multiplicity,
    });
  }

  /** Emit `send <payload> [via <via>] [to <to>];` or `then [m] send ...;`. */
  addSend(
    payload: string,
    options: {
      to?: string;
      via?: string;
      then?: boolean;
      multiplicity?: string;
    } = {},
  ): this {
    const { to, via, then, multiplicity } = options;
    sequenceText("payload", payload);
    sequenceText("to", to, true);
    sequenceText("via", via, true);
    return this.#addStatement("send", {
      then,
      value: payload,
      target: to ?? "",
      via: via ?? "",
      multiplicity,
    });
  }

  /** Emit `assign <target> := <value>;` or `then [m] assign ...;`. */
  addAssign(
    target: string,
    value: string,
    options: { then?: boolean; multiplicity?: string } = {},
  ): this {
    const { then, multiplicity } = options;
    sequenceText("target", target);
    sequenceText("value", value);
    return this.#addStatement("assign", { then, target, value, multiplicity });
  }

  /** Emit `if <condition> { <body> } [else { <elseBody> }]` or `then [m] if ...`. */
  addIf(
    condition: string,
    body: Body,
    elseBody?: Body,
    options: { then?: boolean; multiplicity?: string } = {},
  ): this {
    const { then, multiplicity } = options;
    sequenceText("condition", condition);
    if (!(body instanceof Body)) {
      throw new TypeError(`body must be Body, not ${nameOf(body)}`);
    }
    if (elseBody !== undefined && !(elseBody instanceof Body)) {
      throw new TypeError(`else_body must be Body or None, not ${nameOf(elseBody)}`);
    }
    return this.#addStatement("if", {
      then,
      condition,
      body: body.operations,
      else_body: elseBody !== undefined && elseBody.operations.length > 0 ? elseBody.operations : [],
      multiplicity,
    });
  }

  /** Emit `while <condition> { <body> } [until <until>];` or `then [m] while ...`. */
  addWhile(
    condition: string,
    body: Body,
    options: { until?: string; then?: boolean; multiplicity?: string } = {},
  ): this {
    const { until, then, multiplicity } = options;
    sequenceText("condition", condition);
    sequenceText("until", until, true);
    if (!(body instanceof Body)) {
      throw new TypeError(`body must be Body, not ${nameOf(body)}`);
    }
    return this.#addStatement("while", {
      then,
      condition,
      until: until ?? "",
      body: body.operations,
      multiplicity,
    });
  }

  /** Emit `loop { <body> } [until <until>];` or `then [m] loop ...`. */
  addLoop(
    body: Body,
    options: { until?: string; then?: boolean; multiplicity?: string } = {},
  ): this {
    const { until, then, multiplicity } = options;
    sequenceText("until", until, true);
    if (!(body instanceof Body)) {
      throw new TypeError(`body must be Body, not ${nameOf(body)}`);
    }
    return this.#addStatement("loop", {
      then,
      until: until ?? "",
      body: body.operations,
      multiplicity,
    });
  }

  /** Emit `for <variable> [: <type>] in <collection> { <body> }` or `then [m] for ...`. */
  addFor(
    variable: string,
    collection: string,
    body: Body,
    options: { type?: string; then?: boolean; multiplicity?: string } = {},
  ): this {
    const { type, then, multiplicity } = options;
    sequenceText("variable", variable);
    sequenceText("collection", collection);
    sequenceText("type", type, true);
    if (!(body instanceof Body)) {
      throw new TypeError(`body must be Body, not ${nameOf(body)}`);
    }
    return this.#addStatement("for", {
      then,
      parameter: variable,
      value: collection,
      typeName: type ?? "",
      body: body.operations,
      multiplicity,
    });
  }

  /** Emit `terminate [<occurrence>];` or `then [m] terminate ...;`. */
  addTerminate(
    occurrence?: string,
    options: { then?: boolean; multiplicity?: string } = {},
  ): this {
    const { then, multiplicity } = options;
    sequenceText("occurrence", occurrence, true);
    return this.#addStatement("terminate", {
      then,
      value: occurrence ?? "",
      multiplicity,
    });
  }

  /** Emit `if <guard> then <ref>;`. */
  addGuardedThen(guard: string, ref: string): this {
    sequenceText("guard", guard);
    sequenceText("ref", ref);
    this.#operations.push(
      sequenceStatement("", "if", "", { condition: guard }, { ref }),
    );
    return this;
  }

  /** Emit `else <ref>;`. */
  addElse(ref: string): this {
    sequenceText("ref", ref);
    this.#operations.push(sequenceStatement("", "else", "", {}, { ref }));
    return this;
  }
}

interface MemberOptions {
  type?: string | undefined;
  multiplicity?: string | undefined;
  value?: string | undefined;
  specializes?: string | readonly string[] | undefined;
  abstract?: boolean | undefined;
  redefines?: string | readonly string[] | undefined;
  default?: boolean | undefined;
  direction?: string | undefined;
  metadata?: string | readonly string[] | undefined;
  expression?: string | undefined;
  doc?: string | undefined;
}

function notationReferences(label: string, values: string | readonly string[] | undefined): string[] {
  if (values === undefined) {
    return [];
  }
  const references = typeof values === "string" ? [values] : [...values];
  if (!Array.isArray(references)) {
    throw new TypeError(`${label} must be a notation string or sequence of strings`);
  }
  if (!references.every((reference) => typeof reference === "string")) {
    throw new TypeError(`${label} must contain only notation strings`);
  }
  return references;
}

function parameterPairs(parameters: readonly (readonly [string, string])[] | undefined, argument: string): readonly [string, string][] {
  if (parameters === undefined) {
    return [];
  }
  if (!Array.isArray(parameters)) {
    throw new TypeError(
      `${argument} must be a list of 2-tuples of strings, not ${nameOf(parameters)}`,
    );
  }
  return parameters.map((pair, index) => {
    if (!Array.isArray(pair) || pair.length !== 2) {
      throw new TypeError(`${argument}[${index}] must be a 2-tuple of strings`);
    }
    if (!pair.every((value) => typeof value === "string")) {
      throw new TypeError(`${argument}[${index}] name and type must be strings`);
    }
    return [pair[0], pair[1]] as [string, string];
  });
}

function optionalText(value: TextField, argument: string): void {
  if (value !== undefined && typeof value !== "string") {
    throw new TypeError(`${argument} must be notation text or None, not ${nameOf(value)}`);
  }
}

function targetId(target: unknown): string {
  if (typeof target === "string") {
    return target;
  }
  const ident = (target as { id?: unknown } | null)?.id;
  if (typeof ident === "string" && ident !== "") {
    return ident;
  }
  throw new TypeError(`target must be a symbol id (FQN) or a Symbol, not ${nameOf(target)}`);
}

function ownerId(owner: unknown): string {
  return typeof owner === "string" ? owner : targetId(owner);
}

/** Operations to perform on a loaded model, and the call that performs them. */
export class Editor {
  readonly #modelHash: string;
  readonly #apply: (modelHash: string, operations: readonly EditOperationData[]) => Promise<EditResult>;
  #operations: unknown[][] = [];
  #applied = false;

  constructor(
    modelHash: string,
    apply: (modelHash: string, operations: readonly EditOperationData[]) => Promise<EditResult>,
  ) {
    this.#modelHash = modelHash;
    this.#apply = apply;
  }

  /** The operations collected so far, in the order they were added. */
  get operations(): readonly EditOperationData[] {
    return [...this.#operations];
  }

  /** Whether this editor has been applied. */
  get applied(): boolean {
    return this.#applied;
  }

  /** How many operations are collected. */
  get length(): number {
    return this.#operations.length;
  }

  /** Set the value expression of one of the model's features. */
  setValue(target: EditTarget, value: string): this {
    if (typeof value !== "string") {
      throw new TypeError(
        `value must be SysML notation for an expression, not ${nameOf(value)}: ` +
          "write it as it should read in the file",
      );
    }
    this.#add(["set_value", targetId(target), value]);
    return this;
  }

  /** Rename one of the model's declarations. */
  rename(target: EditTarget, newName: string): this {
    if (typeof newName !== "string") {
      throw new TypeError(`new_name must be a name, not ${nameOf(newName)}`);
    }
    this.#add(["rename", targetId(target), newName]);
    return this;
  }

  /** Add one declaration, using strings for all SysML/KerML notation. */
  addMember(owner: EditTarget, kind: string, name: string, options: MemberOptions = {}): this {
    const {
      type,
      multiplicity,
      value,
      specializes,
      abstract = false,
      redefines,
      default: isDefault = false,
      direction,
      metadata,
      expression,
      doc,
    } = options;
    if (typeof kind !== "string") {
      throw new TypeError(`kind must be notation text, not ${nameOf(kind)}`);
    }
    for (const [label, text] of [
      ["kind", kind],
      ["type", type],
      ["multiplicity", multiplicity],
      ["value", value],
      ["expression", expression],
    ] as const) {
      if (text !== undefined && typeof text !== "string") {
        throw new TypeError(`${label} must be notation text, not ${nameOf(text)}`);
      }
    }
    if (typeof name !== "string") {
      throw new TypeError(`name must be notation text, not ${nameOf(name)}`);
    }
    const ownerName = ownerId(owner);
    const specializeList = notationReferences("specializes", specializes);
    const redefineList = notationReferences("redefines", redefines);
    const hasMetadata = metadata !== undefined;
    const metadataList = notationReferences("metadata", metadata);
    if (typeof abstract !== "boolean") {
      throw new TypeError("abstract must be bool");
    }
    if (typeof isDefault !== "boolean") {
      throw new TypeError("default must be bool");
    }
    if (direction !== undefined && typeof direction !== "string") {
      throw new TypeError(`direction must be notation text, not ${nameOf(direction)}`);
    }
    if (doc !== undefined && typeof doc !== "string") {
      throw new TypeError(`doc must be text, not ${nameOf(doc)}`);
    }
    const base: unknown[] = [
      "add_member", ownerName, kind, name, type ?? "", multiplicity ?? "",
      value ?? "", [...specializeList],
    ];
    if (
      abstract ||
      redefineList.length > 0 ||
      isDefault ||
      direction !== undefined ||
      hasMetadata ||
      kind === "ref" ||
      kind === "return" ||
      kind === "" ||
      expression !== undefined ||
      doc !== undefined
    ) {
      base.push(abstract, [...redefineList], isDefault, direction ?? "");
    }
    if (hasMetadata) {
      base.push([...metadataList]);
    }
    if (expression !== undefined || doc !== undefined) {
      base.push(expression ?? "");
    }
    if (doc !== undefined) {
      base.push(doc);
    }
    this.#add(base);
    return this;
  }

  /** Add an `objective` member, unnamed when `name` is omitted. */
  addObjective(owner: EditTarget, options: { name?: string; type?: string } = {}): this {
    const { name, type } = options;
    if (name !== undefined && typeof name !== "string") {
      throw new TypeError(`name must be notation text, not ${nameOf(name)}`);
    }
    if (type !== undefined && typeof type !== "string") {
      throw new TypeError(`type must be notation text, not ${nameOf(type)}`);
    }
    return this.addMember(owner, "objective", name ?? "", { type });
  }

  /** Add `verify <requirement>;` to a verification case or its objective. */
  addVerify(owner: EditTarget, requirement: string): this {
    if (typeof requirement !== "string") {
      throw new TypeError(`requirement must be notation text, not ${nameOf(requirement)}`);
    }
    this.#add(["add_verify", ownerId(owner), requirement]);
    return this;
  }

  /** Add a `metadata` usage or `@` shorthand with optional values. */
  addMetadata(
    owner: EditTarget,
    metadataType: string,
    options: {
      values?: Readonly<Record<string, string>> | readonly (readonly [string, string])[];
      name?: string;
      about?: string | readonly string[];
      shorthand?: boolean;
    } = {},
  ): this {
    const { values, name, about, shorthand = false } = options;
    if (typeof metadataType !== "string") {
      throw new TypeError(`metadata_type must be notation text, not ${nameOf(metadataType)}`);
    }
    if (name !== undefined && typeof name !== "string") {
      throw new TypeError(`name must be notation text, not ${nameOf(name)}`);
    }
    if (typeof shorthand !== "boolean") {
      throw new TypeError("shorthand must be bool");
    }
    let bindings: readonly unknown[];
    if (values === undefined) {
      bindings = [];
    } else if (Array.isArray(values)) {
      bindings = values;
    } else if (typeof values === "object") {
      bindings = Object.entries(values);
    } else {
      throw new TypeError("values must be a mapping or a sequence of (feature, value) pairs");
    }
    const normalized: [string, string][] = bindings.map((pair: unknown, index: number) => {
      if (!Array.isArray(pair) || pair.length !== 2) {
        throw new TypeError(`values[${index}] must be a pair of strings`);
      }
      if (!pair.every((text) => typeof text === "string")) {
        throw new TypeError(`values[${index}] feature and value must be strings`);
      }
      return [pair[0], pair[1]] as [string, string];
    });
    const aboutList = notationReferences("about", about);
    this.#add([
      "add_metadata", ownerId(owner), metadataType, name ?? "", aboutList, normalized, shorthand,
    ]);
    return this;
  }

  /** Add a metadata prefix to an existing declaration. */
  addMetadataPrefix(target: EditTarget, metadataType: string): this {
    if (typeof metadataType !== "string") {
      throw new TypeError(`metadata_type must be notation text, not ${nameOf(metadataType)}`);
    }
    this.#add(["add_metadata_prefix", ownerId(target), metadataType]);
    return this;
  }

  /** Add `doc /* body *\/` as the first body member of a declaration. */
  addDocumentation(
    target: EditTarget,
    body: string,
    options: { name?: string; locale?: string; replace?: boolean } = {},
  ): this {
    const { name, locale, replace = false } = options;
    if (typeof body !== "string") {
      throw new TypeError(`body must be text, not ${nameOf(body)}`);
    }
    for (const [label, text] of [
      ["name", name],
      ["locale", locale],
    ] as const) {
      if (text !== undefined && typeof text !== "string") {
        throw new TypeError(`${label} must be text, not ${nameOf(text)}`);
      }
    }
    if (typeof replace !== "boolean") {
      throw new TypeError(`replace must be a bool, not ${nameOf(replace)}`);
    }
    this.#add([
      "add_documentation", targetId(target), body, name ?? "", locale ?? "", replace,
    ]);
    return this;
  }

  /** Add `comment [name] [about a, b] [locale "..."] /* body *\/` to a body. */
  addComment(
    owner: EditTarget,
    body: string,
    options: { name?: string; about?: readonly EditTarget[]; locale?: string } = {},
  ): this {
    const { name, about, locale } = options;
    if (typeof body !== "string") {
      throw new TypeError(`body must be text, not ${nameOf(body)}`);
    }
    for (const [label, text] of [
      ["name", name],
      ["locale", locale],
    ] as const) {
      if (text !== undefined && typeof text !== "string") {
        throw new TypeError(`${label} must be text, not ${nameOf(text)}`);
      }
    }
    if (typeof about === "string") {
      throw new TypeError("about must be a sequence of names or symbols, not one name");
    }
    const aboutList = (about ?? []).map((element) => targetId(element));
    this.#add(["add_comment", ownerId(owner), body, name ?? "", aboutList, locale ?? ""]);
    return this;
  }

  /** Write the line note `// text` on its own line above a declaration. */
  addNote(target: EditTarget, text: string): this {
    if (typeof text !== "string") {
      throw new TypeError(`text must be text, not ${nameOf(text)}`);
    }
    if (text.includes("\n") || text.includes("\r")) {
      throw new RangeError("a note is one line: its text may not contain a line break");
    }
    this.#add(["add_note", targetId(target), text]);
    return this;
  }

  /** Add a `satisfy` usage to a body that admits behavior usages. */
  addSatisfy(
    owner: EditTarget,
    requirement: string,
    options: { by?: string; asserted?: boolean; negated?: boolean } = {},
  ): this {
    const { by, asserted = false, negated = false } = options;
    if (typeof requirement !== "string") {
      throw new TypeError(`requirement must be notation text, not ${nameOf(requirement)}`);
    }
    if (by !== undefined && typeof by !== "string") {
      throw new TypeError(`by must be notation text, not ${nameOf(by)}`);
    }
    if (typeof asserted !== "boolean" || typeof negated !== "boolean") {
      throw new TypeError("asserted and negated must be bool");
    }
    this.#add(["add_satisfy", ownerId(owner), requirement, by ?? "", asserted, negated]);
    return this;
  }

  /** Add a `require` or `assume` constraint to a requirement-like body. */
  addRequirementConstraint(
    owner: EditTarget,
    kind: string,
    expression: string,
    name?: string,
  ): this {
    for (const [label, text] of [
      ["kind", kind],
      ["expression", expression],
    ] as const) {
      if (typeof text !== "string") {
        throw new TypeError(`${label} must be notation text, not ${nameOf(text)}`);
      }
    }
    if (name !== undefined && typeof name !== "string") {
      throw new TypeError(`name must be notation text, not ${nameOf(name)}`);
    }
    this.#add(["add_requirement_constraint", ownerId(owner), kind, expression, name ?? ""]);
    return this;
  }

  /** Add a transition with optional trigger, guard and effect clauses. */
  addTransition(
    owner: EditTarget,
    source: string,
    target: string,
    options: { name?: string; trigger?: string; guard?: string; effect?: string } = {},
  ): this {
    const { name, trigger, guard, effect } = options;
    if (typeof source !== "string") {
      throw new TypeError(`source must be notation text, not ${nameOf(source)}`);
    }
    if (typeof target !== "string") {
      throw new TypeError(`target must be notation text, not ${nameOf(target)}`);
    }
    for (const [label, text] of [
      ["name", name],
      ["trigger", trigger],
      ["guard", guard],
      ["effect", effect],
    ] as const) {
      if (text !== undefined && typeof text !== "string") {
        throw new TypeError(`${label} must be notation text, not ${nameOf(text)}`);
      }
    }
    this.#add([
      "add_transition", ownerId(owner), name ?? "", source, target,
      trigger ?? "", guard ?? "", effect ?? "", false,
    ]);
    return this;
  }

  /** Add an entry transition to target in a state body. */
  addEntryTransition(owner: EditTarget, target: string): this {
    if (typeof target !== "string") {
      throw new TypeError(`target must be notation text, not ${nameOf(target)}`);
    }
    this.#add(["add_transition", ownerId(owner), "", "", target, "", "", "", true]);
    return this;
  }

  /** Emit `first <ref>;`. */
  addFirst(owner: EditTarget, ref: string, options: { after?: string } = {}): this {
    const { after } = options;
    if (typeof ref !== "string") {
      throw new TypeError(`ref must be notation text, not ${nameOf(ref)}`);
    }
    if (after !== undefined && typeof after !== "string") {
      throw new TypeError(`after must be a member name, not ${nameOf(after)}`);
    }
    const opts: [string, string | undefined] = [after ?? "", undefined];
    this.#add(sequenceStatement(ownerId(owner), "first", "", {}, { ref, options: opts }));
    return this;
  }

  /** Emit `then [m] <ref>;` or `then [m] <kind> <name> : <type>;`. */
  addThen(
    owner: EditTarget,
    options: {
      ref?: string;
      action?: string;
      type?: string;
      after?: string;
      kind?: string;
      multiplicity?: string;
    } = {},
  ): this {
    const { ref, action, type, after, kind = "action", multiplicity } = options;
    for (const [label, text] of [
      ["ref", ref],
      ["action", action],
      ["type", type],
      ["after", after],
      ["kind", kind],
      ["multiplicity", multiplicity],
    ] as const) {
      if (text !== undefined && typeof text !== "string") {
        throw new TypeError(`${label} must be notation text, not ${nameOf(text)}`);
      }
    }
    const opts: [string, string | undefined] = [after ?? "", multiplicity];
    if ((ref === undefined) === (action === undefined)) {
      throw new RangeError("exactly one of ref and action is required");
    }
    if (ref !== undefined && (type !== undefined || kind !== "action")) {
      throw new RangeError("a then reference takes no type or kind");
    }
    if (ref !== undefined) {
      this.#add(sequenceStatement(ownerId(owner), "then", "", {}, { ref, options: opts }));
    } else {
      this.#add(
        sequenceStatement(ownerId(owner), "then", kind, {}, {
          memberName: action,
          typeName: type ?? "",
          options: opts,
        }),
      );
    }
    return this;
  }

  /** Emit `then [m] accept <payload> [: <type>] [via <via>];`. */
  addAccept(
    owner: EditTarget,
    payload: string,
    options: {
      type?: string;
      via?: string;
      then?: boolean;
      multiplicity?: string;
      after?: string;
    } = {},
  ): this {
    const { type, via, then = true, multiplicity, after } = options;
    sequenceText("payload", payload);
    sequenceText("type", type, true);
    sequenceText("via", via, true);
    const opts = sequenceOptions(after, multiplicity);
    if (typeof then !== "boolean") {
      throw new TypeError(`then must be bool, not ${nameOf(then)}`);
    }
    sequenceKeywordOptions(then ? "then" : "", opts);
    this.#add(
      sequenceStatement(ownerId(owner), then ? "then" : "", "accept", {
        parameter: payload,
        via: via ?? "",
      }, {
        typeName: type ?? "",
        options: opts,
      }),
    );
    return this;
  }

  /** Emit `then [m] send <payload> [via <via>] [to <to>];`. */
  addSend(
    owner: EditTarget,
    payload: string,
    options: {
      to?: string;
      via?: string;
      then?: boolean;
      multiplicity?: string;
      after?: string;
    } = {},
  ): this {
    const { to, via, then = true, multiplicity, after } = options;
    sequenceText("payload", payload);
    sequenceText("to", to, true);
    sequenceText("via", via, true);
    const opts = sequenceOptions(after, multiplicity);
    if (typeof then !== "boolean") {
      throw new TypeError(`then must be bool, not ${nameOf(then)}`);
    }
    sequenceKeywordOptions(then ? "then" : "", opts);
    this.#add(
      sequenceStatement(ownerId(owner), then ? "then" : "", "send", {
        value: payload,
        target: to ?? "",
        via: via ?? "",
      }, {
        options: opts,
      }),
    );
    return this;
  }

  /** Emit `then [m] assign <target> := <value>;`. */
  addAssign(
    owner: EditTarget,
    target: string,
    value: string,
    options: { then?: boolean; multiplicity?: string; after?: string } = {},
  ): this {
    const { then = true, multiplicity, after } = options;
    sequenceText("target", target);
    sequenceText("value", value);
    const opts = sequenceOptions(after, multiplicity);
    if (typeof then !== "boolean") {
      throw new TypeError(`then must be bool, not ${nameOf(then)}`);
    }
    sequenceKeywordOptions(then ? "then" : "", opts);
    this.#add(
      sequenceStatement(ownerId(owner), then ? "then" : "", "assign", {
        target,
        value,
      }, {
        options: opts,
      }),
    );
    return this;
  }

  /** Emit `then [m] if <condition> { <body> } [else { <elseBody> }]`. */
  addIf(
    owner: EditTarget,
    condition: string,
    body: Body,
    elseBody?: Body,
    options: { then?: boolean; multiplicity?: string; after?: string } = {},
  ): this {
    const { then = true, multiplicity, after } = options;
    sequenceText("condition", condition);
    if (!(body instanceof Body)) {
      throw new TypeError(`body must be Body, not ${nameOf(body)}`);
    }
    if (elseBody !== undefined && !(elseBody instanceof Body)) {
      throw new TypeError(`else_body must be Body or None, not ${nameOf(elseBody)}`);
    }
    const opts = sequenceOptions(after, multiplicity);
    if (typeof then !== "boolean") {
      throw new TypeError(`then must be bool, not ${nameOf(then)}`);
    }
    sequenceKeywordOptions(then ? "then" : "", opts);
    this.#add(
      sequenceStatement(ownerId(owner), then ? "then" : "", "if", {
        condition,
        body: body.operations,
        else_body: elseBody !== undefined && elseBody.operations.length > 0 ? elseBody.operations : [],
      }, {
        options: opts,
      }),
    );
    return this;
  }

  /** Emit `then [m] while <condition> { <body> } [until <until>];`. */
  addWhile(
    owner: EditTarget,
    condition: string,
    body: Body,
    options: { until?: string; then?: boolean; multiplicity?: string; after?: string } = {},
  ): this {
    const { until, then = true, multiplicity, after } = options;
    sequenceText("condition", condition);
    sequenceText("until", until, true);
    if (!(body instanceof Body)) {
      throw new TypeError(`body must be Body, not ${nameOf(body)}`);
    }
    const opts = sequenceOptions(after, multiplicity);
    if (typeof then !== "boolean") {
      throw new TypeError(`then must be bool, not ${nameOf(then)}`);
    }
    sequenceKeywordOptions(then ? "then" : "", opts);
    this.#add(
      sequenceStatement(ownerId(owner), then ? "then" : "", "while", {
        condition,
        until: until ?? "",
        body: body.operations,
      }, {
        options: opts,
      }),
    );
    return this;
  }

  /** Emit `then [m] loop { <body> } [until <until>];`. */
  addLoop(
    owner: EditTarget,
    body: Body,
    options: { until?: string; then?: boolean; multiplicity?: string; after?: string } = {},
  ): this {
    const { until, then = true, multiplicity, after } = options;
    sequenceText("until", until, true);
    if (!(body instanceof Body)) {
      throw new TypeError(`body must be Body, not ${nameOf(body)}`);
    }
    const opts = sequenceOptions(after, multiplicity);
    if (typeof then !== "boolean") {
      throw new TypeError(`then must be bool, not ${nameOf(then)}`);
    }
    sequenceKeywordOptions(then ? "then" : "", opts);
    this.#add(
      sequenceStatement(ownerId(owner), then ? "then" : "", "loop", {
        until: until ?? "",
        body: body.operations,
      }, {
        options: opts,
      }),
    );
    return this;
  }

  /** Emit `then [m] for <variable> [: <type>] in <collection> { <body> }`. */
  addFor(
    owner: EditTarget,
    variable: string,
    collection: string,
    body: Body,
    options: { type?: string; then?: boolean; multiplicity?: string; after?: string } = {},
  ): this {
    const { type, then = true, multiplicity, after } = options;
    sequenceText("variable", variable);
    sequenceText("collection", collection);
    sequenceText("type", type, true);
    if (!(body instanceof Body)) {
      throw new TypeError(`body must be Body, not ${nameOf(body)}`);
    }
    const opts = sequenceOptions(after, multiplicity);
    if (typeof then !== "boolean") {
      throw new TypeError(`then must be bool, not ${nameOf(then)}`);
    }
    sequenceKeywordOptions(then ? "then" : "", opts);
    this.#add(
      sequenceStatement(ownerId(owner), then ? "then" : "", "for", {
        parameter: variable,
        value: collection,
        body: body.operations,
      }, {
        typeName: type ?? "",
        options: opts,
      }),
    );
    return this;
  }

  /** Emit `then [m] terminate [<occurrence>];`. */
  addTerminate(
    owner: EditTarget,
    occurrence?: string,
    options: { then?: boolean; multiplicity?: string; after?: string } = {},
  ): this {
    const { then = true, multiplicity, after } = options;
    sequenceText("occurrence", occurrence, true);
    const opts = sequenceOptions(after, multiplicity);
    if (typeof then !== "boolean") {
      throw new TypeError(`then must be bool, not ${nameOf(then)}`);
    }
    sequenceKeywordOptions(then ? "then" : "", opts);
    this.#add(
      sequenceStatement(ownerId(owner), then ? "then" : "", "terminate", {
        value: occurrence ?? "",
      }, {
        options: opts,
      }),
    );
    return this;
  }

  /** Emit `if <guard> then <ref>;`. */
  addGuardedThen(owner: EditTarget, guard: string, ref: string, options: { after?: string } = {}): this {
    const { after } = options;
    sequenceText("guard", guard);
    sequenceText("ref", ref);
    const opts = sequenceOptions(after);
    this.#add(
      sequenceStatement(ownerId(owner), "if", "", { condition: guard }, { ref, options: opts }),
    );
    return this;
  }

  /** Emit `else <ref>;`. */
  addElse(owner: EditTarget, ref: string, options: { after?: string } = {}): this {
    const { after } = options;
    sequenceText("ref", ref);
    const opts = sequenceOptions(after);
    this.#add(sequenceStatement(ownerId(owner), "else", "", {}, { ref, options: opts }));
    return this;
  }

  /** Add an import declaration to a namespace body or the document root. */
  addImport(
    owner: EditTarget,
    target: string,
    options: {
      visibility?: string;
      recursive?: boolean;
      all?: boolean;
      filter?: string | readonly string[];
    } = {},
  ): this {
    const { visibility, recursive = false, all = false, filter } = options;
    if (typeof target !== "string") {
      throw new TypeError(`target must be notation text, not ${nameOf(target)}`);
    }
    if (visibility !== undefined && typeof visibility !== "string") {
      throw new TypeError(`visibility must be notation text or None, not ${nameOf(visibility)}`);
    }
    if (typeof recursive !== "boolean") {
      throw new TypeError(`recursive must be a bool, not ${nameOf(recursive)}`);
    }
    if (typeof all !== "boolean") {
      throw new TypeError(`all must be a bool, not ${nameOf(all)}`);
    }
    let filters: unknown[];
    if (filter === undefined) {
      filters = [];
    } else if (typeof filter === "string") {
      filters = [filter];
    } else if (Array.isArray(filter)) {
      filters = filter;
    } else {
      throw new TypeError(`filter must be notation text or a list of it, not ${nameOf(filter)}`);
    }
    for (const [index, expression] of filters.entries()) {
      if (typeof expression !== "string") {
        throw new TypeError(`filter[${index}] must be notation text`);
      }
    }
    this.#add([
      "add_import", ownerId(owner), visibility ?? "", target, recursive, all, filters,
    ]);
    return this;
  }

  /** Add a `require constraint` to a requirement-like body. */
  addRequireConstraint(owner: EditTarget, expression: string, name?: string): this {
    return this.addRequirementConstraint(owner, "require", expression, name);
  }

  /** Add an `assume constraint` to a requirement-like body. */
  addAssumeConstraint(owner: EditTarget, expression: string, name?: string): this {
    return this.addRequirementConstraint(owner, "assume", expression, name);
  }

  /** Add a connection-like usage between two feature references. */
  addConnection(
    owner: EditTarget,
    kind: string,
    from: string,
    to: string,
    options: { name?: string; type?: string } = {},
  ): this {
    const { name, type } = options;
    for (const [label, text] of [
      ["kind", kind],
      ["from_", from],
      ["to", to],
    ] as const) {
      if (typeof text !== "string") {
        throw new TypeError(`${label} must be notation text, not ${nameOf(text)}`);
      }
    }
    for (const [label, text] of [
      ["name", name],
      ["type", type],
    ] as const) {
      if (text !== undefined && typeof text !== "string") {
        throw new TypeError(`${label} must be notation text, not ${nameOf(text)}`);
      }
    }
    this.#add(["add_connection", ownerId(owner), kind, from, to, name ?? "", type ?? ""]);
    return this;
  }

  /** Add an `allocation ... allocate from to` usage. */
  addAllocation(owner: EditTarget, from: string, to: string, options: { name?: string; type?: string } = {}): this {
    return this.addConnection(owner, "allocation", from, to, options);
  }

  /** Add a `flow ... from from to` usage. */
  addFlow(owner: EditTarget, from: string, to: string, options: { name?: string; type?: string } = {}): this {
    return this.addConnection(owner, "flow", from, to, options);
  }

  /** Add a `succession ... first from then to` usage. */
  addSuccession(owner: EditTarget, from: string, to: string, options: { name?: string; type?: string } = {}): this {
    return this.addConnection(owner, "succession", from, to, options);
  }

  /** Delete a declaration, optionally removing declarations that refer to it. */
  delete(target: EditTarget, options: { cascade?: boolean } = {}): this {
    const { cascade = false } = options;
    if (typeof cascade !== "boolean") {
      throw new TypeError("cascade must be bool");
    }
    this.#add(["delete", targetId(target), cascade]);
    return this;
  }

  /** Move a declaration into another namespace of the same document. */
  move(target: EditTarget, owner: EditTarget): this {
    this.#add(["move", targetId(target), ownerId(owner)]);
    return this;
  }

  /** Have the service perform these operations and return the edited model. */
  async apply(): Promise<EditResult> {
    if (this.#applied) {
      throw new Error(
        "this editor has already been applied: it describes an edit of the model it was made " +
          "from, so build another editor from the edited model rather than applying this one twice",
      );
    }
    if (this.#operations.length === 0) {
      throw new NoEditsError(
        "this editor has no operations: add an edit before applying it",
        { failure: "EDIT_FAILURE_NO_OPERATIONS" },
      );
    }
    const result = await this.#apply(this.#modelHash, this.#operations);
    this.#applied = true;
    return result;
  }

  #add(operation: unknown[]): void {
    if (this.#applied) {
      throw new Error(
        "this editor has already been applied: build another editor from the edited model " +
          "to edit further",
      );
    }
    this.#operations.push(operation);
  }

  toString(): string {
    return `Editor(modelHash=${JSON.stringify(this.#modelHash)}, operations=${this.#operations.length}, applied=${this.#applied})`;
  }

  addPackage(owner: EditTarget, name: string, options: MemberOptions = {}): this {
    return this.addMember(owner, "package", name, options);
  }

  addPartDef(owner: EditTarget, name: string, options: MemberOptions = {}): this {
    return this.addMember(owner, "part def", name, options);
  }

  addPart(owner: EditTarget, name: string, options: MemberOptions = {}): this {
    return this.addMember(owner, "part", name, options);
  }

  addAttributeDef(owner: EditTarget, name: string, options: MemberOptions = {}): this {
    return this.addMember(owner, "attribute def", name, options);
  }

  addAttribute(owner: EditTarget, name: string, options: MemberOptions = {}): this {
    return this.addMember(owner, "attribute", name, options);
  }

  addItemDef(owner: EditTarget, name: string, options: MemberOptions = {}): this {
    return this.addMember(owner, "item def", name, options);
  }

  addItem(owner: EditTarget, name: string, options: MemberOptions = {}): this {
    return this.addMember(owner, "item", name, options);
  }

  addPortDef(owner: EditTarget, name: string, options: MemberOptions = {}): this {
    return this.addMember(owner, "port def", name, options);
  }

  addPort(owner: EditTarget, name: string, options: MemberOptions = {}): this {
    return this.addMember(owner, "port", name, options);
  }

  addClass(owner: EditTarget, name: string, options: MemberOptions = {}): this {
    return this.addMember(owner, "class", name, options);
  }

  addStruct(owner: EditTarget, name: string, options: MemberOptions = {}): this {
    return this.addMember(owner, "struct", name, options);
  }

  addDatatype(owner: EditTarget, name: string, options: MemberOptions = {}): this {
    return this.addMember(owner, "datatype", name, options);
  }

  addClassifier(owner: EditTarget, name: string, options: MemberOptions = {}): this {
    return this.addMember(owner, "classifier", name, options);
  }

  addFeature(owner: EditTarget, name: string, options: MemberOptions = {}): this {
    return this.addMember(owner, "feature", name, options);
  }

  addAssoc(owner: EditTarget, name: string, options: MemberOptions = {}): this {
    return this.addMember(owner, "assoc", name, options);
  }

  addBehavior(owner: EditTarget, name: string, options: MemberOptions = {}): this {
    return this.addMember(owner, "behavior", name, options);
  }

  addFunction(owner: EditTarget, name: string, options: MemberOptions = {}): this {
    return this.addMember(owner, "function", name, options);
  }

  addPredicate(owner: EditTarget, name: string, options: MemberOptions = {}): this {
    return this.addMember(owner, "predicate", name, options);
  }

  addInteraction(owner: EditTarget, name: string, options: MemberOptions = {}): this {
    return this.addMember(owner, "interaction", name, options);
  }

  addMetaclass(owner: EditTarget, name: string, options: MemberOptions = {}): this {
    return this.addMember(owner, "metaclass", name, options);
  }

  /** Add a `calc def` with input parameters and an optional result. */
  addCalcDef(
    owner: EditTarget,
    name: string,
    options: MemberOptions & {
      inputs?: readonly (readonly [string, string])[];
      returnType?: string;
      returnExpression?: string;
    } = {},
  ): this {
    const { inputs, returnType, returnExpression, expression, ...rest } = options;
    const parameters = parameterPairs(inputs, "inputs");
    optionalText(returnType, "return_type");
    optionalText(returnExpression, "return_expression");
    optionalText(expression, "expression");
    if (expression !== undefined && returnExpression !== undefined) {
      throw new RangeError("expression and return_expression both bind the result; give one");
    }
    if (returnExpression !== undefined && (returnType === undefined || returnType === "")) {
      throw new RangeError("return_expression requires return_type");
    }
    const ownerName = ownerId(owner);
    this.addMember(ownerName, "calc def", name, { expression, ...rest });
    const qualifiedName = ownerName === "" ? name : `${ownerName}::${name}`;
    for (const [parameterName, parameterType] of parameters) {
      this.addParameter(qualifiedName, "in", parameterName, { type: parameterType });
    }
    if (returnType !== undefined || returnExpression !== undefined) {
      this.addReturn(qualifiedName, { type: returnType, value: returnExpression });
    }
    return this;
  }

  /** Add a `calc` with input parameters and an optional result. */
  addCalc(
    owner: EditTarget,
    name: string,
    options: MemberOptions & {
      inputs?: readonly (readonly [string, string])[];
      returnType?: string;
      returnExpression?: string;
    } = {},
  ): this {
    const { inputs, returnType, returnExpression, expression, ...rest } = options;
    const parameters = parameterPairs(inputs, "inputs");
    optionalText(returnType, "return_type");
    optionalText(returnExpression, "return_expression");
    optionalText(expression, "expression");
    if (expression !== undefined && returnExpression !== undefined) {
      throw new RangeError("expression and return_expression both bind the result; give one");
    }
    if (returnExpression !== undefined && (returnType === undefined || returnType === "")) {
      throw new RangeError("return_expression requires return_type");
    }
    const ownerName = ownerId(owner);
    this.addMember(ownerName, "calc", name, { expression, ...rest });
    const qualifiedName = ownerName === "" ? name : `${ownerName}::${name}`;
    for (const [parameterName, parameterType] of parameters) {
      this.addParameter(qualifiedName, "in", parameterName, { type: parameterType });
    }
    if (returnType !== undefined || returnExpression !== undefined) {
      this.addReturn(qualifiedName, { type: returnType, value: returnExpression });
    }
    return this;
  }

  /** Add a directional parameter usage. */
  addParameter(
    owner: EditTarget,
    direction: string,
    name: string,
    options: MemberOptions & { kind?: string } = {},
  ): this {
    const { kind, ...rest } = options;
    return this.addMember(owner, kind ?? "", name, { ...rest, direction });
  }

  /** Add a return parameter member. */
  addReturn(owner: EditTarget, options: MemberOptions & { name?: string } = {}): this {
    const { name, ...rest } = options;
    return this.addMember(owner, "return", name ?? "", rest);
  }

  /** Add an `action def` with input and output parameters. */
  addActionDef(
    owner: EditTarget,
    name: string,
    options: MemberOptions & {
      inputs?: readonly (readonly [string, string])[];
      outputs?: readonly (readonly [string, string])[];
    } = {},
  ): this {
    const { inputs, outputs, ...rest } = options;
    const inputParameters = parameterPairs(inputs, "inputs");
    const outputParameters = parameterPairs(outputs, "outputs");
    const ownerName = ownerId(owner);
    this.addMember(ownerName, "action def", name, rest);
    const qualifiedName = ownerName === "" ? name : `${ownerName}::${name}`;
    for (const [parameterName, parameterType] of inputParameters) {
      this.addParameter(qualifiedName, "in", parameterName, { type: parameterType });
    }
    for (const [parameterName, parameterType] of outputParameters) {
      this.addParameter(qualifiedName, "out", parameterName, { type: parameterType });
    }
    return this;
  }

  /** Add an `action` with input and output parameters. */
  addAction(
    owner: EditTarget,
    name: string,
    options: MemberOptions & {
      inputs?: readonly (readonly [string, string])[];
      outputs?: readonly (readonly [string, string])[];
    } = {},
  ): this {
    const { inputs, outputs, ...rest } = options;
    const inputParameters = parameterPairs(inputs, "inputs");
    const outputParameters = parameterPairs(outputs, "outputs");
    const ownerName = ownerId(owner);
    this.addMember(ownerName, "action", name, rest);
    const qualifiedName = ownerName === "" ? name : `${ownerName}::${name}`;
    for (const [parameterName, parameterType] of inputParameters) {
      this.addParameter(qualifiedName, "in", parameterName, { type: parameterType });
    }
    for (const [parameterName, parameterType] of outputParameters) {
      this.addParameter(qualifiedName, "out", parameterName, { type: parameterType });
    }
    return this;
  }

  /** Add a `perform action name : Type` usage. */
  addPerformAction(owner: EditTarget, name: string, options: MemberOptions = {}): this {
    return this.addMember(owner, "perform action", name, options);
  }

  /** Add a `perform <action>;` usage named by its referenced action. */
  addPerform(owner: EditTarget, action: string, options: { doc?: string } = {}): this {
    return this.addMember(owner, "perform", action, options);
  }

  /** Add an `exhibit state` usage. */
  addExhibitState(owner: EditTarget, name: string, options: { type?: string } = {}): this {
    return this.addMember(owner, "exhibit state", name, options);
  }

  /** Add an `exhibit <state>;` usage named by its referenced state. */
  addExhibit(owner: EditTarget, state: string): this {
    return this.addMember(owner, "exhibit", state);
  }

  /** Add an `entry`, `do` or `exit` action to a state body. */
  addStateAction(owner: EditTarget, kind: string, name: string, options: { type?: string } = {}): this {
    const { type } = options;
    if (typeof kind !== "string") {
      throw new TypeError(`kind must be notation text, not ${nameOf(kind)}`);
    }
    if (kind !== "entry" && kind !== "do" && kind !== "exit") {
      throw new RangeError("kind must be 'entry', 'do' or 'exit'");
    }
    return this.addMember(owner, `${kind} action`, name, { type });
  }

  addStateDef(owner: EditTarget, name: string, options: MemberOptions = {}): this {
    return this.addMember(owner, "state def", name, options);
  }

  addState(owner: EditTarget, name: string, options: MemberOptions = {}): this {
    return this.addMember(owner, "state", name, options);
  }

  /** Add a `constraint def`; `expression` writes `{ … }`, `value` writes `= …`. */
  addConstraintDef(owner: EditTarget, name: string, options: MemberOptions = {}): this {
    return this.addMember(owner, "constraint def", name, options);
  }

  /** Add a `constraint`; `expression` writes `{ … }`, `value` writes `= …`. */
  addConstraint(owner: EditTarget, name: string, options: MemberOptions = {}): this {
    return this.addMember(owner, "constraint", name, options);
  }

  /** Add an asserted constraint usage, optionally negated. */
  addAssertConstraint(
    owner: EditTarget,
    options: { name?: string; type?: string; expression?: string; negated?: boolean } = {},
  ): this {
    const { name, type, expression, negated = false } = options;
    if (typeof negated !== "boolean") {
      throw new TypeError("negated must be bool");
    }
    return this.addMember(owner, negated ? "assert not constraint" : "assert constraint", name ?? "", {
      type,
      expression,
    });
  }

  /** Add an anonymous `assert` usage; the assertion is not named by ref. */
  addAssert(owner: EditTarget, ref: string, options: { negated?: boolean } = {}): this {
    const { negated = false } = options;
    if (typeof ref !== "string") {
      throw new TypeError(`ref must be notation text, not ${nameOf(ref)}`);
    }
    if (typeof negated !== "boolean") {
      throw new TypeError("negated must be bool");
    }
    return this.addMember(owner, negated ? "assert not" : "assert", ref);
  }

  addRequirementDef(owner: EditTarget, name: string, options: MemberOptions = {}): this {
    return this.addMember(owner, "requirement def", name, options);
  }

  addRequirement(owner: EditTarget, name: string, options: MemberOptions = {}): this {
    return this.addMember(owner, "requirement", name, options);
  }
}

const SEQUENCE_DETAIL_FIELDS = new Set([
  "condition", "value", "target", "via", "until", "body", "else_body",
  "multiplicity", "parameter",
]);
const ACTION_BODY_MEMBER_KINDS = new Set([
  "accept", "send", "assign", "if", "while", "loop", "for", "terminate",
]);
const ASSERTED_CONSTRAINT_KINDS = new Set([
  "assert", "assert not", "assert constraint", "assert not constraint",
]);
const STATE_ACTION_KINDS = new Set([
  "exhibit state", "exhibit", "entry action", "do action", "exit action",
]);

const EDIT_CAPABILITY_ORDER = [
  CAPABILITY_APPLY_EDITS,
  CAPABILITY_AUTHORING,
  CAPABILITY_CONNECTION_AUTHORING,
  CAPABILITY_SATISFY_AUTHORING,
  CAPABILITY_REQUIREMENT_CONSTRAINT_AUTHORING,
  CAPABILITY_TRANSITION_AUTHORING,
  CAPABILITY_VERIFICATION_OBJECTIVE_AUTHORING,
  CAPABILITY_METADATA_AUTHORING,
  CAPABILITY_METADATA_PREFIX_AUTHORING,
  CAPABILITY_SEQUENCE_AUTHORING,
  CAPABILITY_ACTION_BODY_STATEMENT_AUTHORING,
  CAPABILITY_IMPORT_AUTHORING,
  CAPABILITY_DOCUMENTATION_AUTHORING,
  CAPABILITY_COMMENT_AUTHORING,
  CAPABILITY_MEMBER_MODIFIERS,
  CAPABILITY_IMPLICIT_PARAMETERS,
  CAPABILITY_CONSTRAINT_BODY_AUTHORING,
  CAPABILITY_STATE_ACTION_AUTHORING,
];

// Capabilities an edit request needs as a whole, checked once every operation is read.
const EDIT_CAPABILITIES_CHECKED_LAST = [
  CAPABILITY_ACTION_BODY_STATEMENT_AUTHORING,
  CAPABILITY_MEMBER_MODIFIERS,
  CAPABILITY_IMPLICIT_PARAMETERS,
];

function allText(values: readonly unknown[]): boolean {
  return values.every((value) => typeof value === "string");
}

function memberExtras(extra: readonly unknown[]): [string[], string, string] {
  let metadata: unknown = [];
  let bodyExpression: unknown = "";
  let doc: unknown = "";
  if (extra.length > 0) {
    if (typeof extra[0] === "string") {
      bodyExpression = extra[0];
      if (extra.length === 2) {
        doc = extra[1];
      }
    } else {
      metadata = extra[0];
      if (extra.length > 1) {
        bodyExpression = extra[1];
      }
      if (extra.length > 2) {
        doc = extra[2];
      }
    }
  }
  if (typeof bodyExpression !== "string") {
    throw new RangeError("malformed add_member operation: expression must be notation text");
  }
  if (typeof doc !== "string") {
    throw new RangeError("malformed add_member operation: doc must be text");
  }
  return [metadata as string[], bodyExpression, doc];
}

function addSequenceMessage(add: AddSequenceEdit, operationData: unknown, depth = 0): boolean {
  if (depth > 128) {
    throw new RangeError("nested action-body items exceed the maximum depth");
  }
  if (!Array.isArray(operationData) || (operationData.length !== 8 && operationData.length !== 9)) {
    throw new RangeError("malformed add_sequence operation: expected 8 or 9 fields");
  }
  const data = operationData as readonly unknown[];
  const [, owner, keyword, ref, memberKind, memberName, typeName, after] = data;
  if (!allText([owner, keyword, ref, memberKind, memberName, typeName, after])) {
    throw new RangeError("malformed add_sequence operation: fields must be text");
  }
  const fields = data.length === 9 ? data[8] : {};
  if (typeof fields !== "object" || fields === null || Array.isArray(fields)) {
    throw new RangeError("malformed add_sequence operation: ninth field must be a dictionary");
  }
  const record = fields as Record<string, unknown>;
  const unknown = Object.keys(record).filter((key) => !SEQUENCE_DETAIL_FIELDS.has(key));
  if (unknown.length > 0) {
    throw new RangeError(
      `malformed add_sequence operation: unknown fields ${unknown.sort().join(", ")}`,
    );
  }
  for (const [key, value] of Object.entries(record)) {
    if (key === "body" || key === "else_body") {
      if (!Array.isArray(value)) {
        throw new RangeError(`malformed add_sequence operation: ${key} must be a list`);
      }
    } else if (typeof value !== "string") {
      throw new RangeError(`malformed add_sequence operation: ${key} must be text`);
    }
  }
  add.owner = owner as string;
  add.keyword = keyword as string;
  add.ref = ref as string;
  add.memberKind = memberKind as string;
  add.memberName = memberName as string;
  add.type = typeName as string;
  add.after = after as string;
  for (const key of ["condition", "value", "target", "via", "until", "multiplicity", "parameter"] as const) {
    if (key in record) {
      add[key] = record[key] as string;
    }
  }
  let extended =
    keyword === "if" ||
    keyword === "else" ||
    (keyword === "" && memberKind !== "") ||
    ACTION_BODY_MEMBER_KINDS.has(memberKind as string);
  extended = extended || Object.keys(record).some((key) => SEQUENCE_DETAIL_FIELDS.has(key));
  for (const child of (record["body"] as readonly unknown[][] | undefined) ?? []) {
    const childEdit = create(AddSequenceEditSchema);
    add.body.push(childEdit);
    extended = addSequenceMessage(childEdit, child, depth + 1) || extended;
  }
  for (const child of (record["else_body"] as readonly unknown[][] | undefined) ?? []) {
    const childEdit = create(AddSequenceEditSchema);
    add.elseBody.push(childEdit);
    extended = addSequenceMessage(childEdit, child, depth + 1) || extended;
  }
  return extended;
}

/** Translates edit tuples into an `ApplyEditsRequest`'s operations, gating capabilities. */
export class EditRequestBuilder {
  readonly #info: ServerInfo;
  readonly #requested = new Set<string>([CAPABILITY_APPLY_EDITS]);

  constructor(info: ServerInfo) {
    this.#info = info;
  }

  /** The operations the tuples describe, and the capabilities they need. */
  build(operations: readonly EditOperationData[]): { operations: EditOperation[]; capabilities: string[] } {
    const built = operations.map((data) => this.add(data));
    return { operations: built, capabilities: this.finish() };
  }

  add(operationData: EditOperationData): EditOperation {
    const operation = create(EditOperationSchema);
    switch (operationData[0]) {
      case "set_value": this.#setValue(operation, operationData); break;
      case "rename": this.#rename(operation, operationData); break;
      case "add_member": this.#addMember(operation, operationData); break;
      case "add_connection": this.#addConnection(operation, operationData); break;
      case "add_satisfy": this.#addSatisfy(operation, operationData); break;
      case "add_requirement_constraint": this.#addRequirementConstraint(operation, operationData); break;
      case "add_transition": this.#addTransition(operation, operationData); break;
      case "add_verify": this.#addVerify(operation, operationData); break;
      case "add_metadata": this.#addMetadata(operation, operationData); break;
      case "add_metadata_prefix": this.#addMetadataPrefix(operation, operationData); break;
      case "add_sequence": this.#addSequence(operation, operationData); break;
      case "add_import": this.#addImport(operation, operationData); break;
      case "add_documentation": this.#addDocumentation(operation, operationData); break;
      case "add_comment": this.#addComment(operation, operationData); break;
      case "add_note": this.#addNote(operation, operationData); break;
      case "delete": this.#delete(operation, operationData); break;
      case "move": this.#move(operation, operationData); break;
      default:
        throw new RangeError(
          `unknown edit operation ${JSON.stringify(operationData[0])}: expected set_value, rename, ` +
            "add_member, add_connection, add_satisfy, add_requirement_constraint, " +
            "add_transition, add_verify, add_metadata, add_metadata_prefix, add_sequence, " +
            "add_import, add_documentation, add_comment, add_note, delete or move",
        );
    }
    return operation;
  }

  finish(): string[] {
    for (const capability of EDIT_CAPABILITIES_CHECKED_LAST) {
      if (this.#requested.has(capability)) {
        this.#require(capability);
      }
    }
    return EDIT_CAPABILITY_ORDER.filter((capability) => this.#requested.has(capability));
  }

  #require(...capabilities: string[]): void {
    for (const capability of capabilities) {
      requireCapability(this.#info, capability, upgradeRemedy(capability));
      this.#requested.add(capability);
    }
  }

  #note(capability: string, needed = true): void {
    if (needed) {
      this.#requested.add(capability);
    }
  }

  #setValue(operation: EditOperation, operationData: readonly unknown[]): void {
    const [, target, text] = operationData;
    operation.operation = {
      case: "setValue",
      value: create(SetValueEditSchema, {
        target: target as string,
        value: text as string,
      }),
    };
  }

  #rename(operation: EditOperation, operationData: readonly unknown[]): void {
    const [, target, text] = operationData;
    operation.operation = {
      case: "rename",
      value: create(RenameEditSchema, {
        target: target as string,
        newName: text as string,
      }),
    };
  }

  #addMember(operation: EditOperation, operationData: readonly unknown[]): void {
    if (![8, 12, 13, 14, 15].includes(operationData.length)) {
      throw new RangeError(
        "malformed add_member operation: expected 8, 12, 13, 14 or 15 fields",
      );
    }
    const [, owner, memberKind, name, typeName, multiplicity, value, specializes] = operationData;
    const modifiers = operationData.length >= 12 ? operationData.slice(8, 12) : [];
    const [metadata, bodyExpression, doc] = memberExtras(operationData.slice(12));
    this.#require(CAPABILITY_AUTHORING);
    this.#note(CAPABILITY_IMPLICIT_PARAMETERS, memberKind === "");
    if (memberKind === "objective" && name === "") {
      this.#require(CAPABILITY_VERIFICATION_OBJECTIVE_AUTHORING);
    }
    const add = create(AddMemberEditSchema);
    add.owner = owner as string;
    add.kind = memberKind as string;
    add.name = name as string;
    add.type = typeName as string;
    add.multiplicity = multiplicity as string;
    add.value = value as string;
    add.specializes = [...(specializes as readonly string[])];
    if (doc !== "") {
      this.#require(CAPABILITY_DOCUMENTATION_AUTHORING);
      add.doc = doc;
    }
    if (modifiers.length > 0) {
      this.#memberModifiers(add, memberKind as string, modifiers);
    }
    if (!Array.isArray(metadata) || !allText(metadata)) {
      throw new RangeError(
        "malformed add_member metadata: expected a sequence of notation strings",
      );
    }
    add.metadataPrefixes = [...(metadata as readonly string[])];
    if (metadata.length > 0) {
      this.#require(CAPABILITY_METADATA_AUTHORING);
    }
    add.bodyExpression = bodyExpression;
    if (bodyExpression !== "" || ASSERTED_CONSTRAINT_KINDS.has(memberKind as string)) {
      this.#require(CAPABILITY_CONSTRAINT_BODY_AUTHORING);
    }
    if (STATE_ACTION_KINDS.has(memberKind as string)) {
      this.#require(CAPABILITY_STATE_ACTION_AUTHORING);
    }
    operation.operation = { case: "addMember", value: add };
  }

  #memberModifiers(
    add: { isAbstract: boolean; redefines: string[]; isDefault: boolean; direction: string },
    memberKind: string,
    modifiers: readonly unknown[],
  ): void {
    const [abstract, redefines, isDefault, direction] = modifiers;
    if (typeof abstract !== "boolean" || typeof isDefault !== "boolean") {
      throw new RangeError("malformed add_member modifiers: abstract and default must be bool");
    }
    if (typeof redefines === "string" || !Array.isArray(redefines) || !allText(redefines)) {
      throw new RangeError("malformed add_member modifiers: redefines must be a sequence");
    }
    if (typeof direction !== "string") {
      throw new RangeError("malformed add_member modifiers: direction must be notation text");
    }
    add.isAbstract = abstract;
    add.redefines = [...(redefines as readonly string[])];
    add.isDefault = isDefault;
    add.direction = direction;
    this.#note(
      CAPABILITY_MEMBER_MODIFIERS,
      abstract || (redefines as readonly string[]).length > 0 || isDefault || direction !== "" ||
        memberKind === "ref" || memberKind === "return",
    );
  }

  #addConnection(operation: EditOperation, operationData: readonly unknown[]): void {
    if (operationData.length !== 7) {
      throw new RangeError("malformed add_connection operation: expected 7 fields");
    }
    const [, owner, connectionKind, fromEnd, toEnd, name, typeName] = operationData;
    this.#require(CAPABILITY_AUTHORING, CAPABILITY_CONNECTION_AUTHORING);
    operation.operation = {
      case: "addConnection",
      value: create(AddConnectionEditSchema, {
        owner: owner as string,
        kind: connectionKind as string,
        fromEnd: fromEnd as string,
        toEnd: toEnd as string,
        name: name as string,
        type: typeName as string,
      }),
    };
  }

  #addSatisfy(operation: EditOperation, operationData: readonly unknown[]): void {
    if (operationData.length !== 6) {
      throw new RangeError("malformed add_satisfy operation: expected 6 fields");
    }
    const [, owner, requirement, satisfyingFeature, asserted, negated] = operationData;
    if (typeof asserted !== "boolean" || typeof negated !== "boolean") {
      throw new RangeError("malformed add_satisfy operation: asserted and negated must be bool");
    }
    this.#require(CAPABILITY_AUTHORING, CAPABILITY_SATISFY_AUTHORING);
    operation.operation = {
      case: "addSatisfy",
      value: create(AddSatisfyEditSchema, {
        owner: owner as string,
        requirement: requirement as string,
        satisfyingFeature: satisfyingFeature as string,
        isAsserted: asserted,
        isNegated: negated,
      }),
    };
  }

  #addRequirementConstraint(operation: EditOperation, operationData: readonly unknown[]): void {
    if (operationData.length !== 5) {
      throw new RangeError("malformed add_requirement_constraint operation: expected 5 fields");
    }
    const [, owner, constraintKind, expression, name] = operationData;
    this.#require(CAPABILITY_AUTHORING, CAPABILITY_REQUIREMENT_CONSTRAINT_AUTHORING);
    operation.operation = {
      case: "addRequirementConstraint",
      value: create(AddRequirementConstraintEditSchema, {
        owner: owner as string,
        kind: constraintKind as string,
        expression: expression as string,
        name: name as string,
      }),
    };
  }

  #addTransition(operation: EditOperation, operationData: readonly unknown[]): void {
    if (operationData.length !== 9) {
      throw new RangeError("malformed add_transition operation: expected 9 fields");
    }
    const [, owner, name, source, target, trigger, guard, effect, initial] = operationData;
    if (!allText([owner, name, source, target, trigger, guard, effect]) || typeof initial !== "boolean") {
      throw new RangeError(
        "malformed add_transition operation: text fields and initial must be valid",
      );
    }
    this.#require(CAPABILITY_AUTHORING, CAPABILITY_TRANSITION_AUTHORING);
    operation.operation = {
      case: "addTransition",
      value: create(AddTransitionEditSchema, {
        owner: owner as string,
        name: name as string,
        source: source as string,
        target: target as string,
        trigger: trigger as string,
        guard: guard as string,
        effect: effect as string,
        initial,
      }),
    };
  }

  #addVerify(operation: EditOperation, operationData: readonly unknown[]): void {
    if (operationData.length !== 3) {
      throw new RangeError("malformed add_verify operation: expected 3 fields");
    }
    const [, owner, requirement] = operationData;
    if (typeof owner !== "string" || typeof requirement !== "string") {
      throw new RangeError("malformed add_verify operation: fields must be notation text");
    }
    this.#require(CAPABILITY_AUTHORING, CAPABILITY_VERIFICATION_OBJECTIVE_AUTHORING);
    operation.operation = {
      case: "addVerify",
      value: create(AddVerifyEditSchema, { owner, requirement }),
    };
  }

  #addMetadata(operation: EditOperation, operationData: readonly unknown[]): void {
    if (operationData.length !== 7) {
      throw new RangeError("malformed add_metadata operation: expected 7 fields");
    }
    const [, owner, metadataType, name, about, values, shorthand] = operationData;
    if (!allText([owner, metadataType, name])) {
      throw new RangeError("malformed add_metadata operation: text fields must be strings");
    }
    if (typeof shorthand !== "boolean") {
      throw new RangeError("malformed add_metadata operation: shorthand must be bool");
    }
    if (!Array.isArray(about) || !allText(about)) {
      throw new RangeError("malformed add_metadata operation: about must be notation strings");
    }
    if (typeof values === "string" || !Array.isArray(values)) {
      throw new RangeError("malformed add_metadata operation: values must be feature-value pairs");
    }
    this.#require(CAPABILITY_AUTHORING, CAPABILITY_METADATA_AUTHORING);
    const bindings = (values as readonly unknown[]).map((pair, index) => {
      if (!Array.isArray(pair) || pair.length !== 2 || !allText(pair)) {
        throw new RangeError(
          `malformed add_metadata operation: values[${index}] must be a pair of strings`,
        );
      }
      return create(MetadataFeatureValueSchema, {
        feature: pair[0] as string,
        value: pair[1] as string,
      });
    });
    operation.operation = {
      case: "addMetadata",
      value: create(AddMetadataEditSchema, {
        owner: owner as string,
        metadataType: metadataType as string,
        name: name as string,
        about: [...(about as readonly string[])],
        values: bindings,
        shorthand,
      }),
    };
  }

  #addMetadataPrefix(operation: EditOperation, operationData: readonly unknown[]): void {
    if (operationData.length !== 3) {
      throw new RangeError("malformed add_metadata_prefix operation: expected 3 fields");
    }
    const [, target, metadataType] = operationData;
    if (typeof target !== "string" || typeof metadataType !== "string") {
      throw new RangeError("malformed add_metadata_prefix operation: fields must be notation text");
    }
    this.#require(CAPABILITY_AUTHORING, CAPABILITY_METADATA_PREFIX_AUTHORING);
    operation.operation = {
      case: "addMetadataPrefix",
      value: create(AddMetadataPrefixEditSchema, { target, metadataType }),
    };
  }

  #addSequence(operation: EditOperation, operationData: readonly unknown[]): void {
    if (!Array.isArray(operationData) || (operationData.length !== 8 && operationData.length !== 9)) {
      throw new RangeError("malformed add_sequence operation: expected 8 or 9 fields");
    }
    this.#require(CAPABILITY_AUTHORING, CAPABILITY_SEQUENCE_AUTHORING);
    const add = create(AddSequenceEditSchema);
    this.#note(
      CAPABILITY_ACTION_BODY_STATEMENT_AUTHORING,
      addSequenceMessage(add, operationData),
    );
    operation.operation = { case: "addSequence", value: add };
  }

  #addImport(operation: EditOperation, operationData: readonly unknown[]): void {
    if (operationData.length !== 7) {
      throw new RangeError("malformed add_import operation: expected 7 fields");
    }
    const [, owner, visibility, target, recursive, importAll, filters] = operationData;
    if (
      !allText([owner, visibility, target]) ||
      typeof recursive !== "boolean" ||
      typeof importAll !== "boolean" ||
      !Array.isArray(filters) ||
      !allText(filters)
    ) {
      throw new RangeError(
        "malformed add_import operation: text fields, recursive, import_all and filters must be valid",
      );
    }
    this.#require(CAPABILITY_AUTHORING, CAPABILITY_IMPORT_AUTHORING);
    operation.operation = {
      case: "addImport",
      value: create(AddImportEditSchema, {
        owner: owner as string,
        visibility: visibility as string,
        target: target as string,
        isRecursive: recursive,
        isImportAll: importAll,
        filters: [...(filters as readonly string[])],
      }),
    };
  }

  #addDocumentation(operation: EditOperation, operationData: readonly unknown[]): void {
    if (operationData.length !== 6) {
      throw new RangeError("malformed add_documentation operation: expected 6 fields");
    }
    const [, target, body, name, locale, replace] = operationData;
    if (!allText([target, body, name, locale]) || typeof replace !== "boolean") {
      throw new RangeError(
        "malformed add_documentation operation: text fields and replace must be valid",
      );
    }
    this.#require(CAPABILITY_AUTHORING, CAPABILITY_DOCUMENTATION_AUTHORING);
    operation.operation = {
      case: "addDocumentation",
      value: create(AddDocumentationEditSchema, {
        target: target as string,
        body: body as string,
        name: name as string,
        locale: locale as string,
        replace,
      }),
    };
  }

  #addComment(operation: EditOperation, operationData: readonly unknown[]): void {
    if (operationData.length !== 6) {
      throw new RangeError("malformed add_comment operation: expected 6 fields");
    }
    const [, owner, body, name, about, locale] = operationData;
    const aboutList = Array.isArray(about) ? (about as readonly unknown[]) : null;
    if (!allText([owner, body, name, locale]) || aboutList === null || !allText(aboutList)) {
      throw new RangeError(
        "malformed add_comment operation: text fields and about must be valid",
      );
    }
    this.#require(CAPABILITY_AUTHORING, CAPABILITY_COMMENT_AUTHORING);
    operation.operation = {
      case: "addComment",
      value: create(AddCommentEditSchema, {
        owner: owner as string,
        body: body as string,
        name: name as string,
        about: [...(aboutList as readonly string[])],
        locale: locale as string,
      }),
    };
  }

  #addNote(operation: EditOperation, operationData: readonly unknown[]): void {
    if (operationData.length !== 3 || !allText(operationData.slice(1))) {
      throw new RangeError("malformed add_note operation: expected target and text");
    }
    const [, target, text] = operationData;
    this.#require(CAPABILITY_AUTHORING, CAPABILITY_COMMENT_AUTHORING);
    operation.operation = {
      case: "addNote",
      value: create(AddNoteEditSchema, {
        target: target as string,
        text: text as string,
      }),
    };
  }

  #delete(operation: EditOperation, operationData: readonly unknown[]): void {
    if (operationData.length !== 3 || typeof operationData[2] !== "boolean") {
      throw new RangeError("malformed delete operation: expected target and bool cascade");
    }
    const [, target, cascade] = operationData;
    this.#require(CAPABILITY_AUTHORING);
    operation.operation = {
      case: "delete",
      value: create(DeleteEditSchema, {
        target: target as string,
        cascade,
      }),
    };
  }

  #move(operation: EditOperation, operationData: readonly unknown[]): void {
    if (operationData.length !== 3) {
      throw new RangeError("malformed move operation: expected target and owner");
    }
    const [, target, owner] = operationData;
    this.#require(CAPABILITY_AUTHORING);
    operation.operation = {
      case: "move",
      value: create(MoveEditSchema, {
        target: target as string,
        owner: owner as string,
      }),
    };
  }
}

