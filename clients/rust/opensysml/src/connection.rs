use std::collections::VecDeque;
use std::env;
use std::io::{BufRead, BufReader, Read};
use std::path::{Path, PathBuf};
use std::process::{Child, ChildStdin, Command, Stdio};
use std::sync::{Arc, Mutex, OnceLock, PoisonError, Weak};
use std::thread;
use std::time::{Duration, Instant};

use prost::Message;

use crate::binary;
use crate::domain::{
    Capabilities, EvalOptions, Evaluation, Instantiation, Language, Model, ParseOptions,
    ServerInfo, Symbol,
};
use crate::error::{Error, Status};
use crate::wire;

const START_TIMEOUT: Duration = Duration::from_millis(2500);
const STOP_TIMEOUT: Duration = Duration::from_secs(5);
const STDERR_LINES_KEPT: usize = 20;
/// Bound on a buffered response, so a runaway service cannot exhaust memory.
/// A parse of a large model answers far above ureq's 10 MB default.
const RESPONSE_LIMIT: u64 = 1 << 30;

static PRIVATE_SERVICE: OnceLock<Mutex<Weak<PrivateService>>> = OnceLock::new();
static HTTP_AGENT: OnceLock<ureq::Agent> = OnceLock::new();

fn private_service() -> &'static Mutex<Weak<PrivateService>> {
    PRIVATE_SERVICE.get_or_init(|| Mutex::new(Weak::new()))
}

fn http_agent() -> &'static ureq::Agent {
    HTTP_AGENT.get_or_init(|| {
        ureq::Agent::config_builder()
            .http_status_as_error(false)
            .timeout_connect(Some(Duration::from_secs(5)))
            .build()
            .into()
    })
}

/// A blocking connection to sysml-grpc.
#[derive(Clone, Debug)]
pub struct Connection {
    pub(crate) inner: Arc<ConnectionInner>,
}

#[derive(Debug)]
pub(crate) struct ConnectionInner {
    base_url: String,
    info: ServerInfo,
    private: Option<Arc<PrivateService>>,
}

impl Connection {
    /// Start or join the process-wide private sysml-grpc child.
    pub fn private() -> Result<Self, Error> {
        let private = {
            let mut registry = private_service()
                .lock()
                .unwrap_or_else(PoisonError::into_inner);
            if let Some(existing) = registry.upgrade() {
                existing
            } else {
                let started = Arc::new(PrivateService::start()?);
                *registry = Arc::downgrade(&started);
                started
            }
        };
        Self::from_target(private.address.clone(), Some(private))
    }

    /// Connect to an explicitly managed external service.
    pub fn external(host: impl Into<String>, port: u16) -> Result<Self, Error> {
        let host = host.into();
        Self::from_target(format_target(&host, port), None)
    }

    /// Follow the connection precedence: explicit environment, then private child.
    ///
    /// An empty `$OPENSYSML_SERVICE` names no address, as it does for the other
    /// clients, so it starts a private child rather than failing to read it.
    pub fn connect() -> Result<Self, Error> {
        match env::var("OPENSYSML_SERVICE") {
            Ok(target) if !target.trim().is_empty() => {
                let (host, port) = split_target(&target)?;
                Self::external(host, port)
            }
            _ => Self::private(),
        }
    }

    fn from_target(address: String, private: Option<Arc<PrivateService>>) -> Result<Self, Error> {
        let info_wire = Self::rpc_raw_at(&address, "GetServerInfo", wire::ServerInfoRequest {})?;
        let info = ServerInfo::from_wire(info_wire);
        Ok(Self {
            inner: Arc::new(ConnectionInner {
                base_url: address,
                info,
                private,
            }),
        })
    }

    /// The service handshake received at connection time.
    pub fn server_info(&self) -> &ServerInfo {
        &self.inner.info
    }

    /// The capabilities advertised by the service.
    pub fn capabilities(&self) -> &Capabilities {
        &self.inner.info.capabilities
    }

