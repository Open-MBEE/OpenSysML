//! Typed answers of the verification, calculation, analysis, sweep, execution and engine RPCs.

use std::collections::BTreeMap;
use std::fmt;
use std::sync::Arc;
use std::time::Duration;

use crate::capabilities::{upgrade_remedy, CAPABILITY_FEATURE_VALUES};
use crate::domain::{value_from_wire, Capabilities, Diagnostic, Instance, Value};
use crate::error::Error;
use crate::wire;
use crate::wire::FailureReason;

/// The engine selection every service reads as the strongest covering engine.
pub const ENGINE_AUTO: &str = "auto";
/// The engine selection composing every covering engine.
pub const ENGINE_ALL: &str = "all";
/// The engine selection exploring every order of a behavior's choice points.
pub const ENGINE_EXPLORE: &str = "explore";

/// No engine covered the question.
pub const STRENGTH_NOT_COVERED: &str = "not covered";
/// The answer was observed on one run.
pub const STRENGTH_OBSERVED: &str = "observed";
/// The answer is witnessed by a concrete assignment.
pub const STRENGTH_WITNESSED: &str = "witnessed";
/// The answer holds within the bounds the engine ran under.
pub const STRENGTH_BOUNDED: &str = "bounded";
/// The answer is proved.
pub const STRENGTH_PROVED: &str = "proved";

/// A verdict about a constraint.
pub const KIND_CONSTRAINT: &str = "constraint";
/// A verdict about a requirement.
pub const KIND_REQUIREMENT: &str = "requirement";
/// A verdict about a satisfaction assertion.
pub const KIND_SATISFY: &str = "satisfy";
/// A verdict about an analysis case's objective.
pub const KIND_OBJECTIVE: &str = "objective";
/// A verdict about an analysis case's asserted constraint.
pub const KIND_ASSERTION: &str = "assertion";
/// A validated object's own verdict.
pub const KIND_OBJECT: &str = "object";

/// A verification case body that passed.
pub const VERDICT_PASS: &str = "pass";
/// A verification case body that failed.
pub const VERDICT_FAIL: &str = "fail";
/// A verification case body that could not decide.
pub const VERDICT_INCONCLUSIVE: &str = "inconclusive";
/// A verification case body that could not be evaluated.
pub const VERDICT_ERROR: &str = "error";

/// Evaluate the claim against the values the model holds; the default question.
pub const QUESTION_EVALUATE: &str = "evaluate";
/// Whether the claim holds for every assignment its free features can take.
pub const QUESTION_HOLDS: &str = "holds";
/// Whether any assignment of the free features satisfies the claim.
pub const QUESTION_SATISFIABLE: &str = "satisfiable";

/// The claim holds.
pub const STATUS_HOLDS: &str = "holds";
/// The claim is violated.
pub const STATUS_VIOLATED: &str = "violated";
/// Nothing was decided.
pub const STATUS_UNDECIDED: &str = "undecided";
/// Some assignment satisfies the claim.
pub const STATUS_SATISFIABLE: &str = "satisfiable";
/// No assignment satisfies the claim.
pub const STATUS_UNSATISFIABLE: &str = "unsatisfiable";

/// One limit an engine ran under.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Bound {
    /// What the limit was on: `steps`, `runs`, `depth`, `elements`.
    pub name: String,
    /// The limit itself.
    pub limit: i64,
    /// Whether the engine stopped at it, which lowered its strength.
    pub reached: bool,
}

impl fmt::Display for Bound {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{} {}", self.name, self.limit)?;
        if self.reached {
            f.write_str(" reached")?;
        }
        Ok(())
    }
}

/// How far an answer can be trusted: who answered, how strongly, within what.
#[derive(Clone, Debug, Default, Eq, PartialEq)]
pub struct Standing {
    /// Name of the engine that answered; empty when unreported.
    pub engine: String,
    /// One of the `STRENGTH_*` spellings; empty from a service predating `engines`.
    pub strength: String,
    /// The bounds the engine ran under.
    pub bounds: Vec<Bound>,
}

impl Standing {
    fn of(engine: &str, strength: &str, bounds: &[wire::Bound]) -> Self {
        Self {
            engine: engine.to_owned(),
            strength: strength.to_owned(),
            bounds: bounds
                .iter()
                .map(|b| Bound {
                    name: b.name.clone(),
                    limit: b.limit,
                    reached: b.reached,
                })
                .collect(),
        }
    }

    /// Whether the service reported a standing at all.
    pub fn reported(&self) -> bool {
        !self.strength.is_empty()
    }

    /// The bounds the engine stopped at.
    pub fn reached(&self) -> impl Iterator<Item = &Bound> {
        self.bounds.iter().filter(|b| b.reached)
    }
}

impl fmt::Display for Standing {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        if !self.reported() {
            return Ok(());
        }
        f.write_str(&self.strength)?;
        if !self.engine.is_empty() {
            write!(f, " by {}", self.engine)?;
        }
        let reached: Vec<String> = self.reached().map(Bound::to_string).collect();
        if !reached.is_empty() {
            write!(f, " ({})", reached.join(", "))?;
        }
        Ok(())
    }
}

