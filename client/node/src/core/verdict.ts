// Verdicts of the verification the service performs, and the results that carry
// them: validations, calc results, analysis results and sweep tables.

import type {
  CaseEvaluation as PbCaseEvaluation,
  RunAnalysisResponse,
  RunSweepResponse,
  SweepRow as PbSweepRow,
  ValidateInstanceResponse,
  Value,
  Verdict as PbVerdict,
  VerificationVerdict as PbVerificationVerdict,
} from "../generated/sysml_pb.js";
import { FailureReason } from "../generated/sysml_pb.js";
import {
  AnalysisRunError,
  ExecutionError,
  UnsupportedValueError,
  WrongKindError,
  type ModelDiagnostic,
} from "./errors.js";
import type { Instance } from "./model.js";
import { standingOf, type Standing } from "./engines.js";
import {
  decodeValue,
  decodeVerdict,
  formatValue,
  type SysMLValue,
  type SysMLVerdict,
  type VerificationVerdict,
  type WitnessAssignment,
} from "./values.js";

/** Verdict kinds, as the service reports them. */
export const KIND_CONSTRAINT = "constraint";
export const KIND_REQUIREMENT = "requirement";
export const KIND_SATISFY = "satisfy";
export const KIND_OBJECTIVE = "objective";
export const KIND_ASSERTION = "assertion";
export const KIND_OBJECT = "object";

/** VerdictKind values a verification case's body produces. */
export const VERDICT_PASS = "pass";
export const VERDICT_FAIL = "fail";
export const VERDICT_INCONCLUSIVE = "inconclusive";
export const VERDICT_ERROR = "error";

/** The questions a verification answers. */
export const QUESTION_EVALUATE = "evaluate";
export const QUESTION_HOLDS = "holds";
export const QUESTION_SATISFIABLE = "satisfiable";

/** The statuses a verdict reports, as the service spells them. */
export const STATUS_HOLDS = "holds";
export const STATUS_VIOLATED = "violated";
export const STATUS_UNDECIDED = "undecided";
export const STATUS_SATISFIABLE = "satisfiable";
export const STATUS_UNSATISFIABLE = "unsatisfiable";

export type { SysMLVerdict, VerificationVerdict, WitnessAssignment };

/** Whether a verdict's condition holds: the arm it decoded to. */
export function verdictHolds(verdict: SysMLVerdict): boolean {
  return verdict.kind === "holds";
}

/** Whether the verdict is an answer about the model at all: not undecided. */
export function verdictEvaluated(verdict: SysMLVerdict): boolean {
  return verdict.kind !== "undecided";
}

function named(verdict: SysMLVerdict): string {
  const { subject } = verdict;
  const element = subject.element !== "" ? subject.element : subject.elementId;
  let out = element.split(" ").includes(subject.kind) ? element : `${subject.kind} ${element}`;
  if (subject.instancePath !== undefined) {
    out += ` at ${subject.instancePath}`;
  }
  return out;
}

/** Raise an {@link ExecutionError} when the verdict is undecided; else return it. */
export function raiseVerdictError(verdict: SysMLVerdict): SysMLVerdict {
  if (verdict.kind === "undecided") {
    throw new ExecutionError(`${named(verdict)}: ${verdict.error}`);
  }
  return verdict;
}

/** One line saying what the verdict is and why, then its standing when reported. */
export function explainVerdict(verdict: SysMLVerdict): string {
  const { subject } = verdict;
  let subject_ = "";
  if (subject.instanceId !== undefined) {
    subject_ = ` (on ${subject.instanceTypeId ?? "instance"} ID: ${subject.instanceId.toString()})`;
  }
  let line: string;
  if (verdict.kind === "undecided") {
    line = `? ${named(verdict)}${subject_}: ${verdict.error}`;
  } else if (verdict.kind === "holds") {
    line = `✓ ${named(verdict)} holds${subject_}`;
  } else {
    const detail =
      verdict.condition !== ""
        ? `: condition evaluated to false: ${verdict.condition}`
        : ": condition evaluated to false";
    line = `✗ ${named(verdict)} fails${subject_}${detail}`;
  }
  const standing = explainVerdictStanding(verdict.standing);
  return standing === "" ? line : `${line} — ${standing}`;
}

