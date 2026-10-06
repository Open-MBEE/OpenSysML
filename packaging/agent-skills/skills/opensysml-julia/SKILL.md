---
name: opensysml-julia
description: Parse, query, instantiate, execute and verify SysML v2 models from Julia 1.10+ with the `OpenSysML` package (client/julia/OpenSysML), which drives the OpenSysML sysml-grpc service over Connect-JSON — requiring a clean model, evaluating expressions to native Julia values, running actions and calcs, checking requirements, telling the OpenSysMLError subtypes apart. Use when writing Julia that reads or runs SysML v2 models.
---

# Using OpenSysML from Julia

`OpenSysML` is a Julia 1.10+ client for the `sysml-grpc` service, speaking Connect-JSON over
`HTTP.jl`. It starts a private service for you. Reference: `docs/guide/09-clients.md` (From
Julia), `client/julia/OpenSysML/README.md` and `docs/reference/julia-api.md` in the OpenSysML
repository.

## Install and get the service binary

The package is not in the General registry; develop it from an OpenSysML checkout:

```julia
using Pkg
Pkg.develop(path = "client/julia/OpenSysML")
```

Binary resolution is `$OPENSYSML_BINARY`, then `$OPENSYSML_GRPC_BINARY`, then
`~/.opensysml/bin/sysml-grpc` (shared with the other clients), then `sysml-grpc` on `PATH`. A
missing binary is an error, not an implicit download: set `$OPENSYSML_GRPC_VERSION=latest` or a
release tag to download one, verified against the package's SHA-256 pins. `make build-grpc` in the
checkout produces `bin/sysml-grpc` to point `$OPENSYSML_BINARY` at.

## Load, and require a clean model

```julia
using OpenSysML

conn = connect()                     # $OPENSYSML_SERVICE, else a private child
try
    model = parse_file(conn, "model.sysml")
    if !isok(model)
        for d in errors(model)       # Diagnostic: severity, message, code, span
            println(d.code, ": ", d.message)
        end
    end
    raise_for_errors(model)          # throws ModelError naming the errors
finally
    close(conn)                      # stops a private child; never an external service
end
```

A model with syntax errors still parses; branch on `d.code` (`"syntax"`, `"unresolved"`, ...),
never on message text. `parse_file(conn, path; strict=true)` throws `ModelError` instead of
returning a broken model; `strict_conformance=true` makes OpenSysML-only notation an error.
`parse_source(conn, text)` parses inline source; `parse_sources` takes several documents, which
`parse_file` cannot (imports across files do not resolve from one). `external("host:port")` and
`private(; binary=...)` are the explicit constructors.

## Evaluate, instantiate, run

```julia
evaluate(model, "Demo::sedan::mass")                      # 1200.0
evaluate(model, "mass"; subject="Demo::sedan")            # 1200.0

inst = instantiate(model, "Demo::sedan")
inst["mass"]                                              # 1200.0

out = execute_action(model, "Demo::addFive"; inputs=Dict("x" => 10))
out["result"]                                             # 15

c = calc(model, "Demo::Margin"; arguments=[1200.0, 2000.0])
c.value                                                   # 800.0
```

Values decode to native Julia (`Int64`, `BigInt`, `Float64`, `Bool`, `String`, `Complex`) or to
the package's own types (`Quantity`, `InstanceRef`, ...). The "no value" results (undetermined,
unset, null) are answers, not errors; do not read them as 0 or `false`.

## Verify

```julia
v = verify_requirement(model, "Demo::Vehicle::lightEnough"; subject="Demo::truck")
holds(v)                   # false — an answer, not an exception
v.condition                # "mass < 2000.0"
v.error                    # non-empty only when evaluation failed (undecided)
OpenSysML.explain(v)       # one readable line; `explain` is not exported
satisfied(model)           # every `assert satisfy R by x;` holds
```

`verify_constraint` takes the same `subject`; `verify_satisfaction` returns the `Validation` that
`violated` and `undecided` filter.

## Errors

Everything thrown descends from `OpenSysMLError`:

- `ConnectError` (alias `ServiceError`) — the call was refused: `ModelFileNotFoundError` (an
  unreadable path), `ModelNotFoundError`, `InvalidRequestError`, `ServiceTimeoutError`,
  `UnsupportedOperationError`, `ServiceUnavailableError`, `ServiceCallError`.
- `DiagnosticError` — the call was answered and reports a failure: `ModelError` (parse errors) and
  `ExecutionError` (`ExecutionFailure` for an expression that will not evaluate, `WrongKindError`,
  `AnalysisRunError`).
- `SymbolNotFoundError`, `MissingCapabilityError` (service too old), `StaleServiceError`,
  `TransportError`, `ChecksumMismatchError`, `UnsupportedValueError`.

## Pitfalls

- `symbol(model, id)` and `find(model, name)` return `nothing` for an unknown name;
  `model["Pkg::Name"]` throws `SymbolNotFoundError`.
- Use qualified names (`Package::Def::feature`) wherever a symbol is named.
- Reuse one connection; the service caches parses.
- Write models with the `sysml-v2-modeling` skill; `sysml -validate` reports the same diagnostics as
  `errors(model)`.