/// One engine the service registers.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct EngineInfo {
    /// The spelling an engine selection names it by.
    pub name: String,
    /// The strongest evidence it may claim, one of the `STRENGTH_*` spellings.
    pub authority: String,
    /// The question kinds it answers.
    pub answers: Vec<String>,
    /// The bounds it runs under.
    pub bounds: Vec<String>,
    /// The external process it needs, empty for an in-process engine.
    pub process: String,
    /// Where that process was found, empty when it was not.
    pub process_found: String,
    /// Whether it can run here.
    pub ready: bool,
    /// Why it cannot, when it cannot.
    pub unavailable: String,
    /// `built-in`, `tool` or `engine`.
    pub kind: String,
    /// How it is spoken to, `-` for a built-in engine.
    pub protocol: String,
    /// The manifest an external engine was read from.
    pub source: String,
    /// The resolved command of an external engine.
    pub command: String,
    /// The version its manifest declares.
    pub version: String,
    /// Whether this service runs it.
    pub served: bool,
}

impl From<EngineInfo> for wire::EngineInfo {
    fn from(info: EngineInfo) -> Self {
        Self {
            name: info.name,
            authority: info.authority,
            answers: info.answers,
            bounds: info.bounds,
            process: info.process,
            process_found: info.process_found,
            ready: info.ready,
            unavailable: info.unavailable,
            kind: info.kind,
            protocol: info.protocol,
            source: info.source,
            command: info.command,
            version: info.version,
            served: info.served,
        }
    }
}

impl From<wire::EngineInfo> for EngineInfo {
    fn from(pb: wire::EngineInfo) -> Self {
        Self {
            name: pb.name,
            authority: pb.authority,
            answers: pb.answers,
            bounds: pb.bounds,
            process: pb.process,
            process_found: pb.process_found,
            ready: pb.ready,
            unavailable: pb.unavailable,
            kind: pb.kind,
            protocol: pb.protocol,
            source: pb.source,
            command: pb.command,
            version: pb.version,
            served: pb.served,
        }
    }
}

impl fmt::Display for EngineInfo {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.name)?;
        if !self.kind.is_empty() {
            write!(f, " [{}]", self.kind)?;
        }
        write!(
            f,
            ": {}; answers {}",
            self.authority,
            self.answers.join(", ")
        )?;
        if self.ready {
            f.write_str("; ready")
        } else {
            write!(f, "; unavailable: {}", self.unavailable)
        }
    }
}

/// What the body of one verification case verifying a requirement answered.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct VerificationVerdict {
    /// FQN of the verification case.
    pub case_id: String,
    /// One of the `VERDICT_*` spellings.
    pub kind: String,
    /// Why the body answered as it did.
    pub detail: String,
    /// Whether the case is a subcase of another.
    pub subcase: bool,
    /// FQN of the requirement it verifies.
    pub requirement_id: String,
}

impl VerificationVerdict {
    /// Whether the body passed.
    pub fn passed(&self) -> bool {
        self.kind == VERDICT_PASS
    }
}

impl From<VerificationVerdict> for wire::VerificationVerdict {
    fn from(verdict: VerificationVerdict) -> Self {
        Self {
            case_id: verdict.case_id,
            kind: verdict.kind,
            detail: verdict.detail,
            subcase: verdict.subcase,
            requirement_id: verdict.requirement_id,
        }
    }
}

impl From<wire::VerificationVerdict> for VerificationVerdict {
    fn from(pb: wire::VerificationVerdict) -> Self {
        Self {
            case_id: pb.case_id,
            kind: pb.kind,
            detail: pb.detail,
            subcase: pb.subcase,
            requirement_id: pb.requirement_id,
        }
    }
}

impl fmt::Display for VerificationVerdict {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let mark = match self.kind.as_str() {
            VERDICT_PASS => "\u{2713}",
            VERDICT_FAIL => "\u{2717}",
            _ => "?",
        };
        write!(
            f,
            "{mark} verification {} verdict: {}",
            self.case_id, self.kind
        )?;
        if self.subcase {
            f.write_str(" (subcase)")?;
        }
        if !self.detail.is_empty() {
            write!(f, " \u{2014} {}", self.detail)?;
        }
        Ok(())
    }
}

/// One feature's value in the assignment witnessing a verdict.
#[derive(Clone, Debug, PartialEq)]
pub struct WitnessAssignment {
    /// The qualified feature name, chain steps appended with `.`.
    pub feature: String,
    /// The value the evaluator replayed for it.
    pub value: Option<Value>,
    /// The base units the magnitude is expressed in; empty for none.
    pub unit: String,
    /// The solver's exact value as text.
    pub exact: String,
}

/// One verification's answer.
#[derive(Clone, Debug)]
pub struct Verdict {
    /// What was verified, one of the `KIND_*` spellings.
    pub kind: String,
    /// FQN of the element verified; empty for an anonymous satisfaction assertion.
    pub element_id: String,
    /// The element as a reader names it.
    pub element: String,
    /// Whether the condition holds.
    pub holds: bool,
    /// The condition that evaluated to false, as written, when named.
    pub condition: String,
    /// Instance the verdict is about, 0 for declared values alone.
    pub instance_id: i64,
    /// FQN of that instance's type.
    pub instance_type_id: String,
    /// Set when evaluation failed rather than the model answering false.
    pub error: String,
    /// FQN of the requirement a `satisfy` verdict asserts satisfied.
    pub requirement_id: String,
    /// Where the object sits in a validated one; empty for the root.
    pub instance_path: String,
    /// The question answered; empty from a service predating `verification_questions`.
    pub question: String,
    /// One of the `STATUS_*` spellings.
    pub status: String,
    /// The free features' values witnessing a violated or satisfiable answer.
    pub witness: Vec<WitnessAssignment>,
    /// Who answered, how strongly, within what bounds.
    pub standing: Standing,
    /// What the bodies of the cases verifying this verdict's requirement answered.
    pub verifications: Vec<VerificationVerdict>,
    /// Diagnostics the service reported.
    pub diagnostics: Vec<Diagnostic>,
    /// What kind of failure [`Verdict::error`] is; unspecified when it is empty.
    pub reason: FailureReason,
    instances: Arc<[Instance]>,
    wire: wire::Verdict,
}

