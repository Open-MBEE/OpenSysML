#![allow(missing_docs)]

// Its own test binary: the private child is process-wide, and this test needs
// to be the one that starts it.

use std::env;

use opensysml::{Connection, TOKEN_ENV};

// A private child is this process's own, on loopback: it serves without the
// token meant for a remote service, even when the environment names one.
#[test]
fn a_private_child_does_not_inherit_the_environment_token() {
    env::set_var(TOKEN_ENV, "not-for-the-child");
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
    // The second connection shares the child and carries a different token; a
    // child demanding the first would refuse it.
    env::set_var(TOKEN_ENV, "another-token");
    let second = Connection::private().expect("the shared child serves a connection");
    assert!(!second.server_info().version.is_empty());
    env::remove_var(TOKEN_ENV);
    drop(second);
    drop(first);
}
