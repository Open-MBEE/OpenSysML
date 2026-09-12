# Alignment with the UML precise-semantics specifications

Whether, and how far, the runtime should align with the OMG precise-semantics family that
SysML v1 execution tools ran under: **fUML** (the executable subset of UML activities and
classes), **PSCS** (composite structures: parts, ports, connectors) and **PSSM** (state
machines, which ships a conformance test suite of UML models with expected traces). The runtime's
position is stated in [the architecture](../architecture.md) (the runtime section's *Spec Alignment* paragraph): the SysML v2
metamodel and the bundled KerML semantic library govern execution, UML 2.5.1 is the fallback only
where v2 has no production and the library no performance, and the runtime is not a UML or fUML
activity engine. This note tests that position against the three specifications clause by
clause, counts where a port could change behavior, assesses the PSSM test suite as a referee, and
recommends what to do. It changes nothing; implementation, if any, is a later decision.

Written for a maintainer who has read neither the precise-semantics specifications nor the state
executor: each row quotes or paraphrases the clause, then the library, then the code, then gives
a verdict. Every runtime claim names a file and function and the test or fixture that pins it;
every specification claim names a clause read in the pinned document.

## Sources, pinned

### The precise-semantics specifications

| Specification | Version, OMG document | Read | Machine-readable artifacts |
|---|---|---|---|
| **PSSM** — Precise Semantics of UML State Machines | 1.0, `formal/19-05-01`, <https://www.omg.org/spec/PSSM/1.0> | Clause 2 (conformance), Clause 6 (scope of the subset), §8.5.4 state-machine configuration, §8.5.5 state activations, §8.5.6 do-activity, §8.5.7 pseudostates, §8.5.8 transition activations, §8.5.9 event pool and deferral, §8.5.10 run-to-completion, Clause 9 (test suite: 9.1–9.3 architecture, 9.4 test cases) | `ptc/18-11-04` syntax (`PSSM_Syntax.xmi`), `ptc/18-11-05` semantics (`PSSM_Semantics.xmi`), `ptc/18-11-06` test suite (`PSSM_TestSuite.xmi`), all linked from the specification page |
| **fUML** — Semantics of a Foundational Subset for Executable UML Models | 1.5, `formal/21-03-01`, <https://www.omg.org/spec/FUML/1.5> | §2.3 conformance and semantic variation points, §7.3 the excluded features (`TimeEvent`, `ChangeEvent`, interruptible regions), §8.8.1 common behaviors (execution, object activation, event pool, `GetNextEventStrategy`), §8.9.1 activities (tokens, offers, node activations), §8.9.2 actions (`ChoiceStrategy`, `FirstChoiceStrategy`, decision, fork, join, merge, expansion regions, structured nodes, accept-event, send-signal, call, start-object-behavior, destroy) | `ptc/20-05-12`, `ptc/20-05-13`, `ptc/20-05-14` |
| **PSCS** — Precise Semantics of UML Composite Structures | 1.2, `formal/19-02-01`, <https://www.omg.org/spec/PSCS/1.2> | §8.1 overview, §8.4 structured classifiers and `CS_RequestPropagationStrategy` / `CS_DefaultRequestPropagationStrategy`, §8.6 `CS_DefaultConstructStrategy`, `CS_Object` (`sendIn`/`sendOut`/`dispatchIn`/`dispatchOut`), `CS_Link`, delegation and assembly connectors, destruction of parts and links | `ptc/18-08-04` through `ptc/18-08-09` |

Versions and document numbers are those the OMG specification pages listed at the time of
writing; the PDFs and the XMI files were fetched from those pages and read, not recalled. The
spec-numbering convention here is the document's own (PSSM `8.5.9`, fUML `8.8.1`).

### The specifications the runtime already follows

| Specification | Version, OMG document | Clauses this note relies on |
|---|---|---|
| **SysML v2** | 2.0, `formal/26-03-02`, <https://www.omg.org/spec/SysML/2.0> | §7.18.1–§7.18.3 states and transitions (entry/do/exit, `parallel`, transition steps, `done`, a transition to `terminate`); §8.4.13.4 control nodes (`ForkNode`, `JoinNode`, `MergeNode`, `DecisionNode`); §8.4.13.5–§8.4.13.6 send and accept actions; §8.4.13.8 terminate action |
| **KerML** | 1.0, `formal/26-03-01`, <https://www.omg.org/spec/KerML/1.0> | §9.2.11.1 `StatePerformances`, §9.2.12 `TransitionPerformances`, §9.2.5 `Occurrences` (`isDispatch`, `dispatchScope`, `isRunToCompletion`, `runToCompletionScope`, `incomingTransferSort`), §9.2.10 `ControlPerformances`, §9.2.13 `Clocks` |
| **Bundled semantic library** | the pilot implementation's library snapshot shipped under `internal/core/libs/stdlib` (EPL-2.0, see its `NOTICE`) | `Kernel Semantic Library/Occurrences.kerml`, `StatePerformances.kerml`, `TransitionPerformances.kerml`, `ControlPerformances.kerml`, `Clocks.kerml` — the text the runtime executes against, quoted where it differs in wording from the specification PDF |

The record of which pilot release and which OMG documents the conformance work pins is
[spec-compliance](../../project/spec-compliance.md); this note does not re-pin them.

### The PSSM test suite

- **Where.** Published beside the specification as `ptc/18-11-06`, a UML 2.5 XMI file
  (`PSSM_TestSuite.xmi`), linked from <https://www.omg.org/spec/PSSM/1.0>. Clause 9 of the PDF
  describes it and reproduces the model of every test with its expected trace.
- **Licence.** The specification's front matter grants a limited licence to use, copy and
  distribute the specification (of which the suite is a machine-readable part) subject to
  retaining its notices, for informational use, without modification and without commercial
  resale or transfer. The suite is **not vendored, copied or committed** to this repository, and
  no fixture here is derived from it; the two illustrative translations in this note are written
  by hand from the PDF's figures and are examples in prose, not fixtures.
- **Form.** Clause 9.1 describes the suite as itself a fUML/PSCS/PSSM-conformant model. Each
  test is a UML class specializing the abstract `SemanticTest` (its `SemanticTestSuite` owner
  registers the concrete tests), with a `Tester` object that sends a `Start` signal to a
  `Target` object whose classifier behavior is the state machine under test; every entry, exit,
  effect and do-activity body calls a `trace(String)` operation that appends a segment to a
  `TraceBuilder`, and the test compares the resulting `::`-separated string
  (`S(entry)::S(doActivity)::T(effect)`, in the PDF's own notation) with the expected trace(s).
  A test may declare several expected traces where PSSM admits alternative valid orderings
  (orthogonal regions, do-activity interleaving); Clause 9.3 describes each test with the
  requirement it covers, its model, its expected result and the run-to-completion steps that
  produce it, and Clause 9.4 maps the requirements of Clause 8 to the tests covering them. Clause
  9.1 states the design: each test verifies one requirement extracted from UML Clause 14 "as
  formally interpreted according to the semantic model defined in Clause 8".
- **Count.** Traversing `PSSM_TestSuite.xmi` for concrete `uml:Class` elements whose direct
  generalization is `SemanticTest` gives **103** tests, grouped by the package they live in:

  | Area | Tests | Area | Tests |
  |---|---|---|---|
  | Behavior | 5 | Terminate | 3 |
  | Transition | 15 | Final | 1 |
  | Event | 16 | History | 8 |
  | Entering | 5 | Deferred | 10 |
  | Exiting | 5 | Redefinition | 6 |
  | Entry (entry points) | 6 | Standalone (a state machine that is itself the active test target, not a classifier behavior) | 3 |
  | Exit (exit points) | 3 | Junction | 6 |
  | Choice | 5 | Other (UML Figure 14.2, the transition execution algorithm example) | 1 |
  | Fork | 2 | | |
  | Join | 3 | **Total** | **103** |

  Of the 103, 67 declare one expected trace, 22 declare two, 5 three, and the remaining 9 declare
  between four and 84 alternatives (the largest are the orthogonal-region and do-activity
  interleaving tests). The XMI's packages are the PDF's test categories; the PDF's coverage
  clause (9.4) groups the entry-point and exit-point categories under "Encapsulated".

## The semantic map

Each row: what the precise-semantics specification says; what SysML v2 and the KerML library say;
what the runtime does today, with the file and function and the test or fixture pinning it; one
of four verdicts.

- **agrees** — PSSM (or fUML, PSCS) and the runtime's v2-grounded behavior coincide.
- **differs because v2 differs** — the v2 library or notation specifies otherwise; both texts
  are quoted.
- **differs, v2 silent** — v2 gives no rule; the runtime's behavior is a tool choice and PSSM
  offers one.
- **gap** — a concept the runtime has no behavior for at all.

Test names are conformance fixtures under `internal/core/runtime/testdata/conformance/` (run by
`TestExecutionConformance`; a `.trace.golden` beside one is compared by `TestExecutionTrace`),
subtests of `TestRuntimeRobustness` in `internal/core/runtime/robustness_test.go`, or unit tests
in the runtime package. File paths are relative to `internal/core/runtime/` unless stated.

### State machines (PSSM)

#### The event pool and the run-to-completion step

#### Event dispatch order and deferred events

#### Completion events and completion transitions

#### Entry, do and exit

#### Transition selection, priority and conflicts

#### Orthogonal regions and the recorded choice points

#### History

#### Choice and junction

#### Fork and join

#### Transition kinds: external, local, internal

#### Terminate

#### Time and change events against the simulation clock

#### Object lifecycle

### Actions (fUML)

### Composite structures (PSCS)

## The count

## The test suite as a referee: a capability map

## Options

## Recommendation

## Findings about our own conformance

## Open decisions