impl Verdict {
    fn from_wire(
        pb: wire::Verdict,
        instances: Arc<[Instance]>,
        diagnostics: &[Diagnostic],
        verifications: Vec<VerificationVerdict>,
    ) -> Result<Self, Error> {
        let witness = pb
            .witness
            .iter()
            .map(|w| {
                Ok(WitnessAssignment {
                    feature: w.feature.clone(),
                    value: w.value.clone().map(value_from_wire).transpose()?,
                    unit: w.unit.clone(),
                    exact: w.exact.clone(),
                })
            })
            .collect::<Result<_, Error>>()?;
        Ok(Self {
            kind: pb.kind.clone(),
            element_id: pb.element_id.clone(),
            element: if pb.element.is_empty() {
                pb.element_id.clone()
            } else {
                pb.element.clone()
            },
            holds: pb.holds,
            condition: pb.condition.clone(),
            instance_id: pb.instance_id,
            instance_type_id: pb.instance_type_id.clone(),
            error: pb.error.clone(),
            requirement_id: pb.requirement_id.clone(),
            instance_path: pb.instance_path.clone(),
            question: pb.question.clone(),
            status: pb.status.clone(),
            witness,
            standing: Standing::of(&pb.engine, &pb.strength, &pb.bounds),
            verifications,
            diagnostics: diagnostics.to_vec(),
            reason: reason_of(pb.failure_reason),
            instances,
            wire: pb,
        })
    }

    /// The message this was built from; for conformance tooling and debugging.
    pub fn wire(&self) -> &wire::Verdict {
        &self.wire
    }

    /// Whether `holds` is an answer about the model at all.
    pub fn evaluated(&self) -> bool {
        self.error.is_empty()
    }

    /// The objects the call reported: this verdict's own and those reachable from it.
    pub fn instances(&self) -> &[Instance] {
        &self.instances
    }

    /// The object this verdict is about, when the call reported it.
    pub fn instance(&self) -> Option<&Instance> {
        if self.instance_id == 0 {
            return None;
        }
        self.instances.iter().find(|i| i.id() == self.instance_id)
    }

    /// Fail with an [`Error::Execution`] when evaluation failed; a false verdict is an answer.
    pub fn require_evaluated(&self) -> Result<&Self, Error> {
        if self.error.is_empty() {
            Ok(self)
        } else {
            Err(Error::Execution {
                message: format!("{}: {}", self.named(), self.error),
                reason: self.reason,
                diagnostics: self.diagnostics.clone(),
            })
        }
    }

    fn named(&self) -> String {
        let mut named = if self.element.split_whitespace().any(|w| w == self.kind) {
            self.element.clone()
        } else {
            format!("{} {}", self.kind, self.element)
        };
        if !self.instance_path.is_empty() {
            named.push_str(" at ");
            named.push_str(&self.instance_path);
        }
        named
    }
}

impl fmt::Display for Verdict {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let subject = if self.instance_id != 0 {
            let ty = if self.instance_type_id.is_empty() {
                "instance"
            } else {
                &self.instance_type_id
            };
            format!(" (on {ty} ID: {})", self.instance_id)
        } else {
            String::new()
        };
        if !self.error.is_empty() {
            write!(f, "? {}{subject}: {}", self.named(), self.error)?;
        } else if self.holds {
            write!(f, "\u{2713} {} holds{subject}", self.named())?;
        } else {
            write!(
                f,
                "\u{2717} {} fails{subject}: condition evaluated to false",
                self.named()
            )?;
            if !self.condition.is_empty() {
                write!(f, ": {}", self.condition)?;
            }
        }
        if self.standing.reported() {
            write!(f, " \u{2014} {}", self.standing)?;
        }
        Ok(())
    }
}

/// Every assertion about an object and the objects it holds, as `%validate` answers.
#[derive(Clone, Debug)]
pub struct Validation {
    /// One per assertion, root first then each held object in traversal order.
    pub verdicts: Vec<Verdict>,
    /// The object's own verdict, of kind `object`.
    pub summary: Option<Verdict>,
    /// Whether traversal stopped at its bound before reaching every held object.
    pub bounded: bool,
    /// Diagnostics the service reported.
    pub diagnostics: Vec<Diagnostic>,
    /// What the bodies of the verification cases of the requirements met answered.
    pub verifications: Vec<VerificationVerdict>,
    instances: Arc<[Instance]>,
    wire: wire::ValidateInstanceResponse,
}