function explainVerdictStanding(standing: Standing): string {
  if (standing.strength === "") {
    return "";
  }
  let line = standing.strength;
  if (standing.engine !== "") {
    line += ` by ${standing.engine}`;
  }
  const reached = standing.bounds.filter((bound) => bound.reached);
  if (reached.length > 0) {
    line += ` (${reached.map((b) => `${b.name} ${b.limit.toString()} reached`).join(", ")})`;
  }
  return line;
}

/** One line saying what a verification case's body answered and why. */
export function explainVerificationVerdict(verdict: VerificationVerdict): string {
  const mark = verdict.kind === VERDICT_PASS ? "✓" : verdict.kind === VERDICT_FAIL ? "✗" : "?";
  let line = `${mark} verification ${verdict.caseId} verdict: ${verdict.kind}`;
  if (verdict.subcase) {
    line += " (subcase)";
  }
  if (verdict.detail !== "") {
    line += ` — ${verdict.detail}`;
  }
  return line;
}

/** Whether a verification-case verdict is a pass. */
export function verificationPassed(verdict: VerificationVerdict): boolean {
  return verdict.kind === VERDICT_PASS;
}

/** Read a wire `VerificationVerdict`. */
export function decodeVerificationVerdict(pb: PbVerificationVerdict): VerificationVerdict {
  return {
    caseId: pb.caseId,
    kind: pb.kind,
    detail: pb.detail,
    subcase: pb.subcase,
    requirementId: pb.requirementId,
  };
}

/**
 * A verdict as the response reported it: the union of values.ts plus the
 * instances and diagnostics its response carried. A verification's verdict is
 * the answer it gave, never an exception; an undecided arm is an answer too —
 * a failure to evaluate, reported as `error`.
 */
export interface Verdict {
  readonly verdict: SysMLVerdict;
  /** The objects the call reported: the one this verdict is about and those reachable from it. */
  readonly instances: readonly Instance[];
  /** Diagnostics the service reported. */
  readonly diagnostics: readonly ModelDiagnostic[];
}

/** The arm of a `Verdict`, for callers that switch on it directly. */
export function armOf(verdict: Verdict): SysMLVerdict {
  return verdict.verdict;
}

/** Whether the verdict's condition holds. */
export function holds(verdict: Verdict): boolean {
  return verdict.verdict.kind === "holds";
}

/** Raise when the verdict reports the named element is of another kind. */
export function raiseWrongKind(
  pbVerdict: PbVerdict | undefined,
  diagnostics: readonly ModelDiagnostic[],
): void {
  if (pbVerdict?.failureReason === FailureReason.WRONG_KIND) {
    throw new WrongKindError(pbVerdict.error, "wrong_kind", diagnostics);
  }
}

/** Raise the error a failure the service classified: wrong kind, else a run failure. */
export function raiseFailure(
  message: string,
  failureReason: FailureReason,
  diagnostics: readonly ModelDiagnostic[],
): never {
  if (failureReason === FailureReason.WRONG_KIND) {
    throw new WrongKindError(message, "wrong_kind", diagnostics);
  }
  throw new ExecutionError(message, "unspecified", diagnostics);
}

/** Read the error-or-result of an outputs map: undecodable values stay errors in place. */
export function valuesMap(
  entries: Iterable<[string, Value | undefined]>,
): Map<string, SysMLValue | UnsupportedValueError> {
  const out = new Map<string, SysMLValue | UnsupportedValueError>();
  for (const [name, pb] of entries) {
    try {
      out.set(name, decodeValue(pb));
    } catch (error) {
      out.set(
        name,
        error instanceof UnsupportedValueError
          ? error
          : new UnsupportedValueError(error instanceof Error ? error.message : String(error)),
      );
    }
  }
  return out;
}

/** The verifications the requirement of `pbVerdict` was reported for. */
export function verificationsFor(
  verifications: readonly VerificationVerdict[],
  pbVerdict: PbVerdict,
): VerificationVerdict[] {
  if (pbVerdict.requirementId === "") {
    return [];
  }
  return verifications.filter((v) => v.requirementId === pbVerdict.requirementId);
}

