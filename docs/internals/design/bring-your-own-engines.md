# Bring your own engine: external engines, strategies and the trust they earn

A design for letting a user add to the [analysis framework](analysis-framework.md) without
changing OpenSysML: an **engine** of their own that answers questions about a model (a model
checker, a simulator, a statistical analyzer), a **strategy** of their own inside an existing
engine (a scheduling policy, a sampler), or a **tool** of their own that computes an action's
outputs — as a process speaking a documented protocol, as a WebAssembly module, or as Go
compiled into a binary they build. Nothing here is implemented. The note fixes the manifest that
registers them, the protocol they speak, the standing their answers are given and how they earn
more, and the surfaces that show them, so the work can be reviewed before code is written.

The framework note leaves this open in one sentence — *a WebAssembly or gRPC engine host is
compatible with the contract and left to a later note* — and designs one of the three kinds in
full: the tool manifest and the `tool:<name>` engine. This note generalizes that design to
engines and strategies, keeps every rule the framework fixed (the strength scale, the replay
gate, the referee, dispatch, determinism under `-jobs`), and adds the one thing external code
needs that built-in code does not: a rule for how much its word is worth.

## The problem this answers

The framework as designed admits three ways to add an engine, and two of them are closed to a
user:

| Way in | Who can use it | Cost |
|--------|----------------|------|
| A Go type implementing `Engine`, registered into `Default()` | a contributor to this repository | a pull request; the contract lives under `internal/` and cannot be imported |
| An executable in the tool manifest (`OPENSYSML_TOOLS`) | anyone with a binary | none — but it answers only `compute`: one invocation, inputs in, outputs out, *observed* |
| A Go `plugin` | nobody | rejected by the framework note: platform-limited, toolchain-locked, an ABI to protect |

So a site that owns a model checker for its own formalism, a simulator with a licence, a
scheduling heuristic that reproduces its fielded dispatcher, or a sampler tuned to its parameter
space, cannot bring it to a model without forking the binary. Three needs the tool protocol does
not meet:

- **An engine answers a question, not an action.** `holds`, `outcomes`, `sensitive` and
  `satisfiable` take a subject, bindings, free variables and bounds, run for a long time, need
  to be cancelled, and return a claim with a strength and a witness. `compute` takes inputs and
  returns outputs.
- **A strategy is consulted many times inside one run.** A scheduling policy is asked at every
  choice point which enabled token moves; a sampler is asked once per sweep for its rows. One
  process invocation per question is the wrong grain.
- **An external answer needs a standing.** A tool's is *observed* because nothing in OpenSysML
  knows what it should have computed. An external model checker's `unsat` is a claim of proof
  the framework can neither verify nor, without a rule, refuse — and the strength scale exists
  so that no result is printed stronger than it was earned.

## What exists to build on

- **The framework's engine contract**, unchanged: `Engine` with `Describe`, `Covers` and `Run`;
  `Result` with `Claim`, `Strength`, `Bounds`, `Witness`, `Reason`; the registry that refuses a
  duplicate name; dispatch by `auto`, an explicit name, or `all`. An external engine is one more
  `Engine` the coordinator sees; the adapter that speaks to the process is the whole addition.
- **The tool manifest and protocol.** One entry per executable in a directory the environment
  names, one JSON object in on standard input, one out on standard output, every failure a typed
  error, no value invented. The engine protocol is this protocol with a session and more
  message kinds.
- **The framework already trusts external processes, by declaration and probe.** `solve`
  reports *proved* on z3's `unsat`; `solve.DeclaredCapabilities` states what a known backend
  supports without running it and `Capabilities.Probed` records when the answers came from
  running it instead; a capability the backend refused stops a query before it is sent. The
  manifest declares, the handshake probes, and a mismatch refuses — the same shape.
- **The replay policy.** The [SMT design](smt-model-checking.md)'s `-schedule replay:<file>`
  follows a witness move for move and refuses a move that is not enabled, naming it. It is the
  gate every external witness passes through, and the file format every external engine
  writes.
