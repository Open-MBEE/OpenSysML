# MCP server (`sysml-mcp`)

`sysml-mcp` serves OpenSysML to coding agents over the
[Model Context Protocol](https://modelcontextprotocol.io): the agent's host (OpenCode, Claude
Code, Cursor, Codex, VS Code, Gemini CLI, Devin, ...) starts the binary in the project directory
and calls what it offers as tools — the language server's diagnostics, navigation, editing and
rendering over the documents under the workspace root; the runtime's one-shot parsing,
evaluation, instantiation, execution, verification, querying and conversion; and named REPL
sessions whose declarations, instances and runs persist across calls. The protocol is spoken over
stdin and stdout, so the host starts the server itself; nothing is typed at it, and it has no
port — unless `-http` names one, when a single long-lived server serves every agent that connects
over the Streamable HTTP transport ([Transports](#transports)).

The documents under the root (`-root`, the current directory by default, which a host sets to
the project) are managed as one workspace, so a declaration in one resolves against the others,
and the standard library is the copy embedded in the binary unless `OPENSYSML_LIBRARY_PATH`
names another — the same workspace [`sysml-lsp`](lsp.md) serves an editor.

## Installing

The quickest way is npm: `npx -y @openmbee/sysml-mcp` installs the server on first use, with
nothing preinstalled but Node 20.11 or later, which is how the agent hosts' own documentation
configures a server. [`@openmbee/sysml-mcp`](https://www.npmjs.com/package/@openmbee/sysml-mcp)
has no dependencies; its `sysml-mcp` command picks the platform package
`@openmbee/sysml-mcp-<os>-<cpu>` (linux/darwin × x64/arm64, win32 x64) for the machine, at
exactly its own version, and runs the release binary that package carries — the same bytes as
the release archive, verified against the release's `.sha256` sidecars when the package was
built — with the arguments and stdio it was given. Its version is the OpenSysML release:
`npx -y @openmbee/sysml-mcp@0.10.1` runs the `v0.10.1` server.

```bash
claude mcp add --transport stdio sysml -- npx -y @openmbee/sysml-mcp -root .
codex mcp add sysml -- npx -y @openmbee/sysml-mcp -root .
```

Every release also publishes `sysml-mcp-<os>-<arch>.tar.gz` (`.zip` on Windows) for linux/amd64,
linux/arm64, darwin/amd64, darwin/arm64 and windows/amd64: the binary under its plain name, the
`LICENSE` and, on Unix, its manual page under `share/man/man1/`. It is its own archive, not part
of the `opensysml-<os>-<arch>` bundle, so:

```bash
# the install script, which verifies the archive against the release's SHA256SUMS.txt
curl -fsSL https://opensysml.org/install.sh | sh -s -- --tools sysml,sysml-lsp,sysml-mcp
# or by hand
tar xzf sysml-mcp-linux-amd64.tar.gz && sudo install sysml-mcp /usr/local/bin/
# or from source
go install github.com/Open-MBEE/OpenSysML/cmd/sysml-mcp@latest
```

The install scripts' default tool list stays `sysml,sysml-lsp`, so a pinned older release
still installs; `sysml-mcp` is installed when it is named (`--tools sysml,sysml-lsp,sysml-mcp`)
or with `--tools all`, which installs every tool the chosen release published.
`irm https://opensysml.org/install.ps1 | iex` with `-Tools sysml,sysml-lsp,sysml-mcp` does the
same on Windows, the Homebrew formula and the Scoop manifest install it beside `sysml` and
`sysml-lsp` by default, and the Windows MSI has it as the *MCP server* feature. A checkout builds it with
`make build-mcp` (into `bin/sysml-mcp`) or as part of `make build`. `sysml-mcp -version` reports
the build.

## Finding it in your host

Each release is also listed in the official [MCP Registry](https://registry.modelcontextprotocol.io)
as `org.opensysml/sysml-mcp`, pointing at the npm package `@openmbee/sysml-mcp`, so a host that
browses the registry offers it without any of the above: the GitHub MCP Registry lists it, VS
Code finds it under *MCP: Browse MCP Servers* (or the Extensions view's MCP Servers section),
and third-party directories that mirror the registry carry the same entry. Installing from there
configures `npx -y @openmbee/sysml-mcp`, which downloads the release binary for the platform
and runs it; the `-root` argument the listing declares is the workspace folder, the current
directory by default.

## Configuring a host

Each host reads a configuration file of its own, and `-print-config <host>` writes the exact
block for it to stdout:

```bash
sysml-mcp -print-config                     # list the hosts
sysml-mcp -print-config opencode            # the block for one
sysml-mcp -print-config codex >> ~/.codex/config.toml
```

The block names the server `sysml`, so its tools appear to the agent under that prefix
(`sysml/diagnostics` or `mcp__sysml__diagnostics`, as the host spells it), and names the binary
by its resolved absolute path (`os.Executable`, symlinks followed), so the host finds it wherever
it was installed and however it starts the process. To name a command instead — `sysml-mcp` when
the binary is on `PATH`, or a wrapper — give `-print-config-command`; to have the host run the
server from npm instead, `npx -y @openmbee/sysml-mcp`, give `-print-config-npx`, which writes
`npx` as the command and `-y @openmbee/sysml-mcp` as the first arguments:

```bash
sysml-mcp -print-config cursor -print-config-command sysml-mcp
sysml-mcp -print-config vscode -print-config-npx
```

The blocks below were written with `-print-config-command /usr/local/bin/sysml-mcp`; with
`-print-config-npx` the command is `"npx"` and the `args` array begins
`"-y", "@openmbee/sysml-mcp"` (OpenCode's `command` array becomes
`["npx", "-y", "@openmbee/sysml-mcp"]`, and Codex's `args = ["-y", "@openmbee/sysml-mcp"]`). A
host that should serve fewer tools or another folder gets them as arguments: add
`"-tools", "lsp,runtime"` or `"-root", "models"` to the `args` array (the `command` array in
OpenCode's case). Hosts start the server with the project as its working directory, which is
what `-root` defaults to.

### OpenCode — `opencode.json`

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "sysml": {
      "type": "local",
      "command": [
        "/usr/local/bin/sysml-mcp"
      ],
      "enabled": true
    }
  }
}
```

OpenCode reads `opencode.json` in the project and `~/.config/opencode/opencode.json` for every
project; `"type": "local"` is a server OpenCode starts itself. The `mcp` key sits beside the
`lsp` key that declares `sysml-lsp` ([editors and agents](../guide/08-editors.md#opencode)), and
this repository's own [`opencode.json`](../../opencode.json) declares both, with `sysml-mcp`
expected on `PATH`.

### Claude Code — `.mcp.json`

```json
{
  "mcpServers": {
    "sysml": {
      "type": "stdio",
      "command": "/usr/local/bin/sysml-mcp",
      "args": []
    }
  }
}
```

`.mcp.json` in the project root is shared with the project (Claude Code asks before using it);
`claude mcp add --scope user sysml -- /usr/local/bin/sysml-mcp` writes the same declaration
into the user's settings instead.

### Cursor — `.cursor/mcp.json`

```json
{
  "mcpServers": {
    "sysml": {
      "command": "/usr/local/bin/sysml-mcp",
      "args": []
    }
  }
}
```

Project-wide in `.cursor/mcp.json`, or for every project in `~/.cursor/mcp.json`.

### Codex — `~/.codex/config.toml`

```toml
[mcp_servers.sysml]
command = "/usr/local/bin/sysml-mcp"
args = []
```

A table of Codex's `config.toml`; `>>` appends it. The Codex CLI and the Codex IDE extension read
the same file.

### VS Code — `.vscode/mcp.json`

```json
{
  "servers": {
    "sysml": {
      "type": "stdio",
      "command": "/usr/local/bin/sysml-mcp",
      "args": []
    }
  }
}
```

GitHub Copilot's agent mode reads `.vscode/mcp.json` in the workspace, or the `mcp` section of the
user settings for every workspace (`MCP: Add Server` in the command palette writes either).

### Gemini CLI — `.gemini/settings.json`

```json
{
  "mcpServers": {
    "sysml": {
      "command": "/usr/local/bin/sysml-mcp",
      "args": []
    }
  }
}
```

`.gemini/settings.json` in the project or `~/.gemini/settings.json` for the user; the
`mcpServers` key is merged into the file's other settings.

### Devin

```json
{
  "mcpServers": {
    "sysml": {
      "command": "/usr/local/bin/sysml-mcp",
      "args": []
    }
  }
}
```

Devin runs custom MCP servers from its organization settings (*Settings → MCP Marketplace → Add
custom*): paste the `mcpServers` block, with the command resolving on the machine Devin runs on
— a path under the repository checkout after `make build-mcp`, or `sysml-mcp` once the
environment's setup installs it on `PATH`.

## The tools

The server offers three groups, chosen with `-tools lsp,runtime,session` (every group by
default): `lsp` is the language server's workspace over the documents under the root, `runtime`
the one-shot service also served by [`sysml-grpc`](service-transports.md) — one tool per RPC,
with the RPC's request as the arguments and its response as the structured result (the
[conformance suite](https://github.com/Open-MBEE/OpenSysML/tree/main/conformance) runs through these tools as protocol `mcp`,
so they answer the scenarios the service does) — and
`session` persistent REPL sessions, one tool per `%` command. Serving fewer groups keeps the
catalogue small for an agent that needs only one of them. The catalogue below is the server's
own `tools/list`, with each tool's description cut to its first sentence; the tool's full
description, read by the agent, says what every argument does.

No two groups share a tool name: the `lsp` group's `workspace_diagnostics`, `render_view` and
`render_document` work on a document of the workspace, the `runtime` group's `diagnostics`,
`render_model_view` and `render_model_document` on a model given as `paths`, inline `sources` or
a `modelHash`. Every server also serves `tool_catalog`, which
answers this catalogue — each tool's group, the `sysml-grpc` RPC or `sysml` flag it stands for
and a one-line summary — narrowed to the groups served, so an agent can discover the surface
without this page.

### `lsp`

| Tool | Does | Writes |
|---|---|---|
| `apply_model_edit` | Edit a model by operations on its elements rather than by text: setValue, rename, addMember, addConnection, addSequence (action-body items), delete, move, and the diagram-layout annotations setLayout, setRoute, setCanvas, setStyle. | yes |
| `close_document` | Release a document held open by open_document: the server goes back to the file on disk (or forgets the document when there is no file). | yes |
| `code_actions` | List the quick fixes and refactorings available for a range of a document, each with the edits it would make. | no |
| `completion` | Propose completions at a position: keywords, names in scope and members of the type being accessed. | no |
| `definition` | Find where the name at a position is declared. | no |
| `document_symbols` | Outline a document: the elements it declares, nested as they are owned, with their kinds and ranges. | no |
| `documents` | List the document definitions (doc def) declared across the workspace, which render_document renders to Markdown. | no |
| `format` | Format a SysML v2 / KerML document, or only the lines of a range, in the canonical style. | yes |
| `hover` | Explain the element at a position: its kind, qualified name, type, value and documentation, as Markdown. | no |
| `inlay_hints` | List the inferred types and values the editor would show inline for the features of a document or range. | no |
| `open_document` | Hold a document open with the given text, as an editor buffer the server analyzes instead of the file on disk, so text can be checked (diagnostics, hover, render) before it is written. | yes |
| `references` | Find every place in the workspace that refers to the element at a position. | no |
| `rename` | Rename the element declared or referenced at a position, everywhere in the workspace. | yes |
| `render_document` | Render a document definition (doc def) to Markdown, its queries evaluated over the workspace and its diagrams embedded. | no |
| `render_view` | Render a diagram or table of a document: a declared view or a pseudo-view, as Mermaid, Graphviz dot, PlantUML, D2, text, Markdown, CSV or TSV. | no |
| `signature_help` | Describe the invocation enclosing a position: the callable's parameters and which one the position is at. | no |
| `update_document` | Replace the text of a document held open by open_document and re-analyze it. | yes |
| `views` | List the views a document declares and the pseudo-views (#tree, #interconnection, ...) render_view can draw of it. | no |
| `workspace_diagnostics` | Analyze a SysML v2 / KerML document, or the whole workspace, and list the errors, warnings and hints: syntax, unresolved names, type and constraint violations. | no |
| `workspace_info` | Describe the SysML v2 workspace the server has indexed: its root folder and the model documents found under it. | no |
| `workspace_symbols` | Search the workspace's declared elements by name substring. | no |

### `runtime`

| Tool | Does | Writes |
|---|---|---|
| `apply_edits` | Apply source-preserving edits to a parsed model and return the edited notation (the ApplyEdits RPC): set a value, rename with every reference, add a member, import, documentation, comment, connection, transition, satisfy or metadata, delete, move. | no |
| `check_behavior` | Search every schedule of an action or state machine (the CheckBehavior RPC, the REPL's %check-property/%check-diverge settings about %action/%state): a property false at a stable state, a deadlock, a failure or a feature whose final value the schedule decides is witnessed and replayed. | no |
| `compare_results` | Run every configuration a SysML v1 tool's results sidecar indexes against the migrated model (the CompareResults RPC, the CLI's -compare-results with -action, -runs, -seed, -draws, -clock-step and -observe): each on its target, for its recorded run count under its duration mode, or the runs and draws given, and table the tool's and OpenSysML's min, mean, p50, p90 and max of each observable with their relative difference. | no |
| `convert` | Convert a SysML v2 model between the representations OpenSysML writes — textual notation, RDF Turtle, the API's JSON element form — (the Convert RPC, the CLI's -convert). | no |
| `diagnostics` | Report every diagnostic of a parsed model (the GetDiagnostics RPC), as `sysml -validate` does: each with its severity, code, message and position. | no |
| `evaluate` | Evaluate a SysML expression against a parsed model (the Evaluate RPC, the REPL's %eval): arithmetic, feature reads, quantities with units, collection operators. | no |
| `evaluate_calc` | Invoke a calculation (the EvaluateCalc RPC, the REPL's %calc): a calc definition with positional arguments answers its `result`; a calc usage named with no arguments binds its inputs from its own members and answers every output feature in `outputs`. | no |
| `execute_action` | Run an action definition or usage to completion and report its output parameters (the ExecuteAction RPC, the REPL's %run). | no |
| `execute_state` | Run a state machine over a sequence of events and report the states visited, the final context and the simulation clock (the ExecuteState RPC, the REPL's %run on a state). | no |
| `explain_diagnostic` | Explain a diagnostic code (as the CLI's -explain does): what it means, how to resolve it and the specification clause or validation constraint behind it. | no |
| `export_graphs` | Export the lowered graph of an action or state machine — the subject's and every behavior it performs — as the canonical graphs:<version> JSON external analysis engines read (the ExportGraphs RPC). | no |
| `import_data` | Set the feature values a CSV, TSV, JSON or JSON Lines file assigns, one row per element (the ImportData RPC, the CLI's -import with -import-as, -import-map, -import-format and -import-dry-run): a feature the element declares has its value set, one it inherits is redefined in the element's body with the value, as one edit per file; a value the model refuses leaves every row unapplied and answers `error`. | no |
| `instantiate` | Build an object of a part definition or usage and report its feature values (the Instantiate RPC, the REPL's %instantiate): every attribute's value or default, nested parts as instance ids, with every reachable instance in `instances`. | yes |
| `list_engines` | List the analysis engines this build registers (the ListEngines RPC, as the CLI's -engines does): each with the questions it answers (evaluate, outcomes, holds, sensitive, satisfiable, sweep, compute), the strongest evidence it can produce, the bounds it takes, and whether it can run here. | no |
| `migrate` | Migrate a SysML v1 model — UML XMI with the SysML profile, an Eclipse UML2 .uml file or a Cameo/MagicDraw .mdzip archive — to SysML v2 in one of the representations convert writes (the Migrate RPC, the CLI's -migrate). | no |
| `parse` | Parse SysML v2 / KerML documents as one model and return its modelHash, the root namespace of each document and every diagnostic (the ParseSources RPC). | no |
| `query` | Select elements of a parsed model with a SysML v2 API & Services Query — scope, select, where — or OSLC query text (the Query RPC). | no |
| `record_run` | Run an analysis case — once, over the rows of a sweep (`ranges`), or as a Monte Carlo sample (`runs`) — and record the runs into the model as AnalysisRecords declarations (the RecordRun RPC, the CLI's -record-run/-record-into, the REPL's %record). | no |
| `render_model_document` | Render a named document of a model given by modelHash, paths or sources (the workspace group's render_document renders by name over the root) — a part def specializing DocumentQueries::Document — to Markdown or HTML (the RenderDocument RPC, the CLI's -render-document), byte for byte what the CLI writes. | no |
| `render_model_view` | Render a declared view or a targeted pseudo-view of a model given by modelHash, paths or sources as diagram data (the RenderView RPC; the workspace group's render_view renders by name over the root): nodes flattened parents first, edges with their ports, table columns and rows, canvas, notes and notices. | no |
| `run_analysis` | Run an analysis case (the RunAnalysis RPC, the REPL's %analysis): its subject and inputs bound from the arguments and its own declarations, its body run, every declared output answered with the verdict of its objective and of each assertion in its body; a verification case also answers its own verdict. | no |
| `run_document_query` | Run a named document query with parameter bindings and answer its typed rows (the RunDocumentQuery RPC, the REPL's %run-query). | no |
| `run_runs` | Run an action, or a Simulation::MonteCarlo analysis case, a number of times (the RunRuns RPC, the CLI's -runs/-observe, the REPL's %runs): every run is an ordinary run in a context of its own, drawing its modeled randomness from a seed derived from `seed`. | no |
| `run_sweep` | Run an analysis case or calc once per row of a parameter sweep (the RunSweep RPC, the CLI's -sweep): each row an ordinary run with the swept parameters bound to that row's values, stepped through the ranges' cartesian product or drawn with samples and seed. | no |
| `self_check` | Apply the SysMLValidation constraints, and those of any rule packages named, reflectively to every element of a model (the SelfCheck RPC, the CLI's -self-check and -self-check-package): each element is handed to every constraint whose input its metaclass conforms to. | no |
| `server_info` | Report the OpenSysML runtime's version and the named capabilities this build supports. | no |
| `symbol` | Describe one element of a parsed model by qualified name (the GetSymbol RPC): its kind, metadata, children, attributes with their declared values, static type facts, multiplicity and specializations. | no |
| `validate_instance` | Validate an object of a part as a whole (the ValidateInstance RPC, the REPL's %validate): every assertion about an object of symbolId and about the objects it holds — asserted constraints of their types, the requirements they carry, the satisfactions they are the subject of — each answered on the concrete object, with a `summary` verdict about the object as a whole. | no |
| `verify_constraint` | Ask whether a constraint holds (the VerifyConstraint RPC, the REPL's %constraint): against its declared values, or against an object of subjectSymbolId built for the call. | no |
| `verify_requirement` | Ask whether a requirement is satisfied (the VerifyRequirement RPC, the REPL's %requirement): its require constraints evaluated against declared values or against an object of subjectSymbolId. | no |
| `verify_satisfaction` | Evaluate the satisfy assertions a model states (the VerifySatisfaction RPC, the REPL's %satisfy): each against an object of its subject built for the call, answering one Verdict per assertion in declaration order. | no |

### `session`

| Tool | Does | Writes |
|---|---|---|
| `advance` | Advance the debuggers' simulation clock (%advance), running every state event, action token, change-condition poll and do behavior that comes due, in due order, and report what ran and where the debuggers stand. | yes |
| `check` | Ask an SMT solver whether a constraint, requirement or satisfaction can be satisfied at all (%check, experimental), with a satisfying assignment when it can. | no |
| `configure` | Ask an SMT solver which variants an element's conditions permit (%configure, experimental): a consistent selection, the check of the selections given, or every consistent selection up to a limit. | no |
| `debug_breakpoints` | Set breakpoints at nodes of the debugged action (%break), optionally clearing the earlier ones, and report the breakpoints it then has; with nothing to set, lists them. | yes |
| `debug_continue` | Run the debugged action to completion, or to the first breakpoint a token reaches (%continue), and report its results. | yes |
| `debug_current` | Report where the debugging session stands without changing it: the action's execution state, tokens, breakpoints and feature values (%tokens), or the machine's current state, clock, event queue, signals in flight and state data (%current, %events). | no |
| `debug_start` | Start a debugging session on an action (%action) or a state machine (%state), performed by an object when one is named. | yes |
| `debug_step` | Advance the debugged action one token step, or the debugged state machine one step (%step): one event dispatched, one change event, one round of do actions. | yes |
| `debug_stop` | End the debugging session (%stop), letting go of its run; objects the session holds stay. | yes |
| `explain` | Ask an SMT solver which conditions of an unsatisfiable element conflict (%explain, experimental): a minimal set of conditions, each with the element declaring it and its location. | no |
| `features` | Show an object's feature values and what its behaviors are doing (%features): the values its attributes hold, the objects its parts and ports hold, its connectors, and the state of each machine it exhibits and action it performs. | no |
| `instances` | List the objects a session holds (%instances): each with its id, type and lifetime, the ones a later instantiation displaced included. | no |
| `optimize` | Ask an SMT solver for the best values an analysis case's objectives admit (%optimize, experimental), each objective improved as its trade-study definition says, within the case's conditions. | no |
| `render_run` | Render the run a session recorded as a state timeline or a message sequence (%render-run), in a text or diagram form. | no |
| `repl_command` | Run one sysml prompt command (a % line) in a session and return what it printed. | yes |
| `runs` | Run an action count times, each run's modeled randomness seeded on its own, and return the table of the observables with each one's distribution (%runs); or run a Simulation::MonteCarlo analysis case count times on fresh subjects and conclude it. | yes |
| `send` | Send a signal to an object through the runtime's message bus (%send), for the state machine it exhibits or the action parked at a matching accept to take; by default to the object whose behavior is being debugged. | yes |
| `session_clear` | Reset a session, as %clear does: every declaration, object and debugging session goes; the session stays open and empty. | yes |
| `session_close` | Close a REPL session, discarding its model, instances and any run in progress. | yes |
| `session_eval` | Evaluate an expression in a session, as %eval does: a literal, a declared feature or qualified name, an enumeration literal, a unit, or a compound expression over the session's declarations. | no |
| `session_list` | List the open REPL sessions. | no |
| `session_load` | Load model files into a session, as %load does: files, directories or globs, resolved against the workspace root and refused outside it. | yes |
| `session_open` | Open a named REPL session: an empty model that declarations, instances and runs accumulate into across calls. | yes |
| `session_print` | Print a session's model as SysML notation, or one named element with its body, as %print does. | no |
| `session_run` | Run a behavior in a session to completion and report what it produced: an action (%action then %continue), a state machine (%state, then %advance when a duration is given), a calc (%calc) or an analysis case (%analysis). Its `arguments` are expressions evaluated together in one run, as the REPL evaluates an argument list, so a seeded draw in one continues the stream of the draw before it. | yes |
| `session_save` | Write a session's model to a file under the workspace root, as %save does: .sysml notation with comments preserved, or .ttl/.json RDF (experimental). | yes |
| `session_submit` | Submit SysML declarations to a session, as typing them at the prompt does: the text joins the model, the whole model is re-analyzed, and what it declared and the diagnostics about it come back. | yes |
| `solve` | Ask an SMT solver for values satisfying an element's conditions (%solve, experimental), keeping the values the model already fixes; reports the conflict when the fixed values admit none. | no |
| `sweep` | Run a calc or analysis case once per row of the ranges given (%sweep), or over rows drawn from them (%samples), and return the table: each row's inputs, outputs, verdicts and timing. | yes |

### Conventions

- **Positions** in arguments and results are 1-based `line` and `column` (a column counts
  characters, not UTF-16 code units; the server converts).
- **Paths** are absolute or relative to the workspace root (`-root`), and are resolved against
  it; a tool that takes `paths` accepts files, directories and globs. A path must stay under the
  root: one that leaves it, by its form (`../secret.sysml`, an absolute path elsewhere) or
  through a symlink, is refused, and writes are performed through the resolved root itself, so
  a link repointed after the check cannot carry one outside it. The `lsp` tools can also work on
  text held open with `open_document` instead of a file on disk.
- **Writing** happens only on request: a tool marked as writing (`format`, `rename`,
  `apply_model_edit`, `session_save`, ...) returns the edit or the file it would write and
  touches the disk only with `"apply": true`. The session tools that write change the session's
  state (declarations, instances, a run), not files, unless they say so.
- **Results** are structured JSON (`structuredContent`, the same shape the CLI, the REPL or the
  RPC answers) and a readable text with the same content; a tool-level failure is an `isError`
  result with the message, not a protocol error.
- **Budgets.** Long-running work honours the runtime's budgets — `OPENSYSML_MAX_STEPS`,
  `OPENSYSML_MAX_ACTION_STEPS`, `OPENSYSML_MAX_EVENTS`, `OPENSYSML_MAX_DO_STEPS` and
  `OPENSYSML_MAX_ELEMENTS` ([environment variables](environment.md)) — set in the environment the
  host starts the server with; a run that exhausts one is reported as a runaway, with the budget
  named.

## Transports

Without `-http` the protocol is spoken over **stdin and stdout**: the host starts the server,
which serves that one client and exits when the stream closes. Nothing but protocol messages is
written to stdout; logs go to stderr.

With `-http ADDR` the server is started once and speaks the **Streamable HTTP** transport at the
path `/mcp` of that address to every client that connects — remote agents, or several agents over
one project — until it is told to stop with SIGINT or SIGTERM. Stopping closes the clients'
sessions, so their event streams end, and gives the connections up to 30 seconds to drain. The
clients share one workspace, one runtime cache and one set of REPL sessions: a `session_*` name one
client opens is the session every other client sees under that name, and `session_list` lists them
all. A session's calls are served in turn, so two agents using one session interleave rather than
collide; agents that want sessions of their own give them different names.

```bash
sysml-mcp -http 8765                                              # http://localhost:8765/mcp, this machine only
sysml-mcp -http 0.0.0.0:8765 -allow-origin https://agents.example.org
sysml-mcp -http 0.0.0.0:8765 -tls-cert server.crt -tls-key server.key
```

The stance is the one [`sysml-grpc`](service-transports.md) takes, and the two are kept the same:

- **No authentication.** Every client that reaches the address may read and edit the workspace.
  A port alone, or an address with no host, binds loopback, which only this machine reaches;
  binding any other address (`0.0.0.0:8765`, a host name) is logged as a warning, and is for a
  trusted network or behind a proxy that authenticates — a proxy terminates TLS as well as
  authenticating, and then the server stays on loopback behind it.
- **TLS.** `-tls-cert FILE` and `-tls-key FILE`, both given, serve `https` (TLS 1.2 or later);
  one without the other, or a pair that does not load, is a usage error (exit status 2) before
  anything is served. Cleartext is appropriate only on loopback, inside a trusted network or
  behind a TLS-terminating proxy.
- **Origins.** A browser page reaches a server on this machine only through the `Origin` header
  the browser sets, and a page on another site can be pointed at a DNS name rebound to this
  machine, so the server answers a request when its `Origin` header is **absent** (a
  non-browser client), names a **loopback origin** (`localhost`, `127.0.0.0/8` or `::1`, over any
  scheme and port) or names an origin **listed in `-allow-origin`** exactly
  (`scheme://host[:port]`, comma-separated, compared case-insensitively on scheme and host); any
  other origin is refused with 403. The wildcard origin is not accepted: name each one. Only a
  listed origin is granted cross-origin access — the `Access-Control-Allow-Origin` header on its
  responses and an answer to its browser's preflight — so a page served from another port of this
  machine passes the check yet is still refused by its browser unless it is listed: a loopback
  origin is admitted so that it is not mistaken for rebinding, not so that every local page may
  edit the workspace. This is not authentication — it keeps browsers from being used as the way
  in.
- **Protocol version.** A request whose `MCP-Protocol-Version` header names a version the server
  does not speak is refused with 400, as the protocol requires; a request with no header is read
  as the version negotiated at `initialize`. The SDK's check that a loopback server is addressed
  as loopback (the `Host` header) stays on as well.

`-print-config HOST -http ADDR` writes the host's block in its URL form, so the host connects to a
server already running rather than starting one ([Configuring a host](#configuring-a-host)).

## Command line

| Flag | Default | Meaning |
|---|---|---|
| `-root DIR` | `.` | Workspace folder whose model documents the server indexes and edits |
| `-tools GROUPS` | `all` | Tool groups to serve, comma-separated: `lsp`, `runtime`, `session` |
| `-strict` | off | Strict conformance from the start: notation no pinned SysML v2 production admits is an error |
| `-no-record-cache` | off | Parse every document and hold it loaded, reading and writing no interface record (also `OPENSYSML_RECORD_CACHE=0`) |
| `-cache-size N` | `100` | Parsed models the runtime tools hold, at least 1 |
| `-log-level LEVEL` | `warn` | Lowest level logged to stderr: `debug`, `info`, `warn` or `error` |
| `-stdio` | — | Serve over stdin/stdout; the default transport, accepted for hosts that name it; not with `-http` |
| `-http ADDR` | — | Serve over Streamable HTTP at `/mcp` of `host:port`, or of loopback for a port alone; every client shares the workspace, runtime cache and REPL sessions |
| `-allow-origin ORIGINS` | — | Origins (`scheme://host[:port]`, comma-separated) granted cross-origin access; loopback origins and requests with no `Origin` are admitted without a grant, others are refused |
| `-tls-cert FILE`, `-tls-key FILE` | — | Serve `https`; both or neither |
| `-print-config [HOST]` | — | Write a host's configuration block to stdout and exit; alone, list the hosts |
| `-print-config-command CMD` | the binary's path | The command the block names; with `-http ADDR` (and `-tls-cert` for `https`) the block names the server's URL instead |
| `-version`, `-help`, `-man` | — | The build, this table, the manual page |

The exit status is 0 when the protocol was served to its end (the client closed the stream or the
process was told to stop), 1 when the server could not start (the `-http` address in use, say) or
the session ended in a protocol error, and 2 when the command line could not be acted on. `man sysml-mcp` has the same content
once the manual page from the archive is installed.

## Troubleshooting

- **Never run it in a shell.** `sysml-mcp` at a prompt prints nothing and waits: stdin is the
  protocol stream, and the first line typed at it is a protocol error. Let the host start it. To
  see that the binary runs, use `sysml-mcp -version`; to see what a host would get, drive it with
  a client such as `npx @modelcontextprotocol/inspector --cli /usr/local/bin/sysml-mcp --method tools/list`.
- **Logs are on stderr.** Nothing but protocol messages is written to stdout. The host shows the
  server's stderr in its MCP log (OpenCode `opencode.json` → the MCP panel, Claude Code
  `claude --mcp-debug`, VS Code *MCP: List Servers → Show Output*, Cursor's MCP settings page);
  `-log-level debug` in the `args` logs every tool call there.
- **The host cannot find the binary.** A block from `-print-config` names the absolute path, so
  this means the binary moved; print the block again, or use `-print-config-command sysml-mcp`
  and make sure the host's `PATH` has it (a GUI host does not read a shell's profile).
- **"unexpected argument"**: the server takes flags only; a host that passes a file or a
  directory should pass `-root DIR` instead.
- **A tool is missing.** `-tools` left its group out, or the host caches the catalogue: restart
  the server from the host after changing its arguments.
- **A tool answered a budget error.** Raise the `OPENSYSML_MAX_*` variable it names in the host's
  environment for the server (the `env` key most hosts accept beside `command`), not in your
  shell.
