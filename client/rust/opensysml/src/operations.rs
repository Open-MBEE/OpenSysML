//! The service's conversion, query, document, execution, verification and analysis RPCs.

use std::collections::{BTreeMap, HashMap};

use crate::capabilities::{
    upgrade_remedy, CAPABILITY_BIG_INT_VALUES, CAPABILITY_COMPLEX_VALUES, CAPABILITY_CONVERT,
    CAPABILITY_CONVERT_DOCUMENTS, CAPABILITY_DOCUMENT_QUERY, CAPABILITY_ENGINES,
    CAPABILITY_ENUM_VALUES, CAPABILITY_EXPORT_GRAPHS, CAPABILITY_FUNCTION_VALUES,
    CAPABILITY_INFINITY_VALUE, CAPABILITY_INLINE_LANGUAGE, CAPABILITY_MEASUREMENT_REFS,
    CAPABILITY_METAOBJECT_VALUES, CAPABILITY_MIGRATE, CAPABILITY_OSLC_QUERY,
    CAPABILITY_PARSE_SOURCES, CAPABILITY_PERFORMER, CAPABILITY_QUERY, CAPABILITY_RATIONAL_VALUES,
    CAPABILITY_RENDER_DOCUMENT, CAPABILITY_RENDER_DOCUMENT_HTML, CAPABILITY_SCHEDULE,
    CAPABILITY_SCHEDULE_EXPLORE, CAPABILITY_SET_VALUES, CAPABILITY_STATE_TRACE,
    CAPABILITY_STRICT_CONFORMANCE, CAPABILITY_STRUCTURED_VALUES, CAPABILITY_TENSOR_VALUES,
    CAPABILITY_VERIFICATION, CAPABILITY_VERIFICATION_QUESTIONS,
};
use crate::conversion::{conversion_of, request_of, Conversion, ConvertOptions, ConvertSource};
use crate::document::{
    binding_holds_big_int, binding_holds_rational, binding_rationals_as_reals, bindings_to_wire,
    document_event_from_wire, rendered_view_of, result_of, DocumentForm, DocumentQueryResult,
    DocumentValue, RenderViewPorts, RenderedView,
};
use crate::domain::{Model, Value};
use crate::encode::value_to_wire;
use crate::error::Error;
use crate::graphs::Graphs;
use crate::migration::{
    is_v1, migration_of, path_is_v1, MigrateOptions, MigrateSource, Migration,
    MIGRATED_NOT_CONVERTED,
};
use crate::query::{elements_of, Query, QueryElement};
use crate::results::{
    analysis_of, calc_of, diagnostics_of, exploration_of, satisfaction_of, single_verdict,
    sweep_of, validation_of, value_map, ActionRun, AnalysisResult, CalcResult, EngineInfo,
    Exploration, ExploredResponse, Satisfaction, StateRun, SweepRange, SweepTable, Validation,
    Verdict, ENGINE_AUTO, ENGINE_EXPLORE, QUESTION_EVALUATE,
};
use crate::sources::{documents_to_wire, SourceDocument};
use crate::wire;
use crate::wire::FailureReason;
use crate::Connection;

/// The schedule that runs a behavior once per valid order of its choice points.
pub const SCHEDULE_EXPLORE: &str = "explore";

/// Value capabilities a request argument may need, named so an `UNIMPLEMENTED` refusal reads as one.
const VALUE_CAPABILITIES: &[&str] = &[
    CAPABILITY_ENUM_VALUES,
    CAPABILITY_COMPLEX_VALUES,
    CAPABILITY_STRUCTURED_VALUES,
    CAPABILITY_MEASUREMENT_REFS,
    CAPABILITY_FUNCTION_VALUES,
    CAPABILITY_SET_VALUES,
    CAPABILITY_TENSOR_VALUES,
    CAPABILITY_METAOBJECT_VALUES,
    CAPABILITY_INFINITY_VALUE,
];

/// Options for parsing several documents as one model.
#[derive(Clone, Copy, Debug, Default, Eq, PartialEq)]
pub struct SourcesOptions {
    /// Refuse a model the service reported errors for, as [`Error::ModelErrors`].
    pub strict: bool,
    /// Require strict SysML v2 conformance.
    pub strict_conformance: bool,
}

/// How to run an action or state machine.
#[derive(Clone, Debug, Default, Eq, PartialEq)]
pub struct RunOptions {
    /// The scheduling policy choice points resolve under; `None` is the service's default.
    /// An exploring schedule (`explore`, `explore:runs=<n>,depth=<d>`) belongs to the
    /// `explore_*` methods.
    pub schedule: Option<String>,
    /// Qualified name of the part performing the behavior, whose attributes it reads and writes.
    pub performer: Option<String>,
    /// Return a state's documented execution records.
    pub trace: bool,
}