/** Wrap a wire verdict with the instances and diagnostics its response carried,
 * attaching the verifications of its own requirement — every one the response
 * carried when `attachAll` is set, as a single-verdict response asks for. */
export function verdictOf(
  pbVerdict: PbVerdict,
  instances: readonly Instance[],
  diagnostics: readonly ModelDiagnostic[],
  verifications: readonly VerificationVerdict[],
  attachAll = false,
): Verdict {
  return {
    verdict: decodeVerdict(
      pbVerdict,
      attachAll ? verifications : verificationsFor(verifications, pbVerdict),
    ),
    instances,
    diagnostics,
  };
}

/** The outputs an invocation computed, or the features a calc usage wrote. */
export class CalcResult {
  /** The value an invocation returned; undefined when `outputs` carries the answer. */
  readonly value: SysMLValue | UnsupportedValueError | undefined;
  /** Output features a calc usage computed. */
  readonly outputs: ReadonlyMap<string, SysMLValue | UnsupportedValueError>;
  readonly diagnostics: readonly ModelDiagnostic[];
  readonly standing: Standing;

  constructor(init: {
    value?: SysMLValue | UnsupportedValueError;
    outputs: ReadonlyMap<string, SysMLValue | UnsupportedValueError>;
    diagnostics?: readonly ModelDiagnostic[];
    standing?: Standing;
  }) {
    this.value = init.value;
    this.outputs = init.outputs;
    this.diagnostics = init.diagnostics ?? [];
    this.standing = init.standing ?? { engine: "", strength: "", bounds: [] };
  }

  get engine(): string {
    return this.standing.engine;
  }

  toString(): string {
    if (this.outputs.size > 0) {
      return [...this.outputs.entries()]
        .map(([name, val]) => `${name} = ${render(val)}`)
        .join(", ");
    }
    return render(this.value);
  }
}

function render(value: SysMLValue | UnsupportedValueError | undefined): string {
  if (value === undefined) {
    return "undefined";
  }
  return value instanceof UnsupportedValueError ? `<unsupported: ${value.message}>` : formatValue(value);
}

/** One application an analysis case made of one of its own calcs as a value. */
export class CaseEvaluation {
  /** FQN of the calc applied. */
  readonly functionId: string;
  /** What it was applied to; an alternative is the instance it is. */
  readonly arguments: readonly (SysMLValue | UnsupportedValueError)[];
  /** What it computed; undefined when `error` says why nothing was. */
  readonly result: SysMLValue | UnsupportedValueError | undefined;
  /** Why the evaluation computed nothing; empty when it did. */
  readonly error: string;
  /** Whether the case returned this evaluation's argument: a trade study's selection. */
  readonly selected: boolean;
  /** Whether it computed what the selected one did without being selected. */
  readonly tied: boolean;

  constructor(init: {
    functionId: string;
    arguments: readonly (SysMLValue | UnsupportedValueError)[];
    result?: SysMLValue | UnsupportedValueError;
    error?: string;
    selected?: boolean;
    tied?: boolean;
  }) {
    this.functionId = init.functionId;
    this.arguments = init.arguments;
    this.result = init.result;
    this.error = init.error ?? "";
    this.selected = init.selected ?? false;
    this.tied = init.tied ?? false;
  }

  /** Whether the evaluation computed a result. */
  get evaluated(): boolean {
    return this.error === "";
  }

  /** One line saying what was evaluated, what it gave, and whether it was selected. */
  explain(): string {
    const call = `${this.functionId}(${this.arguments.map(render).join(", ")})`;
    let line = this.error !== "" ? `${call}: error: ${this.error}` : `${call} = ${render(this.result)}`;
    if (this.selected) {
      line += " [selected]";
    } else if (this.tied) {
      line += " [tied]";
    }
    return line;
  }

  toString(): string {
    return this.explain();
  }
}

/** What an analysis case computed and decided. */
export class AnalysisResult {
  /** Output features the case computed, by name. */
  readonly outputs: ReadonlyMap<string, SysMLValue | UnsupportedValueError>;
  /** The objective and assertion verdicts, in the order the case states them. */
  readonly verdicts: readonly Verdict[];
  /** The subject the case ran on and the objects reachable from it. */
  readonly instances: readonly Instance[];
  readonly diagnostics: readonly ModelDiagnostic[];
  /** For a verification case, what its body answered and each subcase's. */
  readonly verifications: readonly VerificationVerdict[];
  /** Each application the run made of one of the case's calcs as a value. */
  readonly evaluations: readonly CaseEvaluation[];
  readonly standing: Standing;

