# The fUML reference implementation as an advisory action referee

The action and activity executor has, until now, had no external referee: the
[PSSM referee](pssm-referee.md) covers state machines, the
[pilot execution referee](pilot-execution-referee.md) covers expressions, and actions were
checked only by this repository's own conformance fixtures. This record describes the referee
that closes that gap: the activity tests of ModelDriven's **fUML Reference Implementation**,
translated by rule into SysML v2 textual notation and executed by this runtime, with the
reference implementation's own outputs as the oracle. Like the PSSM referee it is advisory and
opt-in: CI compares committed **bucket counts**, never a pass/fail verdict, and a movement in
any count is adjudicated in the change that moves it.

**What a pass means.** The reference implementation executes UML activities under the fUML
semantics; SysML v2 actions are a different language with a different (though closely related)
semantics, and where the two disagree this runtime follows SysML v2 and the Kernel Semantic
Library. A pass therefore says that, for an activity with a defensible SysML v2 mapping, this
runtime computes the output values the reference implementation computes. It is a second
opinion on the action rows of the
[precise-semantics alignment note](../internals/design/precise-semantics-alignment.md) and a
regression oracle for the constructs both languages define alike; it is never a conformance
statement about SysML v2, and it is not a fUML conformance statement about this runtime either.

## The suite, pinned

| | |
|---|---|
| Source | ModelDriven's fUML Reference Implementation, <https://github.com/ModelDriven/fUML-Reference-Implementation>, release `v1.5.0a` at commit `45e506336d4cd56965d4ad3b684149245f899f3a` |
| Test model | `fUML-Tests.uml`, the Eclipse UML2 XMI model behind the implementation's JUnit activity tests: 43 activities, 42 of them packaged directly in the model and run one by one, one (`ActiveClassBehavior`) the classifier behavior of an active class that runs only when that class is instantiated |
| Exception model | `fUML-Exception-Tests.uml`, 12 activities exercising the `RaiseExceptionAction` and exception handlers that fUML 1.4 added: eight tests, a called behavior, and three behaviors owned by a class. SysML v2 has no exception handler; these are fetched for completeness and classified `not-expressible`, never translated |
| Library | `fUML_Library.xmi`, the foundational model library the tests call into (`WriteLine`, the primitive functions); the implementation itself loads the copy inside its jar, whose digest the record carries as `libraryDigest` |
| Executable | `fuml-1.5.0a.jar`, the implementation, and its fifteen runtime dependencies from Maven Central |
| SHA-256 | `c7d54bf2…e0f0` (test model), `4e831580…2d74b` (exception model), `7e8bae51…e2da8b7` (library), `4e78a194…de1f06e` (jar); in full in `scripts/fuml-pin.sh` |
| Pin | `scripts/fuml-pin.sh` (`FUML_RI_TAG`, `FUML_RI_COMMIT`, the four `*_SHA256` variables, `FUML_DEPS`) |
| Download | `./scripts/download-fuml-suite.sh`, into the git-ignored `build/fuml/`, idempotently; a file already there with its pinned digest is kept (so a suite placed by hand, without a stamp, is verified and stamped rather than fetched), one whose digest is not the pinned one is discarded, and the referee refuses to read a suite whose digest is not the pinned one |

The tag names the release; the commit is what every fetch reads from, because a tag is a
mutable ref; the checksums are what the fetch verifies, because the file behind a URL can
change. The three are changed together.

