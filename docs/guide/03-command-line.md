# 3. From the command line

Every check the REPL performs is also available as a command-line flag, so you can check a model
from a script, a Makefile or a continuous integration job without a prompt. This
chapter explains the workflow; every flag and exit status is listed in
[reference/cli.md](../reference/cli.md).

The examples below use the following model, `checks.sysml`:

```sysml
package MyModel {
    part def Sensor {
        attribute reading default = 0.0;
        attribute threshold = 100.0;
    }

    requirement def ReadingRequirement {
        subject sensor : Sensor;
        require constraint {
            sensor.reading <= sensor.threshold
        }
    }
    requirement healthy : ReadingRequirement;

    part hot : Sensor {
        attribute :>> reading = 140.0;
        constraint inRange { reading <= threshold }
    }
    part cold : Sensor {
        attribute :>> reading = 20.0;
        constraint inRange { reading <= threshold }
    }

    part checks {
        assert satisfy healthy by cold;
        assert satisfy healthy by hot;
    }

    calc def Margin {
        in reading;
        in threshold;
        threshold - reading
    }

    action calibrate {
        attribute offset = 0.0;
        first start;
        then action adjust {
            assign offset := offset + 1.5;
        }
        then done;
    }

    state Monitor {
        entry;
        then off;
        state off;
        state warming;
        accept after 10 [SI::s] then running;
        state running;
        transition first off then warming;
    }
}
```

Flags may be combined. `-instantiate` always runs first, whatever order the flags are
written in, so the verdicts that follow apply to the object it created. The object is created
whole — the parts nested in it whose types exhibit or perform behaviors are created and run
with it — so an `-e` expression naming the usage or a feature under it (`-e "ctx.recv.got"`)
reads what that object holds after its behaviors ran, as `%features` shows it:

```bash
$ sysml -instantiate MyModel::cold -constraint MyModel::cold::inRange checks.sysml
✓ package MyModel
✓ Created instance of MyModel::cold
  ID: 1
  Use %features MyModel::cold to inspect
✓ Constraint MyModel::cold::inRange passed (on MyModel::cold ID: 1)
  standing: holds (observed: 1 run under reverse)

$ sysml -satisfy checks.sysml
✓ package MyModel
✓ satisfy healthy by cold holds (on MyModel::cold ID: 1)
  standing: holds (observed: 1 run under reverse)
✗ satisfy healthy by hot fails (on MyModel::hot ID: 2)
  Required condition evaluated to false: sensor.reading <= sensor.threshold
  standing: violated (witnessed: 1 run under reverse)
```

`-self-check` applies the 50 constraints in OpenSysML's `SysMLValidation` library to every
reflectively classified element in the workspace. A clean run reports the number of checked
elements and applications; constraints whose reflective features are not derived are counted
as unevaluated rather than treated as violations. See the [validation-constraint census page](../project/validation-constraints.md).

