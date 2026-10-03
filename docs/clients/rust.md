# Rust

`opensysml` is a blocking Rust client for the service. Choose it when a Rust
application needs typed model, verification and execution results without an
async runtime in its normal dependency tree.

## Install

The client is published to crates.io with core releases:

```toml
[dependencies]
opensysml = "0.9"
```

For checkout development, use a path dependency:

```toml
[dependencies]
opensysml = { path = "../OpenSysML/client/rust/opensysml" }
```

The minimum supported Rust version is 1.83.

## First model

`loads` parses inline notation and returns a typed `Model`. Check its
diagnostics before evaluating an expression for a subject:

```rust
use opensysml::{loads, EvalOptions, Value};

const SOURCE: &str = r#"
package Demo {
  part def Vehicle {
    attribute mass default = 1500.0;
  }
  part sedan : Vehicle {
    attribute :>> mass = 1800.0;
  }
}
"#;

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let model = loads(SOURCE)?;
    if !model.ok() {
        for diagnostic in model.diagnostics() {
            eprintln!("{diagnostic}");
        }
        return Err("The model has errors.".into());
    }

    let result = model.evaluate(
        "mass",
        &EvalOptions {
            subject: Some("Demo::sedan".into()),
            ..Default::default()
        },
    )?;
    match &result.result {
        Value::Real(value) => println!("{value:.1}"),
        other => return Err(format!("Expected a real value, got {other:?}").into()),
    }
    Ok(())
}
```

```text
1800.0
```

The `Model` and its `Value` results expose parsing, symbols, instances,
execution, verification, analysis, editing, conversion and document queries
as blocking typed calls.

## Service source

A crate published from a release tag carries that release's service digests,
stamped from the release checksum manifest at publish time. With nothing
configured, it downloads the release it was built against, verifies it against
those digests and installs it in the shared cache `~/.opensysml/bin/sysml-grpc`.

`opensysml` 0.9.1 and earlier were published without these digests and do not
download the service. For them, set `OPENSYSML_GRPC_BINARY` or put `sysml-grpc`
on `$PATH`.

A crate built from a Git checkout, or asked for another release through
`OPENSYSML_GRPC_VERSION`, needs a matching embedded pin or
`OPENSYSML_ALLOW_UNPINNED_DOWNLOAD`, which trusts the checksum served beside
the binary. The client does not verify the manifest's Sigstore signature
itself. See the
[service-binary reference](../reference/clients.md#providing-the-service-binary).

## Next steps

- [Rust API reference](../reference/rust-api.md)
- [Client README, installation and conformance runner](https://github.com/Open-MBEE/OpenSysML/blob/main/client/rust/README.md)
- [Shared SysML model examples](https://github.com/Open-MBEE/OpenSysML/tree/main/examples)