- **`-engine all` as referee.** The framework runs every covering engine and turns a
  contradiction with `explore` into a disagreement in the interpreter's favor. Run over a corpus
  of models with known outcomes, that is a harness that measures an external engine.
- **Framed messages on standard input.** `cmd/sysml-lsp` speaks the Language Server Protocol
  over standard input and output, and `internal/stdiorpc` serves the gRPC service the same way.
  A long-lived process on a pair of pipes is a transport this repository already runs and
  tests.
- **The public Go API.** `client/opensysml` is the one package with a compatibility
  commitment, answered in process by the engine the importing binary links. It is where a Go
  engine's contract is exposed, because it is the only place a Go program outside this
  repository may import.

## Three kinds of contribution

| Kind | Manifest `kind` | Answers | Consulted | Standing of its answers |
|------|-----------------|---------|-----------|--------------------------|
| **tool** | `tool` | `compute` for an action annotated `ToolExecution` | once per performance | *observed*, as the framework fixes |
| **engine** | `engine` | any of `evaluate`, `outcomes`, `holds`, `sensitive`, `satisfiable`, `sweep`, as it declares | once per question, in a session that lives for the plan | existential claims *witnessed* after replay, else *not covered*; universal claims at the strength the site **admits**, *observed* by default |
| **strategy** | `policy`, `sampler` | a slot inside a built-in engine: which enabled token moves, which rows a sweep runs | many times per run, in a session that lives for the run | none of its own — a run under an external policy is a fixed-policy run, *observed*, as under `reverse` |

A fourth kind is not an engine at all and is named only to exclude it: a **rule written in the
model** — a `DocumentQueries` query or a viewpoint concern whose rows are diagnostics. That is a
future built-in engine over the query layer, listed as such in the framework note, and needs no
manifest because the model carries it.

## The manifest

The framework's tool manifest generalizes to one **manifest entry format** read from two
directories: `OPENSYSML_TOOLS`, whose meaning does not change, and `OPENSYSML_ENGINES`, which
holds engines and strategies. Either may hold any kind; the two names exist so that a site can
keep the executables its models name apart from the engines its analysts choose. Each entry is
one JSON file, named for the engine:

```json
{
  "kind": "engine",
  "name": "spin-bridge",
  "version": "1.4.0",
  "command": ["/opt/spin-bridge/bin/spin-bridge", "--serve"],
  "transport": "stdio",
  "protocol": 1,
  "answers": ["holds", "outcomes"],
  "subjects": ["action"],
  "model": ["sources", "graphs:1"],
  "bounds": ["depth", "steps", "runs"],
  "witness": "schedule",
  "authority": "bounded",
  "admit": "observed"
}
```

The fields are the framework's `Description`, written down so that **listing engines spawns no
process**: `sysml -engines` reads the manifests and prints each entry with its source
(`spin-bridge 1.4.0: engine from /etc/opensysml/engines/spin-bridge.json, admitted observed`),
and the process runs only when a question reaches it. What the entry declares is verified on
the first `describe` of a session (below), and an engine whose own description disagrees with
its manifest — a different name, version, protocol or a smaller set of questions — is refused
with a typed error naming the field, never quietly trusted for what it said or for what the
manifest said.

- `command` is the executable and its arguments, an absolute path or one relative to the
  manifest's directory. `transport` is `stdio` (a child process on a pair of pipes) or `grpc`,
  in which case `address` replaces `command` and the same messages travel as one bidirectional
  streaming RPC; the first stage implements `stdio` and defines `grpc`, since the messages are
  identical and the first external engines are local executables.
- `protocol` is the version of the message set the engine speaks; the host lists the versions
  it serves and refuses an entry outside them.
- `answers`, `subjects`, `bounds` and `witness` (`schedule`, `assignment` or `none`) are the
  framework's `Description`, and `Covers` refuses a question outside them before the process is
  asked.
- `model` names the forms of the model the engine reads (below); `graphs:1` is the lowered-graph
  export at version 1.
