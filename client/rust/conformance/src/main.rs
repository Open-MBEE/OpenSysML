mod compare;
mod normalize;
mod scenario;

use std::collections::{BTreeMap, HashMap};
use std::env;
use std::fs;
use std::io::{BufRead, BufReader};
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::time::Instant;

use base64::prelude::*;
use compare::{compare, label_instance_ids, status_matches};
use normalize::normalize;
use opensysml::{
    wire, AnalysisOptions, Connection, ConvertOptions, ConvertSource, DocumentForm, DocumentValue,
    EditError, Error, EvalOptions, ExploredResponse, FailureReason, IdForm, Language, Layout,
    MigrateOptions, MigrateSource, Model, ModelResponse, ParseOptions, Query, RunOptions,
    SourceDocument, SourcesOptions, Status, SweepOptions, SweepRange, Value as SysmlValue,
    VerifyOptions,
};
use prost::Message;
use prost_reflect::{DescriptorPool, DeserializeOptions, DynamicMessage, SerializeOptions};
use serde::Serialize;
use serde_json::{Deserializer, Value};

use crate::scenario::{load_scenarios, ModelSpec, Scenario};

const SERVICE_NAME: &str = "sysml.SysMLService";
const DESCRIPTOR: &[u8] = include_bytes!("../sysml.descriptor.binpb");

#[derive(Debug, Serialize)]
struct Report {
    service: String,
    total: usize,
    passed: usize,
    failed: usize,
    skipped: usize,
    errored: usize,
    protocols: Vec<Summary>,
}

#[derive(Debug, Serialize)]
struct Summary {
    protocol: String,
    service: String,
    capabilities: Vec<String>,
    total: usize,
    passed: usize,
    failed: usize,
    skipped: usize,
    errored: usize,
    results: Vec<ResultRecord>,
}

#[derive(Debug, Serialize)]
struct ResultRecord {
    id: String,
    outcome: String,
    rpc: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    reason: String,
    #[serde(skip_serializing_if = "Vec::is_empty")]
    failures: Vec<String>,
    status: String,
    duration_ms: f64,
}

struct ServiceGuard {
    child: Child,
    address: String,
}

impl Drop for ServiceGuard {
    fn drop(&mut self) {
        let _ = self.child.kill();
        let _ = self.child.wait();
    }
}

struct Runner {
    connection: Connection,
    fixtures: PathBuf,
    models: HashMap<ModelSpec, Model>,
    pool: DescriptorPool,
}

enum Answer {
    Response(Value),
    Transport(Status, String),
    InBand(Value),
    Unrepresentable(String),
    Other(Error),
}

fn main() {
    if let Err(error) = run() {
        eprintln!("conformance: {error}");
        std::process::exit(1);
    }
}

fn run() -> Result<(), String> {
    let options = Options::parse()?;
    let root = repository_root()?;
    let scenarios_dir = options
        .scenarios
        .unwrap_or_else(|| root.join("conformance").join("scenarios"));
    let fixtures_dir = options
        .fixtures
        .unwrap_or_else(|| root.join("conformance").join("fixtures"));
    let scenarios = load_scenarios(&scenarios_dir)?;
    let binary = options
        .binary
        .or_else(|| env::var_os("OPENSYSML_GRPC_BINARY").map(PathBuf::from))
        .unwrap_or_else(|| root.join("bin").join(binary_name()));
    if !binary.is_file() {
        return Err(format!("sysml-grpc binary not found: {}", binary.display()));
    }
    let service = start_service(&binary)?;
    let (host, port) = split_address(&service.address)?;
    let connection =
        Connection::external(host, port).map_err(|error| format!("connect: {error}"))?;
    let pool =
        DescriptorPool::decode(DESCRIPTOR).map_err(|error| format!("descriptor: {error}"))?;
    let mut runner = Runner {
        connection,
        fixtures: fixtures_dir,
        models: HashMap::new(),
        pool,
    };
    let mut capabilities = runner.connection.server_info().wire().capabilities.clone();
    capabilities.sort();
    let mut summary = Summary {
        protocol: "connect".to_owned(),
        service: binary.display().to_string(),
        capabilities,
        total: 0,
        passed: 0,
        failed: 0,
        skipped: 0,
        errored: 0,
        results: Vec::new(),
    };
    for scenario in scenarios {
        if let Some(pattern) = &options.run {
            if !scenario.id.contains(pattern) {
                continue;
            }
        }
        let result = runner.run(&scenario);
        print_result(&result, options.verbose);
        summary.total += 1;
        match result.outcome.as_str() {
            "pass" => summary.passed += 1,
            "fail" => summary.failed += 1,
            "skip" => summary.skipped += 1,
            _ => summary.errored += 1,
        }
        summary.results.push(result);
    }
    let report = Report {
        service: binary.display().to_string(),
        total: summary.total,
        passed: summary.passed,
        failed: summary.failed,
        skipped: summary.skipped,
        errored: summary.errored,
        protocols: vec![summary],
    };
    println!(
        "total={} passed={} failed={} skipped={} errored={}",
        report.total, report.passed, report.failed, report.skipped, report.errored
    );
    if let Some(path) = options.report {
        let mut data = serde_json::to_vec_pretty(&report).map_err(|error| error.to_string())?;
        data.push(b'\n');
        if path.as_os_str() == "-" {
            print!("{}", String::from_utf8_lossy(&data));
        } else {
            fs::write(&path, data)
                .map_err(|error| format!("writing report {}: {error}", path.display()))?;
        }
    }
    if report.failed > 0 || report.errored > 0 {
        return Err(format!(
            "{} failed or errored scenarios",
            report.failed + report.errored
        ));
    }
    let unexpected_skips = report
        .protocols
        .iter()
        .flat_map(|summary| summary.results.iter())
        .filter(|result| result.outcome == "skip" && !is_expected_skip(&result.reason))
        .count();
    if unexpected_skips > 0 && !options.allow_skips {
        return Err(format!(
            "{} scenarios skipped for missing capabilities; use -allow-skips",
            unexpected_skips
        ));
    }
    Ok(())
}

const UNREPRESENTABLE: &str = "unrepresentable by the typed API: ";

fn is_expected_skip(reason: &str) -> bool {
    reason.starts_with(UNREPRESENTABLE)
}

