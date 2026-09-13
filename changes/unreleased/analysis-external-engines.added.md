- **External analysis engines over standard input.** A directory named by `OPENSYSML_ENGINES`
  holds one JSON file per engine — `kind: engine`, its `name`, `version`, `command`, the
  question kinds it `answers`, the declaration kinds it takes as `subjects`, the `model` forms
  it reads, its `bounds`, the `witness` kind it gives, its `authority` and whether it is
  `concurrent` — and each registers under its name beside `run`, `explore`, `check`, `sweep`
  and `solve`. A plan that reaches the engine starts its command once, hands it the model as
  `sources` (the documents) or `graphs:1` (a new versioned export of the lowered action and
  state graphs — guards, triggers, effects, pseudostate edges, footprints — byte-stable across
  runs and `-jobs` counts), and speaks JSON-RPC lines to it: `describe`, checked against the
  manifest field by field; `covers`; `run`; `cancel`; and `progress` notifications the CLI and
  REPL print to standard error, coalesced to a few a second. The message set is published as
  `docs/reference/engine-protocol.schema.json`. Nothing an engine claims is trusted: a
  `violated` stands as *witnessed* only when its schedule replays under `replay:` and the
  condition is false at the move it names, `sensitive` needs two replaying schedules that end
  the named feature differently, `satisfiable` needs an assignment the evaluator confirms, a
  universal `holds` is *observed* over the `executions` that replay and otherwise *not covered*
  with the claim kept in the reason; `admit` is refused until referee records exist. Every
  failure — a program that does not start, a `describe` that disagrees with the manifest, a
  broken protocol line, an error the engine reports, an exit mid-run, a cancel unanswered by the
  deadline — is a typed *not covered* answer naming it, with the engine's standard error, and
  `auto` advances past it. `sysml -engines` and `%engines` list external engines with their
  kind, protocol and status without starting them; `-engines -probe` and `%engines probe` start
  each once to check its `describe`; `-engine <name>` and `-engine all` reach them like any
  engine. `ListEngines` gains `kind`, `protocol`, `source`, `command`, `version` and `served`,
  and `sysml-grpc` lists external engines but refuses to run them (`failed_precondition`) until
  started with `-serve-external-engines <names|all>`, which advertises `engines_external`.
  `policy`, `sampler` and `module` entries, the `grpc` transport and the `rdf` model form are
  parsed and listed `unavailable` with a reason naming the stage that serves them.
- **`OPENSYSML_TOOL_MAX_OUTPUT`** (default `64M`) bounds what one external process may write
  before it is cut off — a tool's one reply and its standard error, an external engine's one
  protocol line and its standard error — and a tool's relative `executable` path is confined to
  its manifest directory as an engine's `command` is.
