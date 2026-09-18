#![allow(missing_docs)]

use std::env;

use opensysml::{Connection, Error};

/// One test, because `$OPENSYSML_SERVICE` is process-wide.
#[test]
fn the_environment_decides_which_service_is_connected_to() {
    let prior = env::var_os("OPENSYSML_SERVICE");

    for empty in ["", "   "] {
        env::set_var("OPENSYSML_SERVICE", empty);
        match Connection::connect() {
            // An empty variable names no service, so a private child answers.
            Ok(connection) => assert!(connection.private_service_pid().is_some()),
            Err(error) => {
                if env::var("OPENSYSML_REQUIRE_SERVICE").ok().as_deref() == Some("1") {
                    panic!("required sysml-grpc service unavailable: {error}");
                }
                assert!(
                    !matches!(&error, Error::Transport(message) if message.contains("address")),
                    "an empty variable was read as an address: {error}"
                );
            }
        }
    }

    env::set_var("OPENSYSML_SERVICE", "localhost");
    let error = Connection::connect().expect_err("an address without a port is not connectable");
    assert!(
        matches!(&error, Error::Transport(message) if message.contains("host:port")),
        "{error}"
    );

    match prior {
        Some(value) => env::set_var("OPENSYSML_SERVICE", value),
        None => env::remove_var("OPENSYSML_SERVICE"),
    }
}
