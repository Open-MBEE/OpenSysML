// Verdicts of the verification the service performs, and the results that carry
// them: validations, calc results, analysis results and sweep tables.

import type {
  ToolDryRun as PbToolDryRun,
  CaseEvaluation as PbCaseEvaluation,
  RecordRunResponse,
  RunAnalysisResponse,
  RunDistribution as PbRunDistribution,
  RunRunsResponse,
  RunSweepResponse,
  SelfCheckResponse,
  SweepRow as PbSweepRow,
  ValidateInstanceResponse,
  Value,
  Verdict as PbVerdict,
  VerificationVerdict as PbVerificationVerdict,
  ComparedDistribution as PbComparedDistribution,
  ImportDataResponse as PbImportDataResponse,
  CompareResultsResponse as PbCompareResultsResponse,
  Instance as PbInstance,
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
import { emptyStanding, standingOf, type EngineBound, type Standing } from "./engines.js";
import {
  decodeValue,
  decodeVerdict,
  formatValue,
  type SysMLValue,
  type SysMLVerdict,
  type VerificationVerdict,
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

export type { SysMLVerdict, VerificationVerdict };
export type { WitnessAssignment } from "./values.js";

/** A value, or the error decoding it raised. */
export type ValueOrError = SysMLValue | UnsupportedValueError;

/** The mark a verification verdict's line opens with. */
function verificationMark(kind: string): string {
  if (kind === VERDICT_PASS) {
    return "✓";
  }
  return kind === VERDICT_FAIL ? "✗" : "?";
}

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
  const reached = standing.bounds
    .filter((bound) => bound.reached)
    .map((b) => `${b.name} ${b.limit.toString()} reached`);
  if (reached.length > 0) {
    line += ` (${reached.join(", ")})`;
  }
  return line;
}

/** One line saying what a verification case's body answered and why. */
export function explainVerificationVerdict(verdict: VerificationVerdict): string {
  let line = `${verificationMark(verdict.kind)} verification ${verdict.caseId} verdict: ${verdict.kind}`;
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
    instanceId: pb.instanceId,
    instanceTypeId: pb.instanceTypeId,
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
): Map<string, ValueOrError> {
  const out = new Map<string, ValueOrError>();
  for (const [name, pb] of entries) {
    try {
      out.set(name, decodeValue(pb));
    } catch (error) {
      out.set(name, asUnsupported(error));
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
  readonly value: ValueOrError | undefined;
  /** Output features a calc usage computed. */
  readonly outputs: ReadonlyMap<string, ValueOrError>;
  readonly diagnostics: readonly ModelDiagnostic[];
  readonly standing: Standing;

  constructor(init: {
    value?: ValueOrError;
    outputs: ReadonlyMap<string, ValueOrError>;
    diagnostics?: readonly ModelDiagnostic[];
    standing?: Standing;
  }) {
    this.value = init.value;
    this.outputs = init.outputs;
    this.diagnostics = init.diagnostics ?? [];
    this.standing = init.standing ?? emptyStanding();
  }

  get engine(): string {
    return this.standing.engine;
  }

  /** Strength of the evidence the answer rests on; empty when unreported. */
  get strength(): string {
    return this.standing.strength;
  }

  /** The bounds the engine ran under. */
  get bounds(): readonly EngineBound[] {
    return this.standing.bounds;
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

function render(value: ValueOrError | undefined): string {
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
  readonly arguments: readonly ValueOrError[];
  /** What it computed; undefined when `error` says why nothing was. */
  readonly result: ValueOrError | undefined;
  /** Why the evaluation computed nothing; empty when it did. */
  readonly error: string;
  /** Whether the case returned this evaluation's argument: a trade study's selection. */
  readonly selected: boolean;
  /** Whether it computed what the selected one did without being selected. */
  readonly tied: boolean;

  constructor(init: {
    functionId: string;
    arguments: readonly ValueOrError[];
    result?: ValueOrError;
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

/** The external tool call an analysis would make, previewed without running the tool. */
export interface ToolDryRun {
  /** The qualified name of the tool-executing action reached. */
  readonly action: string;
  readonly tool: string;
  readonly version: string;
  readonly manifest: string;
  readonly executable: string;
  readonly protocol: string;
  readonly args: readonly string[];
  readonly env: readonly string[];
  readonly cwd: string;
  readonly stdin: string;
  readonly stdinData: string;
  /** The input file written, or undefined when the tool reads none. */
  readonly inputFile?: { readonly name: string; readonly format: string; readonly data: string } | undefined;
  readonly outputDir: boolean;
  readonly inputs: readonly {
    readonly variable: string;
    readonly parameter: string;
    readonly value: string;
    readonly unit: string;
  }[];
  readonly outputs: readonly { readonly variable: string; readonly parameter: string }[];
  readonly replySource: string;
  /** The preview as `sysml -tool-dry-run` prints it. */
  readonly lines: readonly string[];
}

/** Read a ToolDryRun message into its preview. */
export function toolDryRunOf(pb: PbToolDryRun): ToolDryRun {
  return {
    action: pb.action,
    tool: pb.tool,
    version: pb.version,
    manifest: pb.manifest,
    executable: pb.executable,
    protocol: pb.protocol,
    args: [...pb.args],
    env: [...pb.env],
    cwd: pb.cwd,
    stdin: pb.stdin,
    stdinData: pb.stdinData,
    inputFile: pb.inputFile
      ? { name: pb.inputFile.name, format: pb.inputFile.format, data: pb.inputFile.data }
      : undefined,
    outputDir: pb.outputDir,
    inputs: pb.inputs.map((i) => ({ variable: i.variable, parameter: i.parameter, value: i.value, unit: i.unit })),
    outputs: pb.outputs.map((o) => ({ variable: o.variable, parameter: o.parameter })),
    replySource: pb.replySource,
    lines: [...pb.lines],
  };
}

/** What an analysis case computed and decided. */
export class AnalysisResult {
  /** Output features the case computed, by name. */
  readonly outputs: ReadonlyMap<string, ValueOrError>;
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
  /** Under `toolDryRun`, the tool call the case would make; the outputs are then empty. */
  readonly toolDryRun?: ToolDryRun | undefined;

  constructor(init: {
    outputs: ReadonlyMap<string, ValueOrError>;
    verdicts: readonly Verdict[];
    instances?: readonly Instance[];
    diagnostics?: readonly ModelDiagnostic[];
    verifications?: readonly VerificationVerdict[];
    evaluations?: readonly CaseEvaluation[];
    standing?: Standing;
    toolDryRun?: ToolDryRun | undefined;
  }) {
    this.outputs = init.outputs;
    this.verdicts = init.verdicts;
    this.instances = init.instances ?? [];
    this.diagnostics = init.diagnostics ?? [];
    this.verifications = init.verifications ?? [];
    this.evaluations = init.evaluations ?? [];
    this.standing = init.standing ?? emptyStanding();
    this.toolDryRun = init.toolDryRun;
  }

  get engine(): string {
    return this.standing.engine;
  }

  /** Strength of the evidence the answer rests on; empty when unreported. */
  get strength(): string {
    return this.standing.strength;
  }

  /** The bounds the engine ran under. */
  get bounds(): readonly EngineBound[] {
    return this.standing.bounds;
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
    const lines = [
      ...[...this.outputs.entries()].map(([name, val]) => `${name} = ${render(val)}`),
      ...this.verdicts.map((verdict) => explainVerdict(verdict.verdict)),
      ...this.verifications.map(explainVerificationVerdict),
      ...this.evaluations.map((evaluation) => evaluation.explain()),
    ];
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
    return this.summary?.verdict.standing ?? emptyStanding();
  }

  get engine(): string {
    return this.standing.engine;
  }

  /** Strength of the evidence the answer rests on; empty when unreported. */
  get strength(): string {
    return this.standing.strength;
  }

  /** The bounds the engine ran under. */
  get bounds(): readonly EngineBound[] {
    return this.standing.bounds;
  }

  /** Whether the object is shown valid: at least one assertion, every one holding, all reached. */
  get valid(): boolean {
    return this.summary?.verdict.kind === "holds" && !this.bounded;
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
  readonly inputs: ReadonlyMap<string, ValueOrError>;
  /** What the run produced, by name; a calc's return is named "result". */
  readonly outputs: ReadonlyMap<string, ValueOrError>;
  /** The objective and assertion verdicts of an analysis case; empty for a calc. */
  readonly verdicts: readonly Verdict[];
  /** Wall time of this run, in seconds. */
  readonly seconds: number;
  /** Why this run failed; empty when it did not. */
  readonly error: string;
  /** Each application this run made of one of the case's calcs as a value. */
  readonly evaluations: readonly CaseEvaluation[];

  constructor(init: {
    inputs: ReadonlyMap<string, ValueOrError>;
    outputs: ReadonlyMap<string, ValueOrError>;
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
    this.standing = init.standing ?? emptyStanding();
  }

  get engine(): string {
    return this.standing.engine;
  }

  /** Strength of the evidence the answer rests on; empty when unreported. */
  get strength(): string {
    return this.standing.strength;
  }

  /** The bounds the engine ran under. */
  get bounds(): readonly EngineBound[] {
    return this.standing.bounds;
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
export function valueOrError(pb: Value | undefined): ValueOrError {
  try {
    return decodeValue(pb);
  } catch (error) {
    return asUnsupported(error);
  }
}

/** A decode failure as the UnsupportedValueError it is, or one carrying its message. */
function asUnsupported(error: unknown): UnsupportedValueError {
  if (error instanceof UnsupportedValueError) {
    return error;
  }
  return new UnsupportedValueError(error instanceof Error ? error.message : String(error));
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
  const outputs = new Map<string, ValueOrError>();
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
    toolDryRun: response.toolDryRun ? toolDryRunOf(response.toolDryRun) : undefined,
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

export function sweepRowOf(
  row: PbSweepRow,
  instances: readonly Instance[],
  diagnostics: readonly ModelDiagnostic[],
): SweepRow {
  const inputs = new Map<string, ValueOrError>();
  const outputs = new Map<string, ValueOrError>();
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

/** The value a percentile of the runs fell at or below. */
export interface RunPercentile {
  readonly percentile: number;
  readonly value: SysMLValue;
}

/** How many runs observed a value in `[low, high)`. */
export interface RunHistogramBin {
  readonly low: SysMLValue;
  readonly high: SysMLValue;
  readonly count: bigint;
}

/** How many runs observed one distinct value. */
export interface RunCountedValue {
  readonly value: SysMLValue;
  readonly count: bigint;
}

/** One observable over the completed runs: the extremes, mean, deviation,
 * percentiles and histogram of a numeric one, or each distinct value counted. */
export interface RunDistribution {
  readonly name: string;
  readonly unit: string;
  readonly count: bigint;
  readonly numeric: boolean;
  readonly min: SysMLValue | undefined;
  readonly max: SysMLValue | undefined;
  readonly mean: SysMLValue | undefined;
  readonly deviation: number;
  readonly percentiles: readonly RunPercentile[];
  readonly histogram: readonly RunHistogramBin[];
  readonly values: readonly RunCountedValue[];
}

/** What `runRuns` answers: one row per run, each observable's distribution
 * and, for a Monte Carlo case, its conclusion over the sample. */
export class RunsResult {
  readonly rows: readonly SweepRow[];
  readonly observables: readonly string[];
  readonly distributions: readonly RunDistribution[];
  readonly lines: readonly string[];
  readonly outputs: ReadonlyMap<string, ValueOrError>;
  readonly verdicts: readonly Verdict[];
  readonly unconcluded: string;
  readonly instances: readonly Instance[];
  readonly standing: Standing;
  readonly seed: bigint | undefined;
  readonly diagnostics: readonly ModelDiagnostic[];

  constructor(fields: {
    rows: readonly SweepRow[];
    observables: readonly string[];
    distributions: readonly RunDistribution[];
    lines: readonly string[];
    outputs: ReadonlyMap<string, ValueOrError>;
    verdicts: readonly Verdict[];
    unconcluded: string;
    instances: readonly Instance[];
    standing: Standing;
    seed: bigint | undefined;
    diagnostics: readonly ModelDiagnostic[];
  }) {
    this.rows = fields.rows;
    this.observables = fields.observables;
    this.distributions = fields.distributions;
    this.lines = fields.lines;
    this.outputs = fields.outputs;
    this.verdicts = fields.verdicts;
    this.unconcluded = fields.unconcluded;
    this.instances = fields.instances;
    this.standing = fields.standing;
    this.seed = fields.seed;
    this.diagnostics = fields.diagnostics;
  }

  get length(): number {
    return this.rows.length;
  }

  toString(): string {
    return this.lines.join("\n");
  }
}

function runDistributionOf(pb: PbRunDistribution): RunDistribution {
  return {
    name: pb.name,
    unit: pb.unit,
    count: pb.count,
    numeric: pb.numeric,
    min: pb.min === undefined ? undefined : decodeValue(pb.min),
    max: pb.max === undefined ? undefined : decodeValue(pb.max),
    mean: pb.mean === undefined ? undefined : decodeValue(pb.mean),
    deviation: pb.deviation,
    percentiles: pb.percentiles.map((p) => ({ percentile: p.percentile, value: decodeValue(p.value) })),
    histogram: pb.histogram.map((b) => ({ low: decodeValue(b.low), high: decodeValue(b.high), count: b.count })),
    values: pb.values.map((v) => ({ value: decodeValue(v.value), count: v.count })),
  };
}

export function runsResultOf(
  response: RunRunsResponse,
  instances: readonly Instance[],
  diagnostics: readonly ModelDiagnostic[],
): RunsResult {
  const outputs = new Map<string, ValueOrError>();
  for (const output of response.outputs) {
    outputs.set(output.name, valueOrError(output.value));
  }
  return new RunsResult({
    rows: response.rows.map((row) => sweepRowOf(row, instances, diagnostics)),
    observables: [...response.observables],
    distributions: response.distributions.map(runDistributionOf),
    lines: [...response.lines],
    outputs,
    verdicts: response.verdicts.map((pbVerdict) => verdictOf(pbVerdict, instances, diagnostics, [])),
    unconcluded: response.unconcluded,
    instances,
    standing: standingOf(response),
    seed: response.seed,
    diagnostics,
  });
}

/** One thing the reflective self-check reports; `line` is it as the CLI prints it. */
export interface SelfCheckFinding {
  readonly status: string;
  readonly subject: string;
  readonly location: string;
  readonly constraint: string;
  readonly element: string;
  readonly message: string;
  readonly line: string;
}

/** What `selfCheck` answers: every finding and the counts over them. */
export class SelfCheckResult {
  readonly status: string;
  readonly findings: readonly SelfCheckFinding[];
  readonly elements: bigint;
  readonly checks: bigint;
  readonly violations: bigint;
  readonly unevaluated: bigint;
  readonly evaluationErrors: bigint;
  readonly applications: ReadonlyMap<string, bigint>;
  readonly summary: string;
  readonly diagnostics: readonly ModelDiagnostic[];

  constructor(response: SelfCheckResponse, diagnostics: readonly ModelDiagnostic[]) {
    this.status = response.status;
    this.findings = response.findings.map((f) => ({
      status: f.status,
      subject: f.subject,
      location: f.location,
      constraint: f.constraint,
      element: f.element,
      message: f.message,
      line: f.line,
    }));
    this.elements = response.elements;
    this.checks = response.checks;
    this.violations = response.violations;
    this.unevaluated = response.unevaluated;
    this.evaluationErrors = response.evaluationErrors;
    this.applications = new Map(response.applications.map((a) => [a.constraint, a.count]));
    this.summary = response.summary;
    this.diagnostics = diagnostics;
  }

  /** Whether the check found no violation and no evaluation error. */
  get holds(): boolean {
    return this.status === "holds";
  }

  toString(): string {
    return [...this.findings.map((f) => f.line), this.summary].join("\n");
  }
}

/** One document the records were written into. */
export interface RecordedDocument {
  readonly name: string;
  readonly content: string;
}

/** What `recordRun` answers: the runs' own report, what the model gained and
 * the notation the records were written into. */
export class Recording {
  readonly run: AnalysisResult | undefined;
  readonly sweep: SweepTable | undefined;
  readonly sample: RunsResult | undefined;
  readonly package: string;
  readonly definition: string;
  readonly records: readonly string[];
  readonly source: string;
  readonly content: string;
  readonly documents: readonly RecordedDocument[];
  readonly skipped: bigint;
  readonly reasons: readonly string[];
  readonly modelHash: string;
  readonly diagnostics: readonly ModelDiagnostic[];

  constructor(fields: {
    run: AnalysisResult | undefined;
    sweep: SweepTable | undefined;
    sample: RunsResult | undefined;
    response: RecordRunResponse;
    diagnostics: readonly ModelDiagnostic[];
  }) {
    const { response } = fields;
    this.run = fields.run;
    this.sweep = fields.sweep;
    this.sample = fields.sample;
    this.package = response.package;
    this.definition = response.definition;
    this.records = [...response.records];
    this.source = response.source;
    this.content = response.content;
    this.documents = response.documents.map((d) => ({ name: d.name, content: d.content }));
    this.skipped = response.skipped;
    this.reasons = [...response.reasons];
    this.modelHash = response.modelHash;
    this.diagnostics = fields.diagnostics;
  }
}

/** One assignment an import made. */
export class ImportedValue {
  constructor(
    readonly element: string,
    readonly feature: string,
    readonly value: string,
    readonly redefines: string,
    readonly where: string,
    readonly description: string,
  ) {}
}

/** What one data file set. */
export class ImportedFile {
  constructor(
    readonly name: string,
    readonly values: bigint,
    readonly elements: bigint,
    readonly assignments: readonly ImportedValue[],
  ) {}
}

/** What `importData` answers: what each file set and the model it made. */
export class ImportResult {
  constructor(
    readonly files: readonly ImportedFile[],
    readonly content: string,
    readonly documents: readonly { name: string; content: string }[],
    readonly modelHash: string,
    readonly diagnostics: readonly ModelDiagnostic[],
  ) {}
}

/** A distribution by its statistics; a pooled one knows its count and mean alone. */
export class ComparedDistribution {
  constructor(
    readonly count: bigint,
    readonly min: number,
    readonly mean: number,
    readonly p50: number,
    readonly p90: number,
    readonly max: number,
    readonly deviation: number,
  ) {}
}

/** The relative difference of each statistic as a signed percentage, "" where not compared. */
export interface StatisticDifference {
  readonly min: string;
  readonly mean: string;
  readonly p50: string;
  readonly p90: string;
  readonly max: string;
}

/** One statistic a Simulation::MonteCarlo case declares, the tool's beside the runs'. */
export interface CaseStatistic {
  readonly statistic: string;
  readonly of: string;
  readonly tool: string;
  readonly run: string;
  readonly difference: string;
}

/** The statistics a Simulation::MonteCarlo case declares of an observable. */
export interface CaseStatistics {
  readonly analysis: string;
  readonly rows: readonly CaseStatistic[];
  readonly notes: readonly string[];
}

/** One observable: the tool's distribution beside the runs', or why they are not compared. */
export class ObservableComparison {
  constructor(
    readonly stored: string,
    readonly feature: string,
    readonly unit: string,
    readonly tool: ComparedDistribution | undefined,
    readonly pooled: boolean,
    readonly unstored: boolean,
    readonly run: ComparedDistribution | undefined,
    readonly produced: bigint,
    readonly compared: boolean,
    readonly difference: StatisticDifference | undefined,
    readonly declared: CaseStatistics | undefined,
    readonly notes: readonly string[],
  ) {}
}

/** One configuration's runs beside the tool's results. */
export class ConfigurationComparison {
  constructor(
    readonly id: string,
    readonly name: string,
    readonly behavior: string,
    readonly target: string,
    readonly resultLocation: string,
    readonly status: string,
    readonly unresolved: string,
    readonly storedRuns: bigint,
    readonly storedSnapshots: bigint,
    readonly runs: bigint,
    readonly completed: bigint,
    readonly draws: string,
    readonly seed: bigint | undefined,
    readonly clockStep: number,
    readonly rows: readonly SweepRow[],
    readonly observables: readonly ObservableComparison[],
    readonly errors: readonly string[],
    readonly notes: readonly string[],
    readonly lines: readonly string[],
  ) {}

  /** Whether every run completed and the observables were compared. */
  get holds(): boolean {
    return this.status === "holds";
  }
}

/** What `compareResults` answers: one configuration after another. */
export class Comparison {
  constructor(
    readonly source: string,
    readonly configurations: readonly ConfigurationComparison[],
    readonly standing: Standing,
    readonly diagnostics: readonly ModelDiagnostic[],
  ) {}

  /** Whether every configuration ran and compared. */
  get holds(): boolean {
    return this.configurations.every((c) => c.holds);
  }
}

function comparedDistributionOf(d: PbComparedDistribution | undefined): ComparedDistribution | undefined {
  return d ? new ComparedDistribution(d.count, d.min, d.mean, d.p50, d.p90, d.max, d.deviation) : undefined;
}

/** Decodes an ImportDataResponse. */
export function importResultOf(response: PbImportDataResponse, diagnostics: readonly ModelDiagnostic[]): ImportResult {
  return new ImportResult(
    response.files.map(
      (f) =>
        new ImportedFile(
          f.name,
          f.values,
          f.elements,
          f.assignments.map((a) => new ImportedValue(a.element, a.feature, a.value, a.redefines, a.where, a.description)),
        ),
    ),
    response.content,
    response.documents.map((d) => ({ name: d.name, content: d.content })),
    response.modelHash,
    diagnostics,
  );
}

/** Decodes a CompareResultsResponse; `instancesOf` decodes a configuration's instances. */
export function comparisonOf(
  response: PbCompareResultsResponse,
  instancesOf: (instances: readonly PbInstance[]) => readonly Instance[],
  diagnostics: readonly ModelDiagnostic[],
): Comparison {
  const configurations = response.configurations.map((c) => {
    const instances = instancesOf(c.instances);
    return new ConfigurationComparison(
      c.id,
      c.name,
      c.behavior,
      c.target,
      c.resultLocation,
      c.status,
      c.unresolved,
      c.storedRuns,
      c.storedSnapshots,
      c.runs,
      c.completed,
      c.draws,
      c.seed,
      c.clockStep,
      c.rows.map((row) => sweepRowOf(row, instances, diagnostics)),
      c.observables.map(
        (o) =>
          new ObservableComparison(
            o.stored,
            o.feature,
            o.unit,
            comparedDistributionOf(o.tool),
            o.pooled,
            o.unstored,
            comparedDistributionOf(o.run),
            o.produced,
            o.compared,
            o.difference,
            o.declared,
            o.notes,
          ),
      ),
      c.errors,
      c.notes,
      c.lines,
    );
  });
  return new Comparison(response.source, configurations, standingOf(response), diagnostics);
}
