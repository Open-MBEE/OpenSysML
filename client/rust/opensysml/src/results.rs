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
    pub(crate) fn of(engine: &str, strength: &str, bounds: &[wire::Bound]) -> Self {
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
    /// ID of the object the case ran on as its owner; 0 when it ran on none.
    pub instance_id: i64,
    /// FQN of that object's type, for naming it; empty with `instance_id` 0.
    pub instance_type_id: String,
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
            instance_id: verdict.instance_id,
            instance_type_id: verdict.instance_type_id,
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
            instance_id: pb.instance_id,
            instance_type_id: pb.instance_type_id,
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
        write!(f, "{mark} verification {}", self.case_id)?;
        if self.instance_id != 0 {
            let ty = if self.instance_type_id.is_empty() {
                "instance"
            } else {
                &self.instance_type_id
            };
            write!(f, " (on {ty} ID: {})", self.instance_id)?;
        }
        write!(f, " verdict: {}", self.kind)?;
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

/// One assumption or omission behind a verdict, worded as every surface reports it.
#[derive(Clone, Debug, PartialEq)]
pub struct VerdictNote {
    /// [`NOTE_ASSUMED_ONE`] or [`NOTE_UNCOVERED`].
    pub kind: String,
    /// The feature the note is about, as the report names it.
    pub subject: String,
    /// The note as a sentence.
    pub text: String,
}

/// A feature read as holding one value or object while only the default `0..*` governs it.
pub const NOTE_ASSUMED_ONE: &str = "assumed_one";
/// An object-holding feature whose own assertions the question does not state.
pub const NOTE_UNCOVERED: &str = "uncovered";

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
    /// What the answer rests on, in report order; empty from a service without `verdict_notes`.
    pub notes: Vec<VerdictNote>,
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
        let notes = pb
            .notes
            .iter()
            .map(|n| VerdictNote {
                kind: n.kind.clone(),
                subject: n.subject.clone(),
                text: n.text.clone(),
            })
            .collect();
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
            notes,
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
                trace: Vec::new(),
                trace_dropped: 0,
                final_time: 0.0,
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
    /// The external tool call previewed when the case ran under
    /// `AnalysisOptions.tool_dry_run`; `None` when the case ran for real.
    pub tool_dry_run: Option<ToolDryRun>,
    instances: Arc<[Instance]>,
    wire: wire::RunAnalysisResponse,
}

/// The external tool call an analysis would make, previewed without running the tool: what
/// `AnalysisOptions.tool_dry_run` stops at, as `sysml -tool-dry-run` prints it.
#[derive(Clone, Debug, PartialEq, Eq, Default)]
pub struct ToolDryRun {
    /// The qualified name of the tool-executing action reached.
    pub action: String,
    /// The tool the action names.
    pub tool: String,
    /// The tool version the manifest declares, if any.
    pub version: String,
    /// The manifest file the tool was found in.
    pub manifest: String,
    /// The program that would run.
    pub executable: String,
    /// How the tool is spoken to (`stdio`, `files`, ...).
    pub protocol: String,
    /// The command line the tool would get.
    pub args: Vec<String>,
    /// Environment entries set for it.
    pub env: Vec<String>,
    /// The working directory it would run in.
    pub cwd: String,
    /// How standard input is fed.
    pub stdin: String,
    /// The text written to it.
    pub stdin_data: String,
    /// The input file written; `None` when the tool reads none.
    pub input_file: Option<ToolDryRunFile>,
    /// Whether an output directory would be made.
    pub output_dir: bool,
    /// Each value handed over.
    pub inputs: Vec<ToolDryRunInput>,
    /// Each result read back.
    pub outputs: Vec<ToolDryRunOutput>,
    /// Where the tool's answer would be read from.
    pub reply_source: String,
    /// The preview as `sysml -tool-dry-run` prints it.
    pub lines: Vec<String>,
}

/// The input file a previewed tool call would be given.
#[derive(Clone, Debug, PartialEq, Eq, Default)]
pub struct ToolDryRunFile {
    /// The file's name.
    pub name: String,
    /// Its format.
    pub format: String,
    /// Its contents.
    pub data: String,
}

/// One value a previewed tool call would hand over.
#[derive(Clone, Debug, PartialEq, Eq, Default)]
pub struct ToolDryRunInput {
    /// The model variable the value comes from.
    pub variable: String,
    /// The tool parameter it binds.
    pub parameter: String,
    /// The value, as the tool would read it.
    pub value: String,
    /// Its unit, if any.
    pub unit: String,
}

/// One result a previewed tool call would read back.
#[derive(Clone, Debug, PartialEq, Eq, Default)]
pub struct ToolDryRunOutput {
    /// The model variable the result fills.
    pub variable: String,
    /// The tool parameter it is read from.
    pub parameter: String,
}

