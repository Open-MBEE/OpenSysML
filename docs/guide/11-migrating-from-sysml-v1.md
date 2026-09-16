# 11. Migrating a SysML v1 model

A SysML v1 model exported from its tool as UML XMI, an Eclipse UML2 `.uml` file or a
MagicDraw/Cameo `.mdzip` archive can be read by `sysml -convert` and written as SysML v2 notation
or RDF Turtle. This chapter walks one export through the migration: running it, reading the
report it produces, finishing by hand what the mapping leaves behind, and checking the result as
a v2 model. Every mapping rule, and the format of the report, is in
[reference/sysml-v1-migration.md](../reference/sysml-v1-migration.md).

Migration is **experimental**: the mapping covers structure, ports and connectors, requirements,
constraints, instances and allocations; it does not yet migrate behaviors, operations or units;
and what it writes for a v1 element may change between releases without a compatibility path.
Every run says so on stderr, so a script sees the status without reading this page. It is also
**one way**: a v2 model has no v1 form, so `-convert xmi` is refused, and the migrated notation is
the place to keep working — not a copy to be re-migrated after editing the v1 model.

## The example

The model migrated below is the vehicle fixture the migration's own tests use,
[`internal/core/migrate/testdata/xmi/vehicle.xmi`](../../internal/core/migrate/testdata/xmi/vehicle.xmi):
a small vehicle design of blocks, value types, ports and connectors, a constraint block, an
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
wrote Vehicle.report.txt (migration report: migrated 89 element(s): 68 mapped, 13 approximated, 8 unmapped (3 skipped as profile or library content))
wrote Vehicle.sysml (sysml, 5913 bytes)
```

The summary is the first thing to read: 89 v1 elements, of which 68 have a direct v2 form, 13
were written as the nearest v2 construct, and 8 have none. The 3 skipped are the SysML profile
application, the profile itself and a diagram — not model content, so they count against nothing.
The command exits 0 when the notation was written, whatever the report says; it exits non-zero
and writes nothing when the input cannot be read as XMI at all, or holds no model.

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
# migrated 89 element(s): 68 mapped, 13 approximated, 8 unmapped (3 skipped as profile or library content)

## unmapped (8)
«Verify» Abstraction	Requirements::<Abstraction>	_dep_verify_block	(a verify whose client is not a test case has no v2 form; applied stereotypes «Verify»)
«Refine» Abstraction	Requirements::<Abstraction>	_dep_refine	(its end Vehicle Design::Drive is not migrated; applied stereotypes «Refine»)
«Allocate» Abstraction	Requirements::<Abstraction>	_dep_allocate	(its end Vehicle Design::Drive is not migrated; applied stereotypes «Allocate»)
Activity	Vehicle Design::Drive	_act_drive	(behaviors are not migrated yet)
«Unit» InstanceSpecification	Vehicle Design::Value Types::kilogram	_unit_kg	(units and quantity kinds are not migrated; use the SI and ISQ libraries; applied stereotypes «Unit» (quantityKind = Vehicle Design::Value Types::mass; symbol = kg))
«QuantityKind» InstanceSpecification	Vehicle Design::Value Types::mass	_qk_mass	(units and quantity kinds are not migrated; use the SI and ISQ libraries; applied stereotypes «QuantityKind»)
StateMachine	Vehicle Design::Vehicle::Vehicle States	_sm_vehicle	(behaviors are not migrated yet)
Operation	Vehicle Design::Vehicle::start	_op_start	(operations and receptions are not migrated; v2 has no operation)

## approximated (13)
«Trace» Abstraction	Requirements::<Abstraction>	_dep_trace	(a trace is written as a plain dependency)
«TestCase» Activity	Requirements::Mass Test	_tc_mass	-> Requirements::'Mass Test'	(the test case's behavior is not migrated; only its verified requirements are)
Actor	Vehicle Design::Driver	_actor_driver	-> 'Vehicle Design'::Driver	(a UML actor is written as a part def)
«FlowPort» Port	Vehicle Design::Engine::speedIn	_port_speedIn	-> 'Vehicle Design'::Engine::speedIn	(a port typed by a DataType is written as a port holding one directed attribute)
…
```

Each line is the v1 element's kind with its applied stereotypes, its qualified name in the v1
model, its `xmi:id`, the v2 element it became (after `->`) when one was written, and the reason
for the verdict in parentheses. The columns are tab-separated, so `cut` and `awk` read them.

- **mapped** — a direct v2 form. Nothing to do.
- **approximated** — written as the nearest v2 construct, and the note says what was lost. Some
  of these are simply how v2 spells the idea (a UML actor is a `part def`; a flow port typed by a
  data type is a port holding one directed attribute; a `«Trace»` is a `dependency`). Others
  record a v1 tag that v2 has no home for (`isEncapsulated`, a value type's `unit` and
  `quantityKind`), or an opaque expression copied verbatim in a language other than SysML, which
  the v2 model will not evaluate.
- **unmapped** — nothing was written for the element. The notation holds a comment where it
  would have gone, so the loss is visible in the file, not only in the report. The three
  `Abstraction`s are unmapped *because* their other end, the activity `Drive`, is: a relationship
  cannot be written to an element that does not exist.
- **skipped** — profile, library and tool-private content. Never model content.

