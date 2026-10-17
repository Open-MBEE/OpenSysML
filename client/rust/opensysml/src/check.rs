//! The answer to `CheckBehavior`: a behavior searched over every schedule.

use std::collections::BTreeMap;

use crate::results::Standing;
use crate::wire;

/// One schedule replayed: pass `schedule` as a run's schedule to follow it.
#[derive(Clone, Debug, Default, PartialEq)]
pub struct CheckWitness {
    /// The replay schedule policy.
    pub schedule: String,
    /// The draws the schedule took.
    pub draws: Vec<String>,
    /// The choices it made at each choice point.
    pub choices: Vec<String>,
    /// The inputs the symbolic engine chose, as `name = value`.
    pub inputs: Vec<String>,
    /// The witness file's name when one was asked for, else empty.
    pub file_name: String,
    /// That file's content.
    pub file_content: String,
}

impl CheckWitness {
    fn from_wire(wire: Option<wire::CheckWitness>) -> Option<Self> {
        wire.map(|w| Self {
            schedule: w.schedule,
            draws: w.draws,
            choices: w.choices,
            inputs: w.inputs,
            file_name: w.file_name,
            file_content: w.file_content,
        })
    }
}

/// One violation witnessed: a property false at a state, a deadlock or a failure.
#[derive(Clone, Debug, Default, PartialEq)]
pub struct CheckViolation {
    /// `property`, `deadlock` or `failure`.
    pub kind: String,
    /// The property false at the state, for a property violation.
    pub property: String,
    /// The failure's message, for a failure.
    pub error: String,
    /// Moves from the start to the violating state.
    pub depth: i32,
    /// The schedule's probability under a weighted scheduler, else 0.
    pub probability: f64,
    /// The schedule reaching it.
    pub witness: Option<CheckWitness>,
    /// The violation as the report prints it.
    pub text: String,
}

/// One final value a feature reaches and the schedule reaching it.
#[derive(Clone, Debug, Default, PartialEq)]
pub struct CheckDivergentValue {
    /// The value as the report prints it.
    pub value: String,
    /// The schedule reaching it.
    pub witness: Option<CheckWitness>,
}

/// A feature whose final value the schedule decides.
#[derive(Clone, Debug, Default, PartialEq)]
pub struct CheckDivergence {
    /// The feature as asked for.
    pub feature: String,
    /// Each final value with a schedule reaching it.
    pub values: Vec<CheckDivergentValue>,
}

/// One final outcome of an exhaustive search.
#[derive(Clone, Debug, Default, PartialEq)]
pub struct CheckFinal {
    /// The outcome as the report prints it.
    pub outcome: String,
    /// Each tracked feature's final value.
    pub values: BTreeMap<String, String>,
    /// A schedule reaching it.
    pub witness: Option<CheckWitness>,
}

/// The explicit-state search's statistics.
#[derive(Clone, Debug, Default, PartialEq)]
pub struct CheckSearch {
    /// The states visited.
    pub states: i32,
    /// The moves taken.
    pub moves: i32,
    /// The deepest schedule.
    pub max_depth: i32,
    /// The bounds the search stopped at: `depth`, `states`, `timeout`.
    pub bounds_hit: Vec<String>,
    /// The clock horizon, meaningful when `horizon_set`.
    pub horizon: f64,
    /// Whether a horizon applied.
    pub horizon_set: bool,
    /// Whether the probability mass was bounded.
    pub mass_bounded: bool,
    /// What the search notes about its scope.
    pub notes: Vec<String>,
    /// The scope the search ran over.
    pub scope: Vec<String>,
    /// The search's status as the report prints it.
    pub status: String,
}