/// What to ask a verification and of whom.
#[derive(Clone, Debug, Default, Eq, PartialEq)]
pub struct VerifyOptions {
    /// Qualified name of the part or usage to evaluate against.
    pub subject: Option<String>,
    /// The engine to ask; `None` or [`ENGINE_AUTO`] lets the service choose.
    pub engine: Option<String>,
    /// The question to ask; `None` or [`QUESTION_EVALUATE`] evaluates the values held.
    pub question: Option<String>,
}

/// How to run an analysis case.
#[derive(Clone, Debug, Default, PartialEq)]
pub struct AnalysisOptions {
    /// Qualified name of a part or usage to instantiate and run the case on.
    pub subject: Option<String>,
    /// Arguments binding the case's `in` parameters in declaration order, the subject excluded.
    pub arguments: Vec<Value>,
    /// Arguments binding parameters by name.
    pub named_arguments: BTreeMap<String, Value>,
    /// The scheduling policy the actions the case performs resolve their choice points under.
    pub schedule: Option<String>,
    /// The engine to ask.
    pub engine: Option<String>,
}

/// How to sweep an analysis case or calc.
#[derive(Clone, Debug, Default, PartialEq)]
pub struct SweepOptions {
    /// Qualified name of a part or usage to instantiate and run an analysis case on.
    pub subject: Option<String>,
    /// Positional arguments every row binds.
    pub arguments: Vec<Value>,
    /// Arguments by name every row binds.
    pub named_arguments: BTreeMap<String, Value>,
    /// Rows to draw uniformly from the ranges rather than step through them; 0 steps.
    pub samples: i64,
    /// Seed the draws are taken from.
    pub seed: u64,
    /// The engine to ask.
    pub engine: Option<String>,
}

fn explores(schedule: Option<&str>) -> bool {
    schedule.is_some_and(|s| s == SCHEDULE_EXPLORE || s.starts_with("explore:"))
}

fn refuse_exploring(schedule: Option<&str>, method: &str) -> Result<(), Error> {
    match schedule {
        Some(schedule) if explores(Some(schedule)) => Err(Error::InvalidRequest(format!(
            "schedule {schedule:?} answers with every outcome, not one run's result: use {method}"
        ))),
        _ => Ok(()),
    }
}

fn require_exploring(schedule: &str) -> Result<(), Error> {
    if explores(Some(schedule)) {
        Ok(())
    } else {
        Err(Error::InvalidRequest(format!(
            "schedule {schedule:?} answers one run's result, not every outcome: spell it \
             'explore' or 'explore:runs=<n>,depth=<d>'"
        )))
    }
}

fn engine_field(engine: Option<&str>) -> String {
    match engine {
        None | Some("") | Some(ENGINE_AUTO) => String::new(),
        Some(engine) => engine.to_owned(),
    }
}

fn question_field(question: Option<&str>) -> String {
    match question {
        None | Some("") | Some(QUESTION_EVALUATE) => String::new(),
        Some(question) => question.to_owned(),
    }
}

fn engine_capabilities(engine: Option<&str>) -> Vec<&'static str> {
    let mut needed = Vec::new();
    if !engine_field(engine).is_empty() {
        needed.push(CAPABILITY_ENGINES);
    }
    if engine == Some(ENGINE_EXPLORE) {
        needed.push(CAPABILITY_SCHEDULE_EXPLORE);
    }
    needed
}

fn question_capabilities(question: Option<&str>) -> Vec<&'static str> {
    if question_field(question).is_empty() {
        Vec::new()
    } else {
        vec![CAPABILITY_VERIFICATION_QUESTIONS]
    }
}

fn schedule_capabilities(schedule: Option<&str>) -> Vec<&'static str> {
    let mut needed = Vec::new();
    if schedule.is_some_and(|s| !s.is_empty()) {
        needed.push(CAPABILITY_SCHEDULE);
    }
    if explores(schedule) {
        needed.push(CAPABILITY_SCHEDULE_EXPLORE);
    }
    needed
}

impl Connection {
    fn require_all(&self, capabilities: &[&str]) -> Result<(), Error> {
        for capability in capabilities {
            self.capabilities()
                .require(capability, upgrade_remedy(capability))?;
        }
        Ok(())
    }

    fn values_to_wire(&self, values: &[Value]) -> Result<Vec<wire::Value>, Error> {
        values
            .iter()
            .map(|value| value_to_wire(value, self.capabilities()))
            .collect()
    }

    fn named_to_wire(
        &self,
        values: &BTreeMap<String, Value>,
    ) -> Result<HashMap<String, wire::Value>, Error> {
        values
            .iter()
            .map(|(name, value)| Ok((name.clone(), value_to_wire(value, self.capabilities())?)))
            .collect()
    }