- `authority` is the strongest strength the engine says it can produce; `admit` is the strongest
  the site allows it to be printed with, and is the subject of the standing section.
- `concurrent`, default true, says whether one process may serve several runs at once; an
  engine that cannot gets one process per run under `-jobs`.

A strategy's entry is the same file with `kind` `policy` or `sampler`, a `protocol`, and no
question fields:

```json
{ "kind": "policy", "name": "priority", "version": "0.3", "command": ["./priority-policy"], "protocol": 1 }
```

Two rules the manifest enforces that the tool manifest already implied:

- **Manifests come from the environment, never from a workspace or a model.** A model may name
  a `toolName`; it may never name an executable. A manifest directory under a workspace is not
  read, and a `.sysml` file cannot register anything. The LSP and the gRPC service read the
  same directories as the CLI and spawn an external engine only when a request names one, never
  on an edit or a parse.
- **No discovery on `PATH`.** `solve` finds z3 and cvc5 on `PATH` because they are two known
  programs with known dialects; an engine is known only by its manifest. A name that is
  registered by two entries is the framework's duplicate-registration error, reported at
  startup in `-engines`, and neither entry answers.

## The engine protocol

A **session** is one process for one plan. The coordinator starts it at the plan's first
question to the engine and ends it when the plan ends or the deadline passes, so an engine that
loads a formalism or warms a solver pays once per plan, and under `-jobs` several questions to
one engine share one process unless the entry declares `"concurrent": false`, in which case the
coordinator starts one process per run. Standard error is captured and printed with a *not
covered* result's reason, never parsed.

Messages are JSON-RPC 2.0 objects, one per line — the envelope `internal/stdiorpc` already
speaks, with the newline in place of its `Content-Length` header — so a request carries an `id`
its answer repeats and a notification carries none. Both sides may send requests; both answer.
The host sends:

| Request | Params | Answer |
|---------|--------|--------|
| `describe` | the protocol versions the host serves | the engine's description — verified against the manifest before anything else is sent |
| `covers` | the question, and the model in the forms the entry asked for | `covers: true`, or `covers: false` with `reason` |
| `run` | the question, the model, the `bounds` the question fixes, the `budget` (deadline as an absolute time, `depth`, `steps`, `runs`, `memory`) | the result (below); `progress` notifications may precede it |
| `cancel` | the `id` of a running `run` | none; the `run` answers with what it has, marked by the bound it reached |

The engine sends `progress` (`{"method":"progress","params":{"id":3,"runs":120,"depth":17}}`),
which the coordinator prints as it prints `explore`'s progress today, and answers. An answer is
`{"id":3,"result":{…}}` or `{"id":3,"error":{"code":"…","message":"…"}}`, where `code` is
`unsupported` (a construct the engine met at run time and had not refused in `covers`),
`budget` (it stopped at a bound of its own), or `internal`.

### What a run carries

The engine cannot read the host's `*Model`, so the request carries the model in the forms the
entry declared, and the question in the model's own names:

- **`sources`**, always: every document of the model as `{path, text}`, and the version of the
  standard library the host embeds. An engine with its own SysML front end reads these and
  nothing else.
- **`graphs:<v>`**: the lowered `ActionGraph` or `StateGraph` of the subject and of every
  behavior it performs, in a versioned JSON form — nodes with their kinds and names, edges with
  guards as expression text, parameters with declared types and units, `HappensBefore` as
  edges, and the footprints once the [explicit-state design](bounded-model-checking.md) lowers
  them. This is a new export of the lowered IR and therefore a new compatibility surface, which
  is why it carries a version and why an engine names the versions it reads; the host serves
  the current version and the one before it, and a change that drops a version is a minor
  release under the rule in `CONTRIBUTING.md`, as any output-format change is.