  constructor(init: {
    outputs: ReadonlyMap<string, SysMLValue | UnsupportedValueError>;
    verdicts: readonly Verdict[];
    instances?: readonly Instance[];
    diagnostics?: readonly ModelDiagnostic[];
    verifications?: readonly VerificationVerdict[];
    evaluations?: readonly CaseEvaluation[];
    standing?: Standing;
  }) {
    this.outputs = init.outputs;
    this.verdicts = init.verdicts;
    this.instances = init.instances ?? [];
    this.diagnostics = init.diagnostics ?? [];
    this.verifications = init.verifications ?? [];
    this.evaluations = init.evaluations ?? [];
    this.standing = init.standing ?? { engine: "", strength: "", bounds: [] };
  }

  get engine(): string {
    return this.standing.engine;
  }

  /** The evaluations of the alternatives the case selected; one for a trade study. */
  get selected(): CaseEvaluation[] {
    return this.evaluations.filter((evaluation) => evaluation.selected);
  }

  /** Whether every objective and assertion holds; a case stating none is satisfied. */
  get satisfied(): boolean {
    return this.verdicts.every((verdict) => verdict.verdict.kind === "holds");
  }

  toString(): string {
    const lines = [...this.outputs.entries()].map(([name, val]) => `${name} = ${render(val)}`);
    lines.push(...this.verdicts.map((verdict) => explainVerdict(verdict.verdict)));
    lines.push(...this.verifications.map(explainVerificationVerdict));
    lines.push(...this.evaluations.map((evaluation) => evaluation.explain()));
    return lines.join("\n");
  }
}

/** Every assertion about one object and the objects it holds, answered. */
export class Validation {
  /** One per assertion, root first then each held object in traversal order. */
  readonly verdicts: readonly Verdict[];
  /** The object's own verdict, of kind 'object'; undefined when the service sent none. */
  readonly summary: Verdict | undefined;
  /** The object validated and every object reachable from it. */
  readonly instances: readonly Instance[];
  /** Whether traversal stopped at its bound before reaching every held object. */
  readonly bounded: boolean;
  readonly diagnostics: readonly ModelDiagnostic[];
  /** What the bodies of the verification cases of the requirements met answered. */
  readonly verifications: readonly VerificationVerdict[];

  constructor(init: {
    verdicts: readonly Verdict[];
    summary?: Verdict;
    instances?: readonly Instance[];
    bounded?: boolean;
    diagnostics?: readonly ModelDiagnostic[];
    verifications?: readonly VerificationVerdict[];
  }) {
    this.verdicts = init.verdicts;
    this.summary = init.summary;
    this.instances = init.instances ?? [];
    this.bounded = init.bounded ?? false;
    this.diagnostics = init.diagnostics ?? [];
    this.verifications = init.verifications ?? [];
  }

  /** The standing of the summary's engine; unreported when the service sent none. */
  get standing(): Standing {
    return this.summary?.verdict.standing ?? { engine: "", strength: "", bounds: [] };
  }

  get engine(): string {
    return this.standing.engine;
  }

  /** Whether the object is shown valid: at least one assertion, every one holding, all reached. */
  get valid(): boolean {
    return (
      this.summary !== undefined &&
      this.summary.verdict.kind === "holds" &&
      !this.bounded
    );
  }

  /** The verdicts the model answered false, as opposed to undecided ones. */
  get violated(): Verdict[] {
    return this.verdicts.filter((verdict) => verdict.verdict.kind === "fails");
  }

  /** The verdicts whose condition could not be evaluated. */
  get undecided(): Verdict[] {
    return this.verdicts.filter((verdict) => verdict.verdict.kind === "undecided");
  }

  /** Raise an {@link ExecutionError} if any assertion could not be decided. */
  raiseForError(): this {
    for (const verdict of this.undecided) {
      raiseVerdictError(verdict.verdict);
    }
    return this;
  }

