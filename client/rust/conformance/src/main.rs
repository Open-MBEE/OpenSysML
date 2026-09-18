mod compare;
mod normalize;
mod scenario;

use std::collections::HashMap;
use std::env;
use std::fs;
use std::io::{BufRead, BufReader};
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::time::Instant;

use compare::{compare, label_instance_ids, status_matches};
use normalize::normalize;
use opensysml::{Connection, Error, EvalOptions, Language, Model, ParseOptions, Status};
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
    Model(String),
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
        if path == PathBuf::from("-") {
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

const MULTI_DOCUMENT_SKIP: &str = "v1 API parses one document at a time, not a model of several";

fn is_expected_skip(reason: &str) -> bool {
    reason.starts_with("v1 API does not cover ")
        || reason == "unrepresentable by the typed API: ParseFile with no source"
        || reason == MULTI_DOCUMENT_SKIP
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
        if !matches!(
            scenario.method(),
            "GetServerInfo"
                | "ParseFile"
                | "GetDiagnostics"
                | "GetSymbol"
                | "Evaluate"
                | "Instantiate"
        ) {
            result.outcome = "skip".to_owned();
            result.status = "-".to_owned();
            result.reason = format!("v1 API does not cover {}", scenario.method());
            result.duration_ms = elapsed_ms(started);
            return result;
        }
        if scenario
            .model
            .as_ref()
            .is_some_and(|spec| !spec.fixtures.is_empty())
        {
            result.outcome = "skip".to_owned();
            result.status = "-".to_owned();
            result.reason = MULTI_DOCUMENT_SKIP.to_owned();
            result.duration_ms = elapsed_ms(started);
            return result;
        }
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
        if scenario.method() == "ParseFile" && request_source(&request).is_none() {
            result.outcome = "skip".to_owned();
            result.status = "-".to_owned();
            result.reason = "unrepresentable by the typed API: ParseFile with no source".to_owned();
            result.duration_ms = elapsed_ms(started);
            return result;
        }
        match self.call(scenario.method(), &request, model.as_ref()) {
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
            Answer::Model(message) => {
                if !status_matches(expect.status.as_deref(), Status::Ok) {
                    result.outcome = "fail".to_owned();
                    result.failures.push(format!(
                        "the call answered OK with in-band error {message:?}, want {}",
                        expect.status.as_deref().unwrap_or("OK")
                    ));
                } else {
                    let mut actual = serde_json::json!({"error": message});
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
            other => Answer::Other(Error::Model(format!("v1 API does not cover {other}"))),
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
            None => {
                return Answer::Other(Error::Decode(
                    "ParseFile source is not representable by typed API".to_owned(),
                ))
            }
        };
        match answer {
            Ok(model) => Answer::Response(self.wire_json("sysml.ParseFileResponse", model.wire())),
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
        Error::Model(message) => Answer::Model(message),
        other => Answer::Other(other),
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