- **`rdf`**: the model as the Turtle the RDF export writes, for engines built on graph stores.
- **The question**: its kind; the subject as a qualified name with the `subjects` binding as the
  CLI states it (`Plant::Reactor::regulate reactor`); the condition as a qualified name and as
  expression text; bindings as `{name, value, unit}`; free inputs as `{name, type, unit,
  domain}` with the domain the SMT design derives from the declared type and any assumed
  constraint; and the bounds the question fixes.

### What a result carries

The framework's `Result`, as JSON: `claim` (`holds`, `violated`, `sensitive`, `value`, `table`,
`none`), `strength` as the engine claims it, `bounds` taken and reached, `witness`, `reason`,
`values`, `elapsed`. The two fields the host treats specially:

- **`witness`** is a file in the `replay:` format the SMT design fixes — the schedule as
  `step N: <token>@<node> first of …` lines naming moves by the labels the lowered-graph export
  gave them — plus `inputs` as `{name, value, unit}`. The host writes it to a file and replays
  it through `explore`'s replay policy before the result is composed; the SMT design's rule
  that *a witness that cannot be followed is never silently resolved* applies to every external
  witness without exception.
- **`strength`** is what the engine *claims*. What is printed is decided by the next section,
  never by this field alone.

### Failures, classified

The framework distinguishes a fault in the question (an `error` that stops the plan, since no
engine would answer differently) from a reason particular to one engine (*not covered*, which
`auto` advances past). An external engine's failures are all of the second kind:

| Failure | Result |
|---------|--------|
| the process does not start, or `describe` disagrees with the manifest | `Covers` refuses: *not covered: engine 'spin-bridge' at /opt/… did not start (exit 127)*; `-engines` lists the same status |
| `covers: false` | *not covered* with the engine's reason, in the plan; `auto` advances |
| a line that is not JSON, a missing `id`, a field of the wrong type, a result whose `claim` and `strength` are not a pair the framework admits | *not covered: engine 'spin-bridge' broke protocol: …*, the session ended; `auto` advances |
| the process exits during a `run`, or the deadline passes without an answer to `cancel` | *not covered* with the exit status or the bound; `auto` advances |
| an `error` answer | *not covered* with its `code` and `message` |
| a witness that fails replay | *not covered: engine 'spin-bridge' reports a violation; its witness does not replay (move 3, 2@vent is not enabled)* — the framework's rule, never *violated* |
| an existential claim from an entry whose `witness` is `none` | *not covered: engine 'spin-bridge' reports a violation and produces no replayable witness* |

The one place the rule differs from `solve`'s is deliberate: a built-in engine that returns a
malformed `Result` has broken its own contract and that is an `error`, because it is this
repository's bug; an external engine that does the same is *not covered*, because the plan's
other engines can still answer and the user chose to let this one try. Under `-engine <name>`
the same *not covered* is the result, printed with the reason, as the framework says of every
explicit selection.

## The standing of an external answer

The framework's strength scale is what makes external engines admissible at all: every result
prints how it is supported, so an external answer does not have to be believed or rejected, only
labeled. Three rules decide the label.

**An existential claim is *witnessed* when its witness replays, and *not covered* otherwise.**
This is the framework's rule for `smt`'s `sat` and it needs no trust: the interpreter is
normative, the replay is the check, and an external model checker whose violation replays has
found a real violation whatever its own soundness. The witness dominates every universal claim
about the same question, as the framework's first composition rule says.

