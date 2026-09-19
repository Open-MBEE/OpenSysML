#![allow(missing_docs)]

use std::env;
use std::io::{BufRead, BufReader};
use std::path::PathBuf;
use std::process::{Child, Command, Stdio};

use opensysml::Connection;

struct ExternalService {
    child: Child,
    host: String,
    port: u16,
}

impl Drop for ExternalService {
    fn drop(&mut self) {
        let _ = self.child.kill();
        let _ = self.child.wait();
    }
}

fn service_binary() -> Option<PathBuf> {
    env::var_os("OPENSYSML_GRPC_BINARY")
        .map(PathBuf::from)
        .filter(|path| path.is_file())
        .or_else(|| {
            let path = PathBuf::from(env!("CARGO_MANIFEST_DIR"))
                .ancestors()
                .nth(2)
                .map(|root| root.join("bin").join("sysml-grpc"))?;
            path.is_file().then_some(path)
        })
}

fn start_external_service() -> Option<ExternalService> {
    let Some(binary) = service_binary() else {
        if env::var("OPENSYSML_REQUIRE_SERVICE").ok().as_deref() == Some("1") {
            panic!("required sysml-grpc binary is unavailable");
        }
        eprintln!("skipping external-service test: sysml-grpc binary unavailable");
        return None;
    };
    let mut child = Command::new(binary)
        .args(["-port", "0", "-health-port", "0", "-report-address"])
        .stdout(Stdio::piped())
        .stderr(Stdio::null())
        .spawn()
        .unwrap_or_else(|error| panic!("could not start external service: {error}"));
    let stdout = child
        .stdout
        .take()
        .expect("external service stdout is piped");
    let mut lines = BufReader::new(stdout).lines();
    let address = lines
        .next()
        .expect("external service exited before reporting an address")
        .expect("could not read external service address");
    let (host, port) = address
        .rsplit_once(':')
        .expect("external service reported host:port");
    ExternalService {
        child,
        host: host.trim_matches(['[', ']']).to_owned(),
        port: port
            .parse()
            .expect("external service reported numeric port"),
    }
    .into()
}

#[test]
fn explicit_and_environment_external_connections_leave_service_running() {
    let Some(mut service) = start_external_service() else {
        return;
    };
    let target = if service.host.contains(':') {
        format!("[{}]:{}", service.host, service.port)
    } else {
        format!("{}:{}", service.host, service.port)
    };
    let explicit = Connection::external(service.host.clone(), service.port)
        .unwrap_or_else(|error| panic!("explicit external connection failed: {error}"));
    assert!(!explicit.server_info().wire().version.is_empty());
    assert_eq!(explicit.private_service_pid(), None);
    drop(explicit);
    assert!(service.child.try_wait().expect("service status").is_none());

    let prior = env::var_os("OPENSYSML_SERVICE");
    env::set_var("OPENSYSML_SERVICE", &target);
    let configured = Connection::connect()
        .unwrap_or_else(|error| panic!("environment external connection failed: {error}"));
    assert!(!configured.server_info().wire().version.is_empty());
    assert_eq!(configured.private_service_pid(), None);
    drop(configured);
    match prior {
        Some(value) => env::set_var("OPENSYSML_SERVICE", value),
        None => env::remove_var("OPENSYSML_SERVICE"),
    }
    assert!(service.child.try_wait().expect("service status").is_none());
}
