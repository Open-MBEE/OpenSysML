# External engines

An external engine is a program the site installs beside OpenSysML that answers analysis
questions — a model checker, a simulator, a solver with a front end of its own — over its
standard input and output. It is registered by a manifest entry in the directory
`OPENSYSML_ENGINES` names, listed by `-engines`, `%engines` and `ListEngines` like any engine of
the build, selected by `-engine <name>`, reached by `-engine auto` after every built-in engine
refused, and consulted by `-engine all` beside them. Its answer never carries a strength the
interpreter did not check: a witness is replayed and the claim evaluated where the witness
says, and a universal claim without executions to replay is *not covered* with the claim kept
in the reason. The design is
[bring your own engine](../internals/design/bring-your-own-engines.md); this page is the
contract an engine author and a site operator hold each other to.

## The manifest entry

`OPENSYSML_ENGINES` names a directory of JSON files, one entry per file, read once at startup
beside `OPENSYSML_TOOLS`; either directory may hold either kind of entry, a `kind` member
telling them apart (`tool` when absent). Both are read by `sysml`, `sysml-grpc` and
`sysml-lsp` alike. An entry of kind `engine`:

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
  "authority": "bounded"
}
```

| Member | Meaning |
|--------|---------|
| `kind` | `engine`. `policy` and `sampler` entries are read and listed too — `unavailable: policy "priority" is not served in this build: scheduling policies are the strategies stage` — and never run; an unknown kind is a manifest fault |
| `name` | The engine's name, as `-engine` selects it: non-empty, no whitespace, not `auto` or `all`, not of the `tool:` form. Two entries of one name, in either directory, are a fault and neither registers |
| `version` | What the engine's own `describe` must repeat, and what the status column shows |
| `command` | The executable and its arguments. An absolute path is taken as written; a relative path — a bare name included — is joined to the manifest's directory, followed through every symbolic link, and refused unless the result is inside that directory. Nothing is looked up on `PATH`. `-engines` prints the resolved path |
| `module` | A WebAssembly module in place of `command`; parsed, listed as not served (the WebAssembly stage), never instantiated |
| `transport` | `stdio` (the default): a child process on a pair of pipes. `grpc` names an `address` in place of a `command`; the entry is listed as not served, since the transport is defined and deferred until a remote engine needs it |
| `protocol` | The version of the message set the engine speaks. This build serves `1`; an entry outside the served set is a fault naming the versions served |
| `answers` | The question kinds the engine covers: `evaluate`, `outcomes`, `holds`, `sensitive`, `satisfiable`, `sweep`, `compute`; non-empty, each at most once |
| `subjects` | The declaration kinds the engine answers about, spelled as the notation does — `action`, `state`, `calc`, `constraint`, `requirement`, `part`, … — matched against the kind of the declaration the question's subject resolves to, definitions and usages alike, so `["action"]` covers `Mission::race` whether it is an action definition or an action usage. Empty covers every subject |
| `model` | The forms of the model each `covers` and `run` carries: `sources` (always sent, listed or not), `graphs:1` ([below](#the-graphs1-model-form)); `rdf` is parsed and the entry listed as not served until the rdf stage |
| `bounds` | The budget bounds the engine honors, of `depth`, `steps`, `runs`, `memory` |
| `witness` | `schedule`, `assignment` or `none` (the default): the kind of witness the engine's existential claims carry. A result carrying the other kind, or an existential claim with no witness at all, is a protocol break |
| `authority` | The strongest strength the engine says it produces for a universal claim: `observed`, `witnessed`, `bounded` or `proved`. It orders the engine under `auto` and is printed by `-engines`; it never decides what a result is worth |
| `admit` | Refused by this build (`admit is not honored by this build: it needs a referee record, which the referee-record stage adds`). Every universal claim is composed as *not covered* unless it rests on executions the interpreter replays |
| `concurrent` | Default `true`: one process holds several `run` requests open at once under `-jobs`. `false`: one process per open request, the coordinator starting another for the next question while one is answering and ending them all with the plan. The plan's output is the same either way |

Unknown members are refused. The security rules the tool manifest implied are enforced for
every entry: a manifest directory under the workspace is not read (`is under the workspace …;
a manifest is read from a directory the environment names outside every workspace`); an entry
or directory writable by anyone but its owner is refused (`is writable by others (mode 0664); a
manifest entry and its directory may be written by their owner alone`); a `.sysml` file can
register nothing. A fault in any entry is reported at startup and registers nothing from that
directory, as a bad run bound is reported.

## Listing, probing, selecting

`-engines` and `%engines` read the manifests and print each entry with its kind, protocol
(`<transport>/<protocol>`), authority, question kinds and status, then one source line per
manifest entry — the file, the resolved command, and `not admitted` for an engine — spawning no
process. The status is what the file can tell: `ready (spin-bridge 1.4.0 at /opt/…)` when the
command resolves to an executable regular file, `unavailable: <why>` when it does not or the
entry is not served.

```console
$ OPENSYSML_ENGINES=/etc/opensysml/engines sysml -engines
engine       kind      protocol  authority    answers          status
check        built-in  -         bounded      outcomes, holds  ready
explore      built-in  -         proved       outcomes         ready
priority     policy    stdio/1   not covered                   unavailable: policy "priority" is not served in this build: scheduling policies are the strategies stage
run          built-in  -         observed     evaluate         ready
solve        built-in  -         proved       satisfiable      ready (z3 at /usr/bin/z3)
spin-bridge  engine    stdio/1   bounded      holds, outcomes  ready (spin-bridge 1.4.0 at /opt/spin-bridge/bin/spin-bridge)
sweep        built-in  -         observed     sweep            ready
priority 0.3: policy from /etc/opensysml/engines/priority.json, runs /etc/opensysml/engines/priority-policy
spin-bridge 1.4.0: engine from /etc/opensysml/engines/spin-bridge.json, runs /opt/spin-bridge/bin/spin-bridge, not admitted
```

`-engines -probe` and `%engines probe` are the one listing that spawns: each external entry is
started once, its `describe` checked against the manifest field by field, and the outcome is
the status — `ready (spin-bridge 1.4.0 at /opt/…; describe agrees)`, or `unavailable:` with the
process's exit or the field that disagrees. An operator checks an installation this way without
asking a question.

`-engine spin-bridge` puts every check of the invocation to that engine alone, its refusal or
*not covered* answer being the verdict, as for any named engine. `-engine auto` reaches an
external engine only after every built-in engine refused the question, whatever authority the
entry declares, since the engine is not admitted; `-engine all` consults it beside the
built-in engines that cover the question and composes the answers by the framework's rules
([Analysis engines](cli.md#analysis-engines)), an external `holds` beside an `explore` witness
being a disagreement resolved in the interpreter's favor. `Question.subject` must be a
declaration of a kind the entry's `subjects` names; otherwise the engine refuses before its
process is asked (`engine "spin-bridge" answers for ["action"], not the state Plant::Reactor`).

`sysml-grpc` lists external engines in `ListEngines` — `kind`, `protocol`, `source`, `command`,
`version` and `served` beside the fields every engine carries ([wire
contract](wire-contract.md#listengines-the-engine-field-and-the-standing-of-an-answer)) — and
does **not** run them until started with `-serve-external-engines <name>,…` or
`-serve-external-engines all`: a request naming one that is not served is the
`FAILED_PRECONDITION` status `engine 'spin-bridge' is not served by this service`, `auto` and
`all` pass over it, and the `engines_external` capability is advertised only when the flag names
at least one entry. The flag is a grant of command execution: every client the transport admits
may then run every named command as the service's user. A name the flag gives that no
manifest engine carries is refused at startup.

## The session

A **session** is one process for one plan — one `sysml` invocation's checks, one REPL question,
one service request. The coordinator starts it at the plan's first question to the engine and
ends it when the plan ends, so an engine that loads a formalism pays once per plan. Under
`-jobs n` several questions to one engine share the process, each an open `run` with its own
`id`, unless the entry says `"concurrent": false`.

Messages are JSON-RPC 2.0 objects, one per line, over the process's standard input and output.
Every message carries `"jsonrpc": "2.0"`; a request carries a numeric `id` its answer repeats; a
notification carries none and gets no answer. Standard error is captured up to
`OPENSYSML_TOOL_MAX_OUTPUT` and printed with a *not covered* reason, never parsed; an engine
that writes more than that to standard error is ended as a protocol break naming the bound.

| Message | From | Params | Answer |
|---------|------|--------|--------|
| `describe` (request) | host | `{"protocols": [1]}`, the versions the host serves | the engine's description: `name`, `version`, `protocol`, `answers`, and optionally `subjects`, `bounds`, `witness`, `model`, `authority`, `concurrent`. Each member present is checked against the manifest and the session is refused naming the first that disagrees (`engine "spin-bridge" describes its answers as ["holds", "outcomes"]; the manifest says ["holds"]`); `name`, `version`, `protocol` and `answers` are always compared, the rest when the engine states them |
| `covers` (request) | host | `{"question": …, "model": …}` | `{"covers": true}` or `{"covers": false, "reason": "…"}` |
| `run` (request) | host | `{"question": …, "model": …, "bounds": [{"name","limit"}], "budget": {"deadline", "depth", "steps", "runs", "memory", "jobs"}}` — the deadline an absolute RFC 3339 time, a zero count the engine's own default | the result ([below](#what-a-result-carries)); `progress` notifications may precede it |
| `cancel` (notification) | host | `{"id": 3}`, a running `run` | none; the `run` answers with what it has, marked by the bound it reached |
| `progress` (notification) | engine | `{"id": 3, "runs": 120, "depth": 17, "steps": 0, "text": "…"}`, any of the counts and text | none |

An answer is `{"jsonrpc":"2.0","id":3,"result":{…}}` or
`{"jsonrpc":"2.0","id":3,"error":{"code":"…","message":"…"}}`, `code` one of `unsupported` (a
construct met at run time and not refused in `covers`), `budget` (the engine stopped at a bound
of its own) and `internal`. The host prints coalesced progress on standard error — the latest
report per open `run`, at most every 250 ms, as `engine spin-bridge: runs 120, depth 17` — and
`-quiet` suppresses it; an engine that sends one per state costs the pipe, not the report. The
last report is kept in the reason of a run that ends without an answer.

Time is the host's: `describe` and `covers` must answer within `OPENSYSML_TOOL_TIMEOUT`; when a
`run`'s deadline passes the host sends `cancel`, waits that timeout as grace, then ends the
process and its children. The `memory` budget is passed to honor and not enforced — a site that
needs a hard limit puts one in `command` (`["/usr/bin/prlimit", "--as=…", "/opt/…"]`,
`systemd-run`, a container).

The JSON Schema of every message in both directions is published at
[`engine-protocol.schema.json`](engine-protocol.schema.json); the tests validate every message
the stand-in engine exchanges against it.

### What a run carries

The question, in the model's own names, and the model in the forms the entry declared:

- **`question`**: `kind`; `subject`, the qualified name as the surface spelled it, and
  `subjectKind`, its declaration kind; `schedule` as `-schedule` spells it; `modelSeed`, the
  seed the runs' modeled draws come from when one is set apart from the schedule (`-seed`,
  `%seed`), absent otherwise; `draws`, the policy the runs' RandomFunctions draws resolve
  under (`-draws`, `%draws`) — `min`, `max` or `average` — absent when they draw at random;
  `free`, what the
  question leaves open (`schedule`, `inputs`); `condition` (`name`, `text`) for `holds`;
  `conditions` for `satisfiable`, one set per query with its `features`, `assertions` and
  `pinned` values; `bindings` as `{name, value, unit}`; `inputs` as `{name, type, unit,
  domain}`; `sweep` with its `ranges` (and `sampled`, `samples`, `seed` for a sampled
  sweep; `runs` and `seed` for a Monte Carlo, which states no range and seeds each run's
  modeled draws from `seed` and the run's number, so it carries no `modelSeed`). A `seed`
  is present, zero included, whenever rows are drawn from it.
- **`model.sources`**, always: `library`, the version of the standard library the host embeds,
  and `documents`, every document of the model as `{path, text}` in path order.
- **`model.graphs`**: the `graphs:1` form, when the entry names it.

### The `graphs:1` model form

`graphs:1` is the lowered `ActionGraph`/`StateGraph` IR of the subject and of every behavior it
performs, as canonical JSON — keys in a fixed order, no insignificant whitespace, elements in
the order the lowering fixes — so the same model exports byte for byte the same form on every
run and under every `-jobs` count. It is what the runtime executes and nothing less: what the
graph carries, the form carries.

```json
{"version": 1, "subject": "Mission::race",
 "actions": [{"name": "Mission::race", "kind": "action",
   "parameters": [], "attributes": [{"name": "x", "types": ["Integer"], "value": {"text": "0", "span": {…}}}],
   "nodes": [{"id": 0, "kind": "start", "span": {…}}, {"id": 1, "kind": "fork", "name": "split", "span": {…}},
             {"id": 2, "kind": "action", "name": "left", "body": [{"kind": "assign", "name": "x", "value": {"text": "1", …}}],
              "footprint": {"writes": [{"symbol": "Mission::race::x", "name": "x"}]}}, …],
   "initial": 0, "finals": [6],
   "edges": [{"source": 0, "target": 1, "decl": {…}}, {"source": 1, "target": 2, "decl": {…}}, …]}]}
