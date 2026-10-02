---
name: opensysml-cli
description: Check, evaluate, execute and convert SysML v2 models with the OpenSysML `sysml` command — validation as a gate, `-e` expressions, running calcs/actions/state machines, constraint/requirement/satisfy verdicts, objects, JSON output, exit statuses and piping REPL commands. Use when a task involves running or checking a .sysml or .kerml file.
---

# Using the `sysml` command

`sysml` loads every file named on the command line as one model, runs what the flags ask, and exits.
With no mode flag it opens the interactive REPL, so in a script always pass a flag. Flags may come
before or after the files. Full reference: `docs/reference/cli.md` in the OpenSysML repository, or
`sysml -help`.

## Always validate first

```bash
sysml -validate model.sysml            # exit 0: analyses cleanly (warnings allowed)
sysml -validate a.sysml b.sysml        # several files are one model; order does not matter
cat model.sysml | sysml -validate -    # `-` reads standard input
```

A model with any error is never run or checked: every mode reports `did not analyse cleanly; no
check was made` and exits 2. Read and fix the diagnostics (`file:line:col: error: ...`, with a caret
under the location) before anything else. Add `-strict` when another SysML v2 tool must read the
model: OpenSysML's own notations then become errors instead of warnings.

## Exit status

| Status | Meaning |
|--------|---------|
| `0` | Done: loaded cleanly, every expression produced a value, every check held, the conversion was written |
| `1` | The model answered false: a constraint, requirement or satisfaction did not hold |
| `2` | Nothing was answered: unreadable file, model with errors, unresolved name, a check that could not be made, a misused flag |
| `3` | Only some of a `-render-documents` set rendered |

Results go to stdout; diagnostics, warnings and failures go to stderr. Branch on the status, not on
the text.

## Evaluating expressions

```bash
sysml -e "10 * 2 + 5"
sysml -e "MyModel::Vehicle::mass" -e "MyModel::Vehicle::mass > 1000.0" model.sysml
```

Each result prints as the expression on a `✓` line followed by an indented `= <value>` line;
`awk '/^ *=/ {print $2}'` extracts the values. `-e` evaluates at model level, over what the
declarations state: a feature the model leaves open reads as `<undetermined>` (still exit 0), so
compare values literally. Use qualified names (`Package::Def::feature`).

## Running behavior

```bash
sysml -calc "MyModel::Margin(20.0, 100.0)" model.sysml      # call expression
sysml -action MyModel::calibrate model.sysml                 # runs to completion, prints Results
sysml -state MyModel::Monitor -advance 15 model.sysml        # advances the clock 15 s
sysml -analysis MyModel::CostAnalysis model.sysml            # out/return values and objective verdict
```

`-action` prints `Final state: Completed` and the action's parameter values under `Results:`. The
command line passes no inputs: an action's `in` parameters take their `default` values (declare
them `in x : Integer default = 21;`), or run the action on an object with `-action "Pkg::act obj"`.
From Python, `execute_action(..., inputs={...})` binds inputs directly.
`-trace` reports every step; use it when a run stops unexpectedly or a value is wrong.

## Checking constraints and requirements

```bash
sysml -constraint MyModel::MassBudget model.sysml
sysml -requirement MyModel::healthy model.sysml
sysml -satisfy model.sysml                       # every `assert satisfy R by x;`
sysml model.sysml -instantiate MyModel::car -validate=MyModel::car   # every assertion an object and its parts carry
```

A false verdict exits 1 and names the condition that evaluated to false. A verdict that could not be
reached (for example a condition over a value the model leaves open) is not a failure: it exits 2
with the reason.

## Objects

`-instantiate MyModel::car` builds an object and prints its ID; a feature with no value shows as
`<unset>`. To read an object's feature values, pipe REPL commands:

```bash
printf '%%instantiate MyModel::car\n%%features MyModel::car depth 0\n' | sysml model.sysml
printf '%%instantiate MyModel::car\n%%features MyModel::car all json\n' | sysml model.sysml | sed -n '/^{/,$p'
```

`%features` lists the features inherited from the standard library too, so grep for the ones you
want; `depth 0` stops it expanding nested objects. With `json` it writes the whole object graph
(`instance.featureValues.<name>.value`), the dependable form for a script. Any REPL command works
this way (`%help` lists them).

## Machine-readable output

Add `-json` to any check mode to get one JSON document on stdout: `status` (worst verdict), `exit`,
`checks` (each with `subject`, `status`, `values`, `lines`), `diagnostics` (with `file`, `line`,
`column`) and `errors`. Prefer it over parsing `✓`/`✗` lines.

## Converting and rendering

```bash
sysml model.sysml -convert sysml -o formatted.sysml     # canonical notation
sysml model.sysml -convert ttl -o model.ttl             # RDF (experimental; a notice goes to stderr)
sysml model.sysml -render MyModel::structureView -render-form mermaid
sysml -query 'oslc.where=sysml:name="wheel"' model.sysml
```

## Pitfalls

- Each file imports what it uses: a `private import ScalarValues::*;` in one file does not serve another.
- A blank line ends a submission in piped REPL input; do not put one inside a declaration.
- Do not treat `<undetermined>` or `<unset>` as zero or false.
