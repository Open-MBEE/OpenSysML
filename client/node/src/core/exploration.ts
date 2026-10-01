// What exploring a behavior under the `explore` scheduling policy found: every
// distinct outcome the runs reached, rather than one run's result.

import { ExecutionError, type ModelDiagnostic, type UnsupportedValueError } from "./errors.js";
import type { SysMLValue } from "./values.js";
import { formatValue } from "./values.js";

/** A value map an outcome reports: an action's outputs, a machine's final context. */
export type OutcomeValues = ReadonlyMap<string, SysMLValue | UnsupportedValueError>;

/** One distinct outcome an exploration reached. */
export class Outcome {
  /** The values the behavior holds at the end, by name; empty for a failed run. */
  readonly outputs: OutcomeValues;
  /** The state a state machine rests in; empty for an action. */
  readonly finalState: string;
  /** The states a state machine entered, in order. */
  readonly statesVisited: readonly string[];
  /** Why the run failed; empty for a run that completed. */
  readonly error: string;
  /** How many of the explored orders reached this outcome. */
  readonly linearizations: number;
  /** Share of the explored orders' likelihood reaching this outcome; a lower bound while incomplete. */
  readonly probability: number;
  /** The choices one run reaching it made, in run order; empty with no choice point. */
  readonly witness: readonly string[];
  /** What the witness run reported, its choice points among them. */
  readonly diagnostics: readonly ModelDiagnostic[];

  constructor(init: {
    outputs: OutcomeValues;
    finalState: string;
    statesVisited: readonly string[];
    error: string;
    linearizations: number;
    probability: number;
    witness: readonly string[];
    diagnostics: readonly ModelDiagnostic[];
  }) {
    this.outputs = init.outputs;
    this.finalState = init.finalState;
    this.statesVisited = init.statesVisited;
    this.error = init.error;
    this.linearizations = init.linearizations;
    this.probability = init.probability;
    this.witness = init.witness;
    this.diagnostics = init.diagnostics;
  }

  /** Whether the runs reaching this outcome failed rather than completed. */
  get failed(): boolean {
    return this.error !== "";
  }

  /** Raise the run's failure as an {@link ExecutionError}, if it failed. */
  raiseForError(): this {
    if (this.error !== "") {
      throw new ExecutionError(this.error, "unspecified", this.diagnostics);
    }
    return this;
  }

  toString(): string {
    if (this.error !== "") {
      return `error: ${this.error}`;
    }
    const parts: string[] = [];
    if (this.finalState !== "") {
      parts.push(`finalState ${this.finalState}`);
    }
    if (this.statesVisited.length > 0) {
      parts.push(`visits ${this.statesVisited.join(", ")}`);
    }
    for (const [name, value] of [...this.outputs.entries()].sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0))) {
      parts.push(`${name} = ${typeof value === "object" && "kind" in value ? formatValue(value) : String(value)}`);
    }
    return parts.length > 0 ? parts.join("; ") : "no outputs";
  }
}

/** Every distinct outcome a behavior reached under `explore`, and how the exploration ended. */
export class Exploration {
  /** The distinct outcomes reached, in the service's canonical order. */
  readonly outcomes: readonly Outcome[];
  /** Whether every linearization within the budget was run. */
  readonly complete: boolean;
  /** How many runs were made. */
  readonly runs: number;
  /** The budgets the exploration ran into; empty when complete. */
  readonly budgetsHit: readonly string[];
  /** The most runs the exploration would make. */
  readonly runsBudget: number;
  /** The most choice points one run would resolve. */
  readonly depthBudget: number;
  /** Whether the outcomes' probabilities are lower bounds. */
  readonly probabilitiesLowerBound: boolean;

  constructor(init: {
    outcomes: readonly Outcome[];
    complete: boolean;
    runs: number;
    budgetsHit: readonly string[];
    runsBudget: number;
    depthBudget: number;
    probabilitiesLowerBound?: boolean;
  }) {
    this.outcomes = init.outcomes;
    this.complete = init.complete;
    this.runs = init.runs;
    this.budgetsHit = init.budgetsHit;
    this.runsBudget = init.runsBudget;
    this.depthBudget = init.depthBudget;
    this.probabilitiesLowerBound = init.probabilitiesLowerBound ?? false;
  }

  get length(): number {
    return this.outcomes.length;
  }

  [Symbol.iterator](): Iterator<Outcome> {
    return this.outcomes[Symbol.iterator]();
  }

  /** How the exploration ended, as `sysml -schedule explore` prints it. */
  get status(): string {
    if (this.complete) {
      return `complete (${this.runs} runs)`;
    }
    const named = this.budgetsHit
      .map((budget) => `${budget} budget ${budget === "depth" ? this.depthBudget : this.runsBudget}`)
      .join(" and ");
    return `incomplete: ${named} hit after ${this.runs} runs; probabilities are lower bounds`;
  }

  /** Raise an {@link ExecutionError} unless every linearization was run. */
  raiseForIncomplete(): this {
    if (!this.complete) {
      throw new ExecutionError(this.status);
    }
    return this;
  }

  toString(): string {
    const lines = this.outcomes.map(
      (outcome) =>
        `${outcome.toString()} (${outcome.linearizations} linearizations; ` +
        `${outcome.witness.join("; ") || "no choice points"})`,
    );
    lines.push(this.status);
    return lines.join("\n");
  }
}