impl Runner {
    fn run(&mut self, scenario: &Scenario) -> ResultRecord {
        let started = Instant::now();
        let mut result = ResultRecord {
            id: scenario.id.clone(),
            outcome: "pass".to_owned(),
            rpc: scenario.method().to_owned(),
            reason: String::new(),
            failures: Vec::new(),
            status: "OK".to_owned(),
            duration_ms: 0.0,
        };
        let missing = scenario
            .requires_capabilities
            .iter()
            .filter(|name| !self.connection.capabilities().has(name))
            .cloned()
            .collect::<Vec<_>>();
        let expect = if missing.is_empty() {
            &scenario.expect
        } else if let Some(expect) = scenario.expect_without_capability.as_ref() {
            expect
        } else {
            result.outcome = "skip".to_owned();
            result.status = "-".to_owned();
            result.reason = format!("missing capability {}", missing.join(", "));
            result.duration_ms = elapsed_ms(started);
            return result;
        };
        let model = match scenario.model.as_ref() {
            Some(spec) => match self.model(spec) {
                Ok(model) => Some(model),
                Err(error) => return errored(result, error, started),
            },
            None => None,
        };
        let request = match self.request(scenario, model.as_ref().map(Model::hash)) {
            Ok(request) => request,
            Err(error) => return errored(result, error, started),
        };
        match self.call(scenario.method(), &request, model.as_ref()) {
            Answer::Unrepresentable(what) => {
                result.outcome = "skip".to_owned();
                result.status = "-".to_owned();
                result.reason = format!("{UNREPRESENTABLE}{what}");
            }
            Answer::Transport(status, message) => {
                result.status = status.canonical_name().to_owned();
                let want = expect.status.as_deref().unwrap_or("OK");
                if !status_matches(expect.status.as_deref(), status) {
                    result.outcome = "fail".to_owned();
                    result.failures.push(format!(
                        "status: {} ({message}), want {want}",
                        result.status
                    ));
                } else if let Some(needle) = &expect.status_message_contains {
                    if !message.contains(needle) {
                        result.outcome = "fail".to_owned();
                        result.failures.push(format!(
                            "status message {message:?} does not contain {needle:?}"
                        ));
                    }
                }
            }
            Answer::InBand(mut actual) => {
                if !status_matches(expect.status.as_deref(), Status::Ok) {
                    result.outcome = "fail".to_owned();
                    result.failures.push(format!(
                        "the call answered OK with in-band error {:?}, want {}",
                        actual.get("error").and_then(Value::as_str).unwrap_or(""),
                        expect.status.as_deref().unwrap_or("OK")
                    ));
                } else {
                    normalize(&mut actual, model.as_ref().map(Model::hash).unwrap_or(""));
                    label_instance_ids(&mut actual);
                    result.failures = compare(expect, &actual);
                    if !result.failures.is_empty() {
                        result.outcome = "fail".to_owned();
                    }
                }
            }
            Answer::Response(mut actual) => {
                result.status = "OK".to_owned();
                if !status_matches(expect.status.as_deref(), Status::Ok) {
                    result.outcome = "fail".to_owned();
                    result.failures.push(format!(
                        "the call succeeded, want status {}",
                        expect.status.as_deref().unwrap_or("OK")
                    ));
                } else {
                    normalize(&mut actual, model.as_ref().map(Model::hash).unwrap_or(""));
                    label_instance_ids(&mut actual);
                    result.failures = compare(expect, &actual);
                    if !result.failures.is_empty() {
                        result.outcome = "fail".to_owned();
                    }
                }
            }
            Answer::Other(error) => return errored(result, error.to_string(), started),
        }
        result.duration_ms = elapsed_ms(started);
        result
    }

    fn model(&mut self, spec: &ModelSpec) -> Result<Model, String> {
        if let Some(model) = self.models.get(spec) {
            return Ok(model.clone());
        }
        if !spec.fixtures.is_empty() {
            let documents = spec
                .fixtures
                .iter()
                .map(|name| {
                    let path = fixture_path(&self.fixtures, name)?;
                    let content = fs::read_to_string(path)
                        .map_err(|error| format!("reading fixture {name}: {error}"))?;
                    Ok(if spec.language.eq_ignore_ascii_case("kerml") {
                        SourceDocument::inline_in(name.clone(), content, Language::Kerml)
                    } else {
                        SourceDocument::inline(name.clone(), content)
                    })
                })
                .collect::<Result<Vec<_>, String>>()?;
            let options = SourcesOptions {
                strict_conformance: spec.strict_conformance,
                ..Default::default()
            };
            let model = self
                .connection
                .parse_sources(&documents, &options)
                .map_err(|error| format!("parsing fixtures {:?}: {error}", spec.fixtures))?;
            self.models.insert(spec.clone(), model.clone());
            return Ok(model);
        }
        let path = fixture_path(&self.fixtures, &spec.fixture)?;
        let source = fs::read_to_string(path)
            .map_err(|error| format!("reading fixture {}: {error}", spec.fixture))?;
        let options = ParseOptions {
            language: if spec.language.eq_ignore_ascii_case("kerml") {
                Language::Kerml
            } else {
                Language::Sysml
            },
            strict_conformance: spec.strict_conformance,
        };
        let model = self
            .connection
            .parse_content(&source, &options)
            .map_err(|error| format!("parsing fixture {}: {error}", spec.fixture))?;
        self.models.insert(spec.clone(), model.clone());
        Ok(model)
    }

    fn request(
        &self,
        scenario: &Scenario,
        model_hash: Option<&str>,
    ) -> Result<DynamicMessage, String> {
        let method = self.method(scenario.method())?;
        let mut tree = scenario.request.clone();
        resolve_placeholders(&mut tree, model_hash, &self.fixtures)?;
        let text = serde_json::to_string(&tree).map_err(|error| error.to_string())?;
        let mut deserializer = Deserializer::from_str(&text);
        let request = DynamicMessage::deserialize_with_options(
            method.input(),
            &mut deserializer,
            &DeserializeOptions::new(),
        )
        .map_err(|error| {
            format!(
                "request does not fit {}: {error}",
                method.input().full_name()
            )
        })?;
        deserializer
            .end()
            .map_err(|error| format!("request has trailing data: {error}"))?;
        Ok(request)
    }

    fn method(&self, name: &str) -> Result<prost_reflect::MethodDescriptor, String> {
        self.pool
            .get_service_by_name(SERVICE_NAME)
            .ok_or_else(|| format!("schema has no service {SERVICE_NAME}"))
            .and_then(|service| {
                service
                    .methods()
                    .find(|method| method.name() == name)
                    .ok_or_else(|| format!("{SERVICE_NAME} has no RPC {name:?}"))
            })
    }

