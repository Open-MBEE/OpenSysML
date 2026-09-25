#![allow(missing_docs)]

#[cfg(unix)]
use std::sync::Mutex;

/// Both tests rewrite the process environment, so they take turns.
#[cfg(unix)]
static ENVIRONMENT: Mutex<()> = Mutex::new(());

#[cfg(unix)]
#[test]
fn private_start_failure_is_observed() {
    use std::env;

    let _environment = ENVIRONMENT
        .lock()
        .unwrap_or_else(|poison| poison.into_inner());
    let prior = env::var_os("OPENSYSML_GRPC_BINARY");
    env::set_var("OPENSYSML_GRPC_BINARY", "/bin/true");
    let result = opensysml::Connection::private();
    match prior {
        Some(value) => env::set_var("OPENSYSML_GRPC_BINARY", value),
        None => env::remove_var("OPENSYSML_GRPC_BINARY"),
    }
    let error = result.expect_err("an immediately exiting binary must fail startup");
    match error {
        opensysml::Error::ServiceStart(message) => {
            assert!(
                message.contains("code Some(0)") || message.contains("stderr tail"),
                "startup failure omitted exit evidence: {message}"
            );
        }
        other => panic!("unexpected startup error: {other}"),
    }
}

/// The address reader sees stdout close before the process is reaped; the
/// exit status must still be waited for rather than read as "still running".
#[cfg(unix)]
#[test]
fn private_start_failure_reports_the_exit_after_stdout_closes() {
    use std::env;
    use std::fs;
    use std::os::unix::fs::PermissionsExt;

    let dir = env::temp_dir().join(format!("opensysml-start-failure-{}", std::process::id()));
    fs::create_dir_all(&dir).expect("create the script directory");
    let script = dir.join("close-stdout-then-exit");
    fs::write(&script, "#!/bin/sh\nexec >&-\nsleep 0.1\nexit 3\n").expect("write the script");
    fs::set_permissions(&script, fs::Permissions::from_mode(0o755)).expect("make it executable");

    let _environment = ENVIRONMENT
        .lock()
        .unwrap_or_else(|poison| poison.into_inner());
    let prior = env::var_os("OPENSYSML_GRPC_BINARY");
    env::set_var("OPENSYSML_GRPC_BINARY", &script);
    let result = opensysml::Connection::private();
    match prior {
        Some(value) => env::set_var("OPENSYSML_GRPC_BINARY", value),
        None => env::remove_var("OPENSYSML_GRPC_BINARY"),
    }
    let _ = fs::remove_dir_all(&dir);
    let error = result.expect_err("a binary that closes stdout and exits must fail startup");
    match error {
        opensysml::Error::ServiceStart(message) => {
            assert!(
                message.contains("code Some(3)"),
                "startup failure lost the exit status: {message}"
            );
        }
        other => panic!("unexpected startup error: {other}"),
    }
}