    /// Parse several documents as one model: files the service reads and named inline content.
    ///
    /// The model's [`Model::documents`] name them in the order given.
    pub fn parse_sources(
        &self,
        documents: &[SourceDocument],
        options: &SourcesOptions,
    ) -> Result<Model, Error> {
        let encoded = documents_to_wire(documents)?;
        let mut capabilities = vec![CAPABILITY_PARSE_SOURCES];
        if documents.iter().any(SourceDocument::has_language) {
            capabilities.push(CAPABILITY_INLINE_LANGUAGE);
        }
        if options.strict_conformance {
            capabilities.push(CAPABILITY_STRICT_CONFORMANCE);
        }
        self.require_all(&capabilities)?;
        let response: wire::ParseSourcesResponse = self.gated_rpc(
            "ParseSources",
            wire::ParseSourcesRequest {
                documents: encoded,
                strict_conformance: options.strict_conformance,
                base_model_hash: String::new(),
            },
            &capabilities,
        )?;
        if !response.error.is_empty() {
            return Err(Error::ModelErrors {
                message: response.error,
                diagnostics: diagnostics_of(&response.diagnostics),
            });
        }
        let names = documents
            .iter()
            .map(SourceDocument::document_name)
            .collect();
        let model = Model::from_sources(response, names, self.clone());
        if options.strict {
            model.require_ok()?;
        }
        Ok(model)
    }

    /// Convert a file, inline content or a cached model to `to_format`: `sysml`, `kerml`, `ttl`,
    /// `turtle`, `rdf`, `api-json` or `json`.
    ///
    /// An experimental conversion says so in [`Conversion::experimental_notice`]. A SysML v1
    /// model — `from_format` `xmi`, `uml` or `mdzip`, or a file with that extension — is refused
    /// with [`Error::InvalidRequest`]: it is migrated, not converted, by [`Connection::migrate`].
    pub fn convert(
        &self,
        to_format: &str,
        source: &ConvertSource,
        options: &ConvertOptions,
    ) -> Result<Conversion, Error> {
        let v1_file = match source {
            ConvertSource::File(path) => options.from_format.is_empty() && path_is_v1(path),
            _ => false,
        };
        if is_v1(&options.from_format) || v1_file {
            let name = match source {
                ConvertSource::File(path) => path.display().to_string(),
                _ => "the source".to_owned(),
            };
            return Err(Error::InvalidRequest(format!(
                "{name} {MIGRATED_NOT_CONVERTED}; call migrate with the same source"
            )));
        }
        let mut required = vec![CAPABILITY_CONVERT];
        if !options.documents.is_empty() {
            required.push(CAPABILITY_CONVERT_DOCUMENTS);
        }
        self.require_all(&required)?;
        let response =
            self.gated_rpc("Convert", request_of(to_format, source, options), &required)?;
        conversion_of(response)
    }

    /// Migrate a SysML v1 model — a Cameo/MagicDraw `.mdzip`, a UML XMI `.xmi` or an Eclipse
    /// UML2 `.uml` export — to `to_format`: `sysml`, `kerml`, `ttl`, `turtle` or `rdf`.
    ///
    /// A migration is ledgered, not lossless: every v1 element lands in the
    /// [`Migration::report`] as mapped, approximated, unmapped or skipped, and the migration is
    /// experimental, as [`Migration::experimental_notice`] says. Inline content is the file's
    /// bytes and needs `from_format` to say which form they are; a `from_format` that is not a
    /// v1 form is refused with [`Error::InvalidRequest`], as a v2 model is converted, not migrated.
    pub fn migrate(
        &self,
        to_format: &str,
        source: &MigrateSource,
        options: &MigrateOptions,
    ) -> Result<Migration, Error> {
        let name = match source {
            MigrateSource::File(path) => path.display().to_string(),
            MigrateSource::Content(_) => "the source".to_owned(),
        };
        if !options.from_format.is_empty() && !is_v1(&options.from_format) {
            return Err(Error::InvalidRequest(format!(
                "{name} is {} input, which is converted, not migrated: only a SysML v1 model                  (xmi, uml or mdzip) is migrated; call convert with the same source",
                options.from_format
            )));
        }
        if matches!(source, MigrateSource::Content(_)) && options.from_format.is_empty() {
            return Err(Error::InvalidRequest(
                "from_format is required for inline content: xmi, uml or mdzip".to_owned(),
            ));
        }
        let source_path = match source {
            MigrateSource::File(path) => Some(std::path::absolute(path)?),
            MigrateSource::Content(_) => None,
        };
        self.require_all(&[CAPABILITY_MIGRATE])?;
        let response = self.gated_rpc(
            "Migrate",
            crate::migration::request_of(to_format, source, options),
            &[CAPABILITY_MIGRATE],
        )?;
        migration_of(response, source_path)
    }