The migration keeps the v1 names, quoting them where v2 requires it, and keeps a requirement's
`id` as its short name, so `Mass Requirement` with id `R1` is `requirement def <R1> 'Mass
Requirement'` and a v1 qualified name can be searched for in the v2 file as it stands.

## Finishing by hand

Open `Vehicle.sysml`. Alongside every element the report calls unmapped or approximated-with-loss
is a comment saying so, at the place in the v2 model where the element belongs:

```sysml
package 'Value Types' {
    attribute def Mass :> ScalarValues::Real {
        /* «ValueType» tags with no v2 form: quantityKind = Vehicle Design::Value Types::mass; unit = Vehicle Design::Value Types::kilogram */
    }
    /* not migrated: «Unit» InstanceSpecification 'kilogram' — units and quantity kinds are not migrated; use the SI and ISQ libraries; applied stereotypes «Unit» (quantityKind = Vehicle Design::Value Types::mass; symbol = kg) */
    …
}
part def Vehicle :> System {
    …
    /* not migrated: Operation 'start' — operations and receptions are not migrated; v2 has no operation */
    /* not migrated: StateMachine 'Vehicle States' — behaviors are not migrated yet */
    …
}
```

Each comment is a to-do, and its note says which way to go:

- **Units and quantity kinds.** v2 ships them: import `SI` and `ISQ` and type the attribute by
  the library's quantity, `attribute def Mass :> ISQ::MassValue;` with `mass = 1200 [SI::kg]` as
  its value, in place of the v1 value type that carried a unit tag. The mapping reports rather
  than guesses here, since the v1 model's own `kilogram` may or may not be the SI one.
- **Behaviors.** The activity `Drive` and the state machine `Vehicle States` are written in the
  v2 notation of [chapter 6](06-behavior.md) — `action def`, `state def`, transitions with
  triggers and guards — and the `«Allocate»` and `«Refine»` relationships that pointed at `Drive`
  are restored once it exists, as an `allocate` and as a `dependency` carrying
  `@ModelingMetadata::Refinement`, the forms the mapping writes when both ends are present.
- **Operations.** v2 has no operation; a v1 operation on a block becomes a `perform action` or
  an action usage owned by the part, its parameters the action's `in` and `out`.
- **Opaque expressions.** A v1 constraint or default is copied verbatim when its text parses as
  a v2 expression, and written as a comment otherwise. `constraint 'positive mass' { mass > 0 }`
  was copied from an opaque expression whose declared language is `English`; it reads the same
  in v2, but a copied expression means what v2 says it means, not what the original language
  did, so the report's `opaque expression copied verbatim (language …)` notes are the
  constraints to read, and the commented ones are the constraints to rewrite.
- **Tags with no v2 form.** `isEncapsulated`, a custom stereotype's tags, a value type's
  `unit`: keep the comment as documentation, or express the intent in v2 terms (a `metadata def`
  for a stereotype that carries meaning the model relies on).

Where the v1 model has a relationship to an element outside the exported file — a used
project, a resource referenced by `href` — that end is an external proxy, and the relationship
is reported unmapped rather than written to nothing. Migrate the other model too, then reconnect
them in v2.

## Checking the result

The migrated notation is an ordinary v2 model from here on: every check in
[chapter 3](03-command-line.md) applies, and the file parses and analyses under the same passes
as a hand-written one. This model's `Vehicle` and its instance `myCar` are intact — the value
`myCar` gives its mass is what the v1 slot held:

```console
$ sysml Vehicle.sysml -eval "'Vehicle Design'::myCar::mass"
Vehicle.sysml:85:9: warning: End feature must have multiplicity 1: an end relates exactly one thing per link; write `[1]` or take it from a feature the end subsets or redefines
        end driver : Driver[0..1];
        ^~~~~~~~~~~~~~~~~~~~~~~~~~
✓ package 'Vehicle Design'
✓ package Requirements
✓ package 'Empty Package'
  = 1350.5
```

Findings on the migrated file are read as on any model, and point at what to revise in the v2
notation — here the association `Drives`, whose v1 end multiplicity `0..1` v2 writes on the end
rather than after its type: `end [0..1] driver : Driver;`. From this point `%save` in the REPL,
`-convert ttl`, the LSP and the clients all take the file as they take any other; nothing
remembers that it was migrated.

## Over gRPC and from a program

The same migration is the service's `Convert` with `from_format` of `xmi`, `uml` or `mdzip` —
inferred from `file_path`'s extension when omitted — and a `to_format` of notation or Turtle.
The response marks it `experimental` with the notice above, which the Python client raises as an
`ExperimentalFeatureWarning` and the Go client reports on the `Conversion`. The report is not on
the wire: a program that needs the element-by-element account runs the command. How each client
exposes it is in [chapter 9](09-clients.md#writing-a-model-back-out), and the wire fields in
[reference/wire-contract.md](../reference/wire-contract.md#conversion-convert).

```python
migrated = opensysml.convert("sysml", file_path="Vehicle.mdzip")
migrated.write("Vehicle.sysml")
```

## Where the mapping stops

What the mapping does not do is deliberate rather than an oversight, and is tracked on the
[roadmap](../project/roadmap.md): behaviors, operations and receptions, units and quantity kinds,
and the report over gRPC. Until then, the report is the contract — a v1 element is either in the
v2 model, or named in the report with the reason it is not, never silently dropped.