impl Validation {
    /// The ValidateInstance response this was read from.
    pub fn wire(&self) -> &wire::ValidateInstanceResponse {
        &self.wire
    }
    /// Whether the object's own verdict holds: every assertion held and every object was reached.
    pub fn valid(&self) -> bool {
        self.summary
            .as_ref()
            .is_some_and(|s| s.holds && s.error.is_empty())
    }
    /// The assertions that evaluated to false.
    pub fn violated(&self) -> impl Iterator<Item = &Verdict> {
        self.verdicts
            .iter()
            .filter(|v| !v.holds && v.error.is_empty())
    }
    /// The assertions that could not be evaluated.
    pub fn undecided(&self) -> impl Iterator<Item = &Verdict> {
        self.verdicts.iter().filter(|v| !v.error.is_empty())
    }
    /// Fail on the first assertion that could not be evaluated.
    pub fn require_evaluated(&self) -> Result<&Self, Error> {
        for verdict in self.undecided() {
            verdict.require_evaluated()?;
        }
        Ok(self)
    }
    /// The engine standing of the object's own verdict.
    pub fn standing(&self) -> Standing {
        self.summary
            .as_ref()
            .map(|s| s.standing.clone())
            .unwrap_or_default()
    }
    /// The object validated and every object reachable from it.
    pub fn instances(&self) -> &[Instance] {
        &self.instances
    }
}

impl fmt::Display for Validation {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let lines: Vec<String> = self
            .verdicts
            .iter()
            .chain(self.summary.iter())
            .map(Verdict::to_string)
            .collect();
        f.write_str(&lines.join("\n"))
    }
}

/// A value the service reported under a name: a calc output, a sweep input.
#[derive(Clone, Debug, PartialEq)]
pub struct NamedValue {
    /// The feature or parameter name.
    pub name: String,
    /// Its value.
    pub value: Value,
}

fn named_values(entries: &[wire::CalcOutput]) -> Result<Vec<NamedValue>, Error> {
    entries
        .iter()
        .map(|entry| {
            let value = entry
                .value
                .clone()
                .ok_or_else(|| Error::Decode(format!("output {} carries no value", entry.name)))?;
            Ok(NamedValue {
                name: entry.name.clone(),
                value: value_from_wire(value)?,
            })
        })
        .collect()
}

fn find_named<'a>(entries: &'a [NamedValue], name: &str) -> Option<&'a Value> {
    entries.iter().find(|e| e.name == name).map(|e| &e.value)
}

/// A calculation's answer.
#[derive(Clone, Debug)]
pub struct CalcResult {
    /// The one value a calculation invoked with arguments returns; `None` when it reports outputs.
    pub value: Option<Value>,
    /// Every output feature a calc usage evaluated from its own members computes.
    pub outputs: Vec<NamedValue>,
    /// Diagnostics the service reported.
    pub diagnostics: Vec<Diagnostic>,
    /// Who answered, how strongly, within what bounds.
    pub standing: Standing,
    wire: wire::EvaluateCalcResponse,
}

impl CalcResult {
    /// The EvaluateCalc response this was read from.
    pub fn wire(&self) -> &wire::EvaluateCalcResponse {
        &self.wire
    }
    /// One output by name.
    pub fn output(&self, name: &str) -> Option<&Value> {
        find_named(&self.outputs, name)
    }
}

/// One evaluation a trade study made of an alternative.
#[derive(Clone, Debug, PartialEq)]
pub struct CaseEvaluation {
    /// FQN of the function evaluated.
    pub function_id: String,
    /// The arguments it was evaluated on.
    pub arguments: Vec<Value>,
    /// What it gave; `None` when it failed.
    pub result: Option<Value>,
    /// Why it failed; empty when it computed a result.
    pub error: String,
    /// Whether the case selected this alternative.
    pub selected: bool,
    /// Whether it tied with another for selection.
    pub tied: bool,
}

impl CaseEvaluation {
    /// Whether the evaluation computed a result.
    pub fn evaluated(&self) -> bool {
        self.error.is_empty()
    }
}

impl fmt::Display for CaseEvaluation {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let args: Vec<String> = self.arguments.iter().map(|a| format!("{a:?}")).collect();
        write!(f, "{}({})", self.function_id, args.join(", "))?;
        match &self.result {
            _ if !self.error.is_empty() => write!(f, ": error: {}", self.error)?,
            Some(result) => write!(f, " = {result:?}")?,
            None => {}
        }
        if self.selected {
            f.write_str(" [selected]")?;
        } else if self.tied {
            f.write_str(" [tied]")?;
        }
        Ok(())
    }
}

fn evaluations(entries: &[wire::CaseEvaluation]) -> Result<Vec<CaseEvaluation>, Error> {
    entries
        .iter()
        .map(|pb| {
            Ok(CaseEvaluation {
                function_id: pb.function_id.clone(),
                arguments: pb
                    .arguments
                    .iter()
                    .cloned()
                    .map(value_from_wire)
                    .collect::<Result<_, _>>()?,
                result: if pb.error.is_empty() {
                    pb.result.clone().map(value_from_wire).transpose()?
                } else {
                    None
                },
                error: pb.error.clone(),
                selected: pb.selected,
                tied: pb.tied,
            })
        })
        .collect()
}

/// What an analysis case computed and the verdicts of its objective and assertions.
#[derive(Clone, Debug)]
pub struct AnalysisResult {
    /// The outputs the case computed.
    pub outputs: Vec<NamedValue>,
    /// The verdicts of its objective and asserted constraints.
    pub verdicts: Vec<Verdict>,
    /// Diagnostics the service reported.
    pub diagnostics: Vec<Diagnostic>,
    /// What the bodies of the verification cases met answered.
    pub verifications: Vec<VerificationVerdict>,
    /// Each evaluation a trade study made of an alternative.
    pub evaluations: Vec<CaseEvaluation>,
    /// Who answered, how strongly, within what bounds.
    pub standing: Standing,
    instances: Arc<[Instance]>,
    wire: wire::RunAnalysisResponse,
}