```

- `version` is `1`; an engine names the versions it reads in `model` and refuses one it does not
  in `covers`. The host serves the current version and the one before it; dropping a version is
  a minor release under the versioning rule in `CONTRIBUTING.md`.
- `actions[]`, one `ActionForm` per lowered action graph — the subject's and every performed
  behavior's, in the order lowering reached them: `name`, `kind`, `scope`, `parameters` (`name`,
  `direction`, `result`, `optional`, `types`, `unit`, `value`, `span`), `attributes`, `nodes`,
  `initial`, `finals`, `edges`, `flows`, `bindings`, `connections`, or `error` when the lowering
  refused it. A `NodeForm` is `id` (its index), `kind`, `name`, `span`, `features`, `body`
  (lowered statements, each with its `kind`, expressions as `{text, span}`), `block`, `accept`
  (`param`, `signalType`, `viaPort`, `viaSelf` for a port path written from `this`, `trigger`), `subflow` (the nested graph or its refusal),
  `performs`, and `footprint` — the places the move reads and writes, the channels it sends on
  and accepts from, the control nodes it joins, `dynamic` when the lowering could not project it
  — present on every node the lowering computed one for. An `EdgeForm` is `source`, `target`,
  `guard` as `{text, span}`, `else` for the branch taken when no guard holds, `probability` as
  `{text, span}` for the weight a `Stochastic::Probability` annotation puts on a succession
  leaving a decision, and `decl`, the span of the succession that declares it.
- `states[]`, one `StateForm` per lowered state machine: `vertices` (the machine, its states
  and pseudostates with `kind`, `parent`, `region`, `regions`, `entry`, `do`, `exit`,
  `deferred`), `regions`, `transitions` (`source`, `target`, `trigger`, `guard`, `effect`,
  `via`), `entryTransitions`, `connections`, `machine`, `initial`, `attributes`.

Every element the lowering names carries its `span` (`document`, `offset`, `len`) into the
model's text, so an engine's answer can name what the model says. A subject that is neither an
action nor a state machine, or one the model does not declare once, is refused with
`ErrGraphsSubject` before the process is asked. The form is written by `modelform.GraphsOf` and
`modelform.MarshalGraphs` in the execution layer beside the analysis framework.

### What a result carries

```json
{"claim": "violated", "strength": "witnessed",
 "bounds": [{"name": "depth", "limit": 40, "reached": false}],
 "witness": {"schedules": ["step 3: 2@left first of 2@left, 3@right; step 4: 3@right first of 3@right"], "at": 4},
 "reason": "x is 2 at the join", "values": [{"name": "x", "value": 2}], "elapsed": 120}
