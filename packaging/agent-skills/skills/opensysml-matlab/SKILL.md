---
name: opensysml-matlab
description: Parse, query, instantiate, execute and verify SysML v2 models from MATLAB (R2019b+) or GNU Octave (7+) with the `+opensysml` package (client/matlab), which drives the OpenSysML sysml-grpc service over Connect-JSON — requiring a clean model, int64 integers, running actions and calcs, checking requirements, branching on opensysml:* error identifiers and opensysml.lastError. Use when writing MATLAB or Octave code that reads or runs SysML v2 models.
---

# Using OpenSysML from MATLAB or Octave

The `+opensysml` package is a client for the `sysml-grpc` service with no protobuf-generated code:
MATLAB uses `matlab.net.http`, Octave a `curl` subprocess. Reference: `docs/guide/09-clients.md`
(From MATLAB), `client/matlab/README.md` and `docs/reference/matlab-api.md` in the OpenSysML
repository.

## Install and get the service binary

```matlab
addpath('client/matlab')     % from an OpenSysML checkout
```

This client never downloads a binary. `opensysml.private` checks `OPENSYSML_GRPC_BINARY`,
`~/.opensysml/bin/sysml-grpc` (shared with the other clients), then `PATH`; `make build-grpc` in
the checkout produces `bin/sysml-grpc`. `private` starts its child through Java's
`ProcessBuilder`, so an Octave build without Java must use a running service:

```matlab
conn = opensysml.connect();                  % OPENSYSML_SERVICE, otherwise a private child
conn = opensysml.external('localhost:50051');
conn = opensysml.private('binary', '/path/to/sysml-grpc', 'timeout', 30);
```

`conn.close()` stops a private child and leaves an external service running. A private child is
not shared across connections.

## Load, and require a clean model

```matlab
conn = opensysml.connect();
cleanup = onCleanup(@() conn.close());       % stops a private child on any exit
model = opensysml.parseFile(conn, 'model.sysml');
if ~model.ok()
    errs = model.errors();                   % cell of diagnostic records
    for k = 1:numel(errs)
        fprintf('%s: %s\n', errs{k}.code, errs{k}.message);
    end
end
model.raiseForErrors();                      % opensysml:diagnostics:model
```

A model with syntax errors still parses; branch on the diagnostic `code` (`'syntax'`,
`'unresolved'`, ...), never on message text. `'raiseForErrors', true` makes the parse itself
raise; `'strict', true` (alias of `strictConformance`) makes OpenSysML-only notation an error.
`opensysml.parseSource(conn, text, 'name', 'p.sysml')` parses inline source. For several files use
`opensysml.parseSources(conn, {opensysml.SourceDocument.file('base.sysml'), ...})`; `parseFile`
reads one document, so imports across files do not resolve.

## Evaluate, instantiate, run

```matlab
model.evaluate('Demo::sedan::mass')                      % 1200 (double)
model.evaluate('mass', 'subject', 'Demo::sedan')         % 1200

inst = model.instantiate('Demo::sedan');
inst.feature_values('mass')                              % containers.Map -> 1200

run = model.executeAction('Demo::addFive', 'inputs', struct('x', int64(10)));
run.outputs.result                                       % int64(15)

c = model.calc('Demo::Margin', 'arguments', {1200.0, 2000.0});
c.value                                                  % 800
```

SysML Integers are exact `int64`; a MATLAB `double` is sent as a Real, so `struct('x', 10)` for an
Integer input fails with a type mismatch — write `int64(10)`. An Integer beyond `int64` is
`struct('bigInteger', '<digits>')`. Sequences are cell arrays. The "no value" results are structs,
not errors, and must not be read as 0 or `false`: `struct('unset', true)` and an undetermined
struct with `reason`, `lower` and `upper`.

## Verify

```matlab
v = model.verifyRequirement('Demo::Vehicle::lightEnough', 'subject', 'Demo::truck');
logical(v)          % false — an answer, not an error
v.condition         % 'mass < 2000.0'
v.error             % non-empty only when evaluation failed (undecided)
v.explain()         % one readable line
model.satisfied()   % every `assert satisfy R by x;` holds
```

`verifyConstraint` takes the same `subject`; `verifySatisfaction` and `validateInstance` return a
`Validation` with `valid`, `violated`, `undecided` and `explain`. `v.raiseForError()` turns an
undecided verdict into an error.

## Errors

Errors carry `opensysml:*` identifiers (plain errors in Octave, `MException`s in MATLAB); branch
on `err.identifier`, and read `opensysml.lastError()` for `diagnostics` and structured `details`.

- `opensysml:connect:<kind>` — the call was refused: `modelFileNotFound` (an unreadable path),
  `modelNotFound`, `symbolNotFound`, `invalidRequest`, `unavailable`, `serviceTimeout`,
  `unsupportedOperation`.
- `opensysml:diagnostics:<kind>` — the call was answered and reports a failure: `model` (parse
  errors), `execution` (an expression that will not evaluate, an unknown symbol, a type mismatch),
  `conversion`, `migration`, `edit:<failure>`.
- `opensysml:missingCapability` (service too old), `opensysml:staleService`,
  `opensysml:transport`, `opensysml:argument`, `opensysml:encode`, `opensysml:decode`.

## Pitfalls

- There is no `model[fqn]` indexing: `model.get('Pkg::Name')` resolves a qualified id,
  `model.find('Name')` a short name and returns empty when absent.
- Use qualified names (`Package::Def::feature`) wherever a symbol is named.
- Reuse one connection; the service caches parses.
- Write models with the `sysml-v2-modeling` skill; `sysml -validate` reports the same diagnostics as
  `model.errors()`.