impl AnalysisResult {
    /// The RunAnalysis response this was read from.
    pub fn wire(&self) -> &wire::RunAnalysisResponse {
        &self.wire
    }
    /// One output by name.
    pub fn output(&self, name: &str) -> Option<&Value> {
        find_named(&self.outputs, name)
    }
    /// The evaluations of the alternatives the case selected.
    pub fn selected(&self) -> impl Iterator<Item = &CaseEvaluation> {
        self.evaluations.iter().filter(|e| e.selected)
    }
    /// Whether every objective and assertion holds; a case stating none is satisfied.
    pub fn satisfied(&self) -> bool {
        self.verdicts.iter().all(|v| v.holds)
    }
    /// The objects the run reported.
    pub fn instances(&self) -> &[Instance] {
        &self.instances
    }
}

/// One run of a parameter sweep.
#[derive(Clone, Debug)]
pub struct SweepRow {
    /// The swept parameters' values for this run.
    pub inputs: Vec<NamedValue>,
    /// The outputs it computed.
    pub outputs: Vec<NamedValue>,
    /// Its verdicts.
    pub verdicts: Vec<Verdict>,
    /// How long the run took.
    pub elapsed: Duration,
    /// Why the run failed; empty when it succeeded.
    pub error: String,
    /// Each evaluation a trade study made of an alternative.
    pub evaluations: Vec<CaseEvaluation>,
}

impl SweepRow {
    /// Whether this run failed rather than producing outputs.
    pub fn failed(&self) -> bool {
        !self.error.is_empty()
    }
    /// Whether the run succeeded and every verdict of it holds.
    pub fn holds(&self) -> bool {
        !self.failed() && self.verdicts.iter().all(|v| v.holds)
    }
    /// One input by name.
    pub fn input(&self, name: &str) -> Option<&Value> {
        find_named(&self.inputs, name)
    }
    /// One output by name.
    pub fn output(&self, name: &str) -> Option<&Value> {
        find_named(&self.outputs, name)
    }
    /// The evaluations of the alternatives the run selected.
    pub fn selected(&self) -> impl Iterator<Item = &CaseEvaluation> {
        self.evaluations.iter().filter(|e| e.selected)
    }
}

/// One row per run of a parameter sweep, in the order the runs were made.
#[derive(Clone, Debug)]
pub struct SweepTable {
    /// The runs.
    pub rows: Vec<SweepRow>,
    /// The swept parameters, in range order.
    pub parameters: Vec<String>,
    /// Whether the rows were drawn rather than stepped through.
    pub sampled: bool,
    /// The seed the draws were taken from.
    pub seed: u64,
    /// Diagnostics the service reported.
    pub diagnostics: Vec<Diagnostic>,
    /// Who answered, how strongly, within what bounds.
    pub standing: Standing,
    instances: Arc<[Instance]>,
    wire: wire::RunSweepResponse,
}

impl SweepTable {
    /// The RunSweep response this was read from.
    pub fn wire(&self) -> &wire::RunSweepResponse {
        &self.wire
    }
    /// The runs that failed.
    pub fn failures(&self) -> impl Iterator<Item = &SweepRow> {
        self.rows.iter().filter(|r| r.failed())
    }
    /// Whether there were runs and every one succeeded with every verdict holding.
    pub fn holds(&self) -> bool {
        !self.rows.is_empty() && self.rows.iter().all(SweepRow::holds)
    }
    /// The objects the runs reported.
    pub fn instances(&self) -> &[Instance] {
        &self.instances
    }
}

/// One swept parameter's range: two endpoints, and the step where the range states one.
#[derive(Clone, Debug, PartialEq)]
pub struct SweepRange {
    /// The input parameter the range binds.
    pub parameter: String,
    /// The first value.
    pub start: Value,
    /// The last value.
    pub end: Value,
    /// The step; a range between whole numbers steps by one without one.
    pub step: Option<Value>,
}

impl SweepRange {
    /// A range from `start` to `end`.
    pub fn new(parameter: impl Into<String>, start: Value, end: Value) -> Self {
        Self {
            parameter: parameter.into(),
            start,
            end,
            step: None,
        }
    }
    /// The same range advancing by `step`.
    pub fn step(mut self, step: Value) -> Self {
        self.step = Some(step);
        self
    }
}

/// One distinct outcome an explored behavior reached.
#[derive(Clone, Debug)]
pub struct Outcome {
    /// The values the behavior holds at the end, by name.
    pub outputs: BTreeMap<String, Value>,
    /// The state a state machine rests in; empty for an action.
    pub final_state: String,
    /// The states a state machine entered, in order.
    pub states_visited: Vec<String>,
    /// Why the runs reaching it failed; empty for completed runs.
    pub error: String,
    /// How many explored orders reached it.
    pub linearizations: i32,
    /// Share of the explored orders' likelihood reaching it.
    pub probability: f64,
    /// The choices one run reaching it made, in run order.
    pub witness: Vec<String>,
    /// What the witness run reported.
    pub diagnostics: Vec<Diagnostic>,
}

