import type {
  CheckBehaviorResponse,
  CheckDivergence as PbCheckDivergence,
  CheckFinal as PbCheckFinal,
  CheckSearch as PbCheckSearch,
  CheckViolation as PbCheckViolation,
  CheckWitness as PbCheckWitness,
} from "../generated/sysml_pb.js";
import { type Standing, standingOf } from "./engines.js";

/** One schedule replayed: pass `schedule` as a run's schedule to follow it. */
export interface CheckWitness {
  schedule: string;
  draws: readonly string[];
  choices: readonly string[];
  inputs: readonly string[];
  /** The witness file's name and content when one was asked for, else empty. */
  fileName: string;
  fileContent: string;
}

/** One violation witnessed: a property false at a state, a deadlock or a failure. */
export interface CheckViolation {
  kind: string;
  property: string;
  error: string;
  depth: number;
  probability: number;
  witness: CheckWitness | undefined;
  text: string;
}

/** A feature whose final value the schedule decides. */
export interface CheckDivergence {
  feature: string;
  values: readonly { value: string; witness: CheckWitness | undefined }[];
}

/** One final outcome of an exhaustive search. */
export interface CheckFinal {
  outcome: string;
  values: ReadonlyMap<string, string>;
  witness: CheckWitness | undefined;
}

/** The explicit-state search's statistics. */
export interface CheckSearch {
  states: number;
  moves: number;
  maxDepth: number;
  boundsHit: readonly string[];
  horizon: number;
  horizonSet: boolean;
  massBounded: boolean;
  notes: readonly string[];
  scope: readonly string[];
  status: string;
}

/** What `CheckBehavior` answered: the verdict over every schedule, each finding witnessed. */
export interface CheckResult {
  /** `holds`, `fails` or `undecided`. */
  verdict: string;
  /** The report as `sysml -check-property` prints it. */
  lines: readonly string[];
  search: CheckSearch | undefined;
  violations: readonly CheckViolation[];
  divergences: readonly CheckDivergence[];
  finals: readonly CheckFinal[];
  claim: string;
  freeInputs: readonly string[];
  assumptions: readonly string[];
  witness: CheckWitness | undefined;
  contrast: CheckWitness | undefined;
  standing: Standing;
  reason: string;
}

/** The options of `Model.checkAction` and `Model.checkStateMachine`. */
export interface CheckOptions {
  /** Constraints or requirements evaluated at every stable state. */
  properties?: readonly string[];
  /** Features whose final value the schedule may decide. */
  diverge?: readonly string[];
  /** Inputs the symbolic engine ranges over. */
  freeInputs?: readonly string[];
  /** Constraints or requirements the symbolic engine assumes. */
  assume?: readonly string[];
  /** Carry each witness's file in the answer. */
  witness?: boolean;
  /** Bounds on moves per schedule, states visited, loop unrollings and seconds; 0 is the default. */
  depth?: number;
  states?: number;
  unroll?: number;
  timeout?: number;
  /** The engine asked: `check` (the default), `smt` or one `listEngines` names. */
  engine?: string;
  /** The part or usage whose object performs the behavior. */
  performer?: string;
}

function witnessOf(pb: PbCheckWitness | undefined): CheckWitness | undefined {
  if (pb === undefined) return undefined;
  return {
    schedule: pb.schedule,
    draws: pb.draws,
    choices: pb.choices,
    inputs: pb.inputs,
    fileName: pb.fileName,
    fileContent: pb.fileContent,
  };
}

function violationOf(pb: PbCheckViolation): CheckViolation {
  return {
    kind: pb.kind,
    property: pb.property,
    error: pb.error,
    depth: pb.depth,
    probability: pb.probability,
    witness: witnessOf(pb.witness),
    text: pb.text,
  };
}

function divergenceOf(pb: PbCheckDivergence): CheckDivergence {
  return {
    feature: pb.feature,
    values: pb.values.map((v) => ({ value: v.value, witness: witnessOf(v.witness) })),
  };
}

function finalOf(pb: PbCheckFinal): CheckFinal {
  return {
    outcome: pb.outcome,
    values: new Map(Object.entries(pb.values)),
    witness: witnessOf(pb.witness),
  };
}

function searchOf(pb: PbCheckSearch | undefined): CheckSearch | undefined {
  if (pb === undefined) return undefined;
  return {
    states: pb.states,
    moves: pb.moves,
    maxDepth: pb.maxDepth,
    boundsHit: pb.boundsHit,
    horizon: pb.horizon,
    horizonSet: pb.horizonSet,
    massBounded: pb.massBounded,
    notes: pb.notes,
    scope: pb.scope,
    status: pb.status,
  };
}

export function checkResultOf(response: CheckBehaviorResponse): CheckResult {
  return {
    verdict: response.verdict,
    lines: response.lines,
    search: searchOf(response.search),
    violations: response.violations.map(violationOf),
    divergences: response.divergences.map(divergenceOf),
    finals: response.finals.map(finalOf),
    claim: response.claim,
    freeInputs: response.freeInputs,
    assumptions: response.assumptions,
    witness: witnessOf(response.witness),
    contrast: witnessOf(response.contrast),
    standing: standingOf(response),
    reason: response.reason,
  };
}