    fn call(&self, method: &str, request: &DynamicMessage, model: Option<&Model>) -> Answer {
        match method {
            "GetServerInfo" => Answer::Response(self.wire_json(
                "sysml.ServerInfoResponse",
                self.connection.server_info().wire(),
            )),
            "ParseFile" => self.parse_file(request),
            "GetDiagnostics" => self.get_diagnostics(request),
            "GetSymbol" => self.get_symbol(request, model),
            "Evaluate" => self.evaluate(request, model),
            "Instantiate" => self.instantiate(request, model),
            "ParseSources" => self.parse_sources(request),
            "Convert" => self.convert(request),
            "Migrate" => self.migrate(request),
            "ApplyEdits" => self.apply_edits(request, model),
            "Query" => self.query(request),
            "RunDocumentQuery" => self.run_document_query(request),
            "RenderDocument" => self.render_document(request),
            "ExecuteAction" => self.execute_action(request),
            "ExecuteState" => self.execute_state(request),
            "ListEngines" => self.list_engines(),
            "VerifyConstraint" => self.verify_constraint(request),
            "VerifyRequirement" => self.verify_requirement(request),
            "VerifySatisfaction" => self.verify_satisfaction(request),
            "ValidateInstance" => self.validate_instance(request),
            "EvaluateCalc" => self.evaluate_calc(request),
            "RunAnalysis" => self.run_analysis(request),
            "RunSweep" => self.run_sweep(request),
            other => Answer::Unrepresentable(format!("no typed method calls {other}")),
        }
    }

    /// The model the scenario carries, or the one the server holds under the hash.
    fn model_for(&self, hash: &str, model: Option<&Model>) -> Model {
        model
            .cloned()
            .unwrap_or_else(|| self.connection.model_by_hash(hash))
    }

    fn parse_file(&self, request: &DynamicMessage) -> Answer {
        let options = ParseOptions {
            language: request
                .get_field_by_name("language")
                .and_then(|value| value.as_ref().as_str().map(|value| value.to_owned()))
                .map(|value| {
                    if value.eq_ignore_ascii_case("kerml") {
                        Language::Kerml
                    } else {
                        Language::Sysml
                    }
                })
                .unwrap_or(Language::Sysml),
            strict_conformance: request
                .get_field_by_name("strict_conformance")
                .and_then(|value| value.as_ref().as_bool())
                .unwrap_or(false),
        };
        let answer = match request_source(request) {
            Some(Source::Content(content)) => self.connection.parse_content(&content, &options),
            Some(Source::File(path)) => self.connection.parse_file(path, &options),
            None => return Answer::Unrepresentable("ParseFile with no source".to_owned()),
        };
        match answer {
            Ok(model) => self.model_json(&model),
            Err(error) => classify_error(error),
        }
    }

    fn get_diagnostics(&self, request: &DynamicMessage) -> Answer {
        let hash = match string_field(request, "model_hash") {
            Ok(hash) => hash,
            Err(error) => return Answer::Other(error),
        };
        match self.connection.diagnostics(&hash) {
            Ok(diagnostics) => {
                let response = opensysml::wire::DiagnosticsResponse {
                    diagnostics: diagnostics
                        .iter()
                        .map(|diagnostic| diagnostic.wire().clone())
                        .collect(),
                    error: String::new(),
                };
                Answer::Response(self.wire_json("sysml.DiagnosticsResponse", &response))
            }
            Err(error) => classify_error(error),
        }
    }

    fn get_symbol(&self, request: &DynamicMessage, model: Option<&Model>) -> Answer {
        let hash = match string_field(request, "model_hash") {
            Ok(hash) => hash,
            Err(error) => return Answer::Other(error),
        };
        let symbol_id = match string_field(request, "symbol_id") {
            Ok(value) => value,
            Err(error) => return Answer::Other(error),
        };
        match self.model_for(&hash, model).symbol(&symbol_id) {
            Ok(symbol) => {
                let response = opensysml::wire::SymbolResponse {
                    symbol: Some(symbol.wire().clone()),
                    error: String::new(),
                };
                Answer::Response(self.wire_json("sysml.SymbolResponse", &response))
            }
            Err(error) => classify_error(error),
        }
    }

    fn evaluate(&self, request: &DynamicMessage, model: Option<&Model>) -> Answer {
        let hash = match string_field(request, "model_hash") {
            Ok(hash) => hash,
            Err(error) => return Answer::Other(error),
        };
        let expression = match string_field(request, "expression") {
            Ok(value) => value,
            Err(error) => return Answer::Other(error),
        };
        let options = EvalOptions {
            context: string_or_none(request, "context_symbol_id"),
            subject: string_or_none(request, "subject_symbol_id"),
        };
        match self.model_for(&hash, model).evaluate(&expression, &options) {
            Ok(evaluation) => {
                Answer::Response(self.wire_json("sysml.EvaluateResponse", evaluation.wire()))
            }
            Err(error) => classify_error(error),
        }
    }

    fn instantiate(&self, request: &DynamicMessage, model: Option<&Model>) -> Answer {
        let hash = match string_field(request, "model_hash") {
            Ok(hash) => hash,
            Err(error) => return Answer::Other(error),
        };
        let symbol_id = match string_field(request, "symbol_id") {
            Ok(value) => value,
            Err(error) => return Answer::Other(error),
        };
        match self.model_for(&hash, model).instantiate(&symbol_id) {
            Ok(instantiation) => {
                Answer::Response(self.wire_json("sysml.InstantiateResponse", instantiation.wire()))
            }
            Err(error) => classify_error(error),
        }
    }

    fn model_json(&self, model: &Model) -> Answer {
        Answer::Response(match model.wire() {
            ModelResponse::File(response) => {
                self.wire_json("sysml.ParseFileResponse", response.as_ref())
            }
            ModelResponse::Sources(response) => {
                self.wire_json("sysml.ParseSourcesResponse", response)
            }
            ModelResponse::Hash => serde_json::json!({ "model_hash": model.hash() }),
        })
    }

    fn in_band<M: Message>(&self, name: &str, response: &M) -> Answer {
        Answer::InBand(self.wire_json(name, response))
    }

    fn parse_sources(&self, request: &DynamicMessage) -> Answer {
        let request: wire::ParseSourcesRequest = match decode(request) {
            Ok(request) => request,
            Err(answer) => return answer,
        };
        use wire::source_document::Source as Kind;
        let mut documents = Vec::new();
        for document in request.documents {
            let language = match document.language.as_str() {
                "" => None,
                "sysml" => Some(Language::Sysml),
                "kerml" => Some(Language::Kerml),
                other => return Answer::Unrepresentable(format!("document language {other:?}")),
            };
            documents.push(match document.source {
                Some(Kind::FilePath(path)) if document.name.is_empty() => {
                    SourceDocument::file(path)
                }
                Some(Kind::FilePath(_)) => {
                    return Answer::Unrepresentable("a named file document".to_owned())
                }
                Some(Kind::Content(content)) => SourceDocument::Inline {
                    name: document.name,
                    content,
                    language,
                },
                None => return Answer::Unrepresentable("a document with no source".to_owned()),
            });
        }
        let options = SourcesOptions {
            strict_conformance: request.strict_conformance,
            ..Default::default()
        };
        match self.connection.parse_sources(&documents, &options) {
            Ok(model) => self.model_json(&model),
            Err(error) => match Failure::of(error) {
                Ok(failure) => self.in_band(
                    "sysml.ParseSourcesResponse",
                    &wire::ParseSourcesResponse {
                        error: failure.error,
                        diagnostics: failure.diagnostics,
                        ..Default::default()
                    },
                ),
                Err(answer) => answer,
            },
        }
    }

