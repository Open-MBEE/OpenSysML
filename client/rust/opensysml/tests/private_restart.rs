#![allow(missing_docs)]

// Its own test binary: the private child is process-wide, and this test kills it.

use std::env;
use std::process::Command;
use std::thread;
use std::time::Duration;

use opensysml::Connection;

#[cfg(unix)]
#[test]
fn a_private_child_that_died_is_replaced() {
    let first = match Connection::private() {
        Ok(connection) => connection,
        Err(error) => {
            if env::var("OPENSYSML_REQUIRE_SERVICE").ok().as_deref() == Some("1") {
                panic!("required sysml-grpc service unavailable: {error}");
            }
            eprintln!("skipping service-backed Rust client test: {error}");
            return;
        }
    };
    let pid = first.private_service_pid().expect("a private child");
    let killed = Command::new("kill")
        .args(["-KILL", &pid.to_string()])
        .status()
        .expect("kill runs");
    assert!(killed.success());
    for _ in 0..50 {
        if first
            .parse_content("package P;", &Default::default())
            .is_err()
        {
            break;
        }
        thread::sleep(Duration::from_millis(20));
    }

    let second = Connection::private()
        .unwrap_or_else(|error| panic!("a dead private child was not replaced: {error}"));
    assert_ne!(second.private_service_pid(), Some(pid));
    second
        .parse_content("package P;", &Default::default())
        .unwrap_or_else(|error| panic!("the replacement service failed: {error}"));
}