impl From<wire::ToolDryRun> for ToolDryRun {
    fn from(pb: wire::ToolDryRun) -> Self {
        ToolDryRun {
            action: pb.action,
            tool: pb.tool,
            version: pb.version,
            manifest: pb.manifest,
            executable: pb.executable,
            protocol: pb.protocol,
            args: pb.args,
            env: pb.env,
            cwd: pb.cwd,
            stdin: pb.stdin,
            stdin_data: pb.stdin_data,
            input_file: pb.input_file.map(|file| ToolDryRunFile {
                name: file.name,
                format: file.format,
                data: file.data,
            }),
            output_dir: pb.output_dir,
            inputs: pb
                .inputs
                .into_iter()
                .map(|input| ToolDryRunInput {
                    variable: input.variable,
                    parameter: input.parameter,
                    value: input.value,
                    unit: input.unit,
                })
                .collect(),
            outputs: pb
                .outputs
                .into_iter()
                .map(|output| ToolDryRunOutput {
                    variable: output.variable,
                    parameter: output.parameter,
                })
                .collect(),
            reply_source: pb.reply_source,
            lines: pb.lines,
        }
    }
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
        self.verdicts.iter().all(|v| v.holds && v.error.is_empty())
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
    /// What kind of failure [`SweepRow::error`] is; unspecified when it is empty.
    pub reason: FailureReason,
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
                trace: Vec::new(),
                trace_dropped: 0,
                final_time: 0.0,
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
                trace: Vec::new(),
                trace_dropped: 0,
                final_time: 0.0,
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
    /// The run's records when `trace` was asked for, in order.
    pub trace: Vec<crate::document::DocumentEvent>,
    /// Oldest trace records the service discarded.
    pub trace_dropped: i32,
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
    /// The documented execution records, when requested.
    pub trace: Vec<crate::document::DocumentEvent>,
    /// The number of oldest records the service discarded.
    pub trace_dropped: i32,
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
            trace: Vec::new(),
            trace_dropped: 0,
            final_time: 0.0,
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
            trace: Vec::new(),
            trace_dropped: 0,
            final_time: 0.0,
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
        tool_dry_run: response.tool_dry_run.map(ToolDryRun::from),
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
    let rows = sweep_rows_of(response.rows, &instances, &diagnostics)?;
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

fn sweep_rows_of(
    rows: Vec<wire::SweepRow>,
    instances: &Arc<[Instance]>,
    diagnostics: &[Diagnostic],
) -> Result<Vec<SweepRow>, Error> {
    rows.into_iter()
        .map(|row| {
            Ok(SweepRow {
                inputs: named_values(&row.inputs)?,
                outputs: named_values(&row.outputs)?,
                verdicts: row
                    .verdicts
                    .into_iter()
                    .map(|pb| Verdict::from_wire(pb, instances.clone(), diagnostics, Vec::new()))
                    .collect::<Result<_, _>>()?,
                elapsed: Duration::from_micros(row.elapsed_micros.max(0).unsigned_abs()),
                error: row.error,
                reason: reason_of(row.failure_reason),
                evaluations: evaluations(&row.evaluations)?,
            })
        })
        .collect()
}

/// The value a percentile of the runs fell at or below.
#[derive(Clone, Debug)]
pub struct RunPercentile {
    /// The percentile, e.g. 50.0.
    pub percentile: f64,
    /// The value.
    pub value: Value,
}

/// How many runs observed a value in `[low, high)`.
#[derive(Clone, Debug)]
pub struct RunHistogramBin {
    /// Lower bound, inclusive.
    pub low: Value,
    /// Upper bound, exclusive.
    pub high: Value,
    /// How many runs.
    pub count: i64,
}

/// How many runs observed one distinct value.
#[derive(Clone, Debug)]
pub struct RunCountedValue {
    /// The value.
    pub value: Value,
    /// How many runs.
    pub count: i64,
}

/// One observable over the completed runs: the extremes, mean, deviation,
/// percentiles and histogram of a numeric one, or each distinct value counted.
#[derive(Clone, Debug)]
pub struct RunDistribution {
    /// The observable.
    pub name: String,
    /// The unit every value carried; empty for none.
    pub unit: String,
    /// How many runs.
    pub count: i64,
    /// Whether the values are numbers, summarised below.
    pub numeric: bool,
    /// The least value of a numeric observable.
    pub min: Option<Value>,
    /// The greatest value of a numeric observable.
    pub max: Option<Value>,
    /// The mean of a numeric observable.
    pub mean: Option<Value>,
    /// The standard deviation of a numeric observable.
    pub deviation: f64,
    /// The percentiles of a numeric observable.
    pub percentiles: Vec<RunPercentile>,
    /// The histogram of a numeric observable.
    pub histogram: Vec<RunHistogramBin>,
    /// Each distinct value of a non-numeric observable, counted.
    pub values: Vec<RunCountedValue>,
}

/// What `run_runs` answers: one row per run, each observable's distribution
/// and, for a Monte Carlo case, its conclusion over the sample.
#[derive(Clone, Debug)]
pub struct RunsResult {
    /// One row per run, in run order.
    pub rows: Vec<SweepRow>,
    /// What every row reports, in column order.
    pub observables: Vec<String>,
    /// Each observable over the completed runs.
    pub distributions: Vec<RunDistribution>,
    /// The report as `sysml -runs` prints it.
    pub lines: Vec<String>,
    /// A Monte Carlo case's conclusion over the sample.
    pub outputs: Vec<NamedValue>,
    /// The conclusion's checks.
    pub verdicts: Vec<Verdict>,
    /// Why a Monte Carlo case has no conclusion; empty when it has one.
    pub unconcluded: String,
    /// The objects the conclusion names.
    pub instances: Arc<[Instance]>,
    /// The engine's name, strength and bounds.
    pub standing: Standing,
    /// The seed the runs drew from; `None` when the draws were fixed.
    pub seed: Option<u64>,
    /// Notes raised.
    pub diagnostics: Vec<Diagnostic>,
    wire: wire::RunRunsResponse,
}

impl RunsResult {
    /// The response as the service sent it.
    pub fn wire(&self) -> &wire::RunRunsResponse {
        &self.wire
    }
}

impl RunsResult {
    /// How many runs were made.
    pub fn len(&self) -> usize {
        self.rows.len()
    }
    /// Whether no run was made.
    pub fn is_empty(&self) -> bool {
        self.rows.is_empty()
    }
    /// One output of the conclusion by name.
    pub fn output(&self, name: &str) -> Option<&Value> {
        find_named(&self.outputs, name)
    }
}

fn optional_value(value: Option<wire::Value>) -> Result<Option<Value>, Error> {
    value.map(value_from_wire).transpose()
}

pub(crate) fn runs_of(
    response: wire::RunRunsResponse,
    capabilities: &Capabilities,
) -> Result<RunsResult, Error> {
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
    let rows = sweep_rows_of(response.rows, &instances, &diagnostics)?;
    let distributions = response
        .distributions
        .into_iter()
        .map(|d| {
            Ok(RunDistribution {
                name: d.name,
                unit: d.unit,
                count: d.count,
                numeric: d.numeric,
                min: optional_value(d.min)?,
                max: optional_value(d.max)?,
                mean: optional_value(d.mean)?,
                deviation: d.deviation,
                percentiles: d
                    .percentiles
                    .into_iter()
                    .map(|p| {
                        Ok(RunPercentile {
                            percentile: p.percentile,
                            value: value_from_wire(p.value.unwrap_or_default())?,
                        })
                    })
                    .collect::<Result<_, Error>>()?,
                histogram: d
                    .histogram
                    .into_iter()
                    .map(|b| {
                        Ok(RunHistogramBin {
                            low: value_from_wire(b.low.unwrap_or_default())?,
                            high: value_from_wire(b.high.unwrap_or_default())?,
                            count: b.count,
                        })
                    })
                    .collect::<Result<_, Error>>()?,
                values: d
                    .values
                    .into_iter()
                    .map(|v| {
                        Ok(RunCountedValue {
                            value: value_from_wire(v.value.unwrap_or_default())?,
                            count: v.count,
                        })
                    })
                    .collect::<Result<_, Error>>()?,
            })
        })
        .collect::<Result<_, Error>>()?;
    Ok(RunsResult {
        rows,
        observables: response.observables,
        distributions,
        lines: response.lines,
        outputs: named_values(&response.outputs)?,
        verdicts: response
            .verdicts
            .into_iter()
            .map(|pb| Verdict::from_wire(pb, instances.clone(), &diagnostics, Vec::new()))
            .collect::<Result<_, _>>()?,
        unconcluded: response.unconcluded,
        standing: Standing::of(&response.engine, &response.strength, &response.bounds),
        instances,
        seed: response.seed,
        diagnostics,
        wire,
    })
}

/// One thing the reflective self-check reports; `line` is it as the CLI prints it.
#[derive(Clone, Debug)]
pub struct SelfCheckFinding {
    /// `fails`, `unevaluated`, `error` or `warning`.
    pub status: String,
    /// `<constraint> for <element>`, or the constraint or package alone.
    pub subject: String,
    /// `document:line:col` of the element; empty for none.
    pub location: String,
    /// The constraint applied.
    pub constraint: String,
    /// The element it was applied to.
    pub element: String,
    /// The reason, for a warning or an error.
    pub message: String,
    /// The finding as `sysml -self-check` prints it.
    pub line: String,
}

/// What `self_check` answers: every finding and the counts over them.
#[derive(Clone, Debug)]
pub struct SelfCheckResult {
    /// `holds`, `fails` or `unresolved`.
    pub status: String,
    /// Each violation, unevaluated check, error and skipped rule constraint.
    pub findings: Vec<SelfCheckFinding>,
    /// Elements checked.
    pub elements: i64,
    /// Checks made.
    pub checks: i64,
    /// Violations found.
    pub violations: i64,
    /// Checks the runtime could not evaluate.
    pub unevaluated: i64,
    /// Checks that errored.
    pub evaluation_errors: i64,
    /// How many elements each constraint was applied to, by name.
    pub applications: BTreeMap<String, i64>,
    /// The one-line total `sysml -self-check` prints last.
    pub summary: String,
    /// Notes raised.
    pub diagnostics: Vec<Diagnostic>,
    wire: wire::SelfCheckResponse,
}

impl SelfCheckResult {
    /// The response as the service sent it.
    pub fn wire(&self) -> &wire::SelfCheckResponse {
        &self.wire
    }
}

impl SelfCheckResult {
    /// Whether the check found no violation and no evaluation error.
    pub fn holds(&self) -> bool {
        self.status == "holds"
    }
}

pub(crate) fn self_check_of(response: wire::SelfCheckResponse) -> Result<SelfCheckResult, Error> {
    let wire = response.clone();
    let diagnostics = diagnostics_of(&response.diagnostics);
    if !response.error.is_empty() {
        return Err(failure_of(&response.error, 0, diagnostics));
    }
    Ok(SelfCheckResult {
        status: response.status,
        findings: response
            .findings
            .into_iter()
            .map(|f| SelfCheckFinding {
                status: f.status,
                subject: f.subject,
                location: f.location,
                constraint: f.constraint,
                element: f.element,
                message: f.message,
                line: f.line,
            })
            .collect(),
        elements: response.elements,
        checks: response.checks,
        violations: response.violations,
        unevaluated: response.unevaluated,
        evaluation_errors: response.evaluation_errors,
        applications: response
            .applications
            .into_iter()
            .map(|a| (a.constraint, a.count))
            .collect(),
        summary: response.summary,
        diagnostics,
        wire,
    })
}

/// One document the records were written into.
#[derive(Clone, Debug)]
pub struct RecordedDocument {
    /// The observable.
    pub name: String,
    /// The notation.
    pub content: String,
}

/// What `record_run` answers: the runs' own report, what the model gained and
/// the notation the records were written into.
#[derive(Clone, Debug)]
pub struct Recording {
    /// The run's result, for a case run once.
    pub run: Option<AnalysisResult>,
    /// The rows, for a case run over ranges.
    pub sweep: Option<SweepTable>,
    /// The runs, for a Monte Carlo sample.
    pub sample: Option<RunsResult>,
    /// The package the records went into.
    pub package: String,
    /// The per-case record definition.
    pub definition: String,
    /// The records made, in order.
    pub records: Vec<String>,
    /// The declarations generated.
    pub source: String,
    /// The recorded notation of a single-document model the records merged into.
    pub content: String,
    /// The document the records went into, or the records as a document of their own.
    pub documents: Vec<RecordedDocument>,
    /// Rows or runs that failed and were not recorded.
    pub skipped: i64,
    /// Why runs were not recorded.
    pub reasons: Vec<String>,
    /// The recorded model, cached for the next request.
    pub model_hash: String,
    /// Notes raised.
    pub diagnostics: Vec<Diagnostic>,
    wire: wire::RecordRunResponse,
}

impl Recording {
    /// The response as the service sent it.
    pub fn wire(&self) -> &wire::RecordRunResponse {
        &self.wire
    }
}

pub(crate) fn recording_of(
    response: wire::RecordRunResponse,
    capabilities: &Capabilities,
) -> Result<Recording, Error> {
    let wire = response.clone();
    let diagnostics = diagnostics_of(&response.diagnostics);
    if !response.error.is_empty() {
        return Err(failure_of(
            &response.error,
            response.failure_reason,
            diagnostics,
        ));
    }
    Ok(Recording {
        run: response
            .run
            .map(|r| analysis_of(r, capabilities))
            .transpose()?,
        sweep: response
            .sweep
            .map(|r| sweep_of(r, capabilities))
            .transpose()?,
        sample: response
            .sample
            .map(|r| runs_of(r, capabilities))
            .transpose()?,
        package: response.package,
        definition: response.definition,
        records: response.records,
        source: response.source,
        content: response.content,
        documents: response
            .documents
            .into_iter()
            .map(|d| RecordedDocument {
                name: d.name,
                content: d.content,
            })
            .collect(),
        skipped: response.skipped,
        reasons: response.reasons,
        model_hash: response.model_hash,
        diagnostics,
        wire,
    })
}

/// One assignment an import made: the feature of the element set to the value,
/// redefining the inherited feature `redefines` when the element did not declare it.
#[derive(Clone, Debug, PartialEq)]
pub struct ImportedValue {
    /// Qualified name of the element whose feature is set.
    pub element: String,
    /// The feature set.
    pub feature: String,
    /// The literal it is set to; "" for a redefinition made only to reach a nested feature.
    pub value: String,
    /// Qualified name of the inherited feature redefined, "" when the element declares it.
    pub redefines: String,
    /// The row or cell of the data it came from.
    pub where_: String,
    /// The line `sysml -import` reports it as.
    pub description: String,
}

/// What one data file set.
#[derive(Clone, Debug, PartialEq)]
pub struct ImportedFile {
    /// The file's name.
    pub name: String,
    /// How many values it set.
    pub values: i64,
    /// How many elements they were set on.
    pub elements: i64,
    /// Each assignment, in data order.
    pub assignments: Vec<ImportedValue>,
}

/// What [`Connection::import_data`](crate::Connection::import_data) answers.
#[derive(Clone, Debug)]
pub struct ImportResult {
    /// One per data file, in order.
    pub files: Vec<ImportedFile>,
    /// The edited content of a one-document model; "" on a dry run.
    pub content: String,
    /// Every edited document, by name; empty on a dry run.
    pub documents: Vec<(String, String)>,
    /// The imported model's hash, cached by the service; "" on a dry run.
    pub model_hash: String,
    /// Notes the import raised.
    pub diagnostics: Vec<Diagnostic>,
    wire: wire::ImportDataResponse,
}

impl ImportResult {
    /// The response as the service answered it.
    pub fn wire(&self) -> &wire::ImportDataResponse {
        &self.wire
    }
}

/// A distribution by its statistics; a pooled one knows its count and mean alone.
#[derive(Clone, Debug, PartialEq)]
pub struct ComparedDistribution {
    /// How many numbers.
    pub count: i64,
    /// The least.
    pub min: f64,
    /// The mean.
    pub mean: f64,
    /// The median.
    pub p50: f64,
    /// The 90th percentile.
    pub p90: f64,
    /// The greatest.
    pub max: f64,
    /// The sample standard deviation.
    pub deviation: f64,
}

/// The relative difference of each statistic as a signed percentage, "" where not compared.
#[derive(Clone, Debug, PartialEq)]
pub struct StatisticDifference {
    /// Of the least.
    pub min: String,
    /// Of the mean.
    pub mean: String,
    /// Of the median.
    pub p50: String,
    /// Of the 90th percentile.
    pub p90: String,
    /// Of the greatest.
    pub max: String,
}

/// One statistic a Simulation::MonteCarlo case declares, the tool's beside the runs'.
#[derive(Clone, Debug, PartialEq)]
pub struct CaseStatistic {
    /// Which statistic.
    pub statistic: String,
    /// Where the case declares it.
    pub of: String,
    /// The tool's value.
    pub tool: String,
    /// The runs' value.
    pub run: String,
    /// Their relative difference.
    pub difference: String,
}

/// The statistics a Simulation::MonteCarlo case declares of an observable.
#[derive(Clone, Debug, PartialEq)]
pub struct CaseStatistics {
    /// The case.
    pub analysis: String,
    /// One per statistic.
    pub rows: Vec<CaseStatistic>,
    /// What was not compared, and why.
    pub notes: Vec<String>,
}

/// One observable: the tool's distribution beside the runs', or why they are not compared.
#[derive(Clone, Debug, PartialEq)]
pub struct ObservableComparison {
    /// The observable the tool stored.
    pub stored: String,
    /// The feature of the run that answers it.
    pub feature: String,
    /// The unit the runs' numbers came in, "" for none.
    pub unit: String,
    /// The tool's distribution, `None` when it stored no number.
    pub tool: Option<ComparedDistribution>,
    /// Whether the tool's distribution was pooled from summaries.
    pub pooled: bool,
    /// Whether the tool stored no result at all.
    pub unstored: bool,
    /// The completed runs' distribution, `None` when not compared.
    pub run: Option<ComparedDistribution>,
    /// How many runs produced a number for the feature.
    pub produced: i64,
    /// Whether the runs were made and are reported.
    pub compared: bool,
    /// The relative difference of each statistic.
    pub difference: Option<StatisticDifference>,
    /// The case's statistics compared.
    pub declared: Option<CaseStatistics>,
    /// Notes on what was pooled or not compared.
    pub notes: Vec<String>,
}

/// One configuration's runs beside the tool's results.
#[derive(Clone, Debug)]
pub struct ConfigurationComparison {
    /// The configuration's id in the sidecar.
    pub id: String,
    /// Its qualified name.
    pub name: String,
    /// The migrated behavior it performs.
    pub behavior: String,
    /// The feature the observables are read from.
    pub target: String,
    /// Where the tool stored the results.
    pub result_location: String,
    /// `holds`, `fails` (a run failed) or `unresolved`.
    pub status: String,
    /// Why the configuration could not be run.
    pub unresolved: String,
    /// The runs the tool stored.
    pub stored_runs: i64,
    /// Over how many snapshots.
    pub stored_snapshots: i64,
    /// The runs OpenSysML made.
    pub runs: i64,
    /// How many completed.
    pub completed: i64,
    /// The draw policy the runs took.
    pub draws: String,
    /// The seed the runs derived theirs from.
    pub seed: Option<u64>,
    /// The clock step, 0 for a continuous clock.
    pub clock_step: f64,
    /// One per run.
    pub rows: Vec<SweepRow>,
    /// The observables compared.
    pub observables: Vec<ObservableComparison>,
    /// The errors failed runs ended in.
    pub errors: Vec<String>,
    /// Notes on the configuration and its runs.
    pub notes: Vec<String>,
    /// The comparison as `sysml -compare-results` prints it.
    pub lines: Vec<String>,
}

impl ConfigurationComparison {
    /// Whether every run completed and the observables were compared.
    pub fn holds(&self) -> bool {
        self.status == "holds"
    }
}

/// What [`Connection::compare_results`](crate::Connection::compare_results) answers.
#[derive(Clone, Debug)]
pub struct Comparison {
    /// The tool the sidecar records as the results' source.
    pub source: String,
    /// One per configuration, in the order the sidecar indexes them.
    pub configurations: Vec<ConfigurationComparison>,
    /// The engine and strength of the last runs made.
    pub standing: Standing,
    /// Notes the comparison raised.
    pub diagnostics: Vec<Diagnostic>,
    wire: wire::CompareResultsResponse,
}

impl Comparison {
    /// The response as the service answered it.
    pub fn wire(&self) -> &wire::CompareResultsResponse {
        &self.wire
    }
}

impl Comparison {
    /// Whether every configuration ran and compared.
    pub fn holds(&self) -> bool {
        self.configurations
            .iter()
            .all(ConfigurationComparison::holds)
    }
}

pub(crate) fn import_result_of(response: wire::ImportDataResponse) -> Result<ImportResult, Error> {
    let wire = response.clone();
    let diagnostics = diagnostics_of(&response.diagnostics);
    if !response.error.is_empty() {
        return Err(failure_of(&response.error, 0, diagnostics));
    }
    Ok(ImportResult {
        files: response
            .files
            .into_iter()
            .map(|f| ImportedFile {
                name: f.name,
                values: f.values,
                elements: f.elements,
                assignments: f
                    .assignments
                    .into_iter()
                    .map(|a| ImportedValue {
                        element: a.element,
                        feature: a.feature,
                        value: a.value,
                        redefines: a.redefines,
                        where_: a.r#where,
                        description: a.description,
                    })
                    .collect(),
            })
            .collect(),
        content: response.content,
        documents: response
            .documents
            .into_iter()
            .map(|d| (d.name, d.content))
            .collect(),
        model_hash: response.model_hash,
        diagnostics,
        wire,
    })
}

fn compared_distribution_of(d: Option<wire::ComparedDistribution>) -> Option<ComparedDistribution> {
    d.map(|d| ComparedDistribution {
        count: d.count,
        min: d.min,
        mean: d.mean,
        p50: d.p50,
        p90: d.p90,
        max: d.max,
        deviation: d.deviation,
    })
}

pub(crate) fn comparison_of(
    response: wire::CompareResultsResponse,
    capabilities: &Capabilities,
) -> Result<Comparison, Error> {
    let wire = response.clone();
    let diagnostics = diagnostics_of(&response.diagnostics);
    if !response.error.is_empty() {
        return Err(failure_of(
            &response.error,
            response.failure_reason,
            diagnostics,
        ));
    }
    let mut configurations = Vec::with_capacity(response.configurations.len());
    for c in response.configurations {
        let instances = instances_of(&c.instances, capabilities)?;
        let rows = c
            .rows
            .into_iter()
            .map(|row| {
                Ok(SweepRow {
                    inputs: named_values(&row.inputs)?,
                    outputs: named_values(&row.outputs)?,
                    verdicts: row
                        .verdicts
                        .into_iter()
                        .map(|pb| {
                            Verdict::from_wire(pb, instances.clone(), &diagnostics, Vec::new())
                        })
                        .collect::<Result<_, _>>()?,
                    elapsed: Duration::from_micros(row.elapsed_micros.max(0).unsigned_abs()),
                    error: row.error,
                    reason: reason_of(row.failure_reason),
                    evaluations: evaluations(&row.evaluations)?,
                })
            })
            .collect::<Result<_, Error>>()?;
        configurations.push(ConfigurationComparison {
            id: c.id,
            name: c.name,
            behavior: c.behavior,
            target: c.target,
            result_location: c.result_location,
            status: c.status,
            unresolved: c.unresolved,
            stored_runs: c.stored_runs,
            stored_snapshots: c.stored_snapshots,
            runs: c.runs,
            completed: c.completed,
            draws: c.draws,
            seed: c.seed,
            clock_step: c.clock_step,
            rows,
            observables: c
                .observables
                .into_iter()
                .map(|o| ObservableComparison {
                    stored: o.stored,
                    feature: o.feature,
                    unit: o.unit,
                    tool: compared_distribution_of(o.tool),
                    pooled: o.pooled,
                    unstored: o.unstored,
                    run: compared_distribution_of(o.run),
                    produced: o.produced,
                    compared: o.compared,
                    difference: o.difference.map(|d| StatisticDifference {
                        min: d.min,
                        mean: d.mean,
                        p50: d.p50,
                        p90: d.p90,
                        max: d.max,
                    }),
                    declared: o.declared.map(|cs| CaseStatistics {
                        analysis: cs.analysis,
                        rows: cs
                            .rows
                            .into_iter()
                            .map(|r| CaseStatistic {
                                statistic: r.statistic,
                                of: r.of,
                                tool: r.tool,
                                run: r.run,
                                difference: r.difference,
                            })
                            .collect(),
                        notes: cs.notes,
                    }),
                    notes: o.notes,
                })
                .collect(),
            errors: c.errors,
            notes: c.notes,
            lines: c.lines,
        });
    }
    Ok(Comparison {
        source: response.source,
        configurations,
        standing: Standing::of(&response.engine, &response.strength, &response.bounds),
        diagnostics,
        wire,
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    fn capabilities(names: &[&str]) -> Capabilities {
        Capabilities::new(wire::ServerInfoResponse {
            capabilities: names.iter().map(|name| (*name).to_owned()).collect(),
            ..Default::default()
        })
    }

    fn verdict(element: &str, holds: bool, error: &str, reason: FailureReason) -> wire::Verdict {
        wire::Verdict {
            kind: "constraint".to_owned(),
            element_id: element.to_owned(),
            holds,
            error: error.to_owned(),
            failure_reason: reason as i32,
            ..Default::default()
        }
    }

    #[test]
    fn a_tool_dry_run_is_read_field_by_field() {
        let response = wire::RunAnalysisResponse {
            tool_dry_run: Some(wire::ToolDryRun {
                action: "Demo::compute".to_owned(),
                tool: "octave".to_owned(),
                version: "8.4".to_owned(),
                manifest: "/tools/octave.json".to_owned(),
                executable: "/usr/bin/octave".to_owned(),
                protocol: "stdio".to_owned(),
                args: vec!["--eval".to_owned(), "run".to_owned()],
                env: vec!["HOME=/tmp".to_owned()],
                cwd: "/tmp/run".to_owned(),
                stdin: "data".to_owned(),
                stdin_data: "x = 1".to_owned(),
                input_file: Some(wire::ToolDryRunFile {
                    name: "in.json".to_owned(),
                    format: "json".to_owned(),
                    data: "{}".to_owned(),
                }),
                output_dir: true,
                inputs: vec![wire::ToolDryRunInput {
                    variable: "x".to_owned(),
                    parameter: "p".to_owned(),
                    value: "250.0".to_owned(),
                    unit: "m".to_owned(),
                }],
                outputs: vec![wire::ToolDryRunOutput {
                    variable: "y".to_owned(),
                    parameter: "q".to_owned(),
                }],
                reply_source: "stdout".to_owned(),
                lines: vec!["would run /usr/bin/octave".to_owned()],
            }),
            ..Default::default()
        };
        let result = analysis_of(response, &capabilities(&[])).unwrap();
        assert!(result.outputs.is_empty());
        let preview = result.tool_dry_run.expect("the preview is typed");
        assert_eq!(preview.action, "Demo::compute");
        assert_eq!(preview.tool, "octave");
        assert_eq!(preview.version, "8.4");
        assert_eq!(preview.args, vec!["--eval", "run"]);
        assert_eq!(preview.env, vec!["HOME=/tmp"]);
        assert_eq!(
            preview.input_file,
            Some(ToolDryRunFile {
                name: "in.json".to_owned(),
                format: "json".to_owned(),
                data: "{}".to_owned(),
            })
        );
        assert!(preview.output_dir);
        assert_eq!(preview.inputs[0].value, "250.0");
        assert_eq!(preview.inputs[0].unit, "m");
        assert_eq!(preview.outputs[0].parameter, "q");
        assert_eq!(preview.reply_source, "stdout");
        assert_eq!(preview.lines, vec!["would run /usr/bin/octave"]);
        let real_run =
            analysis_of(wire::RunAnalysisResponse::default(), &capabilities(&[])).unwrap();
        assert!(real_run.tool_dry_run.is_none());
    }

    #[test]
    fn a_wrong_kind_failure_is_its_own_error() {
        assert!(matches!(
            failure_of("not a calc", FailureReason::WrongKind as i32, Vec::new()),
            Error::WrongKind { .. }
        ));
        match failure_of("undecided", FailureReason::Undecided as i32, Vec::new()) {
            Error::Execution { reason, .. } => assert_eq!(reason, FailureReason::Undecided),
            other => panic!("expected an execution error, got {other:?}"),
        }
        assert_eq!(reason_of(9999), FailureReason::Unspecified);
    }

    #[test]
    fn a_satisfaction_keeps_every_verdict_and_types_their_reasons() {
        let satisfaction = satisfaction_of(
            wire::VerifySatisfactionResponse {
                verdicts: vec![
                    verdict("P::a", true, "", FailureReason::Unspecified),
                    verdict("P::b", false, "no value", FailureReason::Evaluation),
                ],
                ..Default::default()
            },
            &capabilities(&[]),
        )
        .unwrap();
        assert_eq!(satisfaction.verdicts.len(), 2);
        assert!(!satisfaction.holds());
        let failed = &satisfaction.verdicts[1];
        assert_eq!(failed.reason, FailureReason::Evaluation);
        match failed.require_evaluated() {
            Err(Error::Execution { reason, .. }) => assert_eq!(reason, FailureReason::Evaluation),
            other => panic!("expected an execution error, got {other:?}"),
        }
        assert_eq!(satisfaction.wire().verdicts.len(), 2);
    }

    #[test]
    fn a_wrong_kind_verdict_refuses_the_whole_answer() {
        let refused = satisfaction_of(
            wire::VerifySatisfactionResponse {
                verdicts: vec![verdict(
                    "P::a",
                    false,
                    "not a requirement",
                    FailureReason::WrongKind,
                )],
                ..Default::default()
            },
            &capabilities(&[]),
        );
        assert!(matches!(refused, Err(Error::WrongKind { .. })));
    }

    #[test]
    fn reported_instances_need_feature_values() {
        let refused = satisfaction_of(
            wire::VerifySatisfactionResponse {
                instances: vec![wire::Instance::default()],
                ..Default::default()
            },
            &capabilities(&[]),
        );
        assert!(matches!(
            refused,
            Err(Error::MissingCapability { capability, .. }) if capability == CAPABILITY_FEATURE_VALUES
        ));
    }

    #[test]
    fn a_validation_holds_only_when_its_summary_does() {
        let validation = validation_of(
            wire::ValidateInstanceResponse {
                verdicts: vec![
                    verdict("P::a", false, "", FailureReason::Unspecified),
                    verdict("P::b", false, "open", FailureReason::Undecided),
                ],
                summary: Some(verdict("P::x", false, "", FailureReason::Unspecified)),
                bounded: true,
                ..Default::default()
            },
            &capabilities(&[]),
        )
        .unwrap();
        assert!(!validation.valid());
        assert_eq!(validation.violated().count(), 1);
        assert_eq!(validation.undecided().count(), 1);
        assert!(validation.bounded);
        assert!(validation.require_evaluated().is_err());
    }

    #[test]
    fn a_sweep_keeps_the_rows_a_failed_run_made() {
        let table = sweep_of(
            wire::RunSweepResponse {
                rows: vec![
                    wire::SweepRow {
                        outputs: vec![wire::CalcOutput {
                            name: "y".to_owned(),
                            value: Some(wire::Value {
                                kind: Some(wire::value::Kind::IntValue(2)),
                            }),
                        }],
                        elapsed_micros: 5,
                        ..Default::default()
                    },
                    wire::SweepRow {
                        error: "division by zero".to_owned(),
                        failure_reason: FailureReason::Evaluation as i32,
                        elapsed_micros: -1,
                        ..Default::default()
                    },
                ],
                parameters: vec!["x".to_owned()],
                seed: 7,
                engine: "concrete".to_owned(),
                ..Default::default()
            },
            &capabilities(&[]),
        )
        .unwrap();
        assert_eq!(table.rows.len(), 2);
        assert_eq!(table.rows[0].output("y"), Some(&Value::Integer(2)));
        assert_eq!(table.rows[0].elapsed, Duration::from_micros(5));
        assert_eq!(table.rows[1].elapsed, Duration::ZERO);
        assert_eq!(table.rows[1].reason, FailureReason::Evaluation);
        assert_eq!(table.failures().count(), 1);
        assert!(!table.holds());
        assert_eq!(table.standing.engine, "concrete");
        assert_eq!(table.seed, 7);
    }

    #[test]
    fn a_failed_sweep_types_its_reason() {
        let error = sweep_of(
            wire::RunSweepResponse {
                error: "x is not swept".to_owned(),
                failure_reason: FailureReason::AmbiguousSubject as i32,
                ..Default::default()
            },
            &capabilities(&[]),
        )
        .unwrap_err();
        assert!(matches!(
            error,
            Error::Execution {
                reason: FailureReason::AmbiguousSubject,
                ..
            }
        ));
    }

    #[test]
    fn an_analysis_verdict_that_failed_to_evaluate_is_not_satisfied() {
        let result = analysis_of(
            wire::RunAnalysisResponse {
                verdicts: vec![
                    verdict("P::a", true, "", FailureReason::Unspecified),
                    verdict("P::b", true, "unbound", FailureReason::Evaluation),
                ],
                ..Default::default()
            },
            &capabilities(&[]),
        )
        .unwrap();
        assert!(!result.satisfied());
    }

    #[test]
    fn a_calc_answers_one_value_or_its_named_outputs() {
        let single = calc_of(wire::EvaluateCalcResponse {
            result: Some(wire::Value {
                kind: Some(wire::value::Kind::RealValue(2.5)),
            }),
            ..Default::default()
        })
        .unwrap();
        assert_eq!(single.value, Some(Value::Real(2.5)));
        let error = calc_of(wire::EvaluateCalcResponse {
            error: "not a calc".to_owned(),
            failure_reason: FailureReason::WrongKind as i32,
            ..Default::default()
        })
        .unwrap_err();
        assert!(matches!(error, Error::WrongKind { .. }));
    }

    #[test]
    fn an_exploration_keeps_its_status_and_outcomes() {
        let exploration = exploration_of(ExploredResponse::Action(Box::new(
            wire::ExecuteActionResponse {
                outcomes: vec![
                    wire::Outcome {
                        linearizations: 2,
                        ..Default::default()
                    },
                    wire::Outcome {
                        error: "deadlock".to_owned(),
                        linearizations: 1,
                        ..Default::default()
                    },
                ],
                exploration: Some(wire::ExplorationStatus {
                    complete: false,
                    runs: 3,
                    budgets_hit: vec!["runs".to_owned()],
                    runs_budget: 3,
                    ..Default::default()
                }),
                ..Default::default()
            },
        )))
        .unwrap();
        assert_eq!(exploration.outcomes.len(), 2);
        assert!(exploration.outcomes[1].failed());
        assert!(exploration.outcomes[1].require_completed().is_err());
        assert!(!exploration.complete);
        assert!(exploration.require_complete().is_err());
        assert_eq!(exploration.budgets_hit, vec!["runs".to_owned()]);
    }

    #[test]
    fn an_explored_run_without_status_is_undecodable() {
        let error = exploration_of(ExploredResponse::State(Box::default())).unwrap_err();
        assert!(matches!(error, Error::Decode(_)));
    }

    #[test]
    fn engine_and_verification_records_round_trip() {
        let engine = wire::EngineInfo {
            name: "smt".to_owned(),
            ..Default::default()
        };
        assert_eq!(
            wire::EngineInfo::from(EngineInfo::from(engine.clone())),
            engine
        );
        let verification = wire::VerificationVerdict {
            requirement_id: "P::r".to_owned(),
            instance_id: 7,
            instance_type_id: "P::part".to_owned(),
            ..Default::default()
        };
        assert_eq!(
            wire::VerificationVerdict::from(VerificationVerdict::from(verification.clone())),
            verification
        );
    }

    fn verdict_with_notes(notes: Vec<wire::VerdictNote>) -> Verdict {
        let pb = wire::Verdict {
            kind: "constraint".to_owned(),
            element_id: "Tanks::nonNegative".to_owned(),
            question: "holds".to_owned(),
            status: "violated".to_owned(),
            notes,
            ..Default::default()
        };
        Verdict::from_wire(pb, Arc::from(Vec::new()), &[], Vec::new()).expect("a verdict")
    }

    #[test]
    fn a_verdict_carries_the_one_value_assumption() {
        let verdict = verdict_with_notes(vec![wire::VerdictNote {
            kind: "assumed_one".to_owned(),
            subject: "Tanks::Tank::level".to_owned(),
            text: "Assuming Tanks::Tank::level holds one value: it declares no multiplicity, so the default [0..*] governs it".to_owned(),
        }]);
        assert_eq!(
            verdict.notes,
            vec![VerdictNote {
                kind: NOTE_ASSUMED_ONE.to_owned(),
                subject: "Tanks::Tank::level".to_owned(),
                text: "Assuming Tanks::Tank::level holds one value: it declares no multiplicity, so the default [0..*] governs it".to_owned(),
            }]
        );
    }

    #[test]
    fn a_verdict_carries_what_the_question_leaves_uncovered() {
        let verdict = verdict_with_notes(vec![wire::VerdictNote {
            kind: "uncovered".to_owned(),
            subject: "gear".to_owned(),
            text: "Not in the query: what gear asserts about its own objects, as it may hold more than one object; instantiate and validate an object to check it".to_owned(),
        }]);
        assert_eq!(verdict.notes.len(), 1);
        assert_eq!(verdict.notes[0].kind, NOTE_UNCOVERED);
        assert_eq!(verdict.notes[0].subject, "gear");
    }

    #[test]
    fn a_verdict_without_notes_carries_none() {
        assert!(verdict_with_notes(Vec::new()).notes.is_empty());
    }
}