    fn convert(&self, request: &DynamicMessage) -> Answer {
        let request: wire::ConvertRequest = match decode(request) {
            Ok(request) => request,
            Err(answer) => return answer,
        };
        use wire::convert_request::Source as Kind;
        let source = match request.source {
            Some(Kind::FilePath(path)) => ConvertSource::File(PathBuf::from(path)),
            Some(Kind::Content(content)) => ConvertSource::Content(content),
            Some(Kind::ModelHash(hash)) => ConvertSource::Model(hash),
            None => return Answer::Unrepresentable("Convert with no source".to_owned()),
        };
        let id_form = match request.id_form.as_str() {
            "" => None,
            "qualified" => Some(IdForm::Qualified),
            "uuid" => Some(IdForm::Uuid),
            other => return Answer::Unrepresentable(format!("id form {other:?}")),
        };
        let options = ConvertOptions {
            from_format: request.from_format,
            tolerate_syntax_errors: request.tolerate_syntax_errors,
            id_form,
        };
        match self
            .connection
            .convert(&request.to_format, &source, &options)
        {
            Ok(conversion) => {
                Answer::Response(self.wire_json("sysml.ConvertResponse", conversion.wire()))
            }
            Err(error) => match Failure::of(error) {
                Ok(failure) => self.in_band(
                    "sysml.ConvertResponse",
                    &wire::ConvertResponse {
                        error: failure.error,
                        diagnostics: failure.diagnostics,
                        ..Default::default()
                    },
                ),
                Err(answer) => answer,
            },
        }
    }

    fn migrate(&self, request: &DynamicMessage) -> Answer {
        let request: wire::MigrateRequest = match decode(request) {
            Ok(request) => request,
            Err(answer) => return answer,
        };
        use wire::migrate_request::{Layout as WireLayout, Source as Kind};
        let source = match request.source {
            Some(Kind::FilePath(path)) => MigrateSource::File(PathBuf::from(path)),
            Some(Kind::Content(content)) => MigrateSource::Content(content),
            None => return Answer::Unrepresentable("Migrate with no source".to_owned()),
        };
        let options = MigrateOptions {
            from_format: request.from_format,
            report: request.report,
            results: request.results,
            layout: request.layout.map(|layout| match layout {
                WireLayout::LayoutPath(path) => Layout::Path(PathBuf::from(path)),
                WireLayout::LayoutContent(content) => Layout::Content(content),
            }),
            image_base_url: request.image_base_url,
            strict: request.strict,
        };
        match self
            .connection
            .migrate(&request.to_format, &source, &options)
        {
            Ok(migration) => {
                Answer::Response(self.wire_json("sysml.MigrateResponse", migration.wire()))
            }
            Err(Error::Migration { message }) => self.in_band(
                "sysml.MigrateResponse",
                &wire::MigrateResponse {
                    error: message,
                    ..Default::default()
                },
            ),
            Err(error) => classify_error(error),
        }
    }

    fn apply_edits(&self, request: &DynamicMessage, model: Option<&Model>) -> Answer {
        let request: wire::ApplyEditsRequest = match decode(request) {
            Ok(request) => request,
            Err(answer) => return answer,
        };
        if !request.accept_documents && model.is_some_and(|model| model.documents().len() > 1) {
            return Answer::Unrepresentable(
                "an edit of a model of several documents that does not read them".to_owned(),
            );
        }
        let mut editor = self.model_for(&request.model_hash, model).edit();
        if !request.document.is_empty() {
            editor.in_document(request.document);
        }
        for operation in request.operations {
            editor.add_operation(operation);
        }
        match editor.apply() {
            Ok(result) => {
                Answer::Response(self.wire_json("sysml.ApplyEditsResponse", result.wire()))
            }
            Err(Error::Edit(refused)) => {
                let EditError {
                    message,
                    failure,
                    diagnostics,
                    referring_elements,
                    referrers,
                } = *refused;
                self.in_band(
                    "sysml.ApplyEditsResponse",
                    &wire::ApplyEditsResponse {
                        error: message,
                        failure: failure.code(),
                        diagnostics: diagnostics_wire(&diagnostics),
                        referring_elements,
                        referrers: referrers
                            .into_iter()
                            .map(|referrer| wire::Referrer {
                                name: referrer.name,
                                document: referrer.document,
                            })
                            .collect(),
                        ..Default::default()
                    },
                )
            }
            Err(error) => classify_error(error),
        }
    }

    fn query(&self, request: &DynamicMessage) -> Answer {
        let request: wire::QueryRequest = match decode(request) {
            Ok(request) => request,
            Err(answer) => return answer,
        };
        let answer = match (request.query, request.oslc_query.is_empty()) {
            (Some(_), false) => {
                return Answer::Unrepresentable("a query in both forms at once".to_owned())
            }
            (Some(query), true) => match Query::try_from(query) {
                Ok(query) => self.connection.query(&request.model_hash, &query),
                Err(error) => Err(error),
            },
            (None, false) => self
                .connection
                .query_oslc(&request.model_hash, &request.oslc_query),
            (None, true) => self.connection.query(&request.model_hash, &Query::new()),
        };
        match answer {
            Ok(elements) => Answer::Response(self.wire_json(
                "sysml.QueryResponse",
                &wire::QueryResponse {
                    elements: elements.into_iter().map(Into::into).collect(),
                },
            )),
            Err(error) => classify_error(error),
        }
    }

    fn run_document_query(&self, request: &DynamicMessage) -> Answer {
        let request: wire::RunDocumentQueryRequest = match decode(request) {
            Ok(request) => request,
            Err(answer) => return answer,
        };
        let mut bindings = Vec::new();
        for binding in request.bindings {
            let values = match binding
                .values
                .into_iter()
                .map(DocumentValue::try_from)
                .collect::<Result<Vec<_>, _>>()
            {
                Ok(values) => values,
                Err(error) => return classify_error(error),
            };
            bindings.push((binding.parameter, values));
        }
        match self
            .connection
            .run_document_query(&request.model_hash, &request.query_id, &bindings)
        {
            Ok(result) => {
                Answer::Response(self.wire_json("sysml.RunDocumentQueryResponse", result.wire()))
            }
            Err(error) => classify_error(error),
        }
    }

