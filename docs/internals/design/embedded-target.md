# An embedded target for Class A flight software

A design for compiling a SysML v2 behavior — a state machine, an action, the calcs and
constraints they reach — to C that runs on a microcontroller under an RTOS, in a form that
software assurance at NASA's highest software class can accept. The roadmap's [embedded
track](../../project/roadmap.md#track-m--an-embedded-rtos-compatible-target) states the route: a
closed behavior IR, an ahead-of-time C backend over static tables, and a typed refusal of every
model the target cannot bound. This note fixes what that route leaves open once the software
class is Class A: the C profile the backend emits, the semantic basis the generated code is
verified against, the assurance argument, the artifacts under configuration control, the
refusals, the host interface and the stages — so the work can be reviewed before code is
written and each stage judged complete on its own.

Nothing in this note is implemented. The [native compilation record](../../project/native-compilation.md)
describes the calc compiler that exists; this note describes a second backend over the same
front end and why the first cannot simply be extended.

## The problem this answers

Two things exist today. `sysml -compile` translates a calc to C or Go and proves the result
against the interpreter with a differential test; and the roadmap's embedded track names six
items, none started. Between them is a gap that the software class widens.

**The calc backend is host C.** Its prelude (`internal/core/codegen/emit_c.go`) is written for
a compiler on a workstation: `setjmp`/`longjmp` turn a failed evaluation into a status,
`__builtin_add_overflow` and friends check the Integer arithmetic, `unsigned __int128` computes
the once-rounded Integer quotient, an arena allocator (`sysml_arena`, `malloc`-backed) holds
collections, `libm` supplies the transcendental functions, and the program is built with
`-O3 -flto -std=gnu11` and linked with `-lm` (`build.go`, `CFlags`). Each of those is the right
choice for a fast host program and the wrong one for flight code: they are non-portable
extensions, dynamic memory, non-local control flow and an unqualified math library. Function
values are monomorphized into one function body per binding and sequence operands are ordered
through GNU statement expressions — correct, and hard to read, review and measure for
structural coverage. This backend stays as it is; the embedded backend is a second emitter over
the same typed IR.

**The interpreter cannot be shrunk.** `internal/core/runtime` depends on maps, allocation,
`math/big` and the parser and semantic packages, and the lowered graphs it consumes are keyed by
the syntax tree: `lower.ActionGraph` holds `Nodes []ast.Node`, `Edges map[ast.Node][]ActionEdge`,
`Scopes map[ast.Node]*symbols.Scope`; `lower.StateGraph` resolves vertices and scopes through
`*ast.StateNode` and `*symbols.Scope`. Neither graph can leave the process that parsed the
model, so TinyGo or a trimmed runtime is not a route. The roadmap's first embedded item — a
closed behavior IR — is the answer, and it is also where the native track's action and
state phases begin, so it is not embedded-only work.

**Class A changes the argument, not only the target.** For a lower class it is defensible to
compile a model, verify the generated C as if it were hand-written and treat the generator as a
convenience. At Class A the generated code is flight code whose failure can cost a crew, and
two things stop being adequate: an unqualified tool whose output is trusted, and a semantic
oracle that is itself an unqualified program. Today every gate on the native track reads
"agrees with the interpreter". That remains valuable evidence and stops being the requirement.

## The standards this is written against

The project's software development plan and software assurance plan are authoritative; this
note names the standards a NASA Class A project is ordinarily held to so that its artifacts can
be mapped onto whichever of them the plans invoke, and so that nothing here contradicts them.

- **NPR 7150.2**, NASA Software Engineering Requirements, which defines the software classes
  and the engineering requirements that scale with them, and **NASA-STD-8739.8**, the software
  assurance and software safety standard that accompanies it. Class A software is the human-rated
  class; **NPR 8705.2**, the human-rating requirements, apply on top of it for a crewed system.
- **The JPL institutional coding standard for C** and the ten rules it distills ("The Power of
  Ten"): simple control flow — no `goto`, no `setjmp`/`longjmp`, no recursion; a fixed upper
  bound on every loop; no dynamic memory after initialization; short functions; a high density
  of assertions; data in the smallest scope; every return value and parameter checked; a
  restricted preprocessor; restricted pointer use and no function pointers; compilation with
  every warning enabled and static analysis clean. **MISRA C:2012** where the project adopts it.
- **DO-330**, Software Tool Qualification Considerations, borrowed as the *model* for assuring a
  code generator even though it is an airborne standard: a tool whose output is not itself
  verified needs the highest qualification level; a tool whose output is fully verified needs
  less. This note takes both sides — the output is verified as if hand-written *and* the tool is
  assured — because a Class A project should not have to choose.
- **Independent verification and validation**, where the project's plan assigns it. The tool
  cannot satisfy that requirement; it can make the IV&V agent's work possible, which is what the
  traceability and artifact sections below are for.

Two consequences follow immediately. The requirements the generated code is verified against
must be stated independently of the interpreter (next section). And the generated C must be
verifiable by a reviewer and a static analyzer with no knowledge of OpenSysML: readable,
traceable to the model, structurally coverable to the modified condition/decision criterion, and
within a coding standard's subset.

## The semantic basis

The requirement basis is the specification text, not the interpreter:

1. **The OMG texts**: the SysML v2 and KerML specifications and the Kernel Semantic Library, as
   [the behavior semantic oracle](../../project/behavior-semantic-oracle.md) reads them. The
   oracle already separates what the library fixes from what a scheduling policy chose; that
   separation is exactly the line between a requirement and an implementation decision.
2. **A written semantics for the closed IR**, a new document delivered with the IR
   (`docs/internals/design/behavior-ir-semantics.md`, proposed): every node kind, edge kind and
   expression form with its evaluation rule, its error condition and the oracle section it
   derives from. This is the tool operational requirements document in DO-330 terms and the
   requirements document the generated C is verified against.
3. **The execution conformance corpus** (`internal/core/runtime/testdata/conformance/`), each case
   traced to the oracle section it exercises, as the test cases for both the interpreter and
   the compiled C. The cases that list several admissible `outcomes` are excluded from the
   embedded profile by construction (see [Determinism](#determinism-and-boundedness-are-refusals)).

The interpreter's role becomes what it should be: a second implementation of the same written
semantics, whose agreement with the compiled C on the corpus is evidence for both. The
spec-compliance map (`docs/project/spec-compliance.md`) gains a "compiled, embedded" status
column per rule as stages land, in the same four-valued vocabulary it uses today.

## The closed behavior IR

The roadmap's first embedded item, restated as data shapes so that it can be reviewed.

**Closed** means every reference is an index into a table of the same artifact and no field is
a pointer into the syntax tree, a symbol scope or a Go value: nodes, successions, guards,
triggers, effects, states, regions, transitions, pins, features, connections and event types,
each a record in a table, each expression a tree in the typed IR `internal/core/codegen/ir.go`
already defines (`Func`, `Param`, `IntLit`, `Var`, `Binary`, `Call`, `ToReal`, `Declare`,
`Assign`, `If`, `While`, `Return`, …) widened as the native track's value phase needs. The
scheduling rules the interpreter applies are not in the IR; they are in the semantics document,
and the embedded backend applies the one the profile fixes.

**Serializable** means the IR has a canonical text form — deterministic field order, indices
rather than names for references, names kept alongside for the reader — that two runs of the
compiler over one model produce byte for byte, that a diff of two model versions can be read
at the IR level, and that can be handed to a backend in another process. The form is versioned;
a backend refuses a version it does not know.

**Lossless** is proved the same way the calc compiler is: the interpreter gains an entry that
executes *from* the serialized IR rather than from the lowered graphs, and the whole state and
action conformance corpus, with its golden traces, passes through that entry. A construct the
lowering cannot express in the IR is a typed `Unsupported` in the IR (the lowering already has
the shape, `lower.Unsupported`), and the interpreter running from the IR fails on it where the
interpreter running from the graphs would have run — that is the honest signal that the IR is
incomplete, and the exit criterion is that no conformance case hits it.

**A configuration item.** Once a model is compiled for flight, its IR is the artifact the
generated C is traced to and the artifact a re-generation is compared against. It is written
beside the C, and the trace map (below) refers to its indices.

## The freestanding C profile

The embedded backend emits one translation unit and one header per compiled model, plus one
prelude shared by every model, in a subset of ISO C99 (C11 where the project's compiler
supports it) with the following rules. Each rule states how the backend satisfies it: by how it
emits, or by refusing the model.

| Rule | How it is met |
|---|---|
| Freestanding: only `<stdint.h>`, `<stdbool.h>`, `<stddef.h>`, `<limits.h>`; no `<stdio.h>`, `<stdlib.h>`, `<string.h>`, `<math.h>`, `<setjmp.h>` | Emission. The host prelude's includes are not used. |
| No dynamic memory, ever — not only after initialization | Emission. Tables are `static const`; the state vector, the event queues and the token counts are `static` with sizes fixed from the model and its metadata; there is no allocator in the prelude. |
| No recursion, direct or mutual | Refusal. The compiler already follows every invocation; the embedded profile requires the call graph to be acyclic and refuses the cycle by name. Function values are refused (their monomorphization is what makes the host emission unreadable). |
| Every loop has a static upper bound | Refusal. `for v in s` is bounded by `s`'s declared multiplicity upper bound, which must be finite; `while` and `loop … until` need a bound the compiler can read — a literal or an attribute of finite multiplicity iterated over — and refuse otherwise. The interpreter's run-time step budget has no counterpart: a tick's work is bounded by construction, and the bound is in the resource report. |
| No `goto`, `setjmp`/`longjmp`, no non-local exits | Emission. An evaluation failure is a status code returned by every generated function and checked by every caller; the entry points return it to the host. |
| No function pointers | Emission. Dispatch on state × trigger is a `switch`; effects and guards are named functions called directly. |
| No compiler extensions | Emission. Overflow is checked in ISO C before the operation (`b > 0 ? a > INT64_MAX - b : a < INT64_MIN - b`); the once-rounded Integer quotient is refused in the embedded profile until a verified 64-by-64 exact division is written without `__int128`, and Integer `/` refuses in the meantime (Integer `%` and the comparisons do not). No statement expressions, no `__builtin_expect`. |
| Fixed-width types only | Emission. `Integer` is `int64_t` as the interpreter's is, so that the two agree bit for bit; `Boolean` is `bool`; enumerations are `uint8_t` with a `static const` name table for the trace. The embedded library (last stage) adds narrower declared types. |
| No floating point unless the target declares an FPU | Refusal. `Real` and every function over it refuse until the model's `EmbeddedTarget` metadata declares a binary64 FPU; then `+ - * /` and comparison compile with the interpreter's finite-only rule (checked by bit inspection, not `isfinite`), and the transcendental functions still refuse — there is no qualified `libm`. |
| No strings, no I/O | Refusal for `String` values; the trace is a fixed-width binary record the host reads, not text. |
| Structurally coverable to MC/DC | Emission. One decision per `if`; a compound guard is emitted as a sequence of single-condition tests into a local, never as a nested conditional expression; `and`/`or` keep their short-circuit semantics (a right operand's failure is not raised when the left decides) by emitting the second condition inside the first's branch. No conditional expression (`?:`) with a compound condition. |
| Readable and reviewable | Emission. Generated names are the model's qualified names with `::` as `_`, uniquified by a numeric suffix only on collision; every function and table carries a comment naming the model element and its IR index; functions are short — one per guard, effect, entry, exit, node body and calc. |
| Every parameter and return value checked | Emission. Each generated function begins with the checks its IR node carries (multiplicity, `Natural`/`Positive` range, enumeration range) and returns a status on failure; assertions are `SYSML_ASSERT`, a macro the host maps to its own assertion or fault handler. |
| Restricted preprocessor | Emission. The generated code uses `#include` of its own header and the prelude, `SYSML_ASSERT`, and the size constants; no conditional compilation, no token pasting, no function-like macros beyond the assertion. |
| Compiles clean at the highest warning level | Emission and gate. `-std=c99 -pedantic -Wall -Wextra -Wconversion -Wshadow -Werror` on the host compiler in CI; the project's flight compiler is the user's. |

**The prelude.** One file, versioned with the generator, containing the checked `int64_t`
arithmetic, the enumeration and multiplicity checks, the fixed-size queue operations (the event,
outgoing-event and trace queues share them) and the trace record writer, and nothing else — a few
hundred lines that are verified once, to the project's Class A standard, like any other flight
library. Every compiled model links the same prelude version, recorded in its resource report. Adding to the prelude is a change to a
verified library and is treated as such.

**The host backend is unchanged.** `sysml -compile` keeps its GNU-C prelude and its speed; the
embedded profile is selected explicitly (`-compile -target c-embedded`, name to be settled) and
its refusals are its own. A calc that compiles under both profiles produces two different C
programs from one IR, and the differential test runs both.

## Determinism and boundedness are refusals

The interpreter records six kinds of choice point — the order tokens act in a step, which
holding guard a decision follows, which of several writes to one feature stands, which of several
enabled transitions fires, the order sibling regions react, the order executors due at one
instant run (`internal/core/runtime/choice.go`, described in [scheduling](scheduling.md)) — and
lets a policy resolve them. A flight artifact cannot carry an open choice. The embedded profile
therefore holds three rules:

1. **A model with an admissible choice is refused, statically.** The compiler applies the
   oracle's criteria to the IR, one per choice kind. *Token order*: the runtime records the
   choice whenever several tokens can step in one step, not only when they collide, so a fork,
   and any other node that leaves several tokens live, is admitted only when the compiler proves
   the concurrent branches commute — each branch's write set disjoint from every other branch's
   read and write sets, and at most one of them sending, posting an outgoing event or accepting —
   and refused otherwise. *Decision branch*: the guards must be provably exclusive (the last guard
   `else`, or the guards a partition the compiler can read, such as comparisons of one enumeration
   against distinct literals). *Write order*: two writes to one feature in one step refuse
   (subsumed by the commutation rule for forks, stated separately for regions). *Transition*: a
   state with two transitions enabled by one trigger without exclusive guards refuses. *Region
   order*: sibling regions reacting to one event are admitted under the same commutation rule as
   fork branches. *Due order*: two executors sharing a due instant refuse. The refusal names the
   elements and the oracle case that makes the choice admissible. The modeller resolves it in the
   model — sequencing the branches, making the guards a partition — and the fact that they did is
   visible in the model, where a reviewer reads it. The diagnostic trace records the order the
   profile ran commuting branches in; that order is fixed by rule 2 and is not an observable of
   the model.
2. **What remains is declaration order**, the policy the interpreter calls `declared`, stated in
   the semantics document as the profile's rule, so a model that passes rule 1 has exactly one
   execution and the interpreter under `declared` computes it.
3. **Every resource is bounded from the model.** Event queue depth, deferred-event depth, token
   counts, the outgoing-event queue depth and sequence lengths come from multiplicities and from
   the `EmbeddedTarget` metadata, and each has a defined behavior at its limit: a full event queue
   refuses the posted event and returns a status the host sees, counts the refusal, and never
   overwrites; a full outgoing queue fails the `send` with a status the model's step returns to
   the host — silent loss is the one behavior that is never chosen. The refusals the roadmap
   names — unbounded multiplicity, `all T`, dynamic `new`, recursion — stand, with the loop
   bounds and the profile's own from the table above.

Time is a fixed step. `tick(dt)` advances the model's clock by the step the metadata declares, in
integer microseconds; `accept after d` compiles to a down-counter in steps and refuses a `d` that
is not a whole number of steps; `accept at t` refuses unless `t` is relative to the model's own
start. The interpreter's shared clock (`Context.Advance`) is the semantics; the embedded profile is
its restriction to one step size.

## Artifacts and traceability

Compiling a model for the embedded target writes one directory, and every file in it is a
configuration item:

| Artifact | Content |
|---|---|
| `<model>.ir.json` | the closed IR, canonical form, with the IR version |
| `<model>.c`, `<model>.h` | the generated translation unit and its host header |
| `sysml_embedded.c`, `sysml_embedded.h` | the prelude, at its version |
| `<model>.trace.json` | the trace map: for every generated function and table, the IR index, the model element's qualified name, its identity (the `IdentityMetadata` annotation when the model carries one, see [element identity annotations](../../project/element-identity-annotations.md)), and its source span; and for every C line, the IR node it implements |
| `<model>.resources.json` | the resource report, computed by the generator from the IR and the emitted C alone: bytes of `static const` tables, bytes of `static` state (state vector, queues, token counts), the maximum call depth and a stack bound per entry point computed from the acyclic call graph and a per-function frame estimate, the number of generated functions and decisions, the prelude version, and the generator version |
| `<model>.refusals.json` | empty on success; otherwise every construct refused, by element and rule, so a build log is not the only record |

Everything in that directory is a function of the model text and the generator version only. The
figures that depend on a C compiler — code size, the measured stack frames, the toolchain
identity — are not the generator's to state; they come out of the project's own build, with the
project's flight compiler, into a separate `<model>.build.json` that the build step writes beside
the directory. The generator's frame estimate is an input to that measurement, not a substitute.

A **budget file** (`embedded-budget.json`, checked in beside the model) states the limits both
reports must fit — RAM and the estimated stack per entry point, checked by the generator; code
size and measured stack, checked by the build step against `<model>.build.json` — and exceeding
one is a build error, not a warning.

**Reproducibility.** Two compilations of one model text by one generator version produce a
byte-identical directory, on any host: the claim covers the generator-owned artifacts above and
nothing a C compiler touched. The CI gate compiles each fixture twice on one runner and once on
each other operating system the workflows already run on (Linux and Windows today), and diffs
all of them. The generator binaries gain `-trimpath` in the release build (they
are built without it today), the release job records the Go and C toolchain versions, and the
release's checksum manifest is already signed
([releasing](../../project/releasing.md#the-signed-checksum-manifest)), which is what lets a
project pin the generator it qualified.

**Traceability runs both ways.** From a requirement (an oracle section) to the IR rule to the
generated function that implements it, through the trace map; and from a line of C back to the
model element and its identity, for the reviewer and the analyzer's finding. A `#line` directive
is not used — it would point a debugger at the model text and hide the C from the analyzer — the
map is a separate file and the comments are in the C.

## Assuring the generator

What the generator must show, and how each is produced, so that a project can build its tool
qualification argument from repository artifacts rather than from a description of them.

1. **Tool operational requirements.** The IR semantics document and the C profile above, each
   rule numbered, each with its refusal or emission behavior.
2. **Requirements-based tests.** Every rule in the semantics document has at least one
   conformance case that exercises it, and the trace from rule to case is a table in the
   document, checked by a test that fails when a rule has no case.
3. **The differential.** Every state and action conformance case, and every calc case the
   profile admits, is compiled by the embedded backend, built with the host compiler under the
   profile's flags, run on the host with the trace read back as records, and compared to the
   interpreter's golden trace under `declared`; then the same C, unchanged, is built for
   `qemu_cortex_m3` and run under QEMU with the trace read over the serial console, and compared
   again. The second run is what makes "runs on the target" a tested claim rather than an
   inference from the first.
4. **Refusal tests.** Every refusal rule has a fixture that triggers it and a test that pins the
   message and the element it names. A refusal that stops firing is a regression.
5. **Structural coverage of the generator.** `internal/core/codegen`'s tests today run from
   `internal/repl`, so the package's own coverage reads 1.9% — accurate for the repository and
   useless to an auditor. The embedded backend's tests live in its own package, and the
   coverage report for that package is one of the artifacts.
6. **Structural coverage of the output.** The differential builds the generated C with `gcov`
   instrumentation and reports statement, branch and MC/DC-measurable decision coverage per
   fixture; the point is not to reach 100% on the corpus but to demonstrate that the emission
   style *permits* the measurement, which the project's own MC/DC tooling will then take.
7. **Static analysis of the output.** CI runs an open analyzer with a MISRA C:2012 rule set
   (`cppcheck --addon=misra` is the candidate; the project's licensed analyzer is the user's)
   over every fixture's C and fails on a new finding; the accepted findings, if any, are listed
   with a justification per rule in the semantics document.
8. **Known limitations, stated.** Everything the profile refuses is documented as refused, in the
   profile and in the spec-compliance map's embedded column. Nothing compiles approximately;
   that is the native track's third rule and it is load-bearing here.

What this does not do is qualify the generator. Qualification is a determination the project's
assurance organization makes against its plan, with its own review of these artifacts; the
repository's job is that every artifact the determination needs exists, is generated rather
than written, and is reproducible.

## The host interface

The generated header declares, for a model `M`:

```c
sysml_status M_init(void);                                  /* zero the state vector, take initial transitions */
sysml_status M_tick(void);                                  /* advance one declared step; run everything due */
sysml_status M_post(M_event_id id, const M_event *payload); /* enqueue; SYSML_QUEUE_FULL if refused */
sysml_status M_read(M_feature_id id, M_value *out);         /* copy one feature's current value */
sysml_status M_write(M_feature_id id, const M_value *in);   /* set one `in` feature; refused for others */
sysml_status M_state(M_region_id id, M_state_id *out);      /* the active state of one region */
sysml_status M_outgoing(M_outgoing_event *out);             /* next event sent out of the model; SYSML_QUEUE_EMPTY when none */
sysml_status M_trace(M_trace_record *out);                  /* next diagnostic record; SYSML_TRACE_EMPTY when none */
```

No callback into the host exists except the assertion macro; a model that `send`s outside
itself has the send queued as an outgoing event the host drains through `M_outgoing` after each
`M_tick` and routes, which is where an F´ port or a Zephyr message queue attaches. That queue is
part of every build; the diagnostic trace is a separate queue behind `M_trace`, and a build
without the trace writer loses nothing the model does. Every function is re-entrant with
respect to nothing — the host calls them from one task, or serializes; the header says so.
This is the restriction of the C ABI the roadmap plans for host tools and the C client
([a C client, and the C ABI](../../project/roadmap.md#i4--a-c-client-and-the-c-abi)) —
the same header shape, with no allocation and no callbacks — and it is designed with that item
so that the three consumers share one header.

Two proofs, in order. **Zephyr on QEMU**, as the roadmap states: one state machine and one action
from the conformance corpus, linked into a Zephyr application, run under `qemu_cortex_m3` in a
CI container with the Zephyr SDK, the trace read back over the serial console and compared with
the interpreter's golden. **An F´ component** wrapping the same header — `M_post` behind an input
port, `M_tick` behind the rate group, `M_outgoing` drained to output ports and `M_trace` to F´
events — run in F´'s
Linux reference deployment first and on a maintainer's board second, because F´ is the framework a
JPL flight project is likeliest to host this in. The first is the CI gate; the second is the
demonstration.

## Stages

Each stage is a pull request or a small series against `develop`, with its own tests and its
own exit criterion. Size is relative to stage 0 and covers work in this repository only; the
external costs are named at the end.

| Stage | Delivers | Exit criterion | Size |
|---|---|---|---|
| **0 — this record** | the design, reviewed | merged | 1 |
| **1 — the closed IR** | `internal/core/bir` (name to be settled): the tables, the canonical serialization, the lowering from `ActionGraph`/`StateGraph`/`CalcBody`, the interpreter entry that runs from it, and `behavior-ir-semantics.md` with the rule → case trace table | every state and action conformance case and golden trace passes through the IR entry; the serialization is byte-stable; the trace table has no rule without a case | 2 |
| **2 — the prelude and the embedded emitter** | `sysml_embedded.{c,h}`; the emitter over static tables in the profile above; the refusals; the host-side differential and the `gcov` and analyzer gates | the differential passes over every admitted case on the host; every refusal has a fixture; the C compiles clean under the profile's flags | 3 |
| **3 — resources and budget** | the resource report, the budget file, the trace map, reproducibility gate | the same-host and cross-host compilations diff empty; a fixture over budget fails the build; the trace map covers every generated line | 1 |
| **4 — the host interface** | the header above, designed with the C ABI item; `M_write`, `M_outgoing`, `M_trace` | the differential drives fixtures through the header alone | 1 |
| **5 — the proofs** | the Zephyr application and CI job under QEMU; the F´ component and its Linux run | the QEMU trace equals the golden for both fixtures in CI; the F´ deployment runs the same fixtures on Linux | 2 |
| **6 — the embedded library** | `EmbeddedTarget` metadata (MCU, step, queue depths, FPU, budget reference) and a library package of fixed-width numeric types and bounded collections the compiler recognizes | the passes validate the metadata; `Real` compiles only under a declared FPU; a narrower declared type is emitted as such | 1 |

Stages 1 and 2 carry the risk and should not be split further than their exit criteria allow; a
stage whose differential loses a case is redesigned, not merged with a known failure.

**External costs, not in the table:** the project's static analyzer and MC/DC tool on the
generated C, the verification of the prelude to the project's standard, the IV&V review, and
the hardware run. The repository produces what each consumes.

## What this is not

- **Not a real-time guarantee.** The generated code is bounded and reproducible; worst-case
  execution time is a property of the target, the compiler, the cache and the RTOS configuration,
  and the documentation says so rather than implying it. The stack bound in the resource report
  is computed from the call graph and frame estimates and is an input to the project's own timing
  and stack analysis, not a substitute.
- **Not a qualification.** See [Assuring the generator](#assuring-the-generator).
- **Not a replacement for the host backend.** The calc compiler keeps its GNU-C prelude, its
  arena and its speed; a model that needs `Real` transcendental functions, function values or
  the interpreter's step budget uses it or the interpreter.
- **Not the whole language.** Parts, ports, connections, requirements and documents compile in
  the native track's own phases; the embedded profile takes each as that phase lands and
  restricts it. The first flight-relevant artifact is a state machine with the actions and calcs
  it reaches, which is stage 2's scope.
- **Not a decision about floating point.** The profile refuses `Real` until a target declares an
  FPU, and refuses the transcendental functions after that. Whether a project accepts binary64
  arithmetic, fixed point, or a qualified math library is the project's, and the metadata is
  where it is recorded.

## Open questions for the assurance plan

Decisions this note cannot make and the stages need answered before stage 2:

1. **The binding coding standard** — the JPL C standard, MISRA C:2012 with a deviation
   register, or both — which fixes the analyzer rule set in the CI gate and the deviation
   justifications in the semantics document.
2. **Integer width.** `int64_t` keeps bit-for-bit agreement with the interpreter on a 32-bit
   core at the cost of software 64-bit arithmetic; a profile that emits `int32_t` for a declared
   narrower type is stage 6, but whether `Integer` itself may be narrowed needs a policy.
3. **Queue-full behavior.** The profile refuses the event and reports it; an alternative is to
   treat a full queue as a fault that halts the model. Both are deterministic; the plan decides.
4. **Who verifies the prelude**, and to which standard, since it is linked into every model.
5. **Whether the diagnostic trace is required in flight builds** or only in the differential
   and the QEMU proof; a flight build without the trace writer is smaller, keeps `M_outgoing`,
   and the resource report should state both figures.
