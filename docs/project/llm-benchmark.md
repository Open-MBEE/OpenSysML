# LLM SysML v2 Authoring Benchmark

## Overview

**Question:** how well does a language model write SysML v2 textual notation when OpenSysML is the compiler in the loop, and does offering it the `sysml-mcp` tools improve the result?
**Harness:** `go run -C tools ./cmd/llm-bench` (library `tools/bench/llm`)
**Corpus:** 50 natural-language modelling tasks with gold models under `tools/bench/llm/corpus/` (`manifest.json` plus one `.sysml` per task); every gold validates under `sysml -validate` and under the pinned OMG pilot validator (`scripts/download-pilot-sysml-validator.sh`, release `2026-08`), and `TestCorpusGoldValidates` in `tools/bench/llm` asserts both
**Baselines:** one results file per model run, `llm-benchmark-<provider>-<model>.json` in this directory
**Status:** advisory only — nothing here gates CI; CI runs the loop, the scorer, the structural extraction and the MCP plumbing through the `replay` provider over `tools/bench/llm/testdata/replay/synthetic.json` and never calls a live provider

The design follows the generate–compile–repair pattern: the model is asked for a complete
document, the compiler judges it, the diagnostics go back to the model verbatim, and the model
repairs, for a bounded number of rounds. The corpus, prompts, scorer and extraction are this
project's own and the language under test is standard SysML v2.

---

## Method

### Loop

For each cell (task × arm × sample):

1. **Generate.** The model receives the system prompt and the primer with the task text and
   answers with one document. Markdown fences around the answer are stripped
   (`ExtractModel`); nothing else is edited.
2. **Validate.** The document is written to a scratch file and judged by
   `sysml -validate -json`. Any `error` diagnostic fails the round; warnings do not.
3. **Repair.** The error diagnostics are rendered through the feedback template below and sent
   back as the next user turn, with the model's previous answer as the assistant turn, up to
   `DefaultMaxRounds` = 3 repair rounds after the first generation (so at most four documents
   per cell).
4. **Measure.** The last document is scored (below). When `-pilot` is given, every document the
   compiler accepted is also judged by the pilot validator.

### Arms

- **compiler** — the model sees compiler diagnostics only.
- **mcp** — the model additionally has the `sysml-mcp` tools `parse`, `explain_diagnostic`,
  `symbol`, `validate_instance` and `tool_catalog`, served by the project's own MCP server
  (`internal/frontend/mcp`, the same code as `cmd/sysml-mcp`) started in-process over an
  in-memory transport with the LSP and runtime tool groups. Tool calls are forwarded as the
  provider's native tool protocol (OpenAI `tools`/`tool_calls` or Anthropic `tool_use`), answered,
  and the model is asked again; at most `MaxToolCallsPerRound` = 8 tool calls are served per
  round before the model must answer with a document. Calls beyond the budget are refused with
  a tool error instead of being made (recorded with `refused: true`), and a model that still
  calls tools after the refusal fails the cell. Every call is recorded in the results file with
  its arguments, the SHA-256 and size of the answer, and the first 400 bytes of it.

### Providers

| Provider | Transport | Key |
|---|---|---|
| `openai` | OpenAI chat completions, `https://api.openai.com/v1` | `OPENAI_API_KEY` |
| `xai` | OpenAI-compatible, `https://api.x.ai/v1` | `XAI_API_KEY` |
| `openrouter` | OpenAI-compatible, `https://openrouter.ai/api/v1` | `OPENROUTER_API_KEY` |
| `ollama` | OpenAI-compatible, `http://localhost:11434/v1` | none |
| `vllm` | OpenAI-compatible, `http://localhost:8000/v1` | none |
| `anthropic` | Anthropic Messages, `https://api.anthropic.com/v1` | `ANTHROPIC_API_KEY` |
| `replay` | committed fixture, no network | none |

Keys are read from the environment only. A provider whose key is unset exits with status 3 and a
message naming the variable; nothing is logged. `-record <file>` wraps any provider and files
every answer into a replay fixture keyed by cell, with the request kept only as a SHA-256 digest;
`-provider replay -fixture <file>` plays it back (`Strict` replay refuses a request whose digest
differs from the recording).

### Prompts

The three prompt texts live under `tools/bench/llm/prompts/` and are embedded at build time.
They are reproduced here verbatim.

`system.txt`:

```text
You are an expert systems engineer writing SysML v2 textual notation (the OMG SysML v2 standard, KerML-based). You answer with exactly one complete SysML v2 document and nothing else: no prose, no explanation, no Markdown fences. The document must parse and validate against the OMG standard library on its own.
```