```

`claim` is `holds`, `violated`, `sensitive`, `satisfiable`, `value`, `table` or `none`;
`strength` is what the engine *claims*, one of `observed`, `witnessed`, `bounded`, `proved` and
`not covered`, and must be a pair the framework admits with the claim. `bounds` are the bounds
taken, each with whether it was `reached`. `witness`, `executions`, `reason`, `values` (`{name,
value, unit}`, a unit read in the subject's library), `inputs` — the engine's account of the
initial state, `{name, type, sort, domain, free, optional, value}` per feature, as the `smt`
engine reports what it ranged over or pinned — `assumptions`, the constraints it assumed by
name, and `elapsed` (milliseconds) are optional.

A **witness** takes the shape the entry's `witness` and the claim fix. A `schedule` witness
carries `schedules`, each a replay text in the format `-check-witness` writes — `step N:
<token>@<node> first of <the enabled moves>` lines, one per line or joined by `; ` — and for a
`violated` claim `at`, the move the violation is at (absent names the schedule's end); a
`sensitive` witness carries two schedules and the `feature` they diverge on. An `assignment`
witness, the shape `satisfiable` takes, is `inputs` alone: `{name, value, unit}` per free
feature. `executions`, optional on a universal claim, is a list of one-schedule witnesses: the
concrete executions the engine ran and found the claim holding on. A schedule witness may also
carry `inputs`, `{name, value}` per free feature of the question — those the `question` listed
under `inputs`: the features the action leaves unbound and those `-check-input` released — with
the value as JSON or as SysML notation (`"2 * 4"`, `"Mode::Fast"`); the replay fixes them on the
action as it starts, before its defaults and ahead of the first move, as an `smt` witness's are,
and the result lists them. An input the question does not leave free, one given twice or one
the run cannot read is *not covered* naming it. A schedule carries choices, never the run's
RandomFunctions draws: under a `draws` policy the question names, the host's replay resolves
each call to the policy's point — `uniform(1, 10)` is `10` under `max` — as the engine's run
did, and the witness the result reports records those draws under that policy; a drawing run
under no policy has no replay, and its schedule is *not covered* (`its witness does not replay
(… the witness records no draw left for it)`), so a question about a behavior that draws is
put to an engine under a fixed policy.

## The standing of an answer

The host decides what an answer is worth; `strength` alone decides nothing.

| The engine answers | The host does | Standing |
|--------------------|---------------|----------|
| `violated` with one schedule | replays the schedule through the `replay:` policy to the move `at` names and evaluates the condition there | *witnessed* `at move 4 (engine "spin-bridge", replayed): \`maxPressure\` evaluates false there` — a later move that repairs it does not matter; *not covered* `engine "spin-bridge" reports a violation, witnessed; its schedule replays and maxPressure holds at move 4` when it holds, `…; its witness does not replay (move 3: 2@vent is not enabled)` when the schedule does not follow |
| `sensitive` with two schedules and a `feature` | replays both and compares the feature's final values | *witnessed* `x ends as 1 or 2 (engine "spin-bridge", both replayed)` when they differ; *not covered* `…; both schedules replay and \`x\` is 1 under each` when they do not |
| `satisfiable` with an assignment | binds the values to the free features and evaluates every assertion of the query with the evaluator | *witnessed* `assignment of engine "z3-bridge", confirmed by the evaluator` when every assertion is `true`; *not covered* `…; at its assignment x > 3 evaluates false` when one is not or cannot be evaluated |
| `holds`, `value` or `table` with `executions` | replays each execution and evaluates the claim on it | *observed* `observed on 200 executions chosen by engine "sim-bridge", each replayed`; *not covered* `…; execution 3: replays, and after 12 moves \`maxPressure\` evaluates false there` naming the first execution that fails replay or on which the claim fails — the rest are not counted |
| `holds` with no `executions` | nothing to replay | *not covered* `engine "spin-bridge" reports holds, bounded at depth=40 (reached); no executions to replay, and no referee record admits the engine: admission comes with the referee-record stage` — the claim kept in the reason, nothing claimed, `auto` advances |
| `none` | — | *not covered* with the engine's `reason` |