impl Outcome {
    /// Whether the runs reaching this outcome failed rather than completed.
    pub fn failed(&self) -> bool {
        !self.error.is_empty()
    }
    /// Fail with the run's failure, if it failed.
    pub fn require_completed(&self) -> Result<&Self, Error> {
        if self.failed() {
            Err(Error::Execution {
                message: self.error.clone(),
                reason: FailureReason::Unspecified,
                diagnostics: self.diagnostics.clone(),
            })
        } else {
            Ok(self)
        }
    }
}

/// Every distinct outcome a behavior reached under `explore`, and how the exploration ended.
#[derive(Clone, Debug)]
pub struct Exploration {
    /// The outcomes, in the service's canonical order.
    pub outcomes: Vec<Outcome>,
    /// Whether every linearization within the budget was run.
    pub complete: bool,
    /// How many runs were made.
    pub runs: i32,
    /// The budgets the exploration ran into, `runs` or `depth`.
    pub budgets_hit: Vec<String>,
    /// The most runs the exploration would make.
    pub runs_budget: i32,
    /// The most choice points one run would resolve.
    pub depth_budget: i32,
    /// Whether the probabilities are lower bounds.
    pub probabilities_lower_bound: bool,
    wire: ExploredResponse,
}

impl Exploration {
    /// The explored run's response this was read from.
    pub fn wire(&self) -> &ExploredResponse {
        &self.wire
    }
    /// Whether the exploration is complete and no run failed.
    pub fn succeeded(&self) -> bool {
        self.complete && !self.outcomes.iter().any(Outcome::failed)
    }
    /// How the exploration ended, as `sysml -schedule explore` prints it.
    pub fn status(&self) -> String {
        if self.complete {
            return format!("complete ({} runs)", self.runs);
        }
        let named: Vec<String> = self
            .budgets_hit
            .iter()
            .map(|b| {
                let limit = if b == "depth" {
                    self.depth_budget
                } else {
                    self.runs_budget
                };
                format!("{b} budget {limit}")
            })
            .collect();
        format!(
            "incomplete: {} hit after {} runs; probabilities are lower bounds",
            named.join(" and "),
            self.runs
        )
    }
    /// Fail unless every linearization was run.
    pub fn require_complete(&self) -> Result<&Self, Error> {
        if self.complete {
            Ok(self)
        } else {
            Err(Error::Execution {
                message: self.status(),
                reason: FailureReason::Unspecified,
                diagnostics: Vec::new(),
            })
        }
    }
}

/// The response an exploration was read from.
#[derive(Clone, Debug)]
pub enum ExploredResponse {
    /// An explored action.
    Action(Box<wire::ExecuteActionResponse>),
    /// An explored state machine.
    State(Box<wire::ExecuteStateResponse>),
    /// An explored analysis case.
    Analysis(Box<wire::RunAnalysisResponse>),
}

/// Every satisfaction assertion's verdict, as `%satisfy` answers.
#[derive(Clone, Debug)]
pub struct Satisfaction {
    /// One per assertion, in the model's order.
    pub verdicts: Vec<Verdict>,
    /// Diagnostics the service reported.
    pub diagnostics: Vec<Diagnostic>,
    /// What the bodies of the verification cases of the satisfied requirements answered.
    pub verifications: Vec<VerificationVerdict>,
    instances: Arc<[Instance]>,
    wire: wire::VerifySatisfactionResponse,
}

impl Satisfaction {
    /// Whether every assertion held.
    pub fn holds(&self) -> bool {
        self.verdicts.iter().all(|v| v.holds && v.error.is_empty())
    }
    /// The assertions that evaluated to false.
    pub fn violated(&self) -> impl Iterator<Item = &Verdict> {
        self.verdicts
            .iter()
            .filter(|v| !v.holds && v.error.is_empty())
    }
    /// The objects the verdicts are about.
    pub fn instances(&self) -> &[Instance] {
        &self.instances
    }
    /// The VerifySatisfaction response this was read from.
    pub fn wire(&self) -> &wire::VerifySatisfactionResponse {
        &self.wire
    }
}

impl fmt::Display for Satisfaction {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        for (i, verdict) in self.verdicts.iter().enumerate() {
            if i > 0 {
                f.write_str("\n")?;
            }
            write!(f, "{verdict}")?;
        }
        Ok(())
    }
}

/// What one run of an action produced.
#[derive(Clone, Debug)]
pub struct ActionRun {
    /// Output parameters by name.
    pub outputs: BTreeMap<String, Value>,
    /// The performer's attributes as the run left them, keyed `this.<name>`.
    pub performer: BTreeMap<String, Value>,
    /// The simulated time the run ended at; 0 from a service predating `final_time`.
    pub final_time: f64,
    /// Diagnostics the service reported.
    pub diagnostics: Vec<Diagnostic>,
    pub(crate) wire: wire::ExecuteActionResponse,
}

impl ActionRun {
    /// The ExecuteAction response this was read from.
    pub fn wire(&self) -> &wire::ExecuteActionResponse {
        &self.wire
    }
}

/// What one run of a state machine produced.
#[derive(Clone, Debug)]
pub struct StateRun {
    /// The states entered, in order.
    pub states_visited: Vec<String>,
    /// The values the machine holds at the end, by name.
    pub final_context: BTreeMap<String, Value>,
    /// The simulated time the run ended at; 0 from a service predating `final_time`.
    pub final_time: f64,
    /// Diagnostics the service reported.
    pub diagnostics: Vec<Diagnostic>,
    pub(crate) wire: wire::ExecuteStateResponse,
}