    fn render_document(&self, request: &DynamicMessage) -> Answer {
        let request: wire::RenderDocumentRequest = match decode(request) {
            Ok(request) => request,
            Err(answer) => return answer,
        };
        let form = match request.form.as_str() {
            "" | "markdown" => DocumentForm::Markdown,
            "html" => DocumentForm::Html,
            other => return Answer::Unrepresentable(format!("document form {other:?}")),
        };
        match self
            .connection
            .render_document(&request.model_hash, &request.document_id, form)
        {
            Ok(text) => {
                let response = match form {
                    DocumentForm::Markdown => wire::RenderDocumentResponse {
                        markdown: text,
                        ..Default::default()
                    },
                    DocumentForm::Html => wire::RenderDocumentResponse {
                        html: text,
                        ..Default::default()
                    },
                };
                Answer::Response(self.wire_json("sysml.RenderDocumentResponse", &response))
            }
            Err(error) => classify_error(error),
        }
    }

    fn explored(&self, explored: &ExploredResponse) -> Answer {
        Answer::Response(match explored {
            ExploredResponse::Action(response) => {
                self.wire_json("sysml.ExecuteActionResponse", response.as_ref())
            }
            ExploredResponse::State(response) => {
                self.wire_json("sysml.ExecuteStateResponse", response.as_ref())
            }
            ExploredResponse::Analysis(response) => {
                self.wire_json("sysml.RunAnalysisResponse", response.as_ref())
            }
        })
    }

    fn execute_action(&self, request: &DynamicMessage) -> Answer {
        let request: wire::ExecuteActionRequest = match decode(request) {
            Ok(request) => request,
            Err(answer) => return answer,
        };
        let inputs = match named_values_of(request.inputs) {
            Ok(inputs) => inputs,
            Err(answer) => return answer,
        };
        let options = RunOptions {
            schedule: optional(request.schedule),
            performer: optional(request.performer_symbol_id),
            trace: false,
        };
        let hash = &request.model_hash;
        let action = &request.action_symbol_id;
        let answer = if explores(options.schedule.as_deref()) {
            self.connection
                .explore_action(hash, action, &inputs, &options)
                .map(|exploration| self.explored(exploration.wire()))
        } else {
            self.connection
                .execute_action(hash, action, &inputs, &options)
                .map(|run| {
                    Answer::Response(self.wire_json("sysml.ExecuteActionResponse", run.wire()))
                })
        };
        answer.unwrap_or_else(|error| match Failure::of(error) {
            Ok(failure) => self.in_band(
                "sysml.ExecuteActionResponse",
                &wire::ExecuteActionResponse {
                    error: failure.error,
                    diagnostics: failure.diagnostics,
                    ..Default::default()
                },
            ),
            Err(answer) => answer,
        })
    }

    fn execute_state(&self, request: &DynamicMessage) -> Answer {
        let request: wire::ExecuteStateRequest = match decode(request) {
            Ok(request) => request,
            Err(answer) => return answer,
        };
        let options = RunOptions {
            schedule: optional(request.schedule),
            performer: optional(request.performer_symbol_id),
            trace: request.trace,
        };
        let hash = &request.model_hash;
        let machine = &request.state_machine_symbol_id;
        let answer = if explores(options.schedule.as_deref()) {
            self.connection
                .explore_state(hash, machine, &request.events, &options)
                .map(|exploration| self.explored(exploration.wire()))
        } else {
            self.connection
                .execute_state(hash, machine, &request.events, &options)
                .map(|run| {
                    Answer::Response(self.wire_json("sysml.ExecuteStateResponse", run.wire()))
                })
        };
        answer.unwrap_or_else(|error| match Failure::of(error) {
            Ok(failure) => self.in_band(
                "sysml.ExecuteStateResponse",
                &wire::ExecuteStateResponse {
                    error: failure.error,
                    diagnostics: failure.diagnostics,
                    trace: failure.trace,
                    trace_dropped: failure.trace_dropped,
                    ..Default::default()
                },
            ),
            Err(answer) => answer,
        })
    }

    fn list_engines(&self) -> Answer {
        match self.connection.list_engines() {
            Ok(engines) => Answer::Response(self.wire_json(
                "sysml.ListEnginesResponse",
                &wire::ListEnginesResponse {
                    engines: engines.into_iter().map(Into::into).collect(),
                },
            )),
            Err(error) => classify_error(error),
        }
    }

    fn verify_options(subject: String, engine: String, question: String) -> VerifyOptions {
        VerifyOptions {
            subject: optional(subject),
            engine: optional(engine),
            question: optional(question),
        }
    }

    /// A verdict as the single-verdict responses carry it, with the objects reported beside it.
    fn verdict_parts(
        verdict: &opensysml::Verdict,
    ) -> (
        Option<wire::Verdict>,
        Vec<wire::Instance>,
        Vec<wire::Diagnostic>,
    ) {
        (
            Some(verdict.wire().clone()),
            verdict
                .instances()
                .iter()
                .map(|instance| instance.wire().clone())
                .collect(),
            diagnostics_wire(&verdict.diagnostics),
        )
    }

    /// A refused verification: a wrong kind answers in the verdict, anything else beside it.
    fn refused_verdict(error: Error) -> Result<(Option<wire::Verdict>, Failure), Answer> {
        let failure = Failure::of(error)?;
        if failure.reason == FailureReason::WrongKind {
            let verdict = wire::Verdict {
                failure_reason: failure.reason(),
                error: failure.error,
                ..Default::default()
            };
            return Ok((
                Some(verdict),
                Failure {
                    error: String::new(),
                    reason: FailureReason::Unspecified,
                    diagnostics: failure.diagnostics,
                    trace: Vec::new(),
                    trace_dropped: 0,
                },
            ));
        }
        Ok((None, failure))
    }

    fn verify_constraint(&self, request: &DynamicMessage) -> Answer {
        let request: wire::VerifyConstraintRequest = match decode(request) {
            Ok(request) => request,
            Err(answer) => return answer,
        };
        let options =
            Self::verify_options(request.subject_symbol_id, request.engine, request.question);
        let response = match self.connection.verify_constraint(
            &request.model_hash,
            &request.symbol_id,
            &options,
        ) {
            Ok(verdict) => {
                let (verdict, instances, diagnostics) = Self::verdict_parts(&verdict);
                wire::VerifyConstraintResponse {
                    verdict,
                    instances,
                    diagnostics,
                    ..Default::default()
                }
            }
            Err(error) => match Self::refused_verdict(error) {
                Ok((verdict, failure)) => wire::VerifyConstraintResponse {
                    verdict,
                    error: failure.error,
                    diagnostics: failure.diagnostics,
                    ..Default::default()
                },
                Err(answer) => return answer,
            },
        };
        Answer::Response(self.wire_json("sysml.VerifyConstraintResponse", &response))
    }