Under `-engine all` the framework composes the external result with the built-in engines' as it
composes any: a witnessed violation stands over every universal claim, an external `holds`
beside an `explore` witness is a disagreement resolved for the interpreter and demoted to *not
covered* with the disagreement as its reason, and nothing is promoted past what it earned.

## Failures

Every failure of an external engine is *not covered* — a reason particular to that engine,
which `auto` advances past and `-engine <name>` prints as the verdict — never an error that
stops the plan:

| Failure | Result |
|---------|--------|
| the process does not start, exits before `describe`, or its `describe` disagrees with the manifest | the engine refuses the question: `engine "spin-bridge" at /opt/… did not start (exit status 127: <its standard error>)`, `engine "spin-bridge" did not answer describe within 10s (OPENSYSML_TOOL_TIMEOUT)`, or the field named; `-engines -probe` reports the same; plain `-engines` shows what the file can tell |
| `covers: false` | *not covered* with the engine's reason |
| a line that is not JSON, a missing `jsonrpc` or `id`, an `id` no request carries, a member of the wrong type, an `error` whose `code` is not `unsupported`, `budget` or `internal`, a line or the whole of standard error over `OPENSYSML_TOOL_MAX_OUTPUT`, a result whose `claim` and `strength` are not a pair the framework admits, a witness of the wrong kind or count for the question (`a sensitive result's witness is two schedules, not 1`) | *not covered* `engine "spin-bridge" broke protocol: …`; the session ends |
| the process exits during a `run` | *not covered* `engine "spin-bridge" at /opt/… exited during a request (exit status 137: <its standard error>)`, with the last `progress` |
| the deadline passes and `cancel` is not answered within `OPENSYSML_TOOL_TIMEOUT` | *not covered* `engine "spin-bridge" did not answer cancel within 10s (OPENSYSML_TOOL_TIMEOUT); the process was ended`, with the last `progress` |
| an `error` answer | *not covered* with its `code` and `message` |
| a witness that fails replay or evaluation | *not covered* naming the move, the condition or the value ([above](#the-standing-of-an-answer)) |
| a schedule witness for a question that names no action to replay it on | *not covered* `engine "spin-bridge" gives a schedule, and a evaluate question names no action to replay it on` |
| a schedule witness whose `inputs` name a feature the question does not leave free, name one twice, or give a value the run cannot read | *not covered* `engine "spin-bridge" gives its witness the input limit, which the question does not leave free`; `engine "spin-bridge" gives limit twice`; `its witness does not replay (… witness input refused: n: "nothing" does not evaluate …)` |
| a universal claim with no `executions` | *not covered* with the claim kept |
| an `executions` entry that fails | *not covered* naming it; the rest are not counted |

A built-in engine that returns a malformed result has broken this repository's contract and
that is an error; an external engine that does the same is *not covered*, because the plan's
other engines can still answer.

## Bounds

| Variable | Default | Bounds |
|----------|---------|--------|
| `OPENSYSML_TOOL_MAX_OUTPUT` | `64M` | One protocol line on standard output, and the whole of standard error, for an external engine; a tool's one reply and its standard error alike. Bytes, or bytes with a `K`, `M` or `G` suffix. A line over it is a protocol break naming the size; a stream over it ends the process |
| `OPENSYSML_TOOL_TIMEOUT` | `10s` | One `describe` or `covers` round trip; the grace a `run` has to answer `cancel` after the plan's deadline; a tool's one process |
| the plan's deadline | the check's | When a `run` is sent `cancel` |

Nothing an engine writes is interpreted beyond the JSON it is parsed as; a witness is a text the
interpreter replays, never a program it runs.

## What this build does not serve

Listed with a typed reason, never run: `policy` and `sampler` entries (the strategies stage);
`module` entries (the WebAssembly stage); the `grpc` transport (defined, deferred until a
remote engine needs it); the `rdf` model form (the rdf stage); `admit` (the referee-record
stage, which also brings `-referee`). An in-process Go engine over `client/opensysml` is the
in-process stage.