`primer.txt` (`{{.Task}}` is the task text from the manifest):

```text
Write one SysML v2 textual-notation document that models the task below.

Rules:
- Use standard SysML v2 only; the OMG standard library (ScalarValues, ISQ, SI, StandardViewDefinitions, VerificationCases, ...) is available but must be imported explicitly, e.g. `private import ScalarValues::*;`.
- Use exactly the package, definition and usage names the task gives; add nothing the task does not ask for.
- Every referenced name must be declared in the document or imported from the standard library.
- Output only the document text: no commentary, no Markdown fences.

Task:
{{.Task}}
```

`feedback.txt` (one line per error diagnostic, rendered from the `-validate -json` fields):

```text
The document you wrote did not validate. The compiler reported {{len .Diagnostics}} diagnostic(s):

{{range .Diagnostics}}{{.File}}:{{.Line}}:{{.Column}}: {{.Severity}}[{{.Code}}]: {{.Message}}
{{end}}
Repair the document so it validates. Keep everything the task asked for. Output only the complete corrected document: no commentary, no Markdown fences.
```

### Metrics

Every metric is computed per cell, then per sample over the tasks, then reported as the mean and
sample standard deviation over the samples (so with 3 samples the ± is over 3 values).

- **CR@0** — compile rate at round 0: the first document had no `error` diagnostic.
- **FCR** — final compile rate: some document within the rounds had no `error` diagnostic.
- **Repairs** — the round of the first accepted document (0 when the first compiled); the maximum,
  3, when none did.
- **Fidelity** — structural fidelity of the final document against the gold, as the Jaccard
  index of two fact sets built from `sysml -convert api-json` (the SysML v2 API JSON export).
  The facts are, for every element with a non-anonymous `qualifiedName`, the string
  `"<@type> <qualifiedName>"`, and for every relationship element whose `source` and `target`
  both name such elements, the string `"<@type> <source> -> <target>"`, memberships excepted
  (ownership is already carried by the qualified names). `|A ∩ B| / |A ∪ B|`, with two empty sets
  scoring 1. A document that never compiled scores 0.
- **Halluc.** — hallucination rate: the share of cells whose first document drew at least one
  `unresolved` diagnostic, i.e. referenced a name that exists neither in the document nor in the
  standard library. The names are kept per cell.
- **Pilot agrees** — among the compiled final documents the pilot validator judged, the share it
  accepted (an error-free run of `validate-sysml-batch` over the document with the project's
  copy of the standard library as the extension library).
- **Tool calls** — tool calls per cell (0 in the compiler arm by construction).

Per-family and per-difficulty final compile rate and fidelity are kept in each results file.

### Paired comparison

For each metric that is defined per cell (CR@0, FCR, repairs, fidelity) the two arms are paired
by (task, sample), cells with a provider error excluded (such cells are counted under `errors`
in the aggregates and enter no rate or stratum). The results file carries the mean difference
(mcp − compiler), a paired t statistic with its two-sided p-value from Student's t with n − 1
degrees of freedom, and the sign test: the counts of pairs where mcp did better and worse
(fewer repairs is better; higher is better for the other three), and the exact two-sided
binomial p-value over the non-tied pairs. With few pairs the
sign test is the one to read; the t-test assumes approximately normal differences.

---

## Corpus

Ten tasks per family, 15 simple / 20 medium / 15 complex over the five families. Each task
names the package and the definitions and usages it wants, so that a gold exists against which
fidelity is meaningful; the gold is one reading of the task, not the only one, and fidelity
below 1 is expected even for correct answers that chose other element kinds.