impl StateRun {
    /// The ExecuteState response this was read from.
    pub fn wire(&self) -> &wire::ExecuteStateResponse {
        &self.wire
    }
}

pub(crate) fn diagnostics_of(entries: &[wire::Diagnostic]) -> Vec<Diagnostic> {
    entries.iter().cloned().map(Diagnostic::from).collect()
}

/// The error for a failure the service classified by its typed reason.
pub(crate) fn failure_of(message: &str, reason: i32, diagnostics: Vec<Diagnostic>) -> Error {
    if reason == wire::FailureReason::WrongKind as i32 {
        Error::WrongKind {
            message: message.to_owned(),
            diagnostics,
        }
    } else {
        Error::Execution {
            message: message.to_owned(),
            reason: reason_of(reason),
            diagnostics,
        }
    }
}

/// The typed failure reason a wire enum value names; one this client does not know is unspecified.
pub(crate) fn reason_of(reason: i32) -> FailureReason {
    FailureReason::try_from(reason).unwrap_or(FailureReason::Unspecified)
}

pub(crate) fn instances_of(
    entries: &[wire::Instance],
    capabilities: &Capabilities,
) -> Result<Arc<[Instance]>, Error> {
    if !entries.is_empty() {
        capabilities.require(
            CAPABILITY_FEATURE_VALUES,
            upgrade_remedy(CAPABILITY_FEATURE_VALUES),
        )?;
    }
    entries
        .iter()
        .cloned()
        .map(Instance::from_wire)
        .collect::<Result<Vec<_>, _>>()
        .map(Arc::from)
}

pub(crate) fn value_map(
    entries: &std::collections::HashMap<String, wire::Value>,
) -> Result<BTreeMap<String, Value>, Error> {
    entries
        .iter()
        .map(|(name, value)| Ok((name.clone(), value_from_wire(value.clone())?)))
        .collect()
}

fn verifications_of(entries: &[wire::VerificationVerdict]) -> Vec<VerificationVerdict> {
    entries
        .iter()
        .cloned()
        .map(VerificationVerdict::from)
        .collect()
}

fn verifications_for(all: &[VerificationVerdict], pb: &wire::Verdict) -> Vec<VerificationVerdict> {
    if pb.requirement_id.is_empty() {
        return Vec::new();
    }
    all.iter()
        .filter(|v| v.requirement_id == pb.requirement_id)
        .cloned()
        .collect()
}

fn reject_wrong_kind(pb: &wire::Verdict, diagnostics: &[Diagnostic]) -> Result<(), Error> {
    if pb.failure_reason == wire::FailureReason::WrongKind as i32 {
        return Err(Error::WrongKind {
            message: pb.error.clone(),
            diagnostics: diagnostics.to_vec(),
        });
    }
    Ok(())
}

pub(crate) fn single_verdict(
    verdict: Option<wire::Verdict>,
    instances: &[wire::Instance],
    error: &str,
    diagnostics: &[wire::Diagnostic],
    verifications: &[wire::VerificationVerdict],
    capabilities: &Capabilities,
) -> Result<Verdict, Error> {
    let diagnostics = diagnostics_of(diagnostics);
    if !error.is_empty() {
        return Err(Error::Execution {
            message: error.to_owned(),
            reason: FailureReason::Unspecified,
            diagnostics,
        });
    }
    let pb = verdict.ok_or_else(|| Error::Decode("response carries no verdict".to_owned()))?;
    reject_wrong_kind(&pb, &diagnostics)?;
    let instances = instances_of(instances, capabilities)?;
    let verifications = verifications_of(verifications);
    Verdict::from_wire(pb, instances, &diagnostics, verifications)
}

pub(crate) fn satisfaction_of(
    response: wire::VerifySatisfactionResponse,
    capabilities: &Capabilities,
) -> Result<Satisfaction, Error> {
    let wire = response.clone();
    let diagnostics = diagnostics_of(&response.diagnostics);
    if !response.error.is_empty() {
        return Err(failure_of(
            &response.error,
            response.failure_reason,
            diagnostics,
        ));
    }
    for pb in &response.verdicts {
        reject_wrong_kind(pb, &diagnostics)?;
    }
    let instances = instances_of(&response.instances, capabilities)?;
    let verifications = verifications_of(&response.verification_verdicts);
    let verdicts = response
        .verdicts
        .into_iter()
        .map(|pb| {
            let own = verifications_for(&verifications, &pb);
            Verdict::from_wire(pb, instances.clone(), &diagnostics, own)
        })
        .collect::<Result<_, _>>()?;
    Ok(Satisfaction {
        verdicts,
        diagnostics,
        verifications,
        instances,
        wire,
    })
}

pub(crate) fn validation_of(
    response: wire::ValidateInstanceResponse,
    capabilities: &Capabilities,
) -> Result<Validation, Error> {
    let wire = response.clone();
    let diagnostics = diagnostics_of(&response.diagnostics);
    if !response.error.is_empty() {
        return Err(failure_of(
            &response.error,
            response.failure_reason,
            diagnostics,
        ));
    }
    let instances = instances_of(&response.instances, capabilities)?;
    let verifications = verifications_of(&response.verification_verdicts);
    let verdicts = response
        .verdicts
        .into_iter()
        .map(|pb| {
            let own = verifications_for(&verifications, &pb);
            Verdict::from_wire(pb, instances.clone(), &diagnostics, own)
        })
        .collect::<Result<_, _>>()?;
    let summary = response
        .summary
        .map(|pb| Verdict::from_wire(pb, instances.clone(), &diagnostics, verifications.clone()))
        .transpose()?;
    Ok(Validation {
        verdicts,
        summary,
        bounded: response.bounded,
        diagnostics,
        verifications,
        instances,
        wire,
    })
}