**A universal claim is printed at the strength the site admits, and the site admits *observed*
until it says otherwise.** There is no replay for a proof. The framework trusts z3's `unsat`
as *proved* because the encoding is this repository's and refereed against `explore`; a
third-party engine's encoding is nobody's here. So an external engine's `holds` at claimed
strength *bounded* is composed at `min(claimed, admit)` — *observed* by default — with the
claim preserved: `holds (observed: engine 'spin-bridge' reports bounded at depth 40; admitted
observed, not refereed)`. This follows the tool rule (*a tool's answer is observed*) rather
than discarding the claim as *not covered*, because the engine did run and did report, and an
analyst who trusts it can read what it said. A site raises `admit` in the manifest, and the
framework accepts an `admit` above *observed* only with a **referee record** beside the entry.

**A referee record is earned on a corpus, by `all`.** `sysml -referee <engine> <dir>` runs
every model under `<dir>` that carries an expected outcome set — the repository's conformance
corpus, or a site's own — under `-engine all` with the external engine beside `explore` (and
`smt` and `check` once they exist), and writes `<name>.referee.json` recording the case set's
hash, the count, how many agreed, every disagreement with its case, and the engine's version. An
`admit` above *observed* is accepted only when a record exists for that version, its case set is
the one named, and every case agreed; the standing line then prints the record —
`holds (bounded at depth 40 by engine 'spin-bridge'; refereed on 412 cases)` — so a reader knows
the strength was admitted, not earned in this repository. A record does not make the engine
sound; it makes the site's decision to trust it visible and revocable. The same run under `all`
outside the harness applies the framework's third rule as it stands: an external `holds` beside
an `explore` witness is a disagreement resolved for the interpreter, and the external result
becomes *not covered* with the disagreement as its reason, whatever the record says.

Three consequences the framework's composition rules already carry, stated for external
engines:

- `auto` orders engines by *admitted* authority, and at equal authority a built-in engine
  precedes an external one, so an unrefereed engine is reached by `auto` only when every
  built-in engine refused or ran out. `-engine <name>` selects it directly; `all` includes it.
- An external engine never becomes the referee. Replay is through the interpreter, agreement is
  measured against `explore`, and an external engine's result is never the one a built-in
  result is demoted for.
- Two *observed* results that differ are a *witnessed* sensitivity only when both are
  executions of the interpreter (`reverse` gives `x = 1`, `declared` gives `x = 2`). A
  simulator's `x = 2` beside `run`'s `x = 1` is reported as two observations that differ, both
  engines named, and no claim: the simulator may be wrong, or the model may be sensitive, and
  nothing here can tell which.

## Strategies

A strategy fills a slot a built-in engine already has and asks nothing of the framework's
strength rules, because the engine it runs inside decides the standing: a run under any fixed
policy is *observed*; a sweep's rows are *observed* each. Two slots are designed here.

**Scheduling policy** — `-schedule policy:<name>`, `%schedule policy:<name>`, and the same
value in the `schedule` request field, beside `reverse`, `declared`, `seed:<n>` and `explore`.
The `scheduler` in `internal/core/runtime` resolves every choice point of a run through one
seam, and the external policy is one more resolution: at each choice point the host sends
`choose` with the step, the tokens able to act with their trace labels (`2@heat, 3@vent`) and
whether each is parked or held, and the policy answers with the index it moves; for a due-time
tie (`pickDue`) the same message with the due tokens. The choice is recorded in the trace as
every policy's choices are, so the run replays under `replay:` without the policy present, and a
witness from such a run is a witness like any other. A policy that answers an index out of range,
or does not answer within `OPENSYSML_TOOL_TIMEOUT`, fails the run with a typed error naming the
step, as a bad `seed:` value is refused today. `explore`, `check` and `smt` do not consult it:
they enumerate every choice, and a policy chooses one.

The cost is one round trip per choice point over the session's pipes, which the framework's
measurement of a plan (`plan: …`) reports as the policy's share of the run, so the trade is
visible; a policy that is too slow for a long run is the case the WebAssembly host below is
for.

**Sampler** — `-sampler <name>` beside `-samples <n> -seed <s>`, and `%sampler`. The host sends
`sample` once per sweep with the ranges (`{parameter, type, unit, from, to}`), `n` and the
seed, and the sampler answers with the rows. Rows are validated against the ranges as
`-samples`'s own are — a value outside its range or of the wrong type is a typed error naming
the row — and the table is built and reported as any sweep's is.