  get length(): number {
    return this.verdicts.length;
  }

  [Symbol.iterator](): Iterator<Verdict> {
    return this.verdicts[Symbol.iterator]();
  }

  toString(): string {
    const lines = this.verdicts.map((verdict) => explainVerdict(verdict.verdict));
    if (this.summary !== undefined) {
      lines.push(explainVerdict(this.summary.verdict));
    }
    return lines.join("\n");
  }
}

/** One run of a sweep: what it bound, what it produced, and how long it took. */
export class SweepRow {
  /** The swept parameters as this run bound them, by name. */
  readonly inputs: ReadonlyMap<string, SysMLValue | UnsupportedValueError>;
  /** What the run produced, by name; a calc's return is named "result". */
  readonly outputs: ReadonlyMap<string, SysMLValue | UnsupportedValueError>;
  /** The objective and assertion verdicts of an analysis case; empty for a calc. */
  readonly verdicts: readonly Verdict[];
  /** Wall time of this run, in seconds. */
  readonly seconds: number;
  /** Why this run failed; empty when it did not. */
  readonly error: string;
  /** Each application this run made of one of the case's calcs as a value. */
  readonly evaluations: readonly CaseEvaluation[];

  constructor(init: {
    inputs: ReadonlyMap<string, SysMLValue | UnsupportedValueError>;
    outputs: ReadonlyMap<string, SysMLValue | UnsupportedValueError>;
    verdicts: readonly Verdict[];
    seconds: number;
    error?: string;
    evaluations?: readonly CaseEvaluation[];
  }) {
    this.inputs = init.inputs;
    this.outputs = init.outputs;
    this.verdicts = init.verdicts;
    this.seconds = init.seconds;
    this.error = init.error ?? "";
    this.evaluations = init.evaluations ?? [];
  }

  /** The evaluations of the alternatives this run selected; one for a trade study. */
  get selected(): CaseEvaluation[] {
    return this.evaluations.filter((evaluation) => evaluation.selected);
  }

  /** Whether this run failed rather than producing outputs. */
  get failed(): boolean {
    return this.error !== "";
  }

  toString(): string {
    const inputs = [...this.inputs.entries()].map(([n, v]) => `${n}=${render(v)}`).join(", ");
    if (this.failed) {
      return `${inputs}: ${this.error}`;
    }
    const outputs = [...this.outputs.entries()].map(([n, v]) => `${n} = ${render(v)}`).join(", ");
    return `${inputs}: ${outputs}`;
  }
}

/** Every run of one sweep, in the order the runs were made. */
export class SweepTable {
  /** One row per run. */
  readonly rows: readonly SweepRow[];
  /** The swept parameters, in the order their ranges were given. */
  readonly parameters: readonly string[];
  /** Whether the rows were drawn rather than stepped through. */
  readonly sampled: boolean;
  /** The seed the rows were drawn from; 0 for a swept table. */
  readonly seed: bigint;
  /** The subjects the runs were about and the objects reachable from them. */
  readonly instances: readonly Instance[];
  readonly diagnostics: readonly ModelDiagnostic[];
  readonly standing: Standing;

  constructor(init: {
    rows: readonly SweepRow[];
    parameters: readonly string[];
    sampled?: boolean;
    seed?: bigint;
    instances?: readonly Instance[];
    diagnostics?: readonly ModelDiagnostic[];
    standing?: Standing;
  }) {
    this.rows = init.rows;
    this.parameters = init.parameters;
    this.sampled = init.sampled ?? false;
    this.seed = init.seed ?? 0n;
    this.instances = init.instances ?? [];
    this.diagnostics = init.diagnostics ?? [];
    this.standing = init.standing ?? { engine: "", strength: "", bounds: [] };
  }

  get engine(): string {
    return this.standing.engine;
  }

  /** The runs that failed. */
  get failures(): SweepRow[] {
    return this.rows.filter((row) => row.failed);
  }

  get length(): number {
    return this.rows.length;
  }

  [Symbol.iterator](): Iterator<SweepRow> {
    return this.rows[Symbol.iterator]();
  }

  row(index: number): SweepRow {
    if (index < 0 || index >= this.rows.length) {
      throw new RangeError(`the table has ${this.rows.length} rows, not ${index + 1}`);
    }
    return this.rows[index];
  }