    fn verify_requirement(&self, request: &DynamicMessage) -> Answer {
        let request: wire::VerifyRequirementRequest = match decode(request) {
            Ok(request) => request,
            Err(answer) => return answer,
        };
        let options =
            Self::verify_options(request.subject_symbol_id, request.engine, request.question);
        let response = match self.connection.verify_requirement(
            &request.model_hash,
            &request.symbol_id,
            &options,
        ) {
            Ok(verdict) => {
                let verification_verdicts = verdict
                    .verifications
                    .iter()
                    .cloned()
                    .map(Into::into)
                    .collect();
                let (verdict, instances, diagnostics) = Self::verdict_parts(&verdict);
                wire::VerifyRequirementResponse {
                    verdict,
                    instances,
                    diagnostics,
                    verification_verdicts,
                    ..Default::default()
                }
            }
            Err(error) => match Self::refused_verdict(error) {
                Ok((verdict, failure)) => wire::VerifyRequirementResponse {
                    verdict,
                    error: failure.error,
                    diagnostics: failure.diagnostics,
                    ..Default::default()
                },
                Err(answer) => return answer,
            },
        };
        Answer::Response(self.wire_json("sysml.VerifyRequirementResponse", &response))
    }

    fn verify_satisfaction(&self, request: &DynamicMessage) -> Answer {
        let request: wire::VerifySatisfactionRequest = match decode(request) {
            Ok(request) => request,
            Err(answer) => return answer,
        };
        let options = Self::verify_options(String::new(), request.engine, request.question);
        let scope = optional(request.symbol_id);
        match self
            .connection
            .verify_satisfaction(&request.model_hash, scope.as_deref(), &options)
        {
            Ok(satisfaction) => Answer::Response(
                self.wire_json("sysml.VerifySatisfactionResponse", satisfaction.wire()),
            ),
            Err(error) => match Failure::of(error) {
                Ok(failure) => self.in_band(
                    "sysml.VerifySatisfactionResponse",
                    &wire::VerifySatisfactionResponse {
                        failure_reason: failure.reason(),
                        error: failure.error,
                        diagnostics: failure.diagnostics,
                        ..Default::default()
                    },
                ),
                Err(answer) => answer,
            },
        }
    }

    fn validate_instance(&self, request: &DynamicMessage) -> Answer {
        let request: wire::ValidateInstanceRequest = match decode(request) {
            Ok(request) => request,
            Err(answer) => return answer,
        };
        let engine = optional(request.engine);
        match self.connection.validate_instance(
            &request.model_hash,
            &request.symbol_id,
            engine.as_deref(),
        ) {
            Ok(validation) => Answer::Response(
                self.wire_json("sysml.ValidateInstanceResponse", validation.wire()),
            ),
            Err(error) => match Failure::of(error) {
                Ok(failure) => self.in_band(
                    "sysml.ValidateInstanceResponse",
                    &wire::ValidateInstanceResponse {
                        failure_reason: failure.reason(),
                        error: failure.error,
                        diagnostics: failure.diagnostics,
                        ..Default::default()
                    },
                ),
                Err(answer) => answer,
            },
        }
    }

    fn evaluate_calc(&self, request: &DynamicMessage) -> Answer {
        let request: wire::EvaluateCalcRequest = match decode(request) {
            Ok(request) => request,
            Err(answer) => return answer,
        };
        let arguments = match values_of(request.arguments) {
            Ok(arguments) => arguments,
            Err(answer) => return answer,
        };
        let engine = optional(request.engine);
        match self.connection.calc(
            &request.model_hash,
            &request.symbol_id,
            &arguments,
            engine.as_deref(),
        ) {
            Ok(result) => {
                Answer::Response(self.wire_json("sysml.EvaluateCalcResponse", result.wire()))
            }
            Err(error) => match Failure::of(error) {
                Ok(failure) => self.in_band(
                    "sysml.EvaluateCalcResponse",
                    &wire::EvaluateCalcResponse {
                        failure_reason: failure.reason(),
                        error: failure.error,
                        diagnostics: failure.diagnostics,
                        ..Default::default()
                    },
                ),
                Err(answer) => answer,
            },
        }
    }

    fn run_analysis(&self, request: &DynamicMessage) -> Answer {
        let request: wire::RunAnalysisRequest = match decode(request) {
            Ok(request) => request,
            Err(answer) => return answer,
        };
        let options = match (
            values_of(request.arguments),
            named_values_of(request.named_arguments),
        ) {
            (Ok(arguments), Ok(named_arguments)) => AnalysisOptions {
                subject: optional(request.subject_symbol_id),
                arguments,
                named_arguments,
                schedule: optional(request.schedule),
                engine: optional(request.engine),
            },
            (Err(answer), _) | (_, Err(answer)) => return answer,
        };
        let hash = &request.model_hash;
        let case = &request.symbol_id;
        let answer = if explores(options.schedule.as_deref()) {
            self.connection
                .explore_analysis(hash, case, &options)
                .map(|exploration| self.explored(exploration.wire()))
        } else {
            self.connection
                .run_analysis(hash, case, &options)
                .map(|result| {
                    Answer::Response(self.wire_json("sysml.RunAnalysisResponse", result.wire()))
                })
        };
        answer.unwrap_or_else(|error| match error {
            Error::AnalysisRun { result, .. } => {
                self.in_band("sysml.RunAnalysisResponse", result.wire())
            }
            error => match Failure::of(error) {
                Ok(failure) => self.in_band(
                    "sysml.RunAnalysisResponse",
                    &wire::RunAnalysisResponse {
                        failure_reason: failure.reason(),
                        error: failure.error,
                        diagnostics: failure.diagnostics,
                        ..Default::default()
                    },
                ),
                Err(answer) => answer,
            },
        })
    }