/// The options of [`crate::Model::check_action`] and [`crate::Model::check_state_machine`].
#[derive(Clone, Debug, Default, PartialEq)]
pub struct CheckOptions {
    /// Constraints or requirements evaluated at every stable state.
    pub properties: Vec<String>,
    /// Features whose final value the schedule may decide; `this.<name>` names the performer's.
    pub diverge: Vec<String>,
    /// Inputs the symbolic engine ranges over.
    pub free_inputs: Vec<String>,
    /// Constraints or requirements the symbolic engine assumes.
    pub assume: Vec<String>,
    /// Carry each witness's file in the answer.
    pub witness: bool,
    /// Bounds on moves per schedule, states visited and loop unrollings; 0 is the default.
    pub depth: i32,
    /// The bound on states visited; 0 is the default.
    pub states: i32,
    /// The bound on loop unrollings; 0 is the default.
    pub unroll: i32,
    /// Seconds the search may take; 0 is none.
    pub timeout: f64,
    /// The engine asked: `check` (the default), `smt` or one `list_engines` names.
    pub engine: Option<String>,
    /// The part or usage whose object performs the behavior.
    pub performer: Option<String>,
}

/// What `CheckBehavior` answered: the verdict over every schedule, each finding witnessed.
#[derive(Clone, Debug, Default)]
pub struct CheckResult {
    /// `holds`, `fails` or `undecided`.
    pub verdict: String,
    /// The report as `sysml -check-property` prints it.
    pub lines: Vec<String>,
    /// The explicit-state search's statistics; `None` for a symbolic answer.
    pub search: Option<CheckSearch>,
    /// Each property, deadlock or failure witnessed.
    pub violations: Vec<CheckViolation>,
    /// Each feature whose final value the schedule decided.
    pub divergences: Vec<CheckDivergence>,
    /// Each final outcome of an exhaustive search.
    pub finals: Vec<CheckFinal>,
    /// The symbolic engine's claim, the inputs it ranged over and what it assumed.
    pub claim: String,
    /// The inputs the symbolic engine ranged over.
    pub free_inputs: Vec<String>,
    /// What the symbolic engine assumed.
    pub assumptions: Vec<String>,
    /// The symbolic engine's witness.
    pub witness: Option<CheckWitness>,
    /// The symbolic engine's counter-witness.
    pub contrast: Option<CheckWitness>,
    /// The engine, its strength and the bounds it ran under.
    pub standing: Standing,
    /// Why the verdict is undecided, when it is.
    pub reason: String,
    wire: wire::CheckBehaviorResponse,
}

impl CheckResult {
    pub(crate) fn from_wire(response: wire::CheckBehaviorResponse) -> Self {
        let wire = response.clone();
        Self {
            verdict: response.verdict,
            lines: response.lines,
            search: response.search.map(|s| CheckSearch {
                states: s.states,
                moves: s.moves,
                max_depth: s.max_depth,
                bounds_hit: s.bounds_hit,
                horizon: s.horizon,
                horizon_set: s.horizon_set,
                mass_bounded: s.mass_bounded,
                notes: s.notes,
                scope: s.scope,
                status: s.status,
            }),
            violations: response
                .violations
                .into_iter()
                .map(|v| CheckViolation {
                    kind: v.kind,
                    property: v.property,
                    error: v.error,
                    depth: v.depth,
                    probability: v.probability,
                    witness: CheckWitness::from_wire(v.witness),
                    text: v.text,
                })
                .collect(),
            divergences: response
                .divergences
                .into_iter()
                .map(|d| CheckDivergence {
                    feature: d.feature,
                    values: d
                        .values
                        .into_iter()
                        .map(|v| CheckDivergentValue {
                            value: v.value,
                            witness: CheckWitness::from_wire(v.witness),
                        })
                        .collect(),
                })
                .collect(),
            finals: response
                .finals
                .into_iter()
                .map(|f| CheckFinal {
                    outcome: f.outcome,
                    values: f.values.into_iter().collect(),
                    witness: CheckWitness::from_wire(f.witness),
                })
                .collect(),
            claim: response.claim,
            free_inputs: response.free_inputs,
            assumptions: response.assumptions,
            witness: CheckWitness::from_wire(response.witness),
            contrast: CheckWitness::from_wire(response.contrast),
            standing: Standing::of(&response.engine, &response.strength, &response.bounds),
            reason: response.reason,
            wire,
        }
    }

    /// Whether every schedule kept every property.
    pub fn holds(&self) -> bool {
        self.verdict == "holds"
    }

    /// The protocol message this was decoded from.
    pub fn wire(&self) -> &wire::CheckBehaviorResponse {
        &self.wire
    }
}