Determinism is a property of the interpreter's own policies (`seed:<n>` draws the same values
on every platform) that an external strategy may or may not have. It is tested, not assumed:
`-referee <policy> <dir>` runs the policy twice on every model under `<dir>` and records in its
referee record whether it chose alike, and a run report cites the record —
`policy:priority (external; deterministic on 40 of 40 runs)`, `(external; chose differently on
3 of 40 runs)`, or `(external; not refereed)` when there is none. A non-deterministic policy is
not an error — the trace still replays — but a reader should know the run is one draw.

Slots deliberately not opened here: a reduction heuristic for `check` (its soundness argument
is the design's, and an external persistent-set function could break it invisibly — if wanted,
it is admitted only in the unreduced-comparison mode the explicit-state design already has), an
encoding fragment for `smt` (the encoding is one semantics and stays in one place), and a solver
backend for `solve` (`OPENSYSML_SMT` already names any SMT-LIB2 process, which is that
engine's strategy slot).

## In-process Go

A Go program that already links OpenSysML through `client/opensysml` should be able to add an
engine without a process. The public package gains the framework's contract types — `Engine`,
`Question`, `Result`, `Strength`, `Budget`, `Coverage`, `Description` — and one option:

```go
client, err := opensysml.New(opensysml.WithEngine(myEngine))
```

The engine receives the model through the same public handle the client's other operations use
(parse, lookup, evaluate, the lowered-graph export as Go values rather than JSON), not through
`internal/`, so the compatibility commitment the package already makes covers it. A Go engine
registers with the same `Description` a manifest declares and is listed by `%engines` and
`ListEngines` as `in-process`; its standing is decided by the same rules — its existential
claims replay, its universal claims are admitted at *observed* unless the program passes a
referee record with `WithEngineReferee(name, record)`. The `sysml` binary itself does not load
Go engines; a site that wants one in the CLI builds a `main` of a few lines over the public
package, which the reference documents.

This is the one place the contract's Go shape becomes public API, and the reason to do it late:
the framework's first stages will move the types, and a public package must not.

## WebAssembly

For a strategy consulted at every choice point, or an engine a site wants to distribute without
a build per platform, a **WebAssembly module** is the in-process form of the same protocol: the
manifest entry has `"module": "priority.wasm"` in place of `command`, the host instantiates it
once per session in a runtime implemented in Go without cgo (so `sysml` stays a single static
binary on every platform it ships to), and the messages are the same JSON, passed as bytes
through one exported function `handle(ptr, len) (ptr, len)`. The module has no file system, no
network and no clock beyond the deadline the host passes, which is stricter than a process and
is the point: a policy or sampler needs none of them, and an engine that does is a process. An
engine author writes the message handling once and links it into a module or a program.

The host is the last stage, because it adds a dependency and a runtime to audit, and because
nothing in the protocol depends on it: every engine and strategy above runs as a process first.

## User surface

Existing flags, commands, RPCs and their outputs keep their meaning. Added:

| Surface | Addition |
|---------|----------|
| CLI | `-engines` lists external engines and strategies with kind, source, version, admitted strength and referee status; `-engine <name>` selects an external engine as it selects a built-in one; `-schedule policy:<name>`; `-sampler <name>`; `-referee <engine> <dir>` |
| REPL | `%engines` shows the same; `%schedule policy:<name>`; `%sampler <name>` |
| gRPC/Connect | `ListEngines` includes external engines with a `source` field; `schedule: "policy:<name>"` and a `sampler` field on the sweep request; the `engines_external` service capability |
| Environment | `OPENSYSML_ENGINES` beside `OPENSYSML_TOOLS`; `OPENSYSML_TOOL_TIMEOUT` bounds one message round trip for every kind |
| Report | the standing line names the engine's origin and admission: `(observed: engine 'spin-bridge' reports bounded; admitted observed, not refereed)`, `(bounded at depth 40 by engine 'spin-bridge'; refereed on 412 cases)`, `policy:priority (external; deterministic on 40 of 40 runs)` |
| Reference | `docs/reference/` gains the manifest format, the message set with its JSON Schema, the lowered-graph export format, and the referee record |

Every addition is a new flag, field, value or file under the versioning rule in
`CONTRIBUTING.md` and therefore patch material, with the same caveat the framework note
carries for `-json`: the lowered-graph export is a new output format whose later change is
minor, which is why it is versioned from its first release.

## Security

A manifest entry runs an executable with the user's privileges, as `OPENSYSML_SMT` does today
and as any tool in a build does. What the design adds is that nothing a model or a workspace
contains can cause it:

- Manifests are read from directories the environment names, and from nowhere else. A model
  names a `toolName` or, through `-engine`, an engine name; the mapping to an executable is the
  site's.
- `-engines` prints the path every entry resolves to, so what will run is inspectable before
  anything runs.
- The LSP and the gRPC service spawn an external engine only for a request that names one and
  never for a diagnostic, a completion or a parse. A service deployed for untrusted clients
  restricts `-engine` to built-in engines with `OPENSYSML_ENGINES` unset, which registers none.
- A WebAssembly module runs with no capabilities beyond the messages it is passed.
- Standard error is captured and bounded, as `solve` bounds a solver's, and never interpreted.

## Test contract

- **Manifest:** an entry of each kind loads and lists; a malformed entry, an unknown `kind`, a
  `protocol` outside the served set, a relative `command` outside the manifest directory, and a
  duplicate name each produce a typed error visible in `-engines` and register nothing; a
  manifest directory under the workspace is not read; `-engines` runs with no process spawned
  (asserted by an entry whose command is a script that records its invocation).
- **Handshake:** a stand-in engine whose `describe` differs from its manifest in name, version,
  protocol or `answers` is refused naming the field; one that matches is used.
- **Protocol:** the stand-in exercises `covers` true and false, `run` with `progress` and a
  result of each `claim`, `cancel` at a deadline returning a bounded result, an `error` of each
  code, a line that is not JSON, a missing `id`, an exit mid-run, and no answer to `cancel` —
  each producing the *not covered* result the failure table names, with `auto` advancing to
  `explore` and `-engine <name>` stopping with the reason. Run under `-race` with `-jobs 8` and
  a stand-in that declares `concurrent: false`.
- **Standing:** an external `holds` claimed *bounded* is composed *observed* with the claim in
  the standing line; with a referee record for its version and case set it is composed
  *bounded* naming the record; with a record for another version it is *observed* and
  `-engines` says why; an external `violated` whose witness replays is *witnessed*; one whose
  witness fails replay, or from an entry with `witness: none`, is *not covered*; an external
  `holds` beside an `explore` witness is a disagreement for the interpreter regardless of the
  record; `auto` reaches an unrefereed engine only after every built-in engine refused.
- **Referee harness:** `-referee` over the conformance corpus with a stand-in that agrees on
  every case writes a record that admits it; with a stand-in that disagrees on one case writes a
  record that does not, naming the case; a record's hash changes when the case set does.
- **Strategies:** `policy:<name>` with a stand-in policy produces a trace that replays under
  `replay:` with the policy absent; an out-of-range index and a timeout fail the run with a
  typed error naming the step; `-referee` on a deterministic stand-in records `40 of 40` and
  on one that draws names the count, and the run report cites the record or its absence;
  `explore` under `policy:<name>` is refused; `-sampler` with a stand-in produces a table validated against the ranges, and a row outside
  them is a typed error naming the row.
- **In-process Go:** a test program over `client/opensysml` registers an engine, lists it as
  `in-process`, and gets the same standing rules; `go vet` and the package's compatibility test
  cover the new exported types.
- **WebAssembly:** the stand-in policy compiled to a module gives the same trace as the process
  form on the corpus; a module that reads a file or the clock fails to instantiate with a typed
  error.
- **Wire, CLI and REPL compatibility:** `make proto-breaking` against `develop`, `make
  man-check` and the goldens pass; the JSON Schema for the message set validates every message
  the stand-ins exchange in the tests.

## Stages

Each stage is a pull request into `develop` that leaves the gate green. The stages depend on the
framework's: none starts before the framework's surface stage (`-engines`, `-engine`, the
standing line) and its tools stage (the manifest, the tool protocol) are merged, since this
design extends both; the first stage also needs the `replay:` policy, which the SMT design's
first stage delivers, since it is the gate every external witness passes through.

1. **External engines over standard input.** The manifest entry format for `engine`, read from
   `OPENSYSML_ENGINES`; the session, the message set and its JSON Schema; the `sources` and
   `graphs:1` model forms, the latter as the new versioned export of lowered graphs; the failure
   table; the standing rule with `admit` fixed at *observed* (no record yet); the stand-in
   engine and its tests; `-engines`, `%engines` and `ListEngines` listing external engines; the
   reference pages.
2. **Referee records.** `-referee <engine> <dir>`, the record format, `admit` above *observed*
   accepted against a matching record, the standing line naming it, the disagreement tests.
3. **Strategies.** `policy` and `sampler` entries; `-schedule policy:<name>` through the
   scheduler seam and `-sampler <name>` through the sweep plan; the determinism report; the
   `rdf` model form for engines, since it costs one call to the existing export.
4. **In-process Go.** The contract types and `WithEngine` in `client/opensysml`, the
   lowered-graph export as Go values, `WithEngineReferee`, the example `main`, the reference.
5. **WebAssembly host.** The `module` entry, the runtime, the `handle` ABI, the stand-in policy
   as a module, the capability tests.

The `grpc` transport is defined in stage 1 and implemented when the first remote engine needs
it; the messages do not change.

## What this does not change

- The built-in engines, their strengths and their referee role. `explore` remains the referee;
  an external engine is measured against it and never the reverse.
- The framework's rules. Every rule here is one of the framework's applied to an engine whose
  code is not this repository's: `Covers` refuses before work, a witness replays or is *not
  covered*, a budget reached is never a proof, completion order decides nothing, a
  contradiction is a disagreement for the interpreter. The one addition is `admit`, and it can
  only lower a strength.
- The tool manifest. `OPENSYSML_TOOLS` and the `tool:<name>` engine are as the framework note
  designs them; a tool entry gains nothing and loses nothing by the second directory.
- No accepted model is refused, no result changes, no flag, command, RPC or field is removed or
  renamed. A model that names no tool and a session that sets no manifest see nothing new but
  an empty section in `-engines`.

## Alternatives considered

- **Go plugins.** Rejected by the framework note for reasons that have not changed; the
  in-process form for Go is the public package, whose compatibility surface the repository
  already commits to.
- **Trust an external engine's claimed strength.** The scale would then say *proved* of a
  result nothing here checked, which is the laundering the framework exists to prevent. The
  site can grant it, visibly and against a record; the default cannot.
- **Refuse external universal claims as *not covered*.** Simpler than *observed* with the claim
  kept, and it would throw away an answer the user installed an engine to get. *observed* is
  what the framework already says of a tool, and a reader sees both what the engine said and
  what it is worth here.
- **Discover engines on `PATH` by a naming convention** (`sysml-engine-*`). Any program on
  `PATH` could then register itself into an analysis; the manifest makes the registration a
  file the site wrote and `-engines` can show.
- **Let a model name its engine.** The `AnalysisTooling` metadata names a tool by a name the
  site maps; naming an executable in a model would move the site's decision into the artifact
  the site receives from others.
- **One message per invocation for engines, as for tools.** A question that takes minutes
  needs cancellation and progress, and an engine that warms a solver per question would pay it
  per question; the session costs one long-lived process per plan, which the solver already is.
- **A binary protocol.** The tool protocol, the LSP and the stdio transport are all JSON on
  standard input; an engine author has a JSON library in every language and a schema to
  validate against, and the messages are small beside the model they carry.
- **Hand the engine an in-memory model over gRPC callbacks** instead of the sources and graphs.
  It would let an engine query the host's resolver lazily, at the cost of a second protocol and
  a session per callback; the sources and the versioned graph export are self-contained, can be
  saved with a result, and are what a reproducible run needs anyway.