    /// Parse a file.
    pub fn parse_file(
        &self,
        path: impl AsRef<Path>,
        options: &ParseOptions,
    ) -> Result<Model, Error> {
        if options.strict_conformance {
            self.capabilities().require(
                "strict_conformance",
                "connect to a service advertising strict_conformance",
            )?;
        }
        let request = wire::ParseFileRequest {
            language: options.language.as_str().to_owned(),
            strict_conformance: options.strict_conformance,
            source: Some(wire::parse_file_request::Source::FilePath(
                path.as_ref().to_string_lossy().into_owned(),
            )),
            ..Default::default()
        };
        let response: wire::ParseFileResponse = self.rpc("ParseFile", request)?;
        if !response.error.is_empty() {
            return Err(Error::Model(response.error));
        }
        Model::from_wire(response, self.clone())
    }

    /// Parse inline content.
    pub fn parse_content(&self, content: &str, options: &ParseOptions) -> Result<Model, Error> {
        if options.strict_conformance {
            self.capabilities().require(
                "strict_conformance",
                "connect to a service advertising strict_conformance",
            )?;
        }
        if options.language == Language::Kerml {
            self.capabilities().require(
                "inline_language",
                "connect to a service advertising inline_language",
            )?;
        }
        let request = wire::ParseFileRequest {
            language: options.language.as_str().to_owned(),
            strict_conformance: options.strict_conformance,
            source: Some(wire::parse_file_request::Source::Content(
                content.to_owned(),
            )),
            ..Default::default()
        };
        let response: wire::ParseFileResponse = self.rpc("ParseFile", request)?;
        if !response.error.is_empty() {
            return Err(Error::Model(response.error));
        }
        Model::from_wire(response, self.clone())
    }

    /// Retrieve diagnostics for a model cached by the service.
    pub fn diagnostics(&self, model_hash: &str) -> Result<Vec<crate::Diagnostic>, Error> {
        let response: wire::DiagnosticsResponse = self.rpc(
            "GetDiagnostics",
            wire::DiagnosticsRequest {
                model_hash: model_hash.to_owned(),
            },
        )?;
        if !response.error.is_empty() {
            return Err(Error::Model(response.error));
        }
        Ok(response
            .diagnostics
            .into_iter()
            .map(crate::Diagnostic::from)
            .collect())
    }

    /// Create a handle for a hash obtained elsewhere; the service decides whether it remains cached.
    pub fn model_by_hash(&self, hash: &str) -> Model {
        Model::from_hash(hash, self.clone())
    }

    /// Return the process identifier of the private child, for diagnostics only.
    pub fn private_service_pid(&self) -> Option<u32> {
        self.inner.private.as_ref().map(|service| service.pid())
    }

    /// Call one method of the service with [`crate::wire`] messages: the protocol layer, for
    /// methods the ergonomic surface does not wrap. In-band `error` fields are left to the caller.
    pub fn call<T, R>(&self, method: &str, request: T) -> Result<R, Error>
    where
        T: Message,
        R: Message + Default,
    {
        self.rpc(method, request)
    }

    pub(crate) fn get_symbol(&self, model_hash: &str, symbol_id: &str) -> Result<Symbol, Error> {
        let response: wire::SymbolResponse = self.rpc(
            "GetSymbol",
            wire::GetSymbolRequest {
                model_hash: model_hash.to_owned(),
                symbol_id: symbol_id.to_owned(),
            },
        )?;
        if !response.error.is_empty() {
            return Err(Error::Model(response.error));
        }
        let symbol = response
            .symbol
            .ok_or_else(|| Error::Decode("symbol response has no symbol".to_owned()))?;
        Ok(Symbol::new(
            symbol,
            self.inner.clone(),
            model_hash.to_owned(),
        ))
    }

    pub(crate) fn evaluate(
        &self,
        model_hash: &str,
        expression: &str,
        options: &EvalOptions,
    ) -> Result<Evaluation, Error> {
        if options.subject.is_some() {
            self.capabilities().require(
                "evaluate_subject",
                "connect to a service advertising evaluate_subject",
            )?;
        }
        let response: wire::EvaluateResponse = self.rpc(
            "Evaluate",
            wire::EvaluateRequest {
                model_hash: model_hash.to_owned(),
                expression: expression.to_owned(),
                context_symbol_id: options.context.clone().unwrap_or_default(),
                subject_symbol_id: options.subject.clone().unwrap_or_default(),
            },
        )?;
        if !response.error.is_empty() {
            return Err(Error::Model(response.error.clone()));
        }
        let result_wire = response
            .result
            .clone()
            .ok_or_else(|| Error::Decode("evaluate response has no result".to_owned()))?;
        let result = crate::domain::value_from_wire(result_wire)?;
        Ok(Evaluation::new(result, response))
    }