**Runtime dependencies.** The jar does not bundle its dependencies. They are the runtime scope
of the implementation's `pom.xml` at the pinned commit, as `mvn dependency:build-classpath
-DincludeScope=runtime` resolves it — log4j 1.2, commons-logging, commons-collections,
commons-lang, Xerces, Xalan and its serializer, xml-apis, the StAX API with SJSXP and
stax-utils, and JAXB 3 with its API, core and activation — pinned by Maven Central path and
SHA-256 in `FUML_DEPS`, so a fetch needs neither Maven nor the pom and always builds the same
classpath. `FUML_MAVEN_REPO` names the repository (Maven Central by default; a mirror serves
the same bytes, and the checksums say so). Any file already installed with its pinned digest,
dependency or model or jar, is kept rather than fetched again; `--force` fetches everything.

**Licence and attribution.** The reference implementation, its test models and its library are
copyright Lockheed Martin Corporation and Model Driven Solutions (formerly Data Access
Technologies, Inc.) and are licensed under the **Academic Free License version 3.0**
(<https://opensource.org/licenses/AFL-3.0>), as the repository's `Licensing-Information.txt`
states; the bundled third-party programs it lists carry their own licences (CDDL 1.0 for SJSXP
and JAXB, Apache 2.0 for log4j, commons-logging and Xerces). This repository proceeds as it does
for the PSSM suite: nothing is vendored. The models, jar and dependencies are downloaded at a
pinned commit and checksum into an ignored directory at build time, the models are translated
**in memory**, and only the implementation's computed outputs and event trace (the
expected-record below), bucket counts and per-activity verdicts are committed. No XMI, no
excerpt and no derived `.sysml` model of a fUML test is in the repository; `-keep <dir>` writes
the translated models for debugging only, into a directory the caller names.

## The expected-record

The referee never runs Java. What the reference implementation computes is recorded once, in
**`docs/project/fuml-referee-expected.json`**, and committed; the Go tests and
`cmd/fuml-referee -check` read that record and nothing else. It is regenerated by

```bash
make fuml-expected          # needs a JDK (javac and java on PATH, or JAVA_HOME)
```

which fetches the suite if it is absent, compiles the driver in `scripts/fuml-driver/`
against the pinned jar and dependencies, and runs it over both models. The committed record
is replaced only when every executed activity completes: if one throws or exceeds the
per-activity timeout, the target exits nonzero, leaves the record as it was, and writes the
partial output beside it as `fuml-referee-expected.json.failed` (git-ignored) for diagnosis,
with the failing activities' `error` fields filled in. The record is
byte-stable across runs on one pin: the only nondeterministic content the implementation
emits — object identifiers, which are Java hash codes — is aliased per activity in order of
first appearance (`obj1`, `obj2`, …).

**The driver** (`scripts/fuml-driver/FumlExpected.java`) uses the implementation as a library:
it loads each model with `Fuml.load`, enumerates the `uml:Activity` elements the model file
declares, and for each one packaged directly in the model (the ones the JUnit suite runs)
selects it with `Environment.findElementById` and executes it with
`ExecutionEnvironment.execute`. Selection is **by XMI id, never by name**: the implementation's
name lookup ranges over every named element, and `ForkMerge` resolves to a parameter of
`TestSimpleActivities` before it resolves to the activity. Activities owned by a class or an
action (`ActiveClassBehavior`, the exception model's `C$Impl`, `C_Factory`,
`raiseException$Impl`) run only as part of their owner and are recorded as skipped with that
reason, as are cross-references into the library, which are not declarations. The
implementation reports what it does through `fuml.Debug` as `[event]` lines; the driver
attaches a log4j appender to that logger and captures the lines as structured events instead of
scraping console output. The locus's extent is cleared between activities, as the JUnit suite's
set-up does, so an object created by one test is not read by the next.

**Its shape.** `provenance` names the release tag, the commit, the jar's digest, the
foundational library the jar carries and its digest, and each model with its namespace URI and
digest; `internal/fuml` refuses a record whose provenance is not the current pin's, so a moved
pin cannot be compared against old truth. Each entry of `activities` is one declared activity:

```json
{
  "model": "fUML-Tests.uml",
  "id": "_15_5_1_1a900482_1225499009421_139168_1510",
  "name": "DecisionJoin",
  "executed": true,
  "parameters": [{"name": "output", "direction": "out", "type": "Integer",
                  "lower": 1, "upper": "*", "isOrdered": true, "isUnique": true}],
  "outputs":    [{"parameter": "output",
                  "values": [{"kind": "Integer", "value": 0}, {"kind": "Integer", "value": 1}]}],
  "events":     [{"kind": "Execute", "activity": "DecisionJoin", "id": "_15_5_1_1a900482_1225499009421_139168_1510"},
                 {"kind": "Fire", "activity": "DecisionJoin", "action": "Value(0)", "id": "_15_5_1_…_1531"},
                 {"kind": "Fire", "activity": "DecisionJoin", "action": "Action_A", "id": "_15_5_1_…_1611"},
                 {"kind": "Execute", "activity": "Copier", "id": "_15_5_1_…_826"},
                 {"kind": "Output", "activity": "Copier", "parameter": "output", "value": "0"},
                 {"kind": "Complete", "activity": "Copier", "id": "_15_5_1_…_826"},
                 "…",
                 {"kind": "Complete", "activity": "DecisionJoin", "id": "_15_5_1_1a900482_1225499009421_139168_1510"}]
}
```

`parameters` are the activity's parameters as the implementation loaded them (`upper` is a
natural number or `*`). `outputs` hold, per `out` and `inout` parameter, the values the
execution left in it, in the implementation's order — primitives as JSON numbers, booleans or
strings (`Real` and `UnlimitedNatural` as strings, so `*` and the implementation's own
spelling of a real survive), references as the object they point at with its types and feature
values to a bounded depth. `events` are the implementation's trace in order: `Execute` when
an activity (the one under test or one it calls) starts and `Complete` when that execution
ends, `Fire` when an action runs, `Output` when an output parameter receives a value (and
`Post` where the implementation reports one posted to a parameter node; none of the pinned
activities does). `Execute` and `Complete` nest, so the trace shows which execution each
`Fire` belongs to even when an activity calls itself. The implementation reports elements by
name only; the driver adds the XMI `id` of the activity (`Execute`, `Complete`) or the action
node (`Fire`) where the name identifies exactly one element of the loaded models, and omits it
where it does not — a node name an activity uses twice (the exception model's `Test001` reads `this` at two nodes), or an
action of a library activity such as `WriteLine`. The `Fire` sequence is one legal schedule —
the implementation's, which is sequential — and is compared **advisorily** only; the outputs
are the oracle.

The record also carries the one construct the classifier reads from the trace rather than
from the model. `ExpectedActivity.Refired` names every action the implementation fired more
than once within one execution of its activity — the activity under test or one it calls —
with the activity it belongs to, both XMI ids and the number of fires (`DecisionJoin`'s
`Action_A` twice, `ForkMergeData`'s `Action_B` twice, `ForkMerge`'s `Value(0)` twice,
`TestBooleanFunctions`' `Call(And)` four times). The count is per node `id` and per
`Execute`…`Complete` span: two nodes sharing a name are never mistaken for one node firing
twice (a `Fire` without an `id` counts as nothing), an activity a test calls twice re-fires
nothing by being called twice, and a callee execution — a recursive one included — neither
inherits nor resets its caller's counts. Whether such a re-firing is a design difference is
then the classifier's decision, below.

## Reading the models

`internal/fuml` reads the two test models and the library with the XMI element walker shared
with the PSSM referee (`internal/xmi`). The walker accepts every OMG XMI namespace version:
the models are `20131001`, the library `20110701`, and the two are cross-referenced. The
reader (`ReadModelFile`, `ReadLibraryFile`) accepts a `uml:Model` root or one wrapped in
`xmi:XMI`, and builds an immutable `Model`:

| fUML | `Model` |
|---|---|
| `Activity`, its parameters (direction, type, multiplicity, ordering, uniqueness) and its owner when a class or an action owns it | `Activity`, `Parameter`, `Multiplicity`; `Activity.Owner`, `Class.ClassifierBehavior` |
| Every `ActivityNode` kind fUML defines — control nodes, parameter nodes, buffers, actions, structured nodes — with their `InputPin`s and `OutputPin`s and the nodes a structured node owns | `Node` (`NodeKind`), `Node.Pins`, `Node.Nodes`; `Activity.AllNodes` walks the structured nodes too |
| `ControlFlow` and `ObjectFlow` with `guard` (a literal or an instance value naming the enumeration literal) and `weight` | `Edge` (`EdgeKind`), `Edge.Guard`, `Edge.Weight`; `Node.Incoming`/`Outgoing` index them on the node or pin they touch |
| The behavior a `CallBehaviorAction` calls, the operation a `CallOperationAction` calls or a `CallEvent` names, the signal a `SendSignalAction`/`SignalEvent` names, the feature a structural-feature action reads or writes, the classifier a create/read-extent/reclassify action names | resolved `TypeRef`s: a local element by id, or an **external** reference into the library by `href`, kept with its qualified name (`PrimitiveBehaviors::IntegerFunctions::+`) |
| `Class` with properties, generalizations, operations (and their methods), `Signal`, `Association` and its member ends | `Class`, `Property`, `Operation`, `Signal`, `Association` |
| `ExceptionHandler`, `Trigger` | `Node.Handlers`, `Node.Triggers` |

Every reference the model makes to something it does not declare is a diagnostic on the
`Model`, never a silent gap; the pinned models produce none. The reader is exercised against
the pinned corpus (43 activities with 95 control and 345 object flows, seven classes, three
signals and two associations in the test model; 12 activities in the exception model) and
against a synthetic wrapped model that covers the `20110701` namespace, an external library
reference, guards, weights, an association, an operation call and its accepter.

**Forty-three, not forty-four.** The test model contains 44 `uml:Activity` elements, but one
is the library's `WriteLine` referenced by `href` from `HelloWorld`, not a declaration; the
JUnit suite runs 42 activities directly and `ActiveClassBehavior` through its class. The
reader, the expected-record and the checklist below all count 43.

## Classifying before translating

`Classify(activity, expected)` files each activity into one of three classes from the
constructs it uses and the implementation's trace, before any translation exists, with a
reason that names the construct, why SysML v2 has no spelling for it, and where in the
activity it occurs (`ReadExtentAction (SysML v2 has no classifier extent): ReadExtent(TestClass)`).
The classes decide two of the referee's four buckets outright; the third leaves the verdict to
a run.

| Class | Bucket | Decided by |
|---|---|---|
| **not-expressible** | `not-expressible` | any construct in the construct map's not-expressible rows — `CallOperationAction`, `AcceptCallAction`, `ReplyAction`, `ReadExtentAction`, `ReadIsClassifiedObjectAction`, `TestIdentityAction`, `ReclassifyObjectAction`, `UnmarshallAction`, `ReadLinkAction` and the structural-feature actions on an association end, `CentralBufferNode`, `DataStoreNode`, `DestroyObjectAction`, `RaiseExceptionAction` and `ExceptionHandler`; a library function without a Kernel Function Library counterpart (`BasicInputOutput::WriteLine`, the `UnlimitedNaturalFunctions`) or a parameter or pin typed `UnlimitedNatural`, which `ScalarValues` lacks; the exception model as a whole; and, transitively, a call of or a classifier behavior starting a not-expressible activity, reported as the dependency and its decisive constructs |
| **differs-by-design** | `differs-by-design` | an action the trace fired more than once within one execution of its activity **and** that an object flow feeds — fUML fires it once per token offered to a multiplicity-1 pin; SysML v2 performs the node once with every delivery (`action_node_concurrent_performances`). A merge that delivers two control tokens re-fires its successor in SysML v2 too, so `ForkMerge`'s `Value(0)` stays expressible |
| **expressible** | `pass` or `fail`, by the run | everything else: control and object flow, fork, join, merge and decision with guards, value specifications, calls of activities and of library functions with a counterpart, multi-valued outputs, object creation and feature reads and writes on a class, signals, active classes, `ReadSelfAction`, `StructuredActivityNode` |

A `differs-by-design` classification is the one an alignment row documents; the classifier
finds four such activities in the corpus and lists each re-fired action with its count.
`LibraryCounterpart` maps each of the 46 library functions the test model calls either to its
Kernel Function Library spelling or to a reason; the test pins the seven without one
(`WriteLine` and the six `UnlimitedNaturalFunctions`).

**The checklist.** `TestSuiteClassification` in `internal/fuml/classify_test.go` is the
per-activity checklist: every activity of both models with the class it must receive, the
counts pinned, every non-expressible reason required to name a construct and a location, and
every exception-model activity required to be filed as such. Over the test model:

| Class | Count | Activities |
|---|---|---|
| expressible | 24 | `Copier`, `CopierCaller`, `SimpleDecision`, `ForkJoin`, `ForkMerge`, `NodeEnabler`, `TestNodeEnabler`, `TestIntegerFunctions`, `TestIntegerComparisonFunctions`, `TestRealFunctions`, `TestRealComparisonFunctions`, `TestStringFunctions`, `GenerateBooleanTestData`, `GenerateListTestData`, `TestListFunctions`, `TestGeneralizationAssembly`, `TestClassObjectCreator`, `TestClassWriterReader`, `TestClassAttributeWriter`, `TestClassAttributeValueRemover`, `ActiveClassBehavior`, `ActiveClassBehaviorSender`, `TestSignalReceiver`, `TestSpecializedSignalSend` |
| differs-by-design | 4 | `DecisionJoin` (`Action_A` ×2), `ForkMergeData` (`Action_B` ×2), `TestSimpleActivities` (through both), `TestBooleanFunctions` (`Call(Not)` ×2, `Call(And)`, `Call(Or)`, `Call(Implies)`, `Call(Xor)` ×4) |
| not-expressible | 15 | `HelloWorld` (`WriteLine`), `TestUnlimitedNaturalFunctions`, `TestClassIdentityTester`, `TestClassExtentReader`, `TestClassObjectDestroyer`, `TestCompositeObjectDestroyer`, `TestClassReclassifier`, `TestClassUnmarshaller`, `SelfReader` (`ReadIsClassifiedObjectAction`), `TestAssociationEndWriterReader`, `TestCentralBuffer`, `TestDataStore`, `TestCallAccepter`, `TestCallSender`, `TestCallSend` |

and all 12 activities of the exception model are `not-expressible`. `TestBooleanFunctions`
is in the second class, not the first, because `GenerateBooleanTestData` hands each function
a four-token truth table through multiplicity-1 pins, and `Not` reads `Value(true)` and
`Value(false)` through one pin.

The reader and classifier tests (`TestSuite*`) run only when the suite is present in
`build/fuml/` (`./scripts/download-fuml-suite.sh`). CI downloads and caches it as it does
the PSSM suite, sets `OPENSYSML_REQUIRE_FUML_SUITE=1` so an absent suite fails rather than
skips, and re-runs the suite gates on their own so a skip cannot hide behind a green run. The
expected-record tests need no suite.

## Translating and refereeing

The translation rules, `cmd/fuml-referee` and the committed bucket counts are described here
as they land.