    /// Run a SysML v2 API & Services query over a loaded model.
    pub fn query(&self, model_hash: &str, query: &Query) -> Result<Vec<QueryElement>, Error> {
        let query = query.to_wire()?;
        self.require_all(&[CAPABILITY_QUERY])?;
        let response = self.gated_rpc(
            "Query",
            wire::QueryRequest {
                model_hash: model_hash.to_owned(),
                query: Some(query),
                ..Default::default()
            },
            &[CAPABILITY_QUERY],
        )?;
        Ok(elements_of(response))
    }

    /// Select elements with OSLC Query 3.0 parameter text, such as
    /// `oslc.where=rdf:type="PartUsage"&oslc.select=sysml:name`.
    pub fn query_oslc(&self, model_hash: &str, oslc: &str) -> Result<Vec<QueryElement>, Error> {
        if oslc.is_empty() {
            return Err(Error::Query("an OSLC query needs text".to_owned()));
        }
        let capabilities = [CAPABILITY_QUERY, CAPABILITY_OSLC_QUERY];
        self.require_all(&capabilities)?;
        let response = self.gated_rpc(
            "Query",
            wire::QueryRequest {
                model_hash: model_hash.to_owned(),
                oslc_query: oslc.to_owned(),
                ..Default::default()
            },
            &capabilities,
        )?;
        Ok(elements_of(response))
    }

    /// Run a named document query, binding each parameter to its values.
    pub fn run_document_query<K: AsRef<str>>(
        &self,
        model_hash: &str,
        query_id: &str,
        bindings: &[(K, Vec<DocumentValue>)],
    ) -> Result<DocumentQueryResult, Error> {
        let mut bindings = bindings_to_wire(bindings)?;
        self.require_all(&[CAPABILITY_DOCUMENT_QUERY])?;
        if !self.capabilities().has(CAPABILITY_RATIONAL_VALUES) {
            bindings.iter_mut().for_each(binding_rationals_as_reals);
        }
        if bindings.iter().any(binding_holds_big_int) {
            self.require_all(&[CAPABILITY_BIG_INT_VALUES])?;
        }
        if bindings.iter().any(binding_holds_rational) {
            self.require_all(&[CAPABILITY_RATIONAL_VALUES])?;
        }
        let response = self.gated_rpc(
            "RunDocumentQuery",
            wire::RunDocumentQueryRequest {
                model_hash: model_hash.to_owned(),
                query_id: query_id.to_owned(),
                bindings,
            },
            &[CAPABILITY_DOCUMENT_QUERY],
        )?;
        result_of(response)
    }

    /// Render a named document in `form`.
    pub fn render_document(
        &self,
        model_hash: &str,
        document_id: &str,
        form: DocumentForm,
    ) -> Result<String, Error> {
        let mut capabilities = vec![CAPABILITY_RENDER_DOCUMENT];
        if form == DocumentForm::Html {
            capabilities.push(CAPABILITY_RENDER_DOCUMENT_HTML);
        }
        self.require_all(&capabilities)?;
        let response: wire::RenderDocumentResponse = self.gated_rpc(
            "RenderDocument",
            wire::RenderDocumentRequest {
                model_hash: model_hash.to_owned(),
                document_id: document_id.to_owned(),
                form: match form {
                    DocumentForm::Markdown => String::new(),
                    DocumentForm::Html => "html".to_owned(),
                },
            },
            &capabilities,
        )?;
        Ok(match form {
            DocumentForm::Markdown => response.markdown,
            DocumentForm::Html => response.html,
        })
    }

    /// Export the lowered graph of an action or state machine, and of every
    /// behavior it performs, as the canonical `graphs:1` JSON an external
    /// analysis engine is sent.
    pub fn export_graphs(&self, model_hash: &str, subject: &str) -> Result<Graphs, Error> {
        let capabilities = [CAPABILITY_EXPORT_GRAPHS];
        self.require_all(&capabilities)?;
        let response: wire::ExportGraphsResponse = self.gated_rpc(
            "ExportGraphs",
            wire::ExportGraphsRequest {
                model_hash: model_hash.to_owned(),
                subject: subject.to_owned(),
            },
            &capabilities,
        )?;
        Ok(Graphs::from_wire(response))
    }

    /// Render a named view with minimal ports.
    pub fn render_view(&self, model_hash: &str, view_name: &str) -> Result<RenderedView, Error> {
        self.render_view_with_ports(model_hash, view_name, RenderViewPorts::Minimal)
    }