    fn run_sweep(&self, request: &DynamicMessage) -> Answer {
        let request: wire::RunSweepRequest = match decode(request) {
            Ok(request) => request,
            Err(answer) => return answer,
        };
        let mut ranges = Vec::new();
        for range in request.ranges {
            let (Some(start), Some(end)) = (range.start, range.end) else {
                return Answer::Unrepresentable("a sweep range without both ends".to_owned());
            };
            let (start, end, step) = match (
                value_of(start),
                value_of(end),
                range.step.map(value_of).transpose(),
            ) {
                (Ok(start), Ok(end), Ok(step)) => (start, end, step),
                (Err(answer), _, _) | (_, Err(answer), _) | (_, _, Err(answer)) => return answer,
            };
            let mut typed = SweepRange::new(range.parameter, start, end);
            typed.step = step;
            ranges.push(typed);
        }
        let options = match (
            values_of(request.arguments),
            named_values_of(request.named_arguments),
        ) {
            (Ok(arguments), Ok(named_arguments)) => SweepOptions {
                subject: optional(request.subject_symbol_id),
                arguments,
                named_arguments,
                samples: request.samples,
                seed: request.seed,
                engine: optional(request.engine),
            },
            (Err(answer), _) | (_, Err(answer)) => return answer,
        };
        match self
            .connection
            .run_sweep(&request.model_hash, &request.symbol_id, &ranges, &options)
        {
            Ok(table) => Answer::Response(self.wire_json("sysml.RunSweepResponse", table.wire())),
            Err(error) => match Failure::of(error) {
                Ok(failure) => self.in_band(
                    "sysml.RunSweepResponse",
                    &wire::RunSweepResponse {
                        failure_reason: failure.reason(),
                        error: failure.error,
                        diagnostics: failure.diagnostics,
                        ..Default::default()
                    },
                ),
                Err(answer) => answer,
            },
        }
    }

    fn wire_json<M: Message>(&self, name: &str, message: &M) -> Value {
        let descriptor = self
            .pool
            .get_message_by_name(name)
            .expect("descriptor exists");
        let dynamic = DynamicMessage::decode(descriptor, message.encode_to_vec().as_slice())
            .expect("generated response decodes against committed descriptor");
        let options = SerializeOptions::new()
            .use_proto_field_name(true)
            .skip_default_fields(false)
            .stringify_64_bit_integers(false);
        let mut bytes = Vec::new();
        dynamic
            .serialize_with_options(&mut serde_json::Serializer::new(&mut bytes), &options)
            .expect("dynamic response serializes");
        serde_json::from_slice(&bytes).expect("serialized dynamic response is JSON")
    }
}

fn classify_error(error: Error) -> Answer {
    match error {
        Error::Service { status, message } => Answer::Transport(status, message),
        Error::InvalidRequest(message) | Error::Query(message) => {
            Answer::Transport(Status::InvalidArgument, message)
        }
        Error::Model(message) => Answer::InBand(serde_json::json!({ "error": message })),
        other => Answer::Other(other),
    }
}

fn explores(schedule: Option<&str>) -> bool {
    schedule.is_some_and(|schedule| schedule == "explore" || schedule.starts_with("explore:"))
}

fn decode<T: Message + Default>(request: &DynamicMessage) -> Result<T, Answer> {
    T::decode(request.encode_to_vec().as_slice())
        .map_err(|error| Answer::Other(Error::Decode(error.to_string())))
}

fn value_of(value: wire::Value) -> Result<SysmlValue, Answer> {
    SysmlValue::try_from(value)
        .map_err(|error| Answer::Unrepresentable(format!("a malformed value ({error})")))
}

fn values_of(values: Vec<wire::Value>) -> Result<Vec<SysmlValue>, Answer> {
    values.into_iter().map(value_of).collect()
}

fn named_values_of(
    values: std::collections::HashMap<String, wire::Value>,
) -> Result<BTreeMap<String, SysmlValue>, Answer> {
    values
        .into_iter()
        .map(|(name, value)| Ok((name, value_of(value)?)))
        .collect()
}

fn optional(text: String) -> Option<String> {
    (!text.is_empty()).then_some(text)
}

fn diagnostics_wire(diagnostics: &[opensysml::Diagnostic]) -> Vec<wire::Diagnostic> {
    diagnostics
        .iter()
        .map(|diagnostic| diagnostic.wire().clone())
        .collect()
}

/// A typed failure the service answered in band, as the fields of the response it came in.
struct Failure {
    error: String,
    reason: FailureReason,
    diagnostics: Vec<wire::Diagnostic>,
    trace: Vec<wire::DocumentEvent>,
    trace_dropped: i32,
}

impl Failure {
    fn of(error: Error) -> Result<Self, Answer> {
        match error {
            Error::Execution {
                message,
                reason,
                diagnostics,
                trace,
                trace_dropped,
            } => Ok(Self {
                error: message,
                reason,
                diagnostics: diagnostics_wire(&diagnostics),
                trace: trace
                    .iter()
                    .map(opensysml::document_event_to_wire)
                    .collect::<Result<_, _>>()
                    .map_err(classify_error)?,
                trace_dropped,
            }),
            Error::WrongKind {
                message,
                diagnostics,
            } => Ok(Self {
                error: message,
                reason: FailureReason::WrongKind,
                diagnostics: diagnostics_wire(&diagnostics),
                trace: Vec::new(),
                trace_dropped: 0,
            }),
            Error::Conversion {
                message,
                diagnostics,
            }
            | Error::ModelErrors {
                message,
                diagnostics,
            } => Ok(Self {
                error: message,
                reason: FailureReason::Unspecified,
                diagnostics: diagnostics_wire(&diagnostics),
                trace: Vec::new(),
                trace_dropped: 0,
            }),
            other => Err(classify_error(other)),
        }
    }

    fn reason(&self) -> i32 {
        self.reason as i32
    }
}

fn errored(mut result: ResultRecord, message: String, started: Instant) -> ResultRecord {
    result.outcome = "error".to_owned();
    result.status = "-".to_owned();
    result.reason = message;
    result.duration_ms = elapsed_ms(started);
    result
}

fn elapsed_ms(started: Instant) -> f64 {
    started.elapsed().as_secs_f64() * 1000.0
}

fn print_result(result: &ResultRecord, verbose: bool) {
    let mark = match result.outcome.as_str() {
        "pass" => "PASS",
        "fail" => "FAIL",
        "skip" => "SKIP",
        _ => "ERR ",
    };
    println!("{mark} {:46} {}", result.id, result.status);
    if !result.reason.is_empty() {
        println!("       {}", result.reason);
    }
    for failure in &result.failures {
        println!("       {failure}");
    }
    if verbose {
        println!("       duration_ms={:.3}", result.duration_ms);
    }
}

fn request_source(request: &DynamicMessage) -> Option<Source> {
    if request.has_field_by_name("content") {
        request.get_field_by_name("content").and_then(|value| {
            value
                .as_ref()
                .as_str()
                .map(|value| Source::Content(value.to_owned()))
        })
    } else if request.has_field_by_name("file_path") {
        request.get_field_by_name("file_path").and_then(|value| {
            value
                .as_ref()
                .as_str()
                .map(|value| Source::File(PathBuf::from(value)))
        })
    } else {
        None
    }
}

enum Source {
    Content(String),
    File(PathBuf),
}

fn string_field(request: &DynamicMessage, name: &str) -> Result<String, Error> {
    request
        .get_field_by_name(name)
        .and_then(|value| value.as_ref().as_str().map(ToOwned::to_owned))
        .ok_or_else(|| Error::Decode(format!("request field {name} is not a string")))
}