`-self-check-package <QualifiedName>` adds the `constraint def`s of a package the loaded
files declare — nested packages included — to the same walk; it implies `-self-check` and
is repeatable. A rule is a `constraint def` whose first `in` parameter is typed by a
`SysML::…` or `KerML::…` metaclass, and it applies to every element that metaclass conforms
to; its verdicts carry the constraint's qualified name. See
[Writing self-check rules](../reference/cli.md#writing-self-check-rules) and the worked
example in `examples/self-check-rules/`.

The `standing:` line under each verdict says what the verdict rests on: the claim, the strength
of the evidence and what earned it. One run under the default schedule is *observed* evidence
that a condition holds, and a run that shows it false is a *witnessed* violation. Every check is
a question put to an analysis engine — `run` here; `-engines` lists them and `-engine` picks
one, or `all` to have every engine that covers the question answer it and compare their
answers ([Analysis engines](../reference/cli.md#analysis-engines)). `-engine check` searches
every schedule of an action or a state machine — of several named together, run on one clock
to `-advance` — for a violation, and `-engine smt` decides a property over every
schedule and every value of the inputs the model leaves unbound — or that `-check-input` frees —
with an SMT solver, so its *proved* stands for any input in its declared domain where a run's
*observed* stands for the inputs as written
([Deciding a property over the inputs](../reference/cli.md#deciding-a-property-over-the-inputs)).
`-check-diverge <feature>` asks either checker whether the schedule decides a feature's final
value: `check` searches the schedules for two that end it differently, `smt` asks a solver for
two at once, and a *sensitive* verdict comes with both schedules as witnesses, each replayable
([Deciding whether the schedule decides a feature](../reference/cli.md#deciding-whether-the-schedule-decides-a-feature)).
A model checker or a simulator installed beside OpenSysML joins them through one JSON file in
the directory `OPENSYSML_ENGINES` names; `-engines` lists it without starting it,
`-engines -probe` starts it once to check that it describes itself as its file does, and
`-engine <name>` puts a check to it — its witness replayed by the interpreter before the
verdict stands ([External engines](../reference/external-engines.md)). `-jobs <n>` (or
`OPENSYSML_JOBS`) lets `n` runs of one check go at once — an exploration's linearizations, a sweep's rows, the
engines `all` consults — without changing what is reported: the result is the same at any count
([Running in parallel](../reference/cli.md#running-in-parallel)).

Every check flag is listed in
[reference/cli.md § Command Reference](../reference/cli.md#command-reference). You can check
one constraint or requirement by name, every satisfaction assertion the model
states, a calculation, an action, or a state machine, and add `-json` for a machine-readable
report.

Build steps can gate on the exit status. It follows the same rules for every non-interactive
run, whether that is a check, an evaluation, a conversion or a plain load: `0` when the requested
operation succeeded, `1` when the model answered false, and `2` when nothing was decided. The full
contract is in [reference/cli.md § Exit status](../reference/cli.md#exit-status).

Status 2 is kept separate from status 1 because an undecided check is not evidence against the
model: treat it as a broken check rather than a failing one. A condition that
evaluates to false is the model's own answer; an unresolved name, or a condition over a feature
that has no value, is not:

```bash
$ sysml -constraint MyModel::hot::inRange -instantiate MyModel::hot checks.sysml; echo "exit=$?"
✓ package MyModel
✓ Created instance of MyModel::hot
  ID: 1
  Use %features MyModel::hot to inspect
✗ Constraint MyModel::hot::inRange failed (on MyModel::hot ID: 1)
  Assertion evaluated to false: reading <= threshold
  standing: violated (witnessed: 1 run under reverse)
exit=1

$ sysml -constraint MyModel::nosuch checks.sysml; echo "exit=$?"
✓ package MyModel
sysml: unresolved reference: MyModel::nosuch
exit=2

$ sysml -requirement MyModel::healthy checks.sysml; echo "exit=$?"
✓ package MyModel
? Requirement MyModel::healthy could not be evaluated
  Error: requirement healthy: sensor subject is unbound: bind it (`subject sensor = <element>`), check it on an object, or assert `satisfy healthy by <element>`
  standing: not covered (requirement healthy: sensor subject is unbound: bind it (`subject sensor = <element>`), check it on an object, or assert `satisfy healthy by <element>`)
exit=2
```

The last case shows a requirement that declares a `subject` that nothing binds,
so the requirement has no object to evaluate against. Declare the intended pairing in the model
as `assert satisfy healthy by hot;` and check it with `-satisfy`, which creates the subject
itself. `-requirement` is for a requirement whose conditions stand on their own, or one
carried by a part that `-instantiate` created.

Verdicts go to standard output. Undecided checks and everything else, including diagnostics
and warnings, go to standard error. So
`sysml -satisfy checks.sysml > verdicts.txt` records the results in the file and leaves any
problems visible on the terminal.

## Checking as a gate

`-validate` does not evaluate any of the model's conditions. It loads the model and reports
what analysis found. Run it as a lint step before relying on any verdict.

```bash
$ sysml -validate checks.sysml; echo "exit=$?"
✓ package MyModel
✓ checks.sysml: no errors
exit=0

$ sysml -validate bad.sysml; echo "exit=$?"   # a file with a syntax error in it
bad.sysml:2:45: error[syntax]: expected an expression
    part def Battery { attribute capacity = ; }
                                            ^
sysml: bad.sysml did not analyse cleanly; no check was made
exit=2
```

The bracketed word after the severity is the diagnostic's *code*, the stable name of the rule
that reported it, the same under `-json` and in the editor. `sysml -explain syntax` says what a
code means and how to resolve it; bare `-explain` lists them all, as does the
[diagnostics reference](../reference/diagnostics.md#catalogue). At the prompt, `%explain-code syntax`
answers the same question without leaving the session.

An explicitly named API element-form `.json` file is converted before validation. Diagnostic
positions refer to the converted SysML notation, not to positions in the JSON document.

A single `-` stands for standard input wherever a file name is accepted, so you can pipe a model
in; its diagnostics are reported against `<stdin>`. To read a file that is actually named `-`,
write `./-`. `-convert` needs `-from` for piped input, because a stream has no file
extension to infer the format from.

```bash
$ cat checks.sysml | sysml -validate -
✓ package MyModel
✓ <stdin>: no errors

$ cat checks.sysml | sysml - -convert ttl -from sysml > model.ttl
```

Every check mode is gated the same way, so a model containing an error never reports a
verdict about itself: a condition read from a model the tool could not fully parse would
describe a different model from the one you wrote. Name as many files as the model
spans, in any order. The gate applies to the model as a whole, so a reference from one file to
a declaration in another resolves correctly.

## Strict conformance

OpenSysML accepts several notations of its own that no SysML v2 production admits: the
`choice`, `junction` and `history` pseudostates. These are deprecated spellings of
what the `StateMachines` library states as metadata — `#choice state pick;` and
friends — reported as warnings that name the replacement, so a model that uses them
still analyses cleanly. `-strict` promotes those warnings
to errors, which turns the run into a test of whether the file is conforming SysML v2. It also
promotes the one well-formedness warning the default mode keeps a warning: a namespace whose
memberships are indistinguishable (`Duplicate of other owned member name` and its alias and
inherited wordings, [Duplicate member names](../reference/diagnostics.md#duplicate-member-names)),
which KerML's `validateNamespaceDistinguishibility` forbids. The flag
applies to `-migrate` too: a strict SysML v1 migration writes no extension notation
at all and refers to no OpenSysML library — see [Portable output with `-strict`](../reference/sysml-v1-migration.md#portable-output-with--strict); `-portable` instead appends the OpenSysML library packages a default migration refers to, so the one file loads elsewhere unchanged.

The state machine below uses the `choice` extension so the difference is visible:

```sysml
package M {
    attribute def Alarm;
    state monitor {
        entry; then off;
        state off;
        transition first off accept Alarm then pick;
        choice pick;
        transition first pick if true then warming;
        state warming;
        accept after 10 [SI::s] then done;
    }
}
```

```bash
$ sysml -validate monitor.sysml; echo "exit=$?"
monitor.sysml:7:9: warning[nonstandard-notation]: `choice pick;` is an OpenSysML extension; write `#choice state pick;` (with `private import StateMachines::*;`)
        choice pick;
        ^~~~~~
✓ package M
✓ monitor.sysml: no errors
exit=0

$ sysml -strict -validate monitor.sysml; echo "exit=$?"
monitor.sysml:7:9: error[nonstandard-notation]: `choice pick;` is an OpenSysML extension; write `#choice state pick;` (with `private import StateMachines::*;`)
        choice pick;
        ^~~~~~
sysml: monitor.sysml did not analyse cleanly; no check was made
exit=2
```

A portable model writes the metadata spelling the finding names instead —
`#choice state pick;` for the `choice` above, once `private import StateMachines::*;`
is added. `-strict` does not
change what parses: the same file produces the same tree and the same findings in the same
places. Only their severity changes, and with it the exit status and the tier gate. It is a
portability check, so turn it on when another SysML v2 tool has to read the model and leave
it off otherwise.

The same promotion applies to a namespace that declares one name twice, which the specification
makes ill-formed (KerML §8.3.2.4.5, "All memberships of a Namespace must be distinguishable
from each other") and the default mode reports as a warning so that published corpora still
analyse cleanly:

```bash
$ sysml -strict -validate dup.sysml; echo "exit=$?"
dup.sysml:2:14: error[name-conflict]: Duplicate of other owned member name
    part def A;
             ^
dup.sysml:3:14: error[name-conflict]: Duplicate of other owned member name
    part def A;
             ^
sysml: dup.sysml did not analyse cleanly; no check was made
exit=2
```

Two members whose metaclasses are unrelated — `part def A;` beside `attribute def A;` — are
distinguishable whatever their names and draw nothing in either mode. A collision between two
*imported* names stays a warning under `-strict`: KerML §7.2.5.4 hides both from the importing
namespace rather than leaving it ill-formed, and the warning marks where OpenSysML binds the
first import's member instead. Each finding names the standard notation to use instead, and
[the conformance audit](../reference/grammar/conformance-audit.md) cites the grammar production
each extension is measured against. The same setting is available as `%strict` at the prompt
([4. The REPL](04-repl.md)), as the `sysml.strictConformance` editor setting
([8. Editors](08-editors.md)) and as `strict_conformance=True` from Python
([Python client](../clients/python/index.md)).

## Lints

Four further warnings, `undeclared-signal`, `port-type-mismatch`,
`deferred-keeper-unmarked` and `rounded-real-literal`, are *lints*: the model is valid SysML v2,
but a `when <name>` that no declaration or `send` accounts for, a connection between ports whose
definitions are unrelated, a deferred signal's accept loop written without the `DeferredKeeper`
marker that names it as the keeper, or a decimal literal such as `0.1` that a `Real` feature can
only hold rounded, is almost always a slip or a surprise. `-strict` leaves them warnings, since they are not about notation.
`-disable-lint <code>` switches one off (`%lint <code> off` at the prompt,
`disabledLints` in an editor). `rounded-real-literal` is opt-in, since most decimals written
to a `Real` round: `-enable-lint rounded-real-literal` switches it on (`%lint
rounded-real-literal on`, `enabledLints`). [The diagnostics reference](../reference/diagnostics.md)
states exactly what each reports.

## Working inside a package-manager project

A SysML v2 package manager installs the projects a model uses into a directory inside the
user's project, each in the OMG project interchange layout: a `.project.json` naming the
project (its `name`, `publisher`, `version` and the `usage` entries whose `resource` IRIs
name the projects it uses in turn) beside the model files. The user's own project root has a
`.project.json` of the same shape. A directory holding one is a project, and `sysml` reads
the file to find what the project uses — never a lock file, and never over the network.

With such a project (here one `sysand` manages, whose installed projects live under
`.sysand/lib`):

```text
proj/
  .project.json                 usage: pkg:example/acme/widgets, …
  model/main.sysml              private import Widgets::*;
  .sysand/lib/
    acme-widgets_1.0.0/
      .project.json             publisher acme, name Widgets, usage: pkg:example/acme/gears
      Widgets.sysml
    acme-gears_2.0.0/
      .project.json
      Gears.kerml
```

the installed projects are found automatically:

```console
$ sysml -validate proj                 # the project and every project it uses
$ sysml -validate proj/model/main.sysml  # Widgets, then Gears, found through the usages
$ sysml -validate proj/.sysand/lib/acme-widgets_1.0.0   # Gears found beside it
```

Naming the project directory loads its own files and those of the projects its usages
select, directly or through their own usages, as the package manager's own file listing
would (`sysml -validate "$(sysand sources)"` still works as before). Naming a file keeps
the usual search for the root namespaces it imports, which now also looks in the file's
project and in the projects that uses. An installed project is matched to a usage by its
own `.project.json`, never by its directory name: the resource IRI's last two path segments
are its publisher and name (compared as lower-case slugs, so `Requirements Management`
answers to `…/ode4hera/requirements-management`), and of several installed versions the one
a `versionConstraint` names exactly is taken — or, when the usage carries none, the one a
version written after `@` in its IRI names (`pkg:registry/acme/gears@1.0.0`) — else the
latest. A usage whose IRI spells something else — a repository's path, as a version-control
IRI does (`pkg:git/github.com/MBSE4U/sysmod-sysmlv2` for the project `mbse4u/sysmod`) — is
linked by elimination once every usage has had its say: of the installed projects no usage
claims (another version of a claimed project counts as claimed), those satisfying its
constraint, when they are all one project, at the highest such version. The link is a
warning on the consumer's `.project.json` — one `-warnings-as-errors` judges as any other —
naming the project taken and the publisher/name the resource should end with to name it. When
several projects are left it, the usage selects nothing and the warning names each candidate
instead; a later usage of the same repository takes the project so learnt, at its own version.
The standard library's usages are never linked to anything installed. OpenSysML reads what any package manager that follows KerML 10.3 project interchange
installs (sysand today) under `lib/` or `.sysand/lib` beside `.project.json`; it does not fetch. Those library directories
hold the installed projects, not the project's own files: a walk of the project directory
(`sysml <dir>`, `-bundle`, the editor's workspace scan) passes over them and loads what lies
there only through the usages that select it. The standard library's
`https://www.omg.org/spec/KerML/…/*.kpar` usages are answered by the bundled library.
Hidden directories other than the library directories stay out of the search, and the
diagnostics of the files loaded this way are reported like those of any other file the
search finds.

A fragment on a repository IRI names a project directory within the tagged tree
(`pkg:git/git.example.org/acme/widgets#packages/gears`): that usage is answered by the
sub-package's own installation — `gears` under `acme` — not by the repository's project. Such a
sub-package also lies inside its parent's installed tree, and a directory below an installed
project that holds a `.project.json` of its own is a project of its own: the parent's files stop
at it — when the project is named, when a file in it is named, in the editor's workspace and in
`-bundle`, which leaves the nested tree out of the parent's archive — so the two installations load
`Gears.sysml` once.

Another provider that installs projects elsewhere needs only its directory named. Each of
the following adds a directory to look for installed projects under, besides the well-known
ones in the project; the directory may hold the projects at any depth:

```console
$ sysml -library-root /path/to/store -validate proj     # repeatable, or a PATH-style list
$ OPENSYSML_LIBRARY_ROOTS=/path/to/store sysml -validate proj
```

The language server reads the same variable, takes `sysml-lsp -library-root`, and a client
that sends settings can name the directories with the `libraryRoots` list setting
([LSP extensions](../reference/lsp.md#library-roots-setting)); see
[Editors](08-editors.md#working-inside-a-package-manager-project).

### Project interchange archives

KerML 1.0 §10.3 also defines the zipped form of a project directory: a *project interchange
archive*, extension `.kpar`, holding `.project.json`, a `.meta.json` (the project's index of
root namespaces to files, its creation time and, usually, a checksum per file) and the model
files, possibly in subdirectories. The OMG publishes the standard library as one archive per
library, and a package manager may install a project as an archive rather than unpacking it.
`sysml` reads an archive wherever it reads a project directory, from memory — nothing is
extracted — and names its members under the archive's path in diagnostics:

```console
$ sysml -validate model.sysml vendor/acme-widgets_1.0.0.kpar   # the archive's files, with the model
$ sysml -library-root /path/to/store -validate proj             # archives under the store are installed projects
$ OPENSYSML_LIBRARY_PATH=/path/to/kpar sysml -validate model.sysml   # a directory of the OMG library archives in place of the bundled library
```

A usage in a `.project.json` is answered by an installed archive as by an installed
directory, matched by the `.project.json` inside it. An archive is checked before anything
is read from it: both files must be at its top level (an archive of a folder that holds the
project is reported as such), `name` and `version` must be present, every usage must name a
`resource`, the index and the checksums must name members, and every checksum whose
algorithm this build computes is verified. A malformed archive named on the command line is
refused with the reason; one found under a library root is a warning on its path and the
load goes on without it. Two shortfalls are read through and warned about on the archive's
path rather than refused: an archive holding the project's folder with the manifests inside
it, as the OMG library releases are laid out, is read as the project in that folder; and an
index entry naming no member is passed over for what the members themselves declare.

`-bundle` writes a project directory as an archive and `-unbundle` writes one back:

```console
$ sysml -bundle proj/.sysand/lib/acme-widgets_1.0.0 -o widgets.kpar
wrote widgets.kpar (Widgets 1.0.0, 4 files)
$ sysml -unbundle widgets.kpar -o widgets
wrote widgets (Widgets 1.0.0, 4 files)
```

Without `-o` the archive is named `<name>-<version>.kpar` after the project, and the
directory `<name>-<version>`. The archive's members are sorted and carry a fixed timestamp,
so bundling the same directory twice gives the same bytes. Which files are members is the
project's `.meta.json`'s to say: a directory that has one is archived as the two manifests
plus exactly the files the metadata records — every `checksum` key and every `index` target,
at their recorded paths — so a repository's README, scripts, caches and version-control
metadata stay out unless recorded, and a recorded file missing from the tree is an error
naming it (`.meta.json records LICENSE, which is not in the directory`). The kept `.meta.json`
is verified on the way: a recorded checksum that no longer matches refuses the bundle. A
directory without a `.meta.json` is archived as KerML 10.3 describes a project archive — its
`.project.json` and its model files (`.sysml`, `.kerml`), outside hidden entries, library
directories (`lib/`, `.sysand/lib`) and nested projects — and the `.meta.json` is generated
with the index, a SHA256 checksum per member and a creation time of `SOURCE_DATE_EPOCH` when
that is set; to carry a license or any other file, record it in a `.meta.json` of your own.
An archive another tool wrote re-bundles from its unbundled directory to the files its
`.meta.json` records. `-unbundle` writes into a new or empty directory only, so an archive
never overwrites a file.

## Running behavior

The debuggers have non-interactive forms that run to completion and report the values they
produce. `-calc` takes a call expression, while `-action` and `-state` take the name of the
behavior, optionally followed by the object performing it. A use case definition or usage is an
action, so `-action` runs its body too (`-analysis` does not take one):

```bash
$ sysml -calc "MyModel::Margin(20.0, 100.0)" checks.sysml
✓ package MyModel
✓ MyModel::Margin(20.0, 100.0)
  = [80.0]
  standing: value (observed: 1 run under reverse)

$ sysml -action MyModel::calibrate checks.sysml
✓ package MyModel
✓ Started action executor for "MyModel::calibrate"
  State: Running
  Tokens: 1
✓ Action completed
  Final state: Completed
  Results:
    offset = 1.5
  standing: value (observed: 1 run under reverse)

$ sysml -state MyModel::Monitor -advance 15 checks.sysml
✓ package MyModel
✓ Started state machine executor for "MyModel::Monitor"
  Current state: off
  Time: 0.0
  Events: 1
✓ Advanced to 15.0 (2 event(s) processed)
  Current state: running
  Last event at: 10.0
  Remaining events: 0
  standing: value (observed: 1 run under reverse)
```

A calculation's result and an action's parameters print in brackets because a feature holds a
number of values bounded by its multiplicity (KerML 8.4.4.12.1), which these surfaces report as a
sequence: `[80.0]` is the one-element sequence a result that states no multiplicity holds, and a
result of multiplicity `0..*` prints every element the same way. `-e` on a plain attribute still
prints the scalar it holds. Up to 0.10.x these surfaces unwrapped a one-element result to its
scalar. Why a bare `return` holds a sequence, and the `return : Real[1]` spelling that prints one
value bare, are explained in [What a result line shows](04-repl.md#what-a-result-line-shows).

An analysis case is a calculation performed as an action, so `-analysis` runs it the way
`-calc` invokes a calc and `-requirement` checks a requirement: it names the case, optionally
with arguments in parentheses that bind the case's `in` parameters (positionally or by name),
and optionally an object to run it on, which becomes the case's `subject`. The report lists the
case's `out` and `return` values with their units, then the verdict of its `objective`
(`satisfied`, `not satisfied` with the violated condition, or `undecided` with the reason):

```sysml
package An {
    private import ScalarValues::*;
    part def Ship {
        attribute cost : Real default = 5.0;
        attribute other : Real default = 7.0;
    }
    calc def Sum { in a : Real; in b : Real; return : Real = a + b; }
    analysis def CostAnalysis {
        subject s : Ship;
        in limit : Real = 20.0;
        out total : Real = Sum(s.cost, s.other);
        objective affordable { require constraint { total <= limit } }
    }
    part ship : Ship;
    part barge : Ship { attribute :>> cost = 30.0; }
    analysis shipCost : CostAnalysis { subject s = ship; }
}
```

```bash
$ sysml -analysis An::shipCost analysis.sysml
✓ package An
✓ An::shipCost
  total = [12.0]
  objective affordable: satisfied
  standing: value (observed: 1 run under reverse)

$ sysml -instantiate An::barge -analysis "An::CostAnalysis An::barge" \
    -analysis "An::CostAnalysis(limit = 50.0) An::barge" analysis.sysml; echo "exit=$?"
✓ package An
✓ Created instance of An::barge
  ID: 1
  Use %features An::barge to inspect
✗ An::CostAnalysis on object #1 of "An::barge"
  total = [37.0]
  objective affordable: not satisfied: total <= limit
  standing: value (observed: 1 run under reverse)
✓ An::CostAnalysis(limit = 50.0) on object #1 of "An::barge"
  total = [37.0]
  objective affordable: satisfied
  standing: value (observed: 1 run under reverse)
exit=1

$ sysml -analysis An::CostAnalysis analysis.sysml; echo "exit=$?"
✓ package An
sysml: analysis run failed: analysis An::CostAnalysis: s subject is unbound: bind it (`subject s = <element>`) or run it on an object
  objective affordable: undecided: analysis An::CostAnalysis: s subject is unbound: bind it (`subject s = <element>`) or run it on an object
  standing: not covered (analysis An::CostAnalysis: s subject is unbound: bind it (`subject s = <element>`) or run it on an object)
exit=2
```

A usage that binds its subject (`subject s = ship;`) needs no object; a definition, or a usage
that binds none, needs one, and is refused by name when none is given — the same rule
`-requirement` applies. The object is one `-instantiate` created, named as `-state` names its
performer. A case whose body performs `action` steps sequenced by `then` runs them as an
action does, each later step reading the outputs of the earlier ones; a body stating no
successions runs its steps in declaration order. An objective violated exits with status 1,
one that could not be decided (a value the condition needs is missing) with status 2, and a
case that stops before producing its outputs — an unbound subject or `in` parameter, a step
that fails, a deadlocked body or one that exhausts the step budget — is reported as an error
naming the case. `-e` reads the results of a package-level usage or of one nested in a part
(`-e An::shipCost.total`, `-e An::holder.inner.total`) by running the case once and keeping
what it computed until a value it depends on changes.

A trade study (`TradeStudies::TradeStudy`) is an analysis case and runs the same way: the
library's own expressions score each alternative the subject lists with the case's
`evaluationFunction` and return the first that scores best. The report adds one line per
evaluation, in subject order, marking the alternative selected and any that tied with it
(`evaluationFunction(An::b (object #2)) = 10.0 [selected]`); `-json` carries them as each
check's `evaluations`. See [Trade studies](06-behavior.md#trade-studies).

A parameter can be swept rather than fixed. `-sweep <param>=<from>..<to>[:<step>]` runs the
`-analysis` case or the `-calc` once per value of the range instead of once, each run an
ordinary run with that value bound and every other argument as given, and reports the runs as
a table of the inputs, the outputs, the verdict and the time that run took:

```bash
$ sysml -instantiate An::barge -analysis "An::CostAnalysis An::barge" \
    -sweep "limit=30.0..40.0:5.0" analysis.sysml; echo "exit=$?"
✓ package An
✓ Created instance of An::barge
  ID: 1
  Use %features An::barge to inspect
sweep An::CostAnalysis — 3 run(s)
limit | total | verdict                   | time
------+-------+---------------------------+--------
30.0  | 37.0  | affordable: not satisfied | 0.416ms
35.0  | 37.0  | affordable: not satisfied | 0.019ms
40.0  | 37.0  | affordable: satisfied     | 0.012ms
  standing: table (observed: 3 rows)
exit=1
```

`-samples <n> -seed <s>` draws `n` values for each range instead of running every value of it,
uniformly and in draw order, so a range needs no step; the seed is required and the table
echoes it, and the same seed draws the same values on every platform (the `time` column is the
wall time each run took, so it is the one column two runs of the same table do not share):

```bash
$ sysml -calc "An::Sum(2.0)" -sweep "b=0.0..10.0" -samples 3 -seed 42 analysis.sysml
✓ package An
samples An::Sum — 3 run(s), seed 42
b                  | result             | time
-------------------+--------------------+--------
8.254725069980449  | 10.254725069980449 | 0.035ms
0.4281995136143024 | 2.4281995136143024 | 0.001ms
7.76073049711954   | 9.760730497119539  | 0.000ms
  standing: table (observed: 3 rows)
```

The endpoints and step carry the syntax and the units an argument carries
(`0.0 [SI::m]..10.0 [SI::m]:2.0 [SI::m]`), `<to>` is included where the step lands on it, a
range between Integers with no step steps by one, and a range between Reals with no step is
refused rather than guessed at. Several `-sweep` flags run their cartesian product, the first
flag varying slowest. A run that fails is a row numbering its error, printed in full under the
table, and the runs after it are still made, so a sweep through a singularity reports which
value broke rather than losing the table. A step of zero, a step whose sign never reaches
`<to>`, an endpoint or step that is not a finite number, a unit that does not convert, a parameter the target declares none of, a case's subject,
one the arguments already bind, and a sweep or sample without an `-analysis`/`-calc` are refused:

```bash
$ sysml -calc "An::Sum(2.0, 3.0)" -sweep "b=0.0..10.0" analysis.sysml; echo "exit=$?"
✓ package An
sysml: invalid sweep parameter: b is both an argument of the invocation and swept
exit=2
```

Sampling is uniform over the range: the bundled library states no probability distribution, so
a distribution asked for by name is refused naming what is missing rather than approximated.
`OPENSYSML_MAX_SWEEP_RUNS` bounds how many runs one table may make (1000 by default), counted
before the first run, and `-json` reports the same rows inside the check the sweep ran. The
REPL's [`%sweep` and `%samples`](04-repl.md#command-summary) do the same interactively.

A state machine takes only its initial transition unless `-advance` says how much simulated
time to run for, whichever `-schedule` or `-engine` answers: `explore` and `-engine check`
search that same point, and when the machine leaves a do behavior, an event, a timer or a
change condition pending there, their standing line says the result is `after the initial
transition only` and that `-advance <time>` runs them. `-advance 0` runs the machine up to the present, dispatching whatever is already
due. The `-action` and `-state` behaviors of one invocation run on one simulation clock, so
`-advance` moves them together: an action that `accept after 5 [SI::s]` and a machine that
accepts the signal it then sends both settle under `-advance 5`, and what comes due at the same
instant in two behaviors runs in the order the scheduling policy `-schedule` names picks (the
behavior started last first by default), reported as a choice point. Without `-advance`, an
action runs to completion on its own, moving the clock to each wait as it reaches it; with it, the action runs only as far as that much time
takes it, and one still parked on the clock when the time is up is reported as undecided with
the instant it waits for. `-advance` without an `-action` or `-state` to run is reported as a
mistake rather than silently ignored. An action that stops before completing, whether through
deadlock or by hitting the step budget, is reported as an undecided check (status 2), because it
produced no outputs to evaluate.

The object after the name is one `-instantiate` created, written as `%state` takes it: the
usage's name, a feature path to a part it holds (`-state "Fleet::driver.r"`, or
`-state "Fleet::Rover::modes Fleet::driver.r"`), or the id the report prints (`-state "#2"`).
Naming the machine the object exhibits attaches to its running machine, with a note saying so,
rather than performing it a second time; a definition the object exhibits as several usages is
refused with the usages to name instead. The object must be the *usage*: `-instantiate
Fleet::rover` gives `-state "Fleet::Rover::modes Fleet::rover"` its object, while `-instantiate
Fleet::Rover` creates an object of the definition, which the flag reports as such — `no instance
of the usage "Fleet::rover": object #1 of "Fleet::Rover" is of its definition "Fleet::Rover",
not of the usage — use %instantiate Fleet::rover to create the usage's object, or name
Fleet::Rover to address it` (the prompt's `%instantiate` is the flag's `-instantiate`). A path
that stops short of an object names the segment that failed.

## Machine-readable results

`-json` reports the same run as a single JSON document, so a build step can read structured
data instead of parsing `✓` and `✗` markers. It reports the checks, not the model; use
`-convert` to serialize the model itself.

```bash
$ sysml -satisfy -json checks.sysml; echo "exit=$?"
{
  "status": "fails",
  "exit": 1,
  "checks": [
    {
      "subject": "satisfy healthy by cold",
      "status": "holds",
      "values": null,
      "lines": [
        "✓ satisfy healthy by cold holds (on MyModel::cold ID: 1)",
        "  standing: holds (observed: 1 run under reverse)"
      ],
      "rows": null,
      "plan": {
        "engine": "auto",
        "standing": "holds (observed: 1 run under reverse)",
        "steps": [
          {
            "engine": "run",
            "status": "answered"
          }
        ],
        "workers": 0,
        "warming": 0
      },
      "results": [
        {
          "engine": "run",
          "claim": "holds",
          "strength": "observed",
          "bounds": [
            {
              "name": "steps",
              "limit": 10000000,
              "reached": false
            },
            {
              "name": "elements",
              "limit": 1000000,
              "reached": false
            }
          ],
          "witness": null,
          "standing": "holds (observed: 1 run under reverse)"
        }
      ]
    },
    {
      "subject": "satisfy healthy by hot",
      "status": "fails",
      "values": null,
      "lines": [
        "✗ satisfy healthy by hot fails (on MyModel::hot ID: 2)",
        "  Required condition evaluated to false: sensor.reading \u003c= sensor.threshold",
        "  standing: violated (witnessed: 1 run under reverse)"
      ],
      "rows": null,
      "plan": {
        "engine": "auto",
        "standing": "violated (witnessed: 1 run under reverse)",
        "steps": [
          {
            "engine": "run",
            "status": "answered"
          }
        ],
        "workers": 0,
        "warming": 0
      },
      "results": [
        {
          "engine": "run",
          "claim": "violated",
          "strength": "witnessed",
          "bounds": [
            {
              "name": "steps",
              "limit": 10000000,
              "reached": false
            },
            {
              "name": "elements",
              "limit": 1000000,
              "reached": false
            }
          ],
          "witness": {
            "schedule": "reverse",
            "choices": []
          },
          "reason": "satisfaction satisfy healthy by hot: require condition evaluated to false: sensor.reading \u003c= sensor.threshold",
          "standing": "violated (witnessed: 1 run under reverse)"
        }
      ]
    }
  ],
  "diagnostics": null,
  "output": [
    "✓ package MyModel"
  ],
  "errors": null
}
exit=1
```

`status` is the worst verdict reached, and `exit` is the code the process exits with.
Values produced by a calculation or a state machine appear under `values`. What analysis
found appears under `diagnostics`, covering both the warnings of a model that analyses cleanly
and the errors of one that does not, each with the `file`, `line` and `column` where it
occurs. Anything that prevented a check from being made appears under `errors`. Each check's
`plan` says which engines were asked and what each did (and, under `-jobs`, how many `workers`
it built and the milliseconds of `warming` spent building them), and `results` holds one entry
per engine that answered, with the strength of its evidence, the bounds it ran under and the witness behind
a violation ([Analysis engines](../reference/cli.md#analysis-engines)). The whole document goes
to standard output, so nothing needs to be read from standard error.

## Validating in CI

A build server wants three things of `-validate` that the lines do not give it: a report in a
form it reads, a way to accept the findings a model already has so that only a new one fails
the build, and a say in whether a warning fails it. `-format` picks the form, `-baseline` names
the accepted findings and `-warnings-as-errors` is the say.

`-format sarif` writes the run as [SARIF 2.1.0](https://docs.oasis-open.org/sarif/sarif/v2.1.0/sarif-v2.1.0.html),
the form GitHub code scanning, GitLab and the other static-analysis dashboards read: one run,
the tool's name and version, one rule per diagnostic code, each finding's severity as its
`level`, its file relative to the working directory with its line and column, and the fixes
analysis offers as `fixes`. `-format junit` writes JUnit XML, the form every CI service's test
tab reads: one testsuite per file, one failing testcase per finding, typed by its severity, and
one passing testcase for a file with no finding, and an error testcase under `sysml` for whatever
stopped the run — a bad baseline, a file that could not be read. `-format json` is `-json`. A
document goes to standard output, or to the file `-o` names, which may not be a file the run
read; the summary line and whatever stopped the run keep going to standard error as well,
since a SARIF document has no place for them. SARIF counts columns in UTF-16 code units, as
its readers do, where the lines and the JSON report count bytes.

```bash
$ sysml -validate -format sarif -o sysml.sarif models
sysml: models/m.sysml did not analyse cleanly; no check was made
$ sysml -validate -format junit models > sysml-junit.xml
```

Run the exact binary your release ships, so the report is about the model and not about a
version difference between the laptop and the build server. In GitHub Actions the repository's
own action installs a release with the install scripts, runs `-validate` and uploads the SARIF
to code scanning, where each finding appears on the pull request's changed lines:

```yaml
jobs:
  sysml:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      security-events: write   # upload-sarif
    steps:
      - uses: actions/checkout@v4
      - uses: Open-MBEE/OpenSysML@v0.11.0   # the release tag is the sysml installed
        with:
          path: models
          format: sarif
          upload-sarif: "true"
```

The action's inputs are `path` (what `-validate` loads), `format`, `baseline`,
`warnings-as-errors`, `strict`, `library-path` (`OPENSYSML_LIBRARY_PATH` for the run),
`version` (a release tag, by default the tag the action was referenced at), `upload-sarif` and
`sarif-file`; it fails the job as `sysml` exits. Without the action, the same job is a shell step
that pipes `install.sh` and then calls `sysml -validate -format sarif -o sysml.sarif`, followed
by `github/codeql-action/upload-sarif` with `sarif_file: sysml.sarif`.

A model that validates with findings you have decided to live with still has to fail the build
on a new one. Record what it has today as a baseline — the JSON report, written to a file — and
hand that file back on every run:

```bash
$ sysml -validate -json -o baseline.json models
$ sysml -validate -baseline baseline.json models; echo "exit=$?"
2 known finding(s) suppressed by baseline.json
✓ package M
✓ models: no errors
exit=0
```

A finding is known when the baseline holds one with the same diagnostic code, the same file
(relative to the working directory, as the report writes it) and the same message. Its line and
column are not compared, so an edit above the finding does not make it new; a change to its
message does, as does the same finding in another file. Known findings are left out of the
report and of the exit status, and the summary says how many were; a JSON or SARIF document
carries the count as `suppressed`. The finding the baseline does not know is reported alone and
judged as it would be without a baseline, so the exit status says exactly whether anything new
was found. Re-record the baseline when a known finding is fixed or deliberately accepted.

A baseline accepts what analysis ran through: a warning, or an error of the last tier. An error
that stopped analysis — a syntax error, a reference that does not resolve — is still one when
the baseline knows it: the tiers after it did not run, so nothing can be said about the model,
and the run is undecided (exit status 2, `did not analyse cleanly: an error the baseline knows
stopped analysis before the later tiers ran`) rather than clean. The finding itself stays out of
the report, and the summary still counts it.

`-warnings-as-errors` makes a warning end the run as an error does, exit status 2 — a warning
of the model, or one a check reports, as a materialization the tool had to bound. It changes
the exit status and nothing else: the report keeps each finding's own severity, so a SARIF
`level` stays `warning` and a dashboard still tells the two apart. Rewriting the severity would
misreport what analysis found; the policy belongs to the build, not to the finding.

## Running from a script

`-e` evaluates an expression without entering the prompt, so you can query a model from a
shell:

```bash
$ sysml -e "RdfInteropDemo::Rover::mass" examples/rdf-interop-demo.sysml
✓ package RdfInteropDemo
✓ RdfInteropDemo::Rover::mass
  = 899.0
```

Two things are worth knowing before a pipeline depends on this:

- **Results go to standard output and problems go to standard error.** Evaluated
  values, conversion output and verdict lines are results. A model's diagnostics and warnings,
  a failed evaluation, a file that could not be read, and even the `wrote <file> …` note of a
  successful `-convert -o` are not, so standard output carries the conversion alone.
- **The exit status says whether the model answered the question**: `0` if it did, `1`
  if it answered false, and `2` if it answered nothing. A warning leaves the status at `0`. The
  status codes are documented in
  [reference/cli.md § Exit status](../reference/cli.md#exit-status), along with a continuous
  integration recipe that gates on them.

---

Next: [4. The REPL](04-repl.md).