    /// Render a named view with the requested port selection.
    pub fn render_view_with_ports(
        &self,
        model_hash: &str,
        view_name: &str,
        ports: RenderViewPorts,
    ) -> Result<RenderedView, Error> {
        self.require_all(&[crate::capabilities::CAPABILITY_RENDER_VIEW])?;
        let response = self.gated_rpc(
            "RenderView",
            wire::RenderViewRequest {
                model_hash: model_hash.to_owned(),
                view: view_name.to_owned(),
                ports: match ports {
                    RenderViewPorts::Minimal => String::new(),
                    RenderViewPorts::Full => "full".to_owned(),
                },
            },
            &[crate::capabilities::CAPABILITY_RENDER_VIEW],
        )?;
        Ok(rendered_view_of(response))
    }

    fn run_capabilities(
        &self,
        schedule: Option<&str>,
        performer: Option<&str>,
    ) -> Result<Vec<&'static str>, Error> {
        let mut capabilities = schedule_capabilities(schedule);
        if performer.is_some_and(|p| !p.is_empty()) {
            capabilities.push(CAPABILITY_PERFORMER);
        }
        self.require_all(&capabilities)?;
        Ok(capabilities)
    }

    fn run_action(
        &self,
        model_hash: &str,
        action_id: &str,
        inputs: &BTreeMap<String, Value>,
        schedule: Option<&str>,
        performer: Option<&str>,
    ) -> Result<wire::ExecuteActionResponse, Error> {
        let inputs = self.named_to_wire(inputs)?;
        let capabilities = self.run_capabilities(schedule, performer)?;
        self.gated_rpc(
            "ExecuteAction",
            wire::ExecuteActionRequest {
                model_hash: model_hash.to_owned(),
                action_symbol_id: action_id.to_owned(),
                inputs,
                schedule: schedule.unwrap_or_default().to_owned(),
                performer_symbol_id: performer.unwrap_or_default().to_owned(),
            },
            &capabilities,
        )
    }

    fn run_state<S: AsRef<str>>(
        &self,
        model_hash: &str,
        machine_id: &str,
        events: &[S],
        schedule: Option<&str>,
        performer: Option<&str>,
        trace: bool,
    ) -> Result<wire::ExecuteStateResponse, Error> {
        let mut capabilities = self.run_capabilities(schedule, performer)?;
        if trace {
            capabilities.push(CAPABILITY_STATE_TRACE);
        }
        self.gated_rpc(
            "ExecuteState",
            wire::ExecuteStateRequest {
                model_hash: model_hash.to_owned(),
                state_machine_symbol_id: machine_id.to_owned(),
                events: events.iter().map(|e| e.as_ref().to_owned()).collect(),
                schedule: schedule.unwrap_or_default().to_owned(),
                performer_symbol_id: performer.unwrap_or_default().to_owned(),
                trace,
            },
            &capabilities,
        )
    }

    /// Execute an action once with `inputs` bound to its input parameters.
    pub fn execute_action(
        &self,
        model_hash: &str,
        action_id: &str,
        inputs: &BTreeMap<String, Value>,
        options: &RunOptions,
    ) -> Result<ActionRun, Error> {
        Self::refuse_action_trace(options)?;
        refuse_exploring(options.schedule.as_deref(), "explore_action")?;
        let response = self.run_action(
            model_hash,
            action_id,
            inputs,
            options.schedule.as_deref(),
            options.performer.as_deref(),
        )?;
        let wire = response.clone();
        let diagnostics = diagnostics_of(&response.diagnostics);
        if !response.error.is_empty() {
            return Err(Error::Execution {
                message: response.error,
                reason: FailureReason::Unspecified,
                diagnostics,
                trace: Vec::new(),
                trace_dropped: 0,
            });
        }
        Ok(ActionRun {
            outputs: value_map(&response.outputs)?,
            performer: value_map(&response.performer_attributes)?,
            final_time: response.final_time,
            diagnostics,
            wire,
        })
    }

    /// Execute an action once per valid order of its choice points; `schedule` is `explore` or
    /// `explore:runs=<n>,depth=<d>`, and `None` is `explore`.
    pub fn explore_action(
        &self,
        model_hash: &str,
        action_id: &str,
        inputs: &BTreeMap<String, Value>,
        options: &RunOptions,
    ) -> Result<Exploration, Error> {
        Self::refuse_action_trace(options)?;
        let schedule = options.schedule.as_deref().unwrap_or(SCHEDULE_EXPLORE);
        require_exploring(schedule)?;
        let response = self.run_action(
            model_hash,
            action_id,
            inputs,
            Some(schedule),
            options.performer.as_deref(),
        )?;
        exploration_of(ExploredResponse::Action(Box::new(response)))
    }

    fn refuse_action_trace(options: &RunOptions) -> Result<(), Error> {
        if options.trace {
            return Err(Error::InvalidRequest(
                "a state trace is only valid for a state run".to_owned(),
            ));
        }
        Ok(())
    }

    /// Run a state machine once, dispatching `events` in order.
    pub fn execute_state<S: AsRef<str>>(
        &self,
        model_hash: &str,
        machine_id: &str,
        events: &[S],
        options: &RunOptions,
    ) -> Result<StateRun, Error> {
        refuse_exploring(options.schedule.as_deref(), "explore_state")?;
        let response = self.run_state(
            model_hash,
            machine_id,
            events,
            options.schedule.as_deref(),
            options.performer.as_deref(),
            options.trace,
        )?;
        let wire = response.clone();
        let diagnostics = diagnostics_of(&response.diagnostics);
        if !response.error.is_empty() {
            return Err(state_failure(response, diagnostics)?);
        }
        Ok(StateRun {
            states_visited: response.states_visited,
            final_context: value_map(&response.final_context)?,
            final_time: response.final_time,
            trace: response
                .trace
                .into_iter()
                .map(document_event_from_wire)
                .collect::<Result<_, _>>()?,
            trace_dropped: response.trace_dropped,
            diagnostics,
            wire,
        })
    }

    /// Run a state machine once per valid order of its choice points, as [`Self::explore_action`].
    pub fn explore_state<S: AsRef<str>>(
        &self,
        model_hash: &str,
        machine_id: &str,
        events: &[S],
        options: &RunOptions,
    ) -> Result<Exploration, Error> {
        let schedule = options.schedule.as_deref().unwrap_or(SCHEDULE_EXPLORE);
        require_exploring(schedule)?;
        if options.trace {
            return Err(Error::InvalidRequest(
                "a trace describes one run, not an exploration".to_owned(),
            ));
        }
        let response = self.run_state(
            model_hash,
            machine_id,
            events,
            Some(schedule),
            options.performer.as_deref(),
            false,
        )?;
        exploration_of(ExploredResponse::State(Box::new(response)))
    }

    /// The engines the service registers, served or not.
    pub fn list_engines(&self) -> Result<Vec<EngineInfo>, Error> {
        self.require_all(&[CAPABILITY_ENGINES])?;
        let response: wire::ListEnginesResponse = self.gated_rpc(
            "ListEngines",
            wire::ListEnginesRequest {},
            &[CAPABILITY_ENGINES],
        )?;
        Ok(response.engines.into_iter().map(EngineInfo::from).collect())
    }

    fn verification_capabilities(
        &self,
        engine: Option<&str>,
        question: Option<&str>,
    ) -> Result<Vec<&'static str>, Error> {
        let mut capabilities = vec![CAPABILITY_VERIFICATION];
        capabilities.extend(engine_capabilities(engine));
        capabilities.extend(question_capabilities(question));
        self.require_all(&capabilities)?;
        Ok(capabilities)
    }

    /// Ask whether a constraint holds, as `%verify` does.
    pub fn verify_constraint(
        &self,
        model_hash: &str,
        constraint_id: &str,
        options: &VerifyOptions,
    ) -> Result<Verdict, Error> {
        let capabilities =
            self.verification_capabilities(options.engine.as_deref(), options.question.as_deref())?;
        let response: wire::VerifyConstraintResponse = self.gated_rpc(
            "VerifyConstraint",
            wire::VerifyConstraintRequest {
                model_hash: model_hash.to_owned(),
                symbol_id: constraint_id.to_owned(),
                subject_symbol_id: options.subject.clone().unwrap_or_default(),
                engine: engine_field(options.engine.as_deref()),
                question: question_field(options.question.as_deref()),
            },
            &capabilities,
        )?;
        single_verdict(
            response.verdict,
            &response.instances,
            &response.error,
            &response.diagnostics,
            &[],
            self.capabilities(),
        )
    }

    /// Ask whether a requirement holds of its subject, with the verdicts of the cases verifying it.
    pub fn verify_requirement(
        &self,
        model_hash: &str,
        requirement_id: &str,
        options: &VerifyOptions,
    ) -> Result<Verdict, Error> {
        let capabilities =
            self.verification_capabilities(options.engine.as_deref(), options.question.as_deref())?;
        let response: wire::VerifyRequirementResponse = self.gated_rpc(
            "VerifyRequirement",
            wire::VerifyRequirementRequest {
                model_hash: model_hash.to_owned(),
                symbol_id: requirement_id.to_owned(),
                subject_symbol_id: options.subject.clone().unwrap_or_default(),
                engine: engine_field(options.engine.as_deref()),
                question: question_field(options.question.as_deref()),
            },
            &capabilities,
        )?;
        single_verdict(
            response.verdict,
            &response.instances,
            &response.error,
            &response.diagnostics,
            &response.verification_verdicts,
            self.capabilities(),
        )
    }

    /// Ask whether the model's satisfaction assertions hold, as `%satisfy` does: those within
    /// `scope`, or every one the model states. `options.subject` must be `None`.
    pub fn verify_satisfaction(
        &self,
        model_hash: &str,
        scope: Option<&str>,
        options: &VerifyOptions,
    ) -> Result<Satisfaction, Error> {
        if let Some(subject) = &options.subject {
            return Err(Error::InvalidRequest(format!(
                "a satisfaction assertion names its own subject; {subject:?} cannot be given"
            )));
        }
        let capabilities =
            self.verification_capabilities(options.engine.as_deref(), options.question.as_deref())?;
        let response = self.gated_rpc(
            "VerifySatisfaction",
            wire::VerifySatisfactionRequest {
                model_hash: model_hash.to_owned(),
                symbol_id: scope.unwrap_or_default().to_owned(),
                engine: engine_field(options.engine.as_deref()),
                question: question_field(options.question.as_deref()),
            },
            &capabilities,
        )?;
        satisfaction_of(response, self.capabilities())
    }

    /// Check every assertion about an object of `part_id` and the objects it holds, as
    /// `%validate` does.
    pub fn validate_instance(
        &self,
        model_hash: &str,
        part_id: &str,
        engine: Option<&str>,
    ) -> Result<Validation, Error> {
        let capabilities = self.verification_capabilities(engine, None)?;
        let response = self.gated_rpc(
            "ValidateInstance",
            wire::ValidateInstanceRequest {
                model_hash: model_hash.to_owned(),
                symbol_id: part_id.to_owned(),
                engine: engine_field(engine),
            },
            &capabilities,
        )?;
        validation_of(response, self.capabilities())
    }

    /// Invoke a calculation with positional `arguments`, as `%calc` does.
    pub fn calc(
        &self,
        model_hash: &str,
        calc_id: &str,
        arguments: &[Value],
        engine: Option<&str>,
    ) -> Result<CalcResult, Error> {
        let capabilities = self.verification_capabilities(engine, None)?;
        let arguments = self.values_to_wire(arguments)?;
        let response = self.gated_rpc(
            "EvaluateCalc",
            wire::EvaluateCalcRequest {
                model_hash: model_hash.to_owned(),
                symbol_id: calc_id.to_owned(),
                arguments,
                engine: engine_field(engine),
            },
            &with_values(capabilities),
        )?;
        calc_of(response)
    }

    fn analysis_request(
        &self,
        model_hash: &str,
        case_id: &str,
        options: &AnalysisOptions,
        schedule: Option<&str>,
    ) -> Result<wire::RunAnalysisResponse, Error> {
        let mut capabilities = self.verification_capabilities(options.engine.as_deref(), None)?;
        let scheduled = schedule_capabilities(schedule);
        self.require_all(&scheduled)?;
        capabilities.extend(scheduled);
        let request = wire::RunAnalysisRequest {
            model_hash: model_hash.to_owned(),
            symbol_id: case_id.to_owned(),
            subject_symbol_id: options.subject.clone().unwrap_or_default(),
            arguments: self.values_to_wire(&options.arguments)?,
            named_arguments: self.named_to_wire(&options.named_arguments)?,
            schedule: schedule.unwrap_or_default().to_owned(),
            engine: engine_field(options.engine.as_deref()),
        };
        self.gated_rpc("RunAnalysis", request, &with_values(capabilities))
    }

    /// Run an analysis case, as `%analysis` does.
    ///
    /// A run that failed but left something to inspect is [`Error::AnalysisRun`], carrying it.
    pub fn run_analysis(
        &self,
        model_hash: &str,
        case_id: &str,
        options: &AnalysisOptions,
    ) -> Result<AnalysisResult, Error> {
        refuse_exploring(options.schedule.as_deref(), "explore_analysis")?;
        if options.engine.as_deref() == Some(ENGINE_EXPLORE) {
            return Err(Error::InvalidRequest(
                "engine 'explore' answers with every outcome, not one run's result: use \
                 explore_analysis"
                    .to_owned(),
            ));
        }
        let response =
            self.analysis_request(model_hash, case_id, options, options.schedule.as_deref())?;
        analysis_of(response, self.capabilities())
    }

    /// Run an analysis case once per valid order of the choice points its actions meet;
    /// `options.schedule` is `explore` or `explore:runs=<n>,depth=<d>`, and `None` is `explore`.
    /// `options.engine` must be `None`.
    pub fn explore_analysis(
        &self,
        model_hash: &str,
        case_id: &str,
        options: &AnalysisOptions,
    ) -> Result<Exploration, Error> {
        let schedule = options.schedule.as_deref().unwrap_or(SCHEDULE_EXPLORE);
        require_exploring(schedule)?;
        if let Some(engine) = &options.engine {
            return Err(Error::InvalidRequest(format!(
                "an exploration runs every order itself; engine {engine:?} cannot be given"
            )));
        }
        let response = self.analysis_request(model_hash, case_id, options, Some(schedule))?;
        exploration_of(ExploredResponse::Analysis(Box::new(response)))
    }

    /// Run an analysis case or calc once per row of a parameter sweep over `ranges`.
    pub fn run_sweep(
        &self,
        model_hash: &str,
        target_id: &str,
        ranges: &[SweepRange],
        options: &SweepOptions,
    ) -> Result<SweepTable, Error> {
        let capabilities = self.verification_capabilities(options.engine.as_deref(), None)?;
        let ranges = ranges
            .iter()
            .map(|range| {
                Ok(wire::SweepRange {
                    parameter: range.parameter.clone(),
                    start: Some(value_to_wire(&range.start, self.capabilities())?),
                    end: Some(value_to_wire(&range.end, self.capabilities())?),
                    step: range
                        .step
                        .as_ref()
                        .map(|step| value_to_wire(step, self.capabilities()))
                        .transpose()?,
                })
            })
            .collect::<Result<_, Error>>()?;
        let request = wire::RunSweepRequest {
            model_hash: model_hash.to_owned(),
            symbol_id: target_id.to_owned(),
            subject_symbol_id: options.subject.clone().unwrap_or_default(),
            arguments: self.values_to_wire(&options.arguments)?,
            named_arguments: self.named_to_wire(&options.named_arguments)?,
            ranges,
            samples: options.samples,
            seed: options.seed,
            engine: engine_field(options.engine.as_deref()),
        };
        let response = self.gated_rpc("RunSweep", request, &with_values(capabilities))?;
        sweep_of(response, self.capabilities())
    }
}