fn string_or_none(request: &DynamicMessage, name: &str) -> Option<String> {
    request
        .get_field_by_name(name)
        .and_then(|value| value.as_ref().as_str().map(ToOwned::to_owned))
        .filter(|value| !value.is_empty())
}

fn resolve_placeholders(
    value: &mut Value,
    model_hash: Option<&str>,
    fixtures: &Path,
) -> Result<(), String> {
    match value {
        Value::Object(object) => {
            for child in object.values_mut() {
                resolve_placeholders(child, model_hash, fixtures)?;
            }
        }
        Value::Array(items) => {
            for child in items {
                resolve_placeholders(child, model_hash, fixtures)?;
            }
        }
        Value::String(text) if text == normalize::MODEL_HASH => {
            let Some(hash) = model_hash else {
                return Err("request names ${model_hash} but scenario declares no model".to_owned());
            };
            *text = hash.to_owned();
        }
        Value::String(text) if text.starts_with("${fixture:") && text.ends_with('}') => {
            let name = &text[10..text.len() - 1];
            let path = fixture_path(fixtures, name)?;
            *text = fs::read_to_string(&path)
                .map_err(|error| format!("reading fixture {name}: {error}"))?;
        }
        Value::String(text) if text.starts_with("${fixture_base64:") && text.ends_with('}') => {
            let name = &text[17..text.len() - 1];
            let path = fixture_path(fixtures, name)?;
            let bytes =
                fs::read(&path).map_err(|error| format!("reading fixture {name}: {error}"))?;
            *text = BASE64_STANDARD.encode(bytes);
        }
        _ => {}
    }
    Ok(())
}

fn fixture_path(fixtures: &Path, name: &str) -> Result<PathBuf, String> {
    if Path::new(name).components().any(|component| {
        matches!(
            component,
            std::path::Component::ParentDir
                | std::path::Component::RootDir
                | std::path::Component::Prefix(_)
        )
    }) {
        return Err(format!(
            "fixture {name:?} is outside {}",
            fixtures.display()
        ));
    }
    Ok(fixtures.join(name))
}

fn start_service(binary: &Path) -> Result<ServiceGuard, String> {
    let mut child = Command::new(binary)
        .args(["-port", "0", "-health-port", "0", "-report-address"])
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::inherit())
        .spawn()
        .map_err(|error| format!("starting {}: {error}", binary.display()))?;
    let stdout = child
        .stdout
        .take()
        .ok_or_else(|| "service stdout was not piped".to_owned())?;
    let mut lines = BufReader::new(stdout).lines();
    let address = lines
        .next()
        .ok_or_else(|| "service exited without reporting an address".to_owned())?
        .map_err(|error| format!("reading service address: {error}"))?;
    Ok(ServiceGuard { child, address })
}

fn split_address(address: &str) -> Result<(String, u16), String> {
    if let Some(rest) = address.strip_prefix('[') {
        let end = rest
            .find(']')
            .ok_or_else(|| format!("invalid service address {address:?}"))?;
        let host = rest[..end].to_owned();
        let port = rest[end + 1..]
            .strip_prefix(':')
            .ok_or_else(|| format!("invalid service address {address:?}"))?
            .parse()
            .map_err(|_| format!("invalid service address {address:?}"))?;
        Ok((host, port))
    } else {
        let (host, port) = address
            .rsplit_once(':')
            .ok_or_else(|| format!("invalid service address {address:?}"))?;
        Ok((
            host.to_owned(),
            port.parse()
                .map_err(|_| format!("invalid service address {address:?}"))?,
        ))
    }
}

fn repository_root() -> Result<PathBuf, String> {
    let current = env::current_dir().map_err(|error| error.to_string())?;
    for root in current.ancestors() {
        if root.join("conformance").join("scenarios").is_dir() {
            return Ok(root.to_owned());
        }
    }
    Err("could not locate repository root".to_owned())
}

fn binary_name() -> &'static str {
    if cfg!(windows) {
        "sysml-grpc.exe"
    } else {
        "sysml-grpc"
    }
}

struct Options {
    binary: Option<PathBuf>,
    scenarios: Option<PathBuf>,
    fixtures: Option<PathBuf>,
    run: Option<String>,
    report: Option<PathBuf>,
    allow_skips: bool,
    verbose: bool,
}

impl Options {
    fn parse() -> Result<Self, String> {
        let mut options = Self {
            binary: None,
            scenarios: None,
            fixtures: None,
            run: None,
            report: None,
            allow_skips: false,
            verbose: false,
        };
        let mut args = env::args().skip(1);
        while let Some(arg) = args.next() {
            let value = |name: &str, args: &mut dyn Iterator<Item = String>| {
                args.next().ok_or_else(|| format!("{name} needs a value"))
            };
            match arg.as_str() {
                "-binary" => options.binary = Some(PathBuf::from(value("-binary", &mut args)?)),
                "-scenarios" => {
                    options.scenarios = Some(PathBuf::from(value("-scenarios", &mut args)?))
                }
                "-fixtures" => {
                    options.fixtures = Some(PathBuf::from(value("-fixtures", &mut args)?))
                }
                "-run" => options.run = Some(value("-run", &mut args)?),
                "-report" => options.report = Some(PathBuf::from(value("-report", &mut args)?)),
                "-allow-skips" => options.allow_skips = true,
                "-v" => options.verbose = true,
                "-h" | "--help" => {
                    println!("opensysml-conformance [-binary PATH] [-scenarios DIR] [-fixtures DIR] [-run SUBSTRING] [-report FILE|-] [-allow-skips] [-v]");
                    std::process::exit(0);
                }
                other => return Err(format!("unknown flag {other:?}")),
            }
        }
        Ok(options)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn fixture_paths_reject_parent_escape() {
        let fixtures = Path::new("/repo/conformance/fixtures");
        assert!(fixture_path(fixtures, "vehicle.sysml").is_ok());
        assert!(fixture_path(fixtures, "nested/vehicle.sysml").is_ok());
        assert!(fixture_path(fixtures, "../secrets.sysml").is_err());
    }

    #[test]
    fn empty_parse_content_is_present_oneof_source() {
        let pool = DescriptorPool::decode(DESCRIPTOR).unwrap();
        let descriptor = pool.get_message_by_name("sysml.ParseFileRequest").unwrap();
        let mut deserializer = Deserializer::from_str(r#"{"content":""}"#);
        let request = DynamicMessage::deserialize_with_options(
            descriptor,
            &mut deserializer,
            &DeserializeOptions::new(),
        )
        .unwrap();
        assert!(request.has_field_by_name("content"));
        assert!(matches!(
            request_source(&request),
            Some(Source::Content(content)) if content.is_empty()
        ));
    }
}