    pub(crate) fn instantiate(
        &self,
        model_hash: &str,
        symbol_id: &str,
    ) -> Result<Instantiation, Error> {
        let response: wire::InstantiateResponse = self.rpc(
            "Instantiate",
            wire::InstantiateRequest {
                model_hash: model_hash.to_owned(),
                symbol_id: symbol_id.to_owned(),
            },
        )?;
        if !response.error.is_empty() {
            return Err(Error::Model(response.error.clone()));
        }
        Instantiation::from_wire(response)
    }

    fn rpc<T, R>(&self, method: &str, request: T) -> Result<R, Error>
    where
        T: Message,
        R: Message + Default,
    {
        Self::rpc_raw_at(&self.inner.base_url, method, request)
    }

    fn rpc_raw_at<T, R>(address: &str, method: &str, request: T) -> Result<R, Error>
    where
        T: Message,
        R: Message + Default,
    {
        let body = request.encode_to_vec();
        let url = format!("http://{address}/sysml.SysMLService/{method}");
        let response = http_agent()
            .post(&url)
            .header("Content-Type", "application/proto")
            .header("Connect-Protocol-Version", "1")
            .send(body)
            .map_err(|error| Error::Transport(error.to_string()))?;
        let status = response.status();
        let bytes = response
            .into_body()
            .into_with_config()
            .limit(RESPONSE_LIMIT)
            .read_to_vec()
            .map_err(|error| match error {
                ureq::Error::BodyExceedsLimit(limit) => Error::Transport(format!(
                    "the {method} response is larger than the {limit} bytes this client buffers"
                )),
                error => Error::Transport(error.to_string()),
            })?;
        if !status.is_success() {
            return Err(connect_error(status.as_u16(), &bytes));
        }
        R::decode(bytes.as_slice()).map_err(|error| Error::Decode(error.to_string()))
    }
}

fn connect_error(status: u16, bytes: &[u8]) -> Error {
    #[derive(serde::Deserialize)]
    struct ConnectError {
        code: Option<String>,
        message: Option<String>,
    }
    let parsed = serde_json::from_slice::<ConnectError>(bytes).ok();
    let code = parsed
        .as_ref()
        .and_then(|item| item.code.as_deref())
        .map(ToOwned::to_owned);
    let message = parsed
        .and_then(|item| item.message)
        .filter(|item| !item.is_empty())
        .unwrap_or_else(|| String::from_utf8_lossy(bytes).into_owned());
    let status = match code.as_deref() {
        Some(code) => Status::from_connect_code(code),
        None => match status {
            400 => Status::InvalidArgument,
            401 => Status::Unauthenticated,
            403 => Status::PermissionDenied,
            404 => Status::NotFound,
            408 => Status::DeadlineExceeded,
            409 => Status::Aborted,
            429 => Status::ResourceExhausted,
            500 => Status::Internal,
            501 => Status::Unimplemented,
            503 => Status::Unavailable,
            _ => Status::Unknown,
        },
    };
    Error::Service { status, message }
}

fn format_target(host: &str, port: u16) -> String {
    if host.contains(':') && !host.starts_with('[') {
        format!("[{host}]:{port}")
    } else {
        format!("{host}:{port}")
    }
}

fn split_target(target: &str) -> Result<(String, u16), Error> {
    let written = target;
    let target = target.trim();
    let target = match target.split_once("://") {
        Some(("http", rest)) => rest.trim_end_matches('/'),
        Some((scheme, _)) => {
            return Err(Error::Transport(format!(
                "service address {written:?} names the {scheme:?} scheme; this client speaks plaintext http"
            )))
        }
        None => target,
    };
    if target.contains('/') {
        return Err(Error::Transport(format!(
            "service address {written:?} names a path; a sysml-grpc address is host:port"
        )));
    }
    if let Some(rest) = target.strip_prefix('[') {
        let Some(end) = rest.find(']') else {
            return Err(Error::Transport(format!(
                "invalid service address {written:?}"
            )));
        };
        let host = &rest[..end];
        let port = rest
            .get(end + 1..)
            .and_then(|value| value.strip_prefix(':'))
            .and_then(|value| value.parse().ok());
        return port
            .map(|port| (host.to_owned(), port))
            .ok_or_else(|| Error::Transport(format!("invalid service address {written:?}")));
    }
    // An unbracketed IPv6 address has colons of its own, so its last one names
    // no port: reading one would connect somewhere nobody asked for.
    if target.matches(':').count() > 1 {
        return Err(Error::Transport(format!(
            "service address {written:?} names no port; write an IPv6 address bracketed, as [{target}]:PORT"
        )));
    }
    let (host, port) = target.rsplit_once(':').ok_or_else(|| {
        Error::Transport(format!(
            "service address must be host:port, got {written:?}"
        ))
    })?;
    let port = port
        .parse()
        .map_err(|_| Error::Transport(format!("invalid service port in {written:?}")))?;
    Ok((host.to_owned(), port))
}