| Task | Family | Difficulty | Title |
|---|---|---|---|
| `S01` | structure | simple | Bicycle part tree |
| `S02` | structure | simple | Laptop attributes |
| `S03` | structure | simple | Vehicle specialization |
| `S04` | structure | medium | Pump ports |
| `S05` | structure | medium | Charging station connection |
| `S06` | structure | medium | House wiring interface |
| `S07` | structure | medium | Heating system flows |
| `S08` | structure | complex | Office multiplicities |
| `S09` | structure | complex | Satellite subsystems |
| `S10` | structure | complex | Robot controller connections |
| `B01` | behaviour | simple | Coffee brewing sequence |
| `B02` | behaviour | simple | Lamp states |
| `B03` | behaviour | simple | Dishwasher action flow |
| `B04` | behaviour | medium | Door lock state machine |
| `B05` | behaviour | medium | Checkout decision |
| `B06` | behaviour | medium | Thermostat guarded transitions |
| `B07` | behaviour | medium | Elevator state actions |
| `B08` | behaviour | complex | Login authentication flow |
| `B09` | behaviour | complex | Traffic light nested states |
| `B10` | behaviour | complex | Vending machine behaviour |
| `R01` | requirements | simple | Kettle requirement |
| `R02` | requirements | simple | Bridge requirement group |
| `R03` | requirements | simple | Drone mass requirement |
| `R04` | requirements | medium | Camera satisfaction |
| `R05` | requirements | medium | Vehicle performance requirements |
| `R06` | requirements | medium | Scale verification case |
| `R07` | requirements | medium | Pacemaker specification |
| `R08` | requirements | complex | Parachute drop test |
| `R09` | requirements | complex | Smoke detector specification and test |
| `R10` | requirements | complex | Insulin pump specification tree |
| `C01` | calculations | simple | Parcel quantities |
| `C02` | calculations | simple | Table area calculation |
| `C03` | calculations | simple | Elevator mass limit |
| `C04` | calculations | medium | Average speed |
| `C05` | calculations | medium | Heater electrical calculations |
| `C06` | calculations | medium | Water tank volume |
| `C07` | calculations | medium | Vehicle mass budget |
| `C08` | calculations | complex | Projectile energy |
| `C09` | calculations | complex | Steel beam safety factor |
| `C10` | calculations | complex | Battery pack runtime |
| `V01` | views | simple | Garden documentation |
| `V02` | views | simple | Clock comments |
| `V03` | views | simple | Ship structure view |
| `V04` | views | medium | Hospital safety viewpoint |
| `V05` | views | medium | Factory line browser view |
| `V06` | views | medium | Aircraft structural rendering |
| `V07` | views | medium | Metro operations view |
| `V08` | views | complex | Library structure and grid views |
| `V09` | views | complex | Spacecraft viewpoints |
| `V10` | views | complex | Wind farm views |

Every gold imports what it uses from the OMG standard library (`ScalarValues`, `ISQ`, `SI`,
`VerificationCases`, `StandardViewDefinitions`, `Views`, …) and uses nothing OpenSysML-specific.

---

## Results

### Live runs

No hosted provider has been run yet: `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `XAI_API_KEY` and
`OPENROUTER_API_KEY` were unset where this record was produced, so those providers were skipped.
The local `ollama` run is in progress; its results file and table land here when it completes.

### Replay check

The committed fixture `tools/bench/llm/testdata/replay/synthetic.json` is not a model: it is
six tasks' gold documents with known mutations (an unresolved `BogusType`, a missing brace, a
Markdown fence, tool calls to each of the five tools), so the test can assert every column
exactly. `TestReplayLoop` in `tools/bench/llm` runs it through both arms and checks compile
rates, repairs, the hallucinated name, fidelity 1 for a gold against itself, the pilot verdict,
the recorded tool calls and the paired comparison.

---

## Limitations

- **Sample size.** Three samples per cell at the default temperature 0 measure provider
  nondeterminism more than model variance; the ± is the sample standard deviation over those
  samples, and the paired tests have at most 150 pairs per model.
- **Prompts.** One system prompt, one primer and one feedback format, all above; no few-shot
  examples and no retrieval. A model that knows the pilot's textual notation well is favoured;
  one that does not can only learn from the diagnostics.
- **Gold is one answer.** Fidelity compares against a single gold reading, so it rewards following
  the task's naming and element kinds and penalises equally valid alternatives.
- **The compiler is the judge.** FCR and CR@0 are OpenSysML's verdicts; the pilot column records
  where the pilot disagrees with them, over compiled documents only.
- **Tool use is the model's choice.** The mcp arm offers tools; a model that never calls them
  measures the same as the compiler arm plus the cost of the tool descriptions in its context.
- **Dates and versions** are in each results file (`recorded`, `model`, `modelReported`,
  `toolchain`). Hosted models change behind a name; re-running later measures a different model.

## Running it

```bash
make build && ./scripts/download-pilot-sysml-validator.sh      # bin/sysml and the pilot
go run -C tools ./cmd/llm-bench run -provider anthropic -model <model> -pilot \
  -out docs/project/llm-benchmark-anthropic-<model>.json
go run -C tools ./cmd/llm-bench report docs/project/llm-benchmark-*.json   # the tables above
go run -C tools ./cmd/llm-bench run -provider replay \
  -fixture tools/bench/llm/testdata/replay/synthetic.json -tasks S01 -samples 2
```

`-tasks`, `-arms`, `-samples`, `-rounds`, `-temperature`, `-max-tokens` and `-base-url` narrow
or redirect a run; `-record` files every answer for later replay.
