#![allow(missing_docs)]

// Its own test binary: the private child is process-wide, so a test elsewhere
// holding a connection in parallel would keep it alive past the drop below.

use std::env;
use std::path::PathBuf;
use std::thread;
use std::time::Duration;

use opensysml::Connection;

fn service_or_skip() -> Option<Connection> {
    match Connection::private() {
        Ok(connection) => Some(connection),
        Err(error) => {
            if env::var("OPENSYSML_REQUIRE_SERVICE").ok().as_deref() == Some("1") {
                panic!("required sysml-grpc service unavailable: {error}");
            }
            eprintln!("skipping service-backed Rust client test: {error}");
            None
        }
    }
}

#[test]
fn private_connections_share_one_child_and_last_drop_stops_it() {
    let Some(first) = service_or_skip() else {
        return;
    };
    let pid = first.private_service_pid();
    let second = match Connection::private() {
        Ok(connection) => connection,
        Err(error) => panic!("second private connection failed: {error}"),
    };
    assert_eq!(pid, second.private_service_pid());
    drop(first);
    let Some(pid) = pid else {
        return;
    };
    drop(second);
    for _ in 0..50 {
        if !PathBuf::from(format!("/proc/{pid}")).exists() {
            return;
        }
        thread::sleep(Duration::from_millis(100));
    }
    panic!("private service {pid} remained after last connection dropped");
}