#[derive(Debug)]
struct PrivateService {
    process: Mutex<Child>,
    stdin: Mutex<Option<ChildStdin>>,
    address: String,
}

impl PrivateService {
    fn start() -> Result<Self, Error> {
        let binary = resolve_binary()?;
        let mut command = Command::new(&binary);
        command.args([
            "-port",
            "0",
            "-health-port",
            "0",
            "-report-address",
            "-exit-with-parent",
        ]);
        command
            .stdin(Stdio::piped())
            .stdout(Stdio::piped())
            .stderr(Stdio::piped());
        #[cfg(unix)]
        {
            use std::os::unix::process::CommandExt;
            command.process_group(0);
        }
        let mut process = command
            .spawn()
            .map_err(|error| Error::ServiceStart(format!("{}: {error}", binary.display())))?;
        let stdin = process.stdin.take();
        let stdout = process
            .stdout
            .take()
            .ok_or_else(|| Error::ServiceStart("service stdout was not piped".to_owned()))?;
        let stderr = process
            .stderr
            .take()
            .ok_or_else(|| Error::ServiceStart("service stderr was not piped".to_owned()))?;
        let tail = Arc::new(Mutex::new(VecDeque::with_capacity(STDERR_LINES_KEPT)));
        let tail_reader = Arc::clone(&tail);
        thread::spawn(move || {
            for line in BufReader::new(stderr).lines().map_while(Result::ok) {
                let mut tail = tail_reader.lock().unwrap_or_else(PoisonError::into_inner);
                if tail.len() == STDERR_LINES_KEPT {
                    tail.pop_front();
                }
                tail.push_back(line);
            }
        });
        let (address_sender, address_receiver) = std::sync::mpsc::channel();
        thread::spawn(move || {
            let mut reader = BufReader::new(stdout);
            let mut line = String::new();
            let result = reader
                .read_line(&mut line)
                .ok()
                .filter(|count| *count > 0)
                .map(|_| line.trim().to_owned());
            let _ = address_sender.send(result);
            let mut discarded = Vec::new();
            let _ = reader.read_to_end(&mut discarded);
        });
        let address = match address_receiver.recv_timeout(START_TIMEOUT) {
            Ok(Some(address)) if !address.is_empty() => address,
            Ok(_) => {
                drop(stdin);
                let code = process
                    .try_wait()
                    .ok()
                    .flatten()
                    .and_then(|status| status.code());
                let message = format!("exited with code {code:?} without serving an address");
                terminate_process(&mut process, Duration::ZERO);
                return Err(Error::ServiceStart(format!(
                    "{message}; {}",
                    stderr_tail(&tail)
                )));
            }
            Err(_) => {
                drop(stdin);
                terminate_process(&mut process, Duration::ZERO);
                return Err(Error::ServiceStart(format!(
                    "did not report a listening address within 2.5s; {}",
                    stderr_tail(&tail)
                )));
            }
        };
        Ok(Self {
            process: Mutex::new(process),
            stdin: Mutex::new(stdin),
            address,
        })
    }

    fn pid(&self) -> u32 {
        self.process
            .lock()
            .unwrap_or_else(PoisonError::into_inner)
            .id()
    }
}

impl Drop for PrivateService {
    fn drop(&mut self) {
        if let Ok(mut stdin) = self.stdin.lock() {
            stdin.take();
        }
        if let Ok(mut process) = self.process.lock() {
            terminate_process(&mut process, Duration::from_secs(2));
        }
    }
}