  toString(): string {
    return this.rows.map((row) => row.toString()).join("\n");
  }
}

/** Read the case evaluations of a response, undecodable values kept as errors. */
export function evaluationsOf(evals: readonly PbCaseEvaluation[]): CaseEvaluation[] {
  return evals.map(
    (pb) =>
      new CaseEvaluation({
        functionId: pb.functionId,
        arguments: pb.arguments.map((argument) => valueOrError(argument)),
        ...(pb.error === "" && pb.result !== undefined
          ? { result: valueOrError(pb.result) }
          : {}),
        error: pb.error,
        selected: pb.selected,
        tied: pb.tied,
      }),
  );
}

/** Decode one wire `Value`, keeping a decode failure as the error it raised. */
export function valueOrError(pb: Value | undefined): SysMLValue | UnsupportedValueError {
  try {
    return decodeValue(pb);
  } catch (error) {
    return error instanceof UnsupportedValueError
      ? error
      : new UnsupportedValueError(error instanceof Error ? error.message : String(error));
  }
}

/** Read the verdicts a validation response carries, and its summary. */
export function validationOf(
  response: ValidateInstanceResponse,
  instances: readonly Instance[],
  diagnostics: readonly ModelDiagnostic[],
): Validation {
  const verifications = response.verificationVerdicts.map(decodeVerificationVerdict);
  const verdicts = response.verdicts.map((pbVerdict) =>
    verdictOf(pbVerdict, instances, diagnostics, verifications),
  );
  return new Validation({
    verdicts,
    ...(response.summary === undefined
      ? {}
      : { summary: verdictOf(response.summary, instances, diagnostics, verifications, true) }),
    instances,
    bounded: response.bounded,
    diagnostics,
    verifications,
  });
}

/** Read a RunAnalysis response, or raise its failure. */
export function analysisResultOf(
  response: RunAnalysisResponse,
  instances: readonly Instance[],
  diagnostics: readonly ModelDiagnostic[],
): AnalysisResult {
  const verifications = response.verificationVerdicts.map(decodeVerificationVerdict);
  const outputs = new Map<string, SysMLValue | UnsupportedValueError>();
  for (const output of response.outputs) {
    outputs.set(output.name, valueOrError(output.value));
  }
  const result = new AnalysisResult({
    outputs,
    verdicts: response.verdicts.map((pbVerdict) =>
      verdictOf(pbVerdict, instances, diagnostics, []),
    ),
    instances,
    diagnostics,
    verifications,
    evaluations: evaluationsOf(response.evaluations),
    standing: standingOf(response),
  });
  if (response.error !== "") {
    throw new AnalysisRunError(response.error, result, { diagnostics });
  }
  return result;
}

/** Read a RunSweep response into a SweepTable. */
export function sweepTableOf(
  response: RunSweepResponse,
  instances: readonly Instance[],
  diagnostics: readonly ModelDiagnostic[],
): SweepTable {
  const rows = response.rows.map((row) => sweepRowOf(row, instances, diagnostics));
  return new SweepTable({
    rows,
    parameters: [...response.parameters],
    sampled: response.sampled,
    seed: response.seed,
    instances,
    diagnostics,
    standing: standingOf(response),
  });
}

function sweepRowOf(
  row: PbSweepRow,
  instances: readonly Instance[],
  diagnostics: readonly ModelDiagnostic[],
): SweepRow {
  const inputs = new Map<string, SysMLValue | UnsupportedValueError>();
  const outputs = new Map<string, SysMLValue | UnsupportedValueError>();
  for (const entry of row.inputs) {
    inputs.set(entry.name, valueOrError(entry.value));
  }
  for (const entry of row.outputs) {
    outputs.set(entry.name, valueOrError(entry.value));
  }
  const verifications: VerificationVerdict[] = [];
  return new SweepRow({
    inputs,
    outputs,
    verdicts: row.verdicts.map((pbVerdict) =>
      verdictOf(pbVerdict, instances, diagnostics, verifications),
    ),
    seconds: Number(row.elapsedMicros) / 1e6,
    error: row.error,
    evaluations: evaluationsOf(row.evaluations),
  });
}
