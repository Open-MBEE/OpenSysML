---
name: opensysml-rust
description: Parse, query, instantiate, execute and verify SysML v2 models from Rust with the blocking `opensysml` crate (crates.io), which drives the OpenSysML sysml-grpc service with no async runtime — requiring a clean model, matching on Value, running actions and calcs, checking requirements, matching the one Error enum that keeps refused calls apart from reported model failures. Use when writing Rust that reads or runs SysML v2 models.
---

# Using OpenSysML from Rust

`opensysml` is a blocking client for the `sysml-grpc` service; it starts a private one for you and
pulls in no async runtime, so it is safe to call from inside one. MSRV is Rust 1.83. Reference:
`docs/guide/09-clients.md` (From Rust), `client/rust/README.md` and `docs/reference/rust-api.md` in
the OpenSysML repository.

## Install and get the service binary

```toml
[dependencies]
opensysml = "0.9"     # or { path = ".../OpenSysML/client/rust/opensysml" } from a checkout
```

Binary resolution is `$OPENSYSML_GRPC_BINARY`, then `~/.opensysml/bin/sysml-grpc` (shared with the
other clients), then a download of the release `$OPENSYSML_GRPC_VERSION` names (default: the one
the crate was built against) verified against the digests embedded in the crate, then `sysml-grpc`
on `PATH`. A crate built from a git checkout has no pin for its own release: point
`$OPENSYSML_GRPC_BINARY` at a `make build-grpc` binary instead. Without a binary the call returns
`Error::BinaryNotFound` listing where it looked.

## Load, and require a clean model

```rust
use opensysml::{Connection, Error, ParseOptions};

let connection = Connection::connect()?;    // $OPENSYSML_SERVICE, else a private child
let model = connection.parse_file("model.sysml", &ParseOptions::default())?;
model.require_ok()?;                        // Error::ModelErrors carrying the diagnostics
for d in model.errors() {                   // Diagnostic: severity, message, code, span
    eprintln!("{}: {}", d.code, d.message);
}
```

A model with syntax errors still parses; branch on `d.code` (`"syntax"`, `"unresolved"`, ...),
never on message text. `opensysml::load(path)` / `loads(source)` connect and parse in one call.
`ParseOptions { strict_conformance: true, ..Default::default() }` makes OpenSysML-only notation an
error. For several files use `connection.parse_sources(...)` with `SourceDocument`s; `parse_file`
reads one document, so imports across files do not resolve. `Connection::private()` and
`Connection::external(host, port)` are the explicit forms; `Drop` stops a private child, and
closing an external connection never stops its service.

## Evaluate, instantiate, run

```rust
use std::collections::BTreeMap;
use opensysml::{RunOptions, Value};

model.eval("Demo::sedan::mass")?;                                // Value::Real(1200.0)

let built = model.instantiate("Demo::sedan")?;
built.instance.feature("mass").and_then(|f| f.value());          // Some(Real(1200.0))

let inputs = BTreeMap::from([("x".to_string(), Value::Integer(10))]);
let run = model.execute_action("Demo::addFive", &inputs, &RunOptions::default())?;
run.outputs.get("result");                                       // Some(Integer(15))

let calc = model.calc("Demo::Margin", &[Value::Real(1200.0), Value::Real(2000.0)], None)?;
calc.value;                                                      // Some(Real(800.0))
```

`Value` is an enum (`Integer`, `BigInteger`, `Real`, `Complex`, `Boolean`, `Text`, `Quantity`,
`EnumLiteral`, `InstanceRef`, `Sequence`, `Set`, ...); match it. `==` is structural; use
`Value::same_value` for SysML equality (`1` and `1.0`, `1 m` and `100 cm`). The "no value"
variants (undetermined, unset, null) are answers, never errors; do not read them as 0 or `false`.

## Verify

```rust
use opensysml::VerifyOptions;

let options = VerifyOptions { subject: Some("Demo::truck".into()), ..Default::default() };
let v = model.verify_requirement("Demo::Vehicle::lightEnough", &options)?;
v.holds;          // false — an answer, not an Err
v.evaluated();    // true: the model answered; false when evaluation failed (see v.error)
v.condition;      // "mass < 2000.0"
println!("{v}");  // one readable line
model.satisfied(None)?;   // every `assert satisfy R by x;` holds
```

`verify_constraint` takes the same options; `verify_satisfaction(scope, &options)` returns every
verdict. `v.require_evaluated()?` turns an undecided verdict into an `Err`.

## Errors

`opensysml::Error` is one enum over every failure:

- `Error::Service { status, message }` — the call was refused; `err.status()` is `Some(Status)`
  (`NotFound` for an unreadable file, ...).
- `Error::Model(..)` — the call was answered and reports a model failure (an expression that will
  not evaluate, `model.symbol` of an unknown name). Related: `ModelErrors` (`require_ok`),
  `SymbolNotFound { name, suggestions }` (`model.lookup`), `Execution`, `WrongKind`,
  `AnalysisRun` (partial result), `Edit`. `err.diagnostics()` returns what any of them carries.
- `MissingCapability` (service too old), `BinaryNotFound`, `ChecksumMismatch`, `UnpinnedRelease`,
  `Transport`, `Decode`, `InvalidRequest`, `UnsupportedValue`.

## Pitfalls

- Reuse one `Connection`; clones share the private child and its parse cache.
- `find`/`get` return `Option`; `symbol` and `lookup` return `Err`.
- Use qualified names (`Package::Def::feature`) wherever a symbol is named.
- Every domain type has `wire()` for a protobuf field the typed surface does not expose yet.
- Write models with the `sysml-v2-modeling` skill; `sysml -validate` reports the same diagnostics as
  `model.diagnostics()`.
