# 10. Troubleshooting

This chapter is organized by symptom. Budgets and the full list of environment variables are
in [reference/environment.md](../reference/environment.md).

**The REPL does not show a prompt:**
- Check that the terminal supports readline (most Unix shells do)
- History is stored in `$XDG_STATE_HOME/sysml/history`, or in `~/.sysml_history` when `XDG_STATE_HOME` is unset; if that path is not writable, history is kept in memory for the session

**Import errors after building:**
- Run `go mod tidy`
- Verify the Go version with `go version` (1.25 or later is required)

**Execution stops with "limit exceeded" or "exceeded max":**
- The run used up one of its budgets; the message names the environment variable that raises it (see [reference/environment.md](../reference/environment.md))
- If the model never terminates, the budget is reporting a real defect, and raising it only delays the error

**`%check`/`%explain` report "no SMT solver found":**
- Solving is an experimental extension and no solver is bundled; install z3 (or cvc5) for your platform, as described in [1. Install: installing a solver](01-install.md#installing-a-solver-optional). `brew install Open-MBEE/tap/opensysml` installs z3 as a dependency
- Point `OPENSYSML_SMT` at a solver installed outside `PATH`; a value that names no executable is reported, not ignored
- No other command needs a solver: `%constraint`, `%requirement` and `%satisfy` evaluate without one

**`%check`/`%explain`/`%configure` report "does not support a feature this query needs":**
- The solver named by `OPENSYSML_SMT` rejected a feature the query needs; the message names the feature, the solver and the operation attempted. No result is reported from a script the solver would not accept
- The features each solver was measured to support, and how to test another solver, are documented in [1. Install: solver compatibility](01-install.md#solver-compatibility--pointing-the-driver-at-another-solver); z3 supports the whole subset
- cvc5 supports every feature the current commands need except objective optimization (`(maximize …)`, a z3 extension): `%optimize` refuses on cvc5, naming the missing extension, while every other command works normally

**`%check` answers `unknown`:**
- With `Reason: the solver ran out of time`, the query took longer than `OPENSYSML_SMT_TIMEOUT` (default `10s`); `unknown` is a verdict in its own right and is never reported as `sat` or `unsat`
- Otherwise the solver could not decide the arithmetic, and the reason it reports says so

**A run fails with "tool 'ModelCenter' is not registered; set OPENSYSML_TOOLS":**
- The action performed — or the `calc def` or calc usage invoked — carries `AnalysisTooling::ToolExecution`, and an annotated action or calc is only ever run by the tool it names — never by evaluating its body. Point `OPENSYSML_TOOLS` at a directory holding one JSON file per tool (`toolName`, `version`, `executable`, `variables`), as [reference/environment.md](../reference/environment.md#external-tools) describes; `sysml -engines` then lists the tool as `tool:ModelCenter` with whether its executable was found
- A tool that exits non-zero, answers something other than one JSON object of `outputs`, omits an output, names one no parameter receives, writes more than `OPENSYSML_TOOL_MAX_OUTPUT` (default 64 MiB), or takes longer than `OPENSYSML_TOOL_TIMEOUT` (default `10s`) fails the performance with that reason; no value is invented in its place

**`sysml -engines` does not list the engine in `OPENSYSML_ENGINES`, or lists it `unavailable`:**
- A manifest directory is read from outside every workspace, and its files and directory must be writable by their owner alone; the message at startup names the file and the rule (`is writable by others (mode 0664)`, `is under the workspace …`, `names a path outside the manifest directory`). One bad entry registers nothing from its directory, so fix the one named and list again
- `command` is confined to the manifest's directory unless absolute: a bare name is not looked up on `PATH`. `unavailable: stat …: no such file or directory` (or `… is not executable`) names the path that was tried; `-engines -probe` starts the engine once and reports what its `describe` disagreed with the file on, field by field ([External engines](../reference/external-engines.md#the-manifest-entry))
- A `policy`, `sampler` or `module` entry is listed `unavailable: … is not served in this build`, naming the stage that will serve it; only `engine` entries run

**An external engine's verdict is `not covered` though the engine reported a violation or `holds`:**
- The interpreter trusts nothing an engine claims: a `violated` stands only when its schedule replays and the condition is false at the move the witness names; `sensitive` needs two replaying schedules that end the named feature differently; `satisfiable` needs an assignment the evaluator confirms; `holds` stands as *observed* only over `executions` that replay, and without them it is *not covered* with the engine's claim kept in the reason, since no referee record admits the engine in this build. The reason says which step failed (`its witness does not replay (move 3: …)`, `its schedule replays and maxPressure holds at move 4`); a `broke protocol` reason names the message the engine got wrong, and its captured standard error follows ([The standing of an answer](../reference/external-engines.md#the-standing-of-an-answer), [Failures](../reference/external-engines.md#failures))
- `engine "…" did not answer cancel within 10s (OPENSYSML_TOOL_TIMEOUT); the process was ended` is the plan's deadline passing with the engine still running: raise the deadline, or the engine's own bounds
- Over the service, `engine '…' is not served by this service` means `sysml-grpc` was not started with `-serve-external-engines` naming it; that is the operator's decision, since the flag lets every client run the engine's command on the server

**Syntax errors:**
- Only SysML v2 textual notation is accepted; graphical notation and XMI are not
- Keywords are case-sensitive
- Multiplicity follows relationships: `part x subsets y [0..1];`

---

## Getting help

- **GitHub Issues:** report defects or request features
- **Discussions:** questions about SysML v2 usage
- **Specification reference:** [OMG SysML v2.1 Beta 1 Specification](https://www.omg.org/spec/SysML/2.0) (2026-08 release)

---

Back to the [guide index](README.md).