fn stderr_tail(tail: &Arc<Mutex<VecDeque<String>>>) -> String {
    let lines = tail.lock().unwrap_or_else(PoisonError::into_inner);
    if lines.is_empty() {
        "stderr was empty".to_owned()
    } else {
        format!(
            "stderr tail: {}",
            lines.iter().cloned().collect::<Vec<_>>().join(" | ")
        )
    }
}

fn terminate_process(process: &mut Child, grace: Duration) {
    if process.try_wait().ok().flatten().is_some() {
        return;
    }
    let deadline = Instant::now() + STOP_TIMEOUT;
    let grace_deadline = std::cmp::min(Instant::now() + grace, deadline);
    while Instant::now() < deadline {
        if process.try_wait().ok().flatten().is_some() {
            return;
        }
        if Instant::now() >= grace_deadline {
            break;
        }
        thread::sleep(Duration::from_millis(10));
    }
    let _ = process.kill();
    let _ = process.wait();
}

/// Resolve the service binary: explicit path, shared cache, download, then `$PATH`.
///
/// A download only runs when `$OPENSYSML_GRPC_VERSION` names a release, and then it
/// precedes `$PATH`, whose binary is of no known version.
fn resolve_binary() -> Result<PathBuf, Error> {
    let mut looked_in = Vec::new();
    if let Ok(path) = env::var("OPENSYSML_GRPC_BINARY") {
        looked_in.push(path.clone());
        let candidate = PathBuf::from(path);
        if candidate.is_file() {
            return Ok(candidate);
        }
    } else {
        looked_in.push("$OPENSYSML_GRPC_BINARY".to_owned());
    }

    let downloader = binary::Downloader::from_env();
    let cached = binary::default_cache_dir().map(|dir| dir.join(binary::binary_file_name()));
    looked_in.push(match &cached {
        Ok(path) => path.display().to_string(),
        Err(error) => format!("the shared cache ({error})"),
    });
    match (downloader, binary::env_release_version()) {
        (Ok(downloader), Some(version)) => return downloader.ensure_binary(Some(&version)),
        // A release was asked for and cannot be downloaded here, so no binary of
        // an unknown version answers for it.
        (Err(error), Some(_)) => return Err(error),
        _ => {
            // The digest-named link, so an install over the cache does not change
            // the binary this start is about to run.
            if let Some(path) = cached
                .ok()
                .as_deref()
                .and_then(binary::stable_cached_binary)
            {
                return Ok(path);
            }
        }
    }

    looked_in.push("sysml-grpc on PATH".to_owned());
    if let Some(path) = env::var_os("PATH") {
        for directory in env::split_paths(&path) {
            let candidate = directory.join(binary::binary_file_name());
            if candidate.is_file() {
                return Ok(candidate);
            }
        }
    }
    Err(Error::BinaryNotFound { looked_in })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn a_release_asked_for_is_not_answered_by_a_binary_of_unknown_version() {
        // Serialized against the other test reading these variables.
        let _guard = ENV.lock().unwrap_or_else(PoisonError::into_inner);
        let restore = Restore::of(&[
            "OPENSYSML_GRPC_BINARY",
            "OPENSYSML_GITHUB_REPO",
            "OPENSYSML_GRPC_VERSION",
        ]);
        env::set_var("OPENSYSML_GRPC_BINARY", "");
        env::set_var("OPENSYSML_GITHUB_REPO", "not-an-owner-repo");
        env::set_var("OPENSYSML_GRPC_VERSION", "v0.1.0");

        let error = resolve_binary().expect_err("the release cannot be downloaded");
        assert!(
            matches!(&error, Error::BinaryDownload(message) if message.contains("owner/repo")),
            "{error}"
        );
        drop(restore);
    }

    #[test]
    fn without_a_release_asked_for_an_unusable_repository_is_no_obstacle() {
        let _guard = ENV.lock().unwrap_or_else(PoisonError::into_inner);
        let restore = Restore::of(&[
            "OPENSYSML_GRPC_BINARY",
            "OPENSYSML_GITHUB_REPO",
            "OPENSYSML_GRPC_VERSION",
        ]);
        env::set_var("OPENSYSML_GRPC_BINARY", "");
        env::set_var("OPENSYSML_GITHUB_REPO", "not-an-owner-repo");
        env::remove_var("OPENSYSML_GRPC_VERSION");

        // Nothing was asked for, so resolution falls through to the cache and $PATH.
        if let Err(error) = resolve_binary() {
            assert!(matches!(error, Error::BinaryNotFound { .. }), "{error}");
        }
        drop(restore);
    }

    #[test]
    fn without_a_home_directory_the_cache_is_not_the_working_directory() {
        let _guard = ENV.lock().unwrap_or_else(PoisonError::into_inner);
        let restore = Restore::of(&[
            "OPENSYSML_GRPC_BINARY",
            "OPENSYSML_GRPC_VERSION",
            "HOME",
            "USERPROFILE",
        ]);
        env::set_var("OPENSYSML_GRPC_BINARY", "");
        env::remove_var("OPENSYSML_GRPC_VERSION");
        env::remove_var("HOME");
        env::remove_var("USERPROFILE");

        assert!(binary::Downloader::from_env().is_err());
        if let Err(Error::BinaryNotFound { looked_in }) = resolve_binary() {
            assert!(
                !looked_in
                    .iter()
                    .any(|place| place.starts_with(".opensysml")),
                "{looked_in:?}"
            );
        }
        drop(restore);
    }

    static ENV: Mutex<()> = Mutex::new(());

    /// Environment variables put back as they were when dropped.
    struct Restore(Vec<(&'static str, Option<std::ffi::OsString>)>);

    impl Restore {
        fn of(names: &[&'static str]) -> Self {
            Self(
                names
                    .iter()
                    .map(|name| (*name, env::var_os(name)))
                    .collect(),
            )
        }
    }

    impl Drop for Restore {
        fn drop(&mut self) {
            for (name, value) in &self.0 {
                match value {
                    Some(value) => env::set_var(name, value),
                    None => env::remove_var(name),
                }
            }
        }
    }

    #[test]
    fn connect_errors_map_canonical_codes() {
        let error = connect_error(404, br#"{"code":"not_found","message":"model not found"}"#);
        assert!(matches!(
            error,
            Error::Service { status: Status::NotFound, message } if message == "model not found"
        ));
    }

    #[test]
    fn http_status_is_used_when_connect_code_is_missing() {
        let error = connect_error(503, b"unrecognized response");
        assert!(matches!(
            error,
            Error::Service {
                status: Status::Unavailable,
                ..
            }
        ));
    }

    #[test]
    fn unknown_connect_code_is_not_replaced_by_http_status() {
        let error = connect_error(500, br#"{"code":"unknown","message":"opaque failure"}"#);
        assert!(matches!(
            error,
            Error::Service {
                status: Status::Unknown,
                message
            } if message == "opaque failure"
        ));

        let error = connect_error(500, b"");
        assert!(matches!(
            error,
            Error::Service {
                status: Status::Internal,
                ..
            }
        ));
    }

    #[test]
    fn bracketed_ipv6_targets_are_split() {
        assert_eq!(
            split_target("[::1]:9000").ok(),
            Some(("::1".to_owned(), 9000))
        );
    }

    #[test]
    fn host_targets_are_split() {
        assert_eq!(
            split_target("localhost:9000").ok(),
            Some(("localhost".to_owned(), 9000))
        );
    }

    #[test]
    fn malformed_targets_are_rejected() {
        for target in ["localhost", "localhost:not-a-port", "[::1", "[::1]9000"] {
            assert!(split_target(target).is_err(), "{target} should be rejected");
        }
    }

    #[test]
    fn surrounding_whitespace_and_an_http_scheme_are_read() {
        for target in [
            "  localhost:9000  ",
            "http://localhost:9000",
            "http://localhost:9000/",
        ] {
            assert_eq!(
                split_target(target).ok(),
                Some(("localhost".to_owned(), 9000)),
                "{target} should be read"
            );
        }
    }

    #[test]
    fn an_unbracketed_ipv6_address_is_not_read_as_a_port() {
        let error = split_target("::1:9000").expect_err("an unbracketed address names no port");
        assert!(
            matches!(&error, Error::Transport(message) if message.contains("[::1:9000]:PORT")),
            "{error}"
        );
    }

    #[test]
    fn addresses_this_client_cannot_reach_are_legible() {
        let error = split_target("https://localhost:9000").expect_err("no tls is spoken");
        assert!(
            matches!(&error, Error::Transport(message) if message.contains("https")),
            "{error}"
        );
        let error = split_target("localhost:9000/sysml").expect_err("no path is served");
        assert!(
            matches!(&error, Error::Transport(message) if message.contains("path")),
            "{error}"
        );
    }
}
