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
  "events":     [{"kind": "Execute", "activity": "DecisionJoin"},
                 {"kind": "Fire", "activity": "DecisionJoin", "action": "Value(0)"},
                 {"kind": "Fire", "activity": "DecisionJoin", "action": "Action_A"},
                 {"kind": "Execute", "activity": "Copier"},
                 {"kind": "Output", "activity": "Copier", "parameter": "output", "value": "0"},
                 "…"]
}
```

`parameters` are the activity's parameters as the implementation loaded them (`upper` is a
natural number or `*`). `outputs` hold, per `out` and `inout` parameter, the values the
execution left in it, in the implementation's order — primitives as JSON numbers, booleans or
strings (`Real` and `UnlimitedNatural` as strings, so `*` and the implementation's own
spelling of a real survive), references as the object they point at with its types and feature
values to a bounded depth. `events` are the implementation's trace in order: `Execute` when
an activity (the one under test or one it calls) starts, `Fire` when an action runs, `Output`
when an output parameter receives a value (and `Post` where the implementation reports one
posted to a parameter node; none of the pinned activities does). The `Fire` sequence is one legal schedule — the
implementation's, which is sequential — and is compared **advisorily** only; the outputs are the
oracle.

The record also carries the constructs the classifier reads from the trace directly. fUML fires
an action once per token offered to a multiplicity-1 pin, so an action downstream of a merge
or a decision that passes two tokens fires twice under one `Execute` (`DecisionJoin`'s
`Action_A`, `ForkMergeData`'s `Action_B`, `ForkMerge`'s `Value(0)`); SysML v2 performs the
node once with every delivery, and `ExpectedActivity.Refired` names such actions so the
classifier can file the activity as `differs-by-design` rather than as a failure.

## Reading, classifying, translating and refereeing

The reader and classifier over the pinned models, the translation rules, `cmd/fuml-referee`
and the committed bucket counts are described here as each lands.