fn with_values(mut capabilities: Vec<&'static str>) -> Vec<&'static str> {
    capabilities.extend_from_slice(VALUE_CAPABILITIES);
    capabilities
}

fn state_failure(
    response: wire::ExecuteStateResponse,
    diagnostics: Vec<crate::domain::Diagnostic>,
) -> Result<Error, Error> {
    let trace = response
        .trace
        .into_iter()
        .map(document_event_from_wire)
        .collect::<Result<_, _>>()?;
    Ok(Error::Execution {
        message: response.error,
        reason: FailureReason::Unspecified,
        diagnostics,
        trace,
        trace_dropped: response.trace_dropped,
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn schedules_and_engines_read_as_the_service_does() {
        assert!(explores(Some("explore")));
        assert!(explores(Some("explore:runs=3,depth=4")));
        assert!(!explores(Some("explorer")));
        assert!(!explores(None));
        assert!(refuse_exploring(Some("explore"), "explore_action").is_err());
        assert!(refuse_exploring(Some("random:seed=1"), "explore_action").is_ok());
        assert!(require_exploring("first").is_err());
        assert_eq!(engine_field(Some(ENGINE_AUTO)), "");
        assert_eq!(engine_field(Some("smt")), "smt");
        assert_eq!(question_field(Some(QUESTION_EVALUATE)), "");
        assert_eq!(engine_capabilities(None), Vec::<&str>::new());
        assert_eq!(
            engine_capabilities(Some(ENGINE_EXPLORE)),
            [CAPABILITY_ENGINES, CAPABILITY_SCHEDULE_EXPLORE]
        );
        assert_eq!(
            question_capabilities(Some("holds")),
            [CAPABILITY_VERIFICATION_QUESTIONS]
        );
        assert_eq!(
            schedule_capabilities(Some("explore")),
            [CAPABILITY_SCHEDULE, CAPABILITY_SCHEDULE_EXPLORE]
        );
    }

    #[test]
    fn failed_state_run_keeps_its_partial_trace() {
        let response = wire::ExecuteStateResponse {
            error: "state machine failed".to_owned(),
            trace: vec![wire::DocumentEvent {
                kind: "entry".to_owned(),
                time: Some(Box::new(wire::DocumentValue {
                    element_type: String::new(),
                    kind: Some(wire::document_value::Kind::RealValue(1.5)),
                })),
                state: "active".to_owned(),
                text: "enter: active".to_owned(),
                ..Default::default()
            }],
            trace_dropped: 2,
            ..Default::default()
        };

        let Error::Execution {
            message,
            trace,
            trace_dropped,
            ..
        } = state_failure(response, Vec::new()).unwrap()
        else {
            panic!("failed state run did not retain its execution error");
        };
        assert_eq!(message, "state machine failed");
        assert_eq!(trace.len(), 1);
        assert_eq!(trace[0].kind, "entry");
        assert_eq!(trace[0].state, "active");
        assert_eq!(trace_dropped, 2);
    }
}
