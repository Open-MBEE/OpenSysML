# 11. Migrating a SysML v1 model

A SysML v1 model exported from its tool as UML XMI, an Eclipse UML2 `.uml` file or a
MagicDraw/Cameo `.mdzip` archive can be read by `sysml -convert` and written as SysML v2 notation
or RDF Turtle. This chapter walks one export through the migration: running it, reading the
report it produces, running a migrated behavior, comparing a migrated run configuration with the
results the tool stored, and finishing by hand what the mapping leaves behind. Every mapping
rule, and the format of the report, is in
[reference/sysml-v1-migration.md](../reference/sysml-v1-migration.md).

Migration is **experimental**: the mapping covers structure, ports and connectors, requirements,
constraints, instances, allocations, user profiles, and behavior — activities, state machines,
operations and receptions, opaque bodies in a bounded JavaScript and English subset, and
simulation run configurations with the results the tool stored for them. It does not migrate
units and quantity kinds, and what it writes for a v1 element may change between releases without
a compatibility path. Every run says so on stderr, so a script sees the status without reading
this page. It is also **one way**: a v2 model has no v1 form, so `-convert xmi` is refused, and
the migrated notation is the place to keep working — not a copy to be re-migrated after editing
the v1 model.

## The example

The model migrated below is the vehicle fixture the migration's own tests use,
[`tests/migrate/testdata/xmi/vehicle.xmi`](../../tests/migrate/testdata/xmi/vehicle.xmi): a
small vehicle design of blocks, value types, ports and connectors, a constraint block, an
instance, a requirements package with satisfy, verify and derive relationships, and — so that the
report has something to say — an activity, a state machine, an operation, a unit and a quantity
kind. Copy it next to you as `Vehicle.xmi` to follow along; your own tool's export works the
same way, whatever its extension:

```bash
sysml Vehicle.xmi   -convert sysml -o Vehicle.sysml    # OMG XMI 2.5.1 with the SysML profile
sysml Vehicle.uml   -convert sysml -o Vehicle.sysml    # Eclipse UML2 / Papyrus
sysml Vehicle.mdzip -convert sysml -o Vehicle.sysml    # MagicDraw / Cameo project archive
sysml export.xml    -convert sysml -o Vehicle.sysml -from xmi   # an extension that does not say
```

The input format is inferred from the extension and named with `-from` when the extension does
not say. Diagrams and the exporting tool's private state are skipped; the model is what is read.

## Running the migration

Ask for the report alongside the notation. Without `-migration-report` only its one-line summary
is printed:

```console
$ sysml Vehicle.xmi -convert sysml -o Vehicle.sysml -migration-report Vehicle.report.txt
note: SysML v1 migration is experimental: the mapping covers structure, ports and connectors, requirements, constraints, instances and allocations, reports every element it approximates or leaves behind, and what it writes for a v1 element may change without a compatibility path; see docs/reference/sysml-v1-migration.md § Status
wrote Vehicle.report.txt (migration report: migrated 92 element(s): 78 mapped, 11 approximated, 3 unmapped (3 skipped as profile, library or notation-only content, 0 as model elements nothing refers to))
wrote Vehicle.sysml (sysml, 5793 bytes)
```

The summary is the first thing to read: 92 v1 elements, of which 78 have a direct v2 form, 11
were written as the nearest v2 construct, and 3 have none. The 3 skipped are the SysML profile
application, the profile itself and a diagram — not model content, so they count against
nothing; the other skipped count is for the model's own elements nothing refers to, such as an
event no trigger names. The command exits 0 when the notation was written, whatever the report
says; it exits non-zero and writes nothing when the input cannot be read as XMI at all, or holds
no model.

A report named `.json` is written as JSON with the same entries, for a script that wants to gate
on the verdicts rather than read them:

```bash
sysml Vehicle.xmi -convert sysml -o Vehicle.sysml -migration-report Vehicle.report.json
```

`-convert ttl` writes the migrated model as RDF Turtle in one step, through the same mapping and
then [the RDF mapping](07-saving-and-rdf.md); the report describes the migration either way.

## Reading the report

The report accounts for every element of the v1 model under one of four verdicts, the ones that
need attention first:

```text
# SysML v1 to v2 migration report: Vehicle.xmi
# exported by Example UML Tool
# migrated 92 element(s): 78 mapped, 11 approximated, 3 unmapped (3 skipped as profile, library or notation-only content, 0 as model elements nothing refers to)

## unmapped (3)
«Verify» Abstraction	Requirements::<Abstraction>	_dep_verify_block	(a verify whose client is not a test case has no v2 form; applied stereotypes «Verify»)
«Unit» InstanceSpecification	Vehicle Design::Value Types::kilogram	_unit_kg	(units and quantity kinds are not migrated; use the SI and ISQ libraries; applied stereotypes «Unit» (quantityKind = Vehicle Design::Value Types::mass; symbol = kg))
«QuantityKind» InstanceSpecification	Vehicle Design::Value Types::mass	_qk_mass	(units and quantity kinds are not migrated; use the SI and ISQ libraries; applied stereotypes «QuantityKind»)

## approximated (11)
«Trace» Abstraction	Requirements::<Abstraction>	_dep_trace	(a trace is written as a plain dependency)
«TestCase» Activity	Requirements::Mass Test	_tc_mass	-> Requirements::'Mass Test'	(the test case's behavior is not migrated; only its verified requirements are)
Actor	Vehicle Design::Driver	_actor_driver	-> 'Vehicle Design'::Driver	(a UML actor is written as a part def)
«FlowPort» Port	Vehicle Design::Engine::speedIn	_port_speedIn	-> 'Vehicle Design'::Engine::speedIn	(a port typed by a DataType is written as a port holding one directed attribute)
…
Property	Vehicle Design::Vehicle::totalMass	_prop_total	-> 'Vehicle Design'::Vehicle::totalMass	(opaque expression copied verbatim (language SysML))

## mapped (78)
…
Operation	Vehicle Design::Vehicle::start	_op_start	-> 'Vehicle Design'::Vehicle::start	(its owner's usage start 2 performs it, as a call on an object does)
…
StateMachine	Vehicle Design::Vehicle::Vehicle States	_sm_vehicle	-> 'Vehicle Design'::Vehicle::'Vehicle States'
```

Each line is the v1 element's kind with its applied stereotypes, its qualified name in the v1
model, its `xmi:id`, the v2 element it became (after `->`) when one was written, and a note on
how, or why not. The columns are tab-separated, so `cut` and `awk` read them.

- **mapped** — a direct v2 form. Nothing to do; a note, when there is one, says how the form was
  chosen (the root model's members are written at the top level; an English constraint body was
  translated to a v2 expression).
- **approximated** — written as the nearest v2 construct, and the note says what was lost. Some
  of these are simply how v2 spells the idea (a UML actor is a `part def`; a flow port typed by a
  data type is a port holding one directed attribute; a `«Trace»` is a `dependency`). Others
  record a v1 tag that v2 has no home for (`isEncapsulated`, a value type's `unit` and
  `quantityKind`), or an opaque expression copied verbatim because its declared language is
  already SysML.
- **unmapped** — nothing was written for the element. The notation holds a comment where it
  would have gone, so the loss is visible in the file, not only in the report. Here these are
  the unit and quantity kind, and a `«Verify»` whose client is a block rather than a test case.
- **skipped** — profile, library and tool-private content, and the model's own elements nothing
  refers to. Never content the v2 model is missing.

The migration keeps the v1 names, quoting them where v2 requires it, and keeps a requirement's
`id` as its short name, so `Mass Requirement` with id `R1` is `requirement def <R1> 'Mass
Requirement'` and a v1 qualified name can be searched for in the v2 file as it stands.

## What the behaviors became

Open `Vehicle.sysml`. The operation `start` and the state machine `Vehicle States` that an
earlier reader might have expected to find as comments are written out — an operation is an
`action def` its owner performs, and a state machine a `state def`:

```sysml
part def Vehicle :> System {
    …
    abstract action def start {
        in key : ScalarValues::Integer;
    }
    action 'start 2' : start;
    state def 'Vehicle States' {
        /* the region has no initial pseudostate: nothing enters it */
        state Off;
    }
    constraint 'positive mass' { mass > 0 }
    …
}
```