pub(crate) fn calc_of(response: wire::EvaluateCalcResponse) -> Result<CalcResult, Error> {
    let wire = response.clone();
    let diagnostics = diagnostics_of(&response.diagnostics);
    if !response.error.is_empty() {
        return Err(failure_of(
            &response.error,
            response.failure_reason,
            diagnostics,
        ));
    }
    let outputs = named_values(&response.outputs)?;
    let value = if outputs.is_empty() {
        response.result.map(value_from_wire).transpose()?
    } else {
        None
    };
    Ok(CalcResult {
        value,
        outputs,
        diagnostics,
        standing: Standing::of(&response.engine, &response.strength, &response.bounds),
        wire,
    })
}

pub(crate) fn analysis_of(
    response: wire::RunAnalysisResponse,
    capabilities: &Capabilities,
) -> Result<AnalysisResult, Error> {
    let wire = response.clone();
    let diagnostics = diagnostics_of(&response.diagnostics);
    let partial = !(response.outputs.is_empty()
        && response.verdicts.is_empty()
        && response.evaluations.is_empty()
        && response.instances.is_empty());
    if !response.error.is_empty() && !partial {
        return Err(failure_of(
            &response.error,
            response.failure_reason,
            diagnostics,
        ));
    }
    let instances = instances_of(&response.instances, capabilities)?;
    let verdicts = response
        .verdicts
        .into_iter()
        .map(|pb| Verdict::from_wire(pb, instances.clone(), &diagnostics, Vec::new()))
        .collect::<Result<_, _>>()?;
    let result = AnalysisResult {
        outputs: named_values(&response.outputs)?,
        verdicts,
        diagnostics,
        verifications: verifications_of(&response.verification_verdicts),
        evaluations: evaluations(&response.evaluations)?,
        standing: Standing::of(&response.engine, &response.strength, &response.bounds),
        instances,
        wire,
    };
    if response.error.is_empty() {
        Ok(result)
    } else {
        Err(Error::AnalysisRun {
            message: response.error,
            result: Box::new(result),
        })
    }
}

pub(crate) fn sweep_of(
    response: wire::RunSweepResponse,
    capabilities: &Capabilities,
) -> Result<SweepTable, Error> {
    let wire = response.clone();
    let diagnostics = diagnostics_of(&response.diagnostics);
    if !response.error.is_empty() {
        return Err(failure_of(
            &response.error,
            response.failure_reason,
            diagnostics,
        ));
    }
    let instances = instances_of(&response.instances, capabilities)?;
    let rows = response
        .rows
        .into_iter()
        .map(|row| {
            Ok(SweepRow {
                inputs: named_values(&row.inputs)?,
                outputs: named_values(&row.outputs)?,
                verdicts: row
                    .verdicts
                    .into_iter()
                    .map(|pb| Verdict::from_wire(pb, instances.clone(), &diagnostics, Vec::new()))
                    .collect::<Result<_, _>>()?,
                elapsed: Duration::from_micros(row.elapsed_micros.max(0).unsigned_abs()),
                error: row.error,
                evaluations: evaluations(&row.evaluations)?,
            })
        })
        .collect::<Result<_, Error>>()?;
    Ok(SweepTable {
        rows,
        parameters: response.parameters,
        sampled: response.sampled,
        seed: response.seed,
        diagnostics,
        standing: Standing::of(&response.engine, &response.strength, &response.bounds),
        instances,
        wire,
    })
}

pub(crate) fn exploration_of(response: ExploredResponse) -> Result<Exploration, Error> {
    let (error, failure_reason, diagnostics, outcomes, status) = match &response {
        ExploredResponse::Action(r) => (&r.error, 0, &r.diagnostics, &r.outcomes, &r.exploration),
        ExploredResponse::State(r) => (&r.error, 0, &r.diagnostics, &r.outcomes, &r.exploration),
        ExploredResponse::Analysis(r) => (
            &r.error,
            r.failure_reason,
            &r.diagnostics,
            &r.outcomes,
            &r.exploration,
        ),
    };
    if !error.is_empty() {
        return Err(failure_of(
            error,
            failure_reason,
            diagnostics_of(diagnostics),
        ));
    }
    let status = status.clone().ok_or_else(|| {
        Error::Decode("an explored run's response carries no exploration status".to_owned())
    })?;
    let outcomes = outcomes
        .iter()
        .map(|pb| {
            Ok(Outcome {
                outputs: value_map(&pb.outputs)?,
                final_state: pb.final_state.clone(),
                states_visited: pb.states_visited.clone(),
                error: pb.error.clone(),
                linearizations: pb.linearizations,
                probability: pb.probability,
                witness: pb.witness.clone(),
                diagnostics: diagnostics_of(&pb.diagnostics),
            })
        })
        .collect::<Result<_, Error>>()?;
    Ok(Exploration {
        outcomes,
        complete: status.complete,
        runs: status.runs,
        budgets_hit: status.budgets_hit,
        runs_budget: status.runs_budget,
        depth_budget: status.depth_budget,
        probabilities_lower_bound: status.probabilities_lower_bound,
        wire: response,
    })
}
