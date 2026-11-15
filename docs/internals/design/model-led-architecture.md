# The self-model as the source of truth for the architecture

*A proposal. Nothing here is implemented; the note exists to be reviewed before code is
written.*

[`examples/self-model/`](../../../examples/self-model/README.md) states OpenSysML's architecture
in SysML v2, and `go test ./examples/` holds parts of it to the implementation. It is still a
description written after the fact: a change lands in Go, and the model follows when a test
fails or when someone remembers. This note proposes the reverse order — a change to the
architecture is a change to the model, carried by the same pull request as the implementation
and its tests — and the four pieces of work that make that order cheap enough to require and
checkable enough to trust.

## 1. Where the model stands

Measured on `develop` at `0757270e94`.

| Measure | Value |
| --- | --- |
| Model | 9 files, about 4,800 lines: 104 `part def`, 40 views, 20 invariants as requirements |
| Gate | `TestSelfModel*` in `examples/`, about 3,100 lines of Go; the model validates in 0.2 s |
| Packages under `internal/` and `cmd/` the model does not name | 66 of 130 |
| Code pull requests in 30 days that edited the model | 20 of 695 |
| Edits to `surfaces.sysml` (1,075 lines) in the same 30 days | 16 |

Three properties of the present arrangement follow from those figures.

**The model is checked only where a test was written for the fact.** Each
`TestSelfModel…MatchesImplementation` instantiates one modelled part and compares it, in Go, with
a registry, a descriptor or a file. A fact nobody wrote a comparison for drifts without a
failure: the surfaces fell behind until one pull request added 800 lines to bring them back.

**The architecture is stated in several places.** The layer table and the layer of every package
are Go maps in `tests/hygiene/layering_test.go`; the pipeline and its invariants are prose in
`docs/internals/architecture.md` and in `AGENTS.md` §3 and §4; the model states all three again.
`document.sysml` renders an architecture document, and nothing publishes it.

**Many invariants hold because the model says so.** `ImmutableSyntaxTree` requires
`tree.immutableAfterParse`, and the model sets that attribute to `true`. The requirement is
satisfied by construction; what verifies it is the test its `verification def` names, and no
gate checks that the named test exists or ran.

## 2. What requiring a model change would cost

The 100 pull requests merged before `0757270e94` that changed non-test code were replayed: a
snapshot of the enumerable facts (packages, RPCs and messages, command-line flags, environment
variables, REPL commands, LSP methods, MCP tools, validation passes, engines, Makefile targets,
CI jobs, the layer table, the invariants of `AGENTS.md` §4) was taken on each side of each
merge and the two were compared.

| | Pull requests |
| --- | --- |
| Changed at least one enumerable fact | 13 |
| — of which edited the model | 2 |
| Changed none | 87 |

Twelve of the thirteen were `feat` (of 22 in the sample) and one `ci`; none of the 52 fixes, 9
refactorings or 8 performance changes moved a fact. New packages account for eight, flags for
three. Four further pull requests changed a protobuf field, the capability list or a tool's
scope, which the snapshot does not count; the range is 13 to 17 in 100. The snapshot is taken
with patterns over the tree, so 13 is a floor.

Two conclusions. The requirement binds about one pull request in seven, nearly all of them
features that are already large, so the authoring cost is small. And whether it binds is
computable from the diff, so a fix never has to argue that it is exempt.

Two costs are real and are addressed below. A model in a few large files collides: sixteen
edits to one file in a month at 3% participation becomes a standing conflict at 15%. And the
model's files are pinned by four corpus baselines (`pilot-differential-baseline.json`, the two
round-trip expectations, `construct_cover.txt`), so every model edit is also a baseline
adjudication.

## 3. Three kinds of statement

The model makes three kinds of statement, and each needs a different check. Treating them alike
is what makes the present gate expensive.

| Kind | Example | Truth comes from | Check |
| --- | --- | --- | --- |
| **Enumerable** | the passes and their tiers, the packages and their layers, the RPCs, flags, variables, tools | the implementation, mechanically | generated as-built facts compared in the model (§4) |
| **Designed** | the tier ladder, the feature-read flow, which stage feeds which | the model | a test that drives the implementation along the modelled structure, as `TestSelfModelQuestionFlowFollowsDispatcher` does |
| **Asserted** | the tree is immutable, the parser never panics | a named test | a `verify` link to a test the gate confirms exists and passed (§4.4) |

## 4. As-built facts, compared in the model

### 4.1 The generated package

A command reads the implementation and writes one SysML package, `OpenSysMLAsBuilt`, holding
what is built and nothing about what was intended: one part per Go package with its path and its
imports, one per registered pass with its tier and scope, one per RPC, flag, environment
variable, REPL command, LSP method, MCP tool, engine, rendering form and report format. Each
reader is the one the corresponding test uses today — `passes.DefaultRegistry()`, the protobuf
descriptor, the `usage` tables behind the manual pages, the MCP catalog — so the command has to
link the root module's packages.

The package is derived, so it is not committed, like the standard-library snapshot and the
combined compliance map. `make self-model-facts` writes it under `build/`; the gate generates it
in memory. A committed copy would conflict on every pull request that adds a package and would
let a stale copy pass.

### 4.2 Conformance as requirements

The hand-written model keeps the design: units, their layer, what each surface serves. A
conformance requirement relates the two, and the tool evaluates it:

```sysml
requirement def EveryPackageIsAllocated {
    subject built : Tree;
    in designed : Architecture;
    require constraint {
        built.packages->forAll { in p : GoPackage;
            designed.units->exists { in u : Unit;
                u.goPackage == p.path and u.layer == p.layer } }
    }
}
```