The vehicle's behaviors are deliberately empty, so the plant fixture beside it,
[`plant.xmi`](../../tests/migrate/testdata/xmi/plant.xmi), is the one to migrate to see a
behavior that *runs*. Its activity `Fill` has opaque JavaScript actions in swimlanes, and each
becomes an `assign` on the part the swimlane represents, in the `action def` notation of
[chapter 6](06-behavior.md):

```sysml
action def Fill {
    first start then 'open valve';
    action 'open valve' {
        assign this.tank.valve.open := true;
    }
    first 'open valve' then 'fill tank';
    action 'fill tank' {
        assign this.tank.volume := this.tank.volume * 2 + (if this.tank.valve.open ? 1 else 0);
    }
    first 'fill tank' then torn;
    action torn {
        /* body not migrated (the name "volume" resolves to nothing readable: nothing visible from Plant::Fill::torn is called volume; …) {JavaScript}:
         * volume = 0;
         */
    }
    …
    action 'count run' {
        assign this.runs := this.runs + 1;
    }
    …
}
```

A body the [opaque-language subset](../reference/sysml-v1-migration.md#the-opaque-language-subset)
cannot read — here a name that resolves through no swimlane — stays a comment naming the token
it refused, and the report says the same. What was translated runs under the debugger of
chapter 6, on an object of the block the activity belongs to:

```console
$ sysml Plant.xmi -convert sysml -o Plant.sysml
$ printf 'part plant : Plant;\n' > plant_inst.sysml
$ sysml Plant.sysml plant_inst.sysml
sysml> %instantiate plant
✓ Created instance of plant
sysml> %action Plant::Fill plant
✓ Started action executor for "Plant::Fill"
sysml> %continue
✓ Action completed
  Final state: Completed
sysml> %eval plant.runs
✓ plant.runs
  = 1
sysml> %eval plant.tank.volume
✓ plant.tank.volume
  = 4.0
```

A v1 state machine is run the same way with `%state`. What each activity node, transition,
trigger and swimlane becomes, and the calls into the fUML and Alf libraries the migration
translates, is in [Behaviors](../reference/sysml-v1-migration.md#behaviors);
[chapter 6](06-behavior.md#behaviors-migrated-from-sysml-v1) covers the duration constraints and
«Probability» tags that become draws from the model seed.

## Run configurations and the tool's results

A simulation tool's run configuration («SimulationConfig» in MagicDraw/Cameo) becomes an
`action def` that performs the target's classifier behavior on a part typed by the migrated
target instance, with the tool's run count and duration mode as `@Simulation::Configuration`
metadata. `-migration-results` writes the snapshots the tool stored of its own runs beside the
notation, indexed per configuration, and `-compare-results` runs each configuration and sets
OpenSysML's numbers beside the tool's:

```console
$ sysml Analysis.xmi -convert sysml -o Analysis.sysml -migration-report Analysis.report.txt -migration-results Analysis.results.json
wrote Analysis.report.txt (migration report: …)
wrote Analysis.results.json (results of 3 run configuration(s): 2 with 12 stored snapshot(s) standing for 16 run(s))
wrote Analysis.sysml (sysml, 6581 bytes)
$ sysml Analysis.sysml -compare-results Analysis.results.json -seed 1
compare 'Group 0' — 15 stored run(s) over 11 snapshot(s) in Results; 3 run(s) by OpenSysML, draws average, seed 1
observable | source               | runs | min   | mean              | p50   | p90   | max
-----------+----------------------+------+-------+-------------------+-------+-------+------
p          | tool                 | 1    | 0.5   | 0.5               | 0.5   | 0.5   | 0.5
           | OpenSysML (target.p) | 3    | 0.5   | 0.5               | 0.5   | 0.5   | 0.5
           | difference           |      | +0.0% | +0.0%             | +0.0% | +0.0% | +0.0%
…
```

A difference is a fact about the migration's fidelity, to be read against the report's
approximations — a duration the tool drew and OpenSysML draws differently, a clock the tool
stepped (`clockStep` in the sidecar, `-clock-step` to override it) — not something to tune away.
`-runs`, `-draws` and `-observe <stored>=<feature>` adjust the comparison; the full account is
under [Run configurations](../reference/sysml-v1-migration.md#run-configurations) and
[Comparing a migrated configuration with the tool's results](../reference/cli.md#comparing-a-migrated-configuration-with-the-tools-results).

## Finishing by hand

Alongside every element the report calls unmapped or approximated-with-loss is a comment
saying so, at the place in the v2 model where the element belongs:

```sysml
package 'Value Types' {
    attribute def Mass :> ScalarValues::Real {
        /* «ValueType» tags with no v2 form: quantityKind = Vehicle Design::Value Types::mass; unit = Vehicle Design::Value Types::kilogram */
    }
    attribute def Speed :> Mass;
    /* not migrated: «Unit» InstanceSpecification 'kilogram' — units and quantity kinds are not migrated; use the SI and ISQ libraries; applied stereotypes «Unit» (quantityKind = Vehicle Design::Value Types::mass; symbol = kg) */
    /* not migrated: «QuantityKind» InstanceSpecification 'mass' — units and quantity kinds are not migrated; use the SI and ISQ libraries; applied stereotypes «QuantityKind» */
    …
}
```

Each comment is a to-do, and its note says which way to go:

- **Units and quantity kinds.** v2 ships them: import `SI` and `ISQ` and type the attribute by
  the library's quantity, `attribute def Mass :> ISQ::MassValue;` with `mass = 1200 [SI::kg]` as
  its value, in place of the v1 value type that carried a unit tag. The mapping reports rather
  than guesses here, since the v1 model's own `kilogram` may or may not be the SI one.
- **Behavior bodies the subset refused.** A `body not migrated` or `guard not migrated` comment
  quotes the original text and names the token or the name that stopped it; rewrite that
  statement in v2 (an `assign`, an `if` guard) where the comment sits. A transition that lost
  every trigger is written as a comment too, since without them it would fire at once.
- **Relationships to an element with no v2 form.** A `«Verify»` from a block, a trace whose end
  is outside the exported file: the comment names both ends, so the relationship can be
  restated in v2 once the missing end exists (migrate the used project too, then reconnect).
- **Opaque expressions in SysML.** A constraint or default whose declared language is already
  SysML is copied verbatim and reported as an approximation, since it means what v2 says it
  means; read those, and rewrite any that relied on v1 semantics. English and JavaScript bodies
  are translated where the subset reaches, and the report's `translated to v2` notes say so.
- **Tags with no v2 form.** `isEncapsulated`, a stereotype applied from a profile the document
  does not define (`«Critical»` on `Engine` here), a value type's `unit`: keep the comment as
  documentation, or express the intent in v2 terms. A profile the document *does* define is
  migrated to `metadata def`s and applied as metadata, so nothing needs doing for those.

## Checking the result

The migrated notation is an ordinary v2 model from here on: every check in
[chapter 3](03-command-line.md) applies, and the file parses and analyses under the same passes
as a hand-written one. This model's `Vehicle` and its instance `myCar` are intact — the value
`myCar` gives its mass is what the v1 slot held:

```console
$ sysml Vehicle.sysml -eval "'Vehicle Design'::myCar::mass"
Vehicle.sysml:51:29: warning: Duplicate of inherited member name 'start' from Part
        abstract action def start {
                            ^~~~~
Vehicle.sysml:91:9: warning: End feature must have multiplicity 1: an end relates exactly one thing per link; write `[1]` or take it from a feature the end subsets or redefines
        end driver : Driver[0..1];
        ^~~~~~~~~~~~~~~~~~~~~~~~~~
✓ package 'Vehicle Design'
✓ package Requirements
✓ package 'Empty Package'
✓ 'Vehicle Design'::myCar::mass
  = 1350.5
```

Findings on the migrated file are read as on any model, and point at what to revise in the v2
notation — here a v1 name that collides with one the `Parts` library gives every part, and the
association `Drives`, whose v1 end multiplicity `0..1` v2 writes on the end rather than after
its type: `end [0..1] driver : Driver;`. From this point `%save` in the REPL, `-convert ttl`, the
LSP and the clients all take the file as they take any other; nothing remembers that it was
migrated.

## Publishing a migrated document with Cameo-style diagrams

A `.mdzip` carries, beside the model, Cameo's own drawing of every diagram: where each symbol
sits, how large it is, the bends of every line, the colours and font a symbol was given by hand,
and the notes anchored on the diagram. The migration reads that drawing from each diagram's
symbol stream and writes it beside the view it produces as
[DiagramLayout](../project/diagram-layout-annotations.md) metadata — `Layout` boxes, `Route`
waypoints, a `Canvas` the size of the diagram frame, a `Style` for a symbol drawn in its own
colours or font, and a `Note` for every comment and text box — so nothing has to be laid out again.
No export from another tool is needed; an [MTIP](https://github.com/Open-MBEE/mtip-cameo) export
given with `-layout` is still honoured and takes precedence for the diagrams it covers
([the precedence](../reference/sysml-v1-migration.md#layout-from-an-mtip-export)).

The published document then draws those views with Graphviz, in Cameo's look, at the stated
geometry:

```bash
export OPENSYSML_DOT=/usr/bin/dot                 # Graphviz; `dot` on PATH when unset
export OPENSYSML_WEASYPRINT=/usr/local/bin/weasyprint   # the PDF engine; on PATH when unset

sysml Project.mdzip -convert sysml -o Project.sysml -migration-report Project.report.txt
sysml Project.sysml -render-document 'Project::DesignDescription' \
    -doc-form pdf -diagram-form dot -render-style cameo -o DesignDescription.pdf
```

- `-diagram-form dot` writes every graph-shaped figure as Graphviz DOT rather than Mermaid, and the
  PDF backend runs Graphviz on it and embeds the SVG. It may be left out: unstated, a view the
  migrator positioned is drawn through Graphviz anyway, and only a view nothing positions is
  Mermaid — with no Graphviz installed the positioned views fall back to Mermaid under a notice
  in the document saying so. A view whose every node is positioned is drawn by `neato -n2`, which
  keeps the stated positions and routes exactly; a view the model places only in part is laid out
  around what it places.
- `-render-style cameo` draws in the look of Cameo Systems Modeler — the diagram frame with its
  `stm [State Machine] Owner [ Name ]` header tab, 11 pt Arial, the pale-yellow gradient states,
  green actions and orange blocks, a state's `do / Activity` compartment, the initial dot, final
  bull's-eye, decision diamond and fork bars, and notes with a folded corner and dashed anchor.
  Without it the same geometry is drawn in the Pilot visualizer's black-and-white `pilot` style,
  the default ([the two styles, measured](../project/view-rendering-forms.md#style)).
- `-doc-form html` and the default Markdown take the same two options: the HTML figure is the
  DOT source for the reader's own Graphviz, or the SVG when the renderer is given images.

The migration report's `layout` section says, per diagram, what was carried and what was not:
how many shown elements were positioned and how many routes written, how many symbols kept their
own colours or font, how many notes were anchored and how many left free, and — by symbol class —
what was dropped. Free content standing for no element and saying nothing — a pasted raster image
(`ImageShape`) above all — is dropped and counted by class; a symbol drawn without a fill is counted
under `USE_FILL_COLOR`, the one presentation property `Style` has no attribute for. Cameo's drop
shadow and its exact corner radius are not drawn: Graphviz has neither.

## Over gRPC and from a program

The same migration is the service's `Convert` with `from_format` of `xmi`, `uml` or `mdzip` —
inferred from `file_path`'s extension when omitted — and a `to_format` of notation or Turtle.
The response marks it `experimental` with the notice above, which the Python client raises as an
`ExperimentalFeatureWarning` and the Go client reports on the `Conversion`. The report and the
results sidecar are not on the wire: a program that needs the element-by-element account runs
the command. How each client exposes the conversion is in
[chapter 9](09-clients.md#writing-a-model-back-out), and the wire fields in
[reference/wire-contract.md](../reference/wire-contract.md#conversion-convert).

```python
migrated = opensysml.convert("sysml", file_path="Vehicle.mdzip")
migrated.write("Vehicle.sysml")
```

## Where the mapping stops

What the mapping does not do is deliberate rather than an oversight, and is tracked on the
[roadmap](../project/roadmap.md): units and quantity kinds, the report and the results sidecar
over gRPC, and identity that survives a second migration of the same model. Until then, the
report is the contract — a v1 element is either in the v2 model, or named in the report with
the reason it is not, never silently dropped.
