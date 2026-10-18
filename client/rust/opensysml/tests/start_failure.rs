#![allow(missing_docs)]

#[cfg(unix)]
use std::sync::Mutex;

/// Every test rewrites the process environment, so they take turns.
#[cfg(unix)]
static ENVIRONMENT: Mutex<()> = Mutex::new(());

/// Starts the private service from `binary` and returns its startup error.
#[cfg(unix)]
fn start_failure(binary: &std::ffi::OsStr) -> opensysml::Error {
    use std::env;

    let _environment = ENVIRONMENT
        .lock()
        .unwrap_or_else(|poison| poison.into_inner());
    let prior = env::var_os("OPENSYSML_GRPC_BINARY");
    env::set_var("OPENSYSML_GRPC_BINARY", binary);
    let result = opensysml::Connection::private();
    match prior {
        Some(value) => env::set_var("OPENSYSML_GRPC_BINARY", value),
        None => env::remove_var("OPENSYSML_GRPC_BINARY"),
    }
    result.expect_err("a binary that serves no address must fail startup")
}

/// Writes `body` as an executable shell script and returns the startup error
/// of running it as the service.
#[cfg(unix)]
fn script_failure(name: &str, body: &str) -> String {
    use std::env;
    use std::fs;
    use std::os::unix::fs::PermissionsExt;

    let dir = env::temp_dir().join(format!(
        "opensysml-start-failure-{}-{name}",
        std::process::id()
    ));
    fs::create_dir_all(&dir).expect("create the script directory");
    let script = dir.join(name);
    fs::write(&script, format!("#!/bin/sh\n{body}")).expect("write the script");
    fs::set_permissions(&script, fs::Permissions::from_mode(0o755)).expect("make it executable");
    let error = start_failure(script.as_os_str());
    let _ = fs::remove_dir_all(&dir);
    match error {
        opensysml::Error::ServiceStart(message) => message,
        other => panic!("unexpected startup error: {other}"),
    }
}

#[cfg(unix)]
#[test]
fn private_start_failure_is_observed() {
    let error = start_failure(std::ffi::OsStr::new("/bin/true"));
    match error {
        opensysml::Error::ServiceStart(message) => {
            assert!(
                message.contains("exited with code 0"),
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
    let message = script_failure("close-stdout-then-exit", "exec >&-\nsleep 0.1\nexit 3\n");
    assert!(
        message.contains("exited with code 3 without serving an address"),
        "startup failure lost the exit status: {message}"
    );
}

/// A child that takes its time to exit after closing stdout, as one does on a
/// loaded machine, is waited for: its own status is reported, not a kill's.
#[cfg(unix)]
#[test]
fn private_start_failure_waits_for_a_slow_exit() {
    let message = script_failure("close-stdout-exit-late", "exec >&-\nsleep 1\nexit 3\n");
    assert!(
        message.contains("exited with code 3 without serving an address"),
        "startup failure lost the exit status of a slow child: {message}"
    );
}

/// A child that dies of a signal is reported by that signal, not as an exit.
#[cfg(unix)]
#[test]
fn private_start_failure_reports_the_signal_that_ended_the_child() {
    let message = script_failure("close-stdout-then-die", "exec >&-\nkill -TERM $$\n");
    assert!(
        message.contains("ended with signal: 15") && message.contains("without serving an address"),
        "startup failure misread a signal death: {message}"
    );
    assert!(
        !message.contains("None"),
        "startup failure leaked an absent exit code: {message}"
    );
}

/// A child that closes stdout and then hangs is killed once its grace runs
/// out, and the error says so rather than inventing an exit.
#[cfg(unix)]
#[test]
fn private_start_failure_kills_a_child_that_hangs_after_closing_stdout() {
    use std::time::{Duration, Instant};

    let started = Instant::now();
    let message = script_failure("close-stdout-then-hang", "exec >&-\nexec sleep 60\n");
    let elapsed = started.elapsed();
    assert!(
        message.contains("was still running 5s later, so it was killed"),
        "startup failure did not report the kill: {message}"
    );
    assert!(
        !message.contains("exited with code"),
        "startup failure invented an exit for a killed child: {message}"
    );
    assert!(
        elapsed >= Duration::from_secs(5) && elapsed < Duration::from_secs(20),
        "the hanging child was not killed at the end of its grace: {elapsed:?}"
    );
}

/// A child that keeps stdout open but never reports is killed at the start
/// timeout, and the error says so.
#[cfg(unix)]
#[test]
fn private_start_failure_kills_a_child_that_never_reports() {
    let message = script_failure("never-report", "exec sleep 60\n");
    assert!(
        message.contains("did not report a listening address within 2.5s, so it was killed"),
        "startup failure did not report the kill: {message}"
    );
}