A two-file prototype of this shape was run through `%requirement` on the current build: it
reports `violated` while a built package has no unit and `holds` once the unit is added. One
requirement of this shape per kind of fact, in both directions (nothing built that is not
designed, nothing designed that is not built), replaces the per-fact comparisons in
`examples/self_model*_test.go`; one Go test evaluates every conformance requirement.

Two things the prototype showed are part of the work:

- The failure reads `built.packages->forAll body` and does not name the package. Each
  conformance requirement is paired with a query that lists the offending elements, and the
  gate prints that table; a failure that does not say what to add is not acceptable for a rule
  every feature meets.
- The layer rule moves into the model with it: *a unit imports only units of its own layer or a
  permitted one* is a requirement over the as-built imports, and
  `tests/hygiene/layering_test.go` keeps only the evaluation.

### 4.3 One file per unit

Before any rule asks more pull requests to edit the model, the model is split so that two
unrelated features do not edit the same file: a directory per present file
(`surfaces/`, `pipeline/`, …) with one file per surface, stage or registry, the package
structure unchanged. Counts stated beside a list (`passCount = 65`) are removed where the
as-built comparison makes them redundant; they are the lines every addition touches.

The model also leaves `examples/`. It is not an example once it is normative, and its place
under `examples/` is what ties it to four corpus baselines. At a top-level `model/` it keeps a
gate of its own (clean analysis, the conformance requirements, the round trip) and stops moving
the corpus figures. The `examples/` entry stays as a pointer.

### 4.4 Asserted invariants name their evidence

Every `#Invariant` requirement is the object of a `verify` from a `verification def` that names
the Go test establishing it. The gate fails when an invariant has no verification, or when the
named test is not found in the package it names. This does not make the assertion true; it makes
the claim *this invariant is tested* one that cannot go stale.

## 5. Tables the code reads from the model

Where the same list is written in Go and in the model, the model's copy becomes the only one:

| Today | After |
| --- | --- |
| `layers`, `permitted` and `packageLayer` in `tests/hygiene/layering_test.go` | the model's layers and allocations, evaluated per §4.2 |
| The repository map and the invariants in `AGENTS.md` §3–§4 | blocks generated from the model between markers, checked the way `make docs-counts` checks its figures |
| The stage and invariant tables in `docs/internals/architecture.md` | the tables `views.sysml` already renders |

The registries stay in Go. Generating implementation from the model is out of scope: a pass is
registered by the code that implements it, and the model is told by the as-built package.

## 6. The architecture document comes from the model

`document.sysml` already renders the architecture document with its diagrams and generated
tables. The documentation build renders it into the site as the architecture page, as the build
already computes the suite figures. `docs/internals/architecture.md` is then retired section by
section: a section moves when the model's document covers what it says, and the prose that
remains is the part that argues a design rather than describes one. User-facing reference pages
(flags, variables, the manual pages) are already generated from the `usage` tables and stay so;
the model learns those facts from the as-built package, so nothing is authored twice.

## 7. The rule for contributors and agents

With §4–§6 in place the rule is short, and for enumerable facts it enforces itself:

1. **A change to the architecture starts in the model.** A feature that adds a unit, a surface
   operation, a pass, an engine, a budget, a flag or an invariant, or changes a modelled flow,
   edits the model in the same pull request, and the model analyses clean. `AGENTS.md` §8 gains
   it as the step before *design the data first*.
2. **Implementation and tests follow in the same pull request.** For an enumerable fact the
   conformance gate fails until model and implementation agree, in either order. For a designed
   flow the pull request adds or extends the test that drives the implementation along it. For
   an asserted invariant it adds the `verify` and the test.
3. **A change that moves no modelled fact needs no model edit**, and the gate decides that, not
   the author.
4. **Review reads the model diff first.** `REVIEW.md` gains the check that a designed or
   asserted statement in the diff is true of the code in the diff, which no gate can decide; the
   pull request template gains one line naming the model elements changed.

Valid SysML is necessary and not sufficient: an agent can write a model that analyses clean and
describes something else. The rule is only as good as the checks of §3, which is why it comes
last.

## 8. Order of work

| Step | Delivers | Done when |
| --- | --- | --- |
| 1 | The model split one file per unit and moved to `model/` | the same packages analyse clean; no corpus baseline names a model file |
| 2 | The as-built package and conformance requirements for packages and layers | 0 unallocated packages; the Go layer maps are deleted |
| 3 | Conformance for the remaining enumerable facts | each per-fact comparison in `self_model*_test.go` is deleted as its requirement lands |
| 4 | `verify` links for every invariant | the gate fails on an invariant without evidence |
| 5 | Generated blocks in `AGENTS.md`; the rendered document published | `architecture.md` holds no table the model renders |
| 6 | The rule in `AGENTS.md`, `REVIEW.md` and the template | a replay of §2 over the following 100 pull requests finds every fact-changing one editing the model |

Steps 1 and 2 are worth doing whether or not the rule is adopted: they close the 66-package gap
and remove a duplicated table.

## 9. Open decisions

- **Where the model lives.** `model/` at the top level is proposed (§4.3); keeping it under
  `examples/` keeps the corpus sweeps over it and the baseline adjudication with every edit.
- **Whether the as-built package is committed.** Not committing is proposed (§4.1); a committed
  copy is readable on GitHub without a build.
- **How far the designed kind goes.** Flows and state machines can be held to the implementation
  only by tests written per flow. Whether new behavior must be modelled at that depth, or only
  where a view or the document draws it, is a matter of how much of the design the model is
  meant to carry.
