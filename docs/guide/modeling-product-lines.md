# Modeling product lines and variability

A product line — a vehicle sold with two engines and two transmissions, a
spacecraft bus flown with or without a sunroof-sized radiator panel — is one
design with choices left open, and a product is that design with every choice
made. SysML v2 writes the open design as a *variation* whose *variants* are the
choices, ties the choices together with assertions, and writes a product as a
specialization that picks one variant per choice. This chapter builds the
vehicle family of the OMG training corpus from its superset model to its solved
configurations, and shows what the validator, the REPL, the solver and the
renderer each say about it. Every command and every output below was run on the
develop build.

The worked model is the three files of
`examples/sysml-v2-training/36. Variability/`, fetched with
`./scripts/download-training-examples.sh`. Three larger models are referenced
where they add something the small one lacks; they arrive with
`./scripts/download-pilot-corpora.sh`:

- `examples/pilot-corpora/sysml-examples/Variability Examples/VehicleVariabilityModel.sysml`,
  the OMG example the training files are cut from, with an optional sunroof and
  variation points inside actions;
- `examples/pilot-corpora/sysml-validation/07-Variant Configuration/7a-Variant Configuration - General Concept.sysml`,
  nested variation points and two configurations;
- `examples/pilot-corpora/sysml-validation/07-Variant Configuration/7b-Variant Configurations.sysml`,
  a family whose assertions tie an engine, a transmission and a requirement
  choice together, with a nested variation on the rear axle.

The vocabulary is that of SysML v2.1 Beta 2 §7.6.7 "Variations and Variants"
and of Annex A.12, the variability example; `./scripts/download-spec-examples.sh`
puts the text at `build/spec-docs/sysml.txt`. Build the tools once with
`make build`; `%configure` needs an SMT solver, see
[Solving the family](#solving-the-family).

## The superset model

A product line is written once as a *superset model*: every part the family
can have, with the choices marked. The mark is the `variation` prefix. SysML
v2.1 Beta 2 §7.6.7 defines it: "A definition or usage is specified as a
variation by placing the keyword `variation` before its kind keyword. A
variation is always abstract, so the `abstract` keyword is not used on a
variation." The members of a variation are its *variants*, each prefixed
`variant`, and "variant usages may only be declared within a variation."

`Variation Definitions.sysml` declares the engine choice as a variation
*definition* whose variants are two separately declared parts, and the
cylinder-bore choice as a variation attribute definition whose variants carry
values:

```sysml
variation attribute def DiameterChoices :> Diameter {
    variant attribute diameterSmall = 70[mm];
    variant attribute diameterLarge = 100[mm];
}

variation part def EngineChoices :> Engine {
    variant '4cylEngine';
    variant '6cylEngine';
}
```

A variation definition specializes the definition its variants are kinds of
(`EngineChoices :> Engine`), so a usage typed by it is an engine whichever
variant is chosen. The bare `variant '4cylEngine';` form is the one §7.6.7
describes for "a non-variant usage … declared to act as a variant": the
variant is the part `'4cylEngine'` declared earlier in the package, which
redefines `cylinder[4]` on `Engine`.

`Variation Usages.sysml` builds the family. `engine` is a usage typed by the
variation definition; `transmission` is a variation *usage*, with its variants
declared in place; and an `assert constraint` says which engine goes with
which transmission:

```sysml
abstract part vehicleFamily : Vehicle {
    part engine : EngineChoices[1];

    variation part transmission : Transmission[1] {
        variant manualTransmission;
        variant automaticTransmission;
    }

    assert constraint {
        (engine == engine::'4cylEngine' and
            transmission == transmission::manualTransmission) xor
        (engine == engine::'6cylEngine' and
            transmission == transmission::automaticTransmission)
    }
}
```

Each variation point is a feature of the family, and each variant is reached
through it: `engine::'4cylEngine'` is the variant of the type of `engine`,
`transmission::manualTransmission` the variant owned by `transmission`. The
`xor` is what makes this a product line rather than a parts list: four
combinations are expressible, two are products. The family itself is
`abstract` because it is not a vehicle anyone builds.

An enumeration is a variation too. §7.8.2 says "an enumeration definition is
always considered to be a variation" whose enumerated values are its
variants; `enum def` is the short form, and the rules of the next section
apply to it unchanged.

Variants nest. In `VehicleVariabilityModel.sysml` the `'6cylEngine'` variant
owns a variation port and binds its cylinder bore to `DiameterChoices`, with
an assertion of its own tying the two together — a choice that only exists
once the six-cylinder engine is chosen:

```sysml
variation part def EngineChoices :> Engine {
    variant '4cylEngine';
    variant '6cylEngine' {
        variation port :>> autoPort {
            variant port autoPort1;
            variant port autoPort2;
        }

        part :>> cylinder {
            attribute :>> diameter : DiameterChoices;
        }

        assert constraint {
            (autoPort == autoPort::autoPort1 and cylinder.diameter == cylinder::diameter::diameterSmall) xor
            (autoPort == autoPort::autoPort2 and cylinder.diameter == cylinder::diameter::diameterLarge)
        }
    }
}
```

The same model puts a variation point on a multiplicity (`sunroof` with
variants `withSunroof[1]` and `withoutSunroof[0]`) and on actions
(`generateTorque`, `amplifyTorque`), and 7a nests one variation point inside
another (`subsystemA` and `subsystemB` under a vehicle). Annex A.12 walks the
vehicle example in prose; its `vehicleFamily` is the one the training files
are cut from.

What you have learned: a variation is an abstract definition or usage whose
members are variants; a variation point is a feature typed by a variation
definition or declared as a variation usage; the assertions of the family say
which variants go together; enumerations are variations; and variants can own
variation points of their own.

## What the validator enforces

The three training files validate clean together:

```console
$ bin/sysml -validate "examples/sysml-v2-training/36. Variability/Variation Definitions.sysml" \
    "examples/sysml-v2-training/36. Variability/Variation Usages.sysml" \
    "examples/sysml-v2-training/36. Variability/Variation Configuration.sysml"
✓ package 'Variation Definitions'
✓ package 'Variation Usages'
✓ package 'Variation Configuration'
✓ examples/sysml-v2-training/36. Variability/Variation Definitions.sysml, examples/sysml-v2-training/36. Variability/Variation Usages.sysml, examples/sysml-v2-training/36. Variability/Variation Configuration.sysml: no errors
```

So do `VehicleVariabilityModel.sysml`, 7a and 7b. Three rules of §7.6.7 are
checked on every file; each is shown here on a small model that breaks it,
with the diagnostic as the build prints it.

A variation must not specialize another variation. A second layer of choices
is written as a nested variation point, not as a subtype:

```sysml
variation part def MoreEngineChoices :> EngineChoices {
    variant part huge : Engine;
}
```

```text
p1.sysml:7:45: error[variation-specialization]: A variation must not specialize another variation.
    variation part def MoreEngineChoices :> EngineChoices {
                                            ^~~~~~~~~~~~~
```

The same rule reaches a variation *usage* typed by a variation definition —
`variation part transmission : TransmissionChoices;` is refused at the type
with the same text — which is what the [last section](#what-the-specification-says-that-tools-disagree-on)
is about.

A variant belongs to a variation, and nothing else owns one:

```sysml
part def Vehicle {
    variant part spare : Engine;
}
```

```text
p2.sysml:4:9: error[variant-outside-variation]: A variant must be an owned member of a variation.
        variant part spare : Engine;
        ^~~~~~~~~~~~~~~~~~~~~~~~~~~~
```

And the converse: every usage a variation owns is a variant. A plain `part`
inside a variation is a modelling slip, not a shared member:

```sysml
variation part def EngineChoices :> Engine {
    variant part small : Engine;
    part big : Engine;
}
```

```text
p3.sysml:5:9: error[variation-member-not-variant]: An owned usage of a variation must be a variant.
        part big : Engine;
        ^~~~~~~~~~~~~~~~~~
```

Because an enumeration definition is a variation, the first rule forbids
specializing one — by a declared variation or by another `enum def`. The
diagnostic says why:

```sysml
enum def Color { red; green; }
variation attribute def ColorChoices :> Color {
    variant attribute red2 : Color;
}
enum def Shade :> Color { dark; }
```

```text
p4.sysml:4:45: error[variation-specialization]: variation `ColorChoices` must not specialize enumeration definition `Color`: every enumeration definition is a variation whose enumerated values are its variants, and a variation must not specialize another variation
    variation attribute def ColorChoices :> Color {
                                            ^~~~~
p4.sysml:7:23: error[variation-specialization]: enumeration definition `Shade` must not specialize enumeration definition `Color`: every enumeration definition is a variation whose enumerated values are its variants, and a variation must not specialize another variation; declare the enumerated values of `Shade` directly
    enum def Shade :> Color { dark; }
                      ^~~~~
```

One more is caught by the parser, from the sentence of §7.6.7 that a
variation "is always abstract, so the `abstract` keyword is not used":

```text
p5.sysml:3:14: error[syntax]: 'variation' cannot follow 'abstract': a prefix says 'abstract' or 'variation', not both
    abstract variation part def EngineChoices :> Engine {
             ^~~~~~~~~
```

Each of these ends `sysml: <file> did not analyse cleanly; no check was
made`, with exit status 2: a file that breaks a variability rule is not
checked further. The probe files are a few lines each and worth having
students write themselves; the paths above are theirs.

What you have learned: the validator holds the shape of a product line —
variations do not specialize variations (enumerations included), variants
live only in variations, and a variation owns nothing but variants — and
says which rule and which line when a model breaks it.

## Configurations

A product is a *configuration*: a specialization of the family that redefines
each variation point to one of its variants. `Variation Configuration.sysml`
has two:

```sysml
part vehicle4Cyl :> vehicleFamily {
    part redefines engine = engine::'4cylEngine';
    part redefines transmission = transmission::manualTransmission;
}

part vehicle6Cyl :> vehicleFamily {
    part redefines engine = engine::'6cylEngine';
    part redefines transmission = transmission::manualTransmission;
}
```

`part redefines engine = …` and `part :>> engine = …` are the same
declaration. The configuration inherits the family's assertion, and that is
the point: `vehicle4Cyl` picks the four-cylinder engine with the manual
transmission, one of the two products the `xor` admits; `vehicle6Cyl` picks
the six-cylinder engine with the manual transmission, a pair the family
forbids. The training corpus ships the inconsistent one deliberately — it is
the configuration the rest of this chapter catches.

Open the three files in the REPL (`bin/sysml` with the three paths) and ask
what each configuration selected. A selection is a value, and `==` compares
it with the variant:

```text
sysml> %eval 'Variation Configuration'::vehicle4Cyl.engine == 'Variation Definitions'::EngineChoices::'4cylEngine'
✓ 'Variation Configuration'::vehicle4Cyl.engine == 'Variation Definitions'::EngineChoices::'4cylEngine'
  = true
sysml> %eval 'Variation Configuration'::vehicle4Cyl.transmission == 'Variation Usages'::vehicleFamily::transmission::manualTransmission
✓ 'Variation Configuration'::vehicle4Cyl.transmission == 'Variation Usages'::vehicleFamily::transmission::manualTransmission
  = true
sysml> %eval 'Variation Configuration'::vehicle6Cyl.engine == 'Variation Definitions'::EngineChoices::'4cylEngine'
✓ 'Variation Configuration'::vehicle6Cyl.engine == 'Variation Definitions'::EngineChoices::'4cylEngine'
  = false
```

The inherited assertion is checked the way any assertion is: `%instantiate` a
configuration, then `%validate` it ([`%instantiate`](../reference/repl-commands.md) and
[`%validate`](../reference/repl-commands.md) in the command reference).
The consistent configuration holds:

```text
sysml> %instantiate 'Variation Configuration'::vehicle4Cyl
✓ Created instance of 'Variation Configuration'::vehicle4Cyl
  ID: 6
  Use %features 'Variation Configuration'::vehicle4Cyl to inspect
sysml> %validate vehicle4Cyl
✓ assert constraint holds (on 'Variation Configuration'::vehicle4Cyl ID: 6)
✓ 'Variation Configuration'::vehicle4Cyl is valid: its 1 assertion holds
  standing: holds (observed: 1 run under reverse)
```

The inconsistent one fails, and the report prints the assertion that failed:

```text
sysml> %instantiate 'Variation Configuration'::vehicle6Cyl
✓ Created instance of 'Variation Configuration'::vehicle6Cyl
  ID: 13
  Use %features 'Variation Configuration'::vehicle6Cyl to inspect
sysml> %validate vehicle6Cyl
✗ assert constraint fails (on 'Variation Configuration'::vehicle6Cyl ID: 13)
  Assertion evaluated to false: engine == engine::4cylEngine and transmission == transmission::manualTransmission xor engine == engine::6cylEngine and transmission == transmission::automaticTransmission
✗ 'Variation Configuration'::vehicle6Cyl is not valid: 1 of 1 assertion fails
  standing: violated (witnessed: 1 run under reverse)
```

`standing` is the runtime's own word for what it established ([the standing
of an answer](../reference/external-engines.md#the-standing-of-an-answer)):
`holds (observed: 1 run …)`
is one evaluation of the assertion on this object, not a proof over every
configuration. The proof is the next section.

What you have learned: a configuration is a specialization that redefines each
variation point with `=` to one variant; the family's assertion is inherited;
`%eval` reads a selection back, and `%instantiate` + `%validate` check one
configuration against the assertion and name it when it fails.

## Solving the family

`%validate` checks the configuration you wrote. `%configure` turns the
question around: given the family, which selections of variants satisfy its
assertions? It hands the assertions to an SMT solver, so it needs `z3` (or
`cvc5`) on `PATH` or named by `OPENSYSML_SMT`; [chapter 1](01-install.md#installing-a-solver-optional)
says how to install one. Without a solver the command refuses rather than
guessing:

```text
error: no SMT solver found: install z3 (`apt install z3`, `brew install z3`) or cvc5, or set OPENSYSML_SMT to a solver executable; looked for [z3 cvc5] on PATH
  standing: not covered (solve refused: no SMT solver found: install z3 (`apt install z3`, `brew install z3`) or cvc5, or set OPENSYSML_SMT to a solver executable; looked for [z3 cvc5] on PATH)
```

Named with the family alone, `%configure` returns one consistent selection:

```text
sysml> %configure 'Variation Usages'::vehicleFamily
✓ Part vehicleFamily permits a selection of variants (z3, 16ms)
  'Variation Usages'::vehicleFamily::engine = 'Variation Definitions'::EngineChoices::'4cylEngine'
  'Variation Usages'::vehicleFamily::transmission = 'Variation Usages'::vehicleFamily::transmission::manualTransmission
  One witness: use %configure 'Variation Usages'::vehicleFamily all for every consistent selection.
  standing: satisfiable (witnessed: 1 query by solve)
```

Fix some variation points yourself with `<variation>=<variant>` and the
solver fills in the rest — or refuses, naming the assertion that refuses
and the source line it is on. The four-cylinder engine with the automatic
transmission is refused:

```text
sysml> %configure 'Variation Usages'::vehicleFamily engine='4cylEngine' transmission=automaticTransmission
✗ the chosen variants are not consistent with Part vehicleFamily (z3, 14ms)
  Already fixed:
    'Variation Usages'::vehicleFamily::engine = Variation Definitions::EngineChoices::4cylEngine  (chosen)
    'Variation Usages'::vehicleFamily::transmission = Variation Usages::vehicleFamily::transmission::automaticTransmission  (chosen)
  In the conflict: 'Variation Usages'::vehicleFamily::engine = Variation Definitions::EngineChoices::4cylEngine, 'Variation Usages'::vehicleFamily::transmission = Variation Usages::vehicleFamily::transmission::automaticTransmission
  Against: required condition { engine == engine::4cylEngine and transmission == transmission::manualTransmission xor engine == engine::6cylEngine and transmission == transmission::automaticTransmission } of vehicleFamily at examples/sysml-v2-training/36. Variability/Variation Usages.sysml:18:4
  standing: unsatisfiable (proved over inputs: 1 query by solve)
```

`all` enumerates every consistent selection and says whether the list is
complete — here two, "which are all of them". That line is the product line's
catalogue, derived from the superset model rather than maintained beside it:

```text
sysml> %configure 'Variation Usages'::vehicleFamily all
✓ Part vehicleFamily permits 2 selections of variants, which are all of them (z3, 15ms)
  1.
    'Variation Usages'::vehicleFamily::engine = 'Variation Definitions'::EngineChoices::'4cylEngine'
    'Variation Usages'::vehicleFamily::transmission = 'Variation Usages'::vehicleFamily::transmission::manualTransmission
  2.
    'Variation Usages'::vehicleFamily::engine = 'Variation Definitions'::EngineChoices::'6cylEngine'
    'Variation Usages'::vehicleFamily::transmission = 'Variation Usages'::vehicleFamily::transmission::automaticTransmission
  standing: satisfiable (witnessed: 1 query by solve)
```

Named with a configuration, `%configure` takes the values the configuration
declares as fixed (`(declared)` rather than `(chosen)`) and asks whether any
selection is consistent with them. For `vehicle4Cyl` there is one; for
`vehicle6Cyl` there is none, and the report is the same conflict the
`%validate` run found, this time proved rather than observed:

```text
sysml> %configure 'Variation Configuration'::vehicle4Cyl
✓ Part vehicle4Cyl permits a selection of variants (z3, 14ms)
  Already fixed:
    'Variation Configuration'::vehicle4Cyl::engine = Variation Definitions::EngineChoices::4cylEngine  (declared)
    'Variation Configuration'::vehicle4Cyl::transmission = Variation Usages::vehicleFamily::transmission::manualTransmission  (declared)
  'Variation Configuration'::vehicle4Cyl::engine = 'Variation Definitions'::EngineChoices::'4cylEngine'
  'Variation Configuration'::vehicle4Cyl::transmission = 'Variation Usages'::vehicleFamily::transmission::manualTransmission
  One witness: use %configure 'Variation Configuration'::vehicle4Cyl all for every consistent selection.
  standing: satisfiable (witnessed: 1 query by solve)
sysml> %configure 'Variation Configuration'::vehicle6Cyl
✗ Part vehicle6Cyl permits no selection of variants consistent with the values it fixes (z3, 14ms)
  Already fixed:
    'Variation Configuration'::vehicle6Cyl::engine = Variation Definitions::EngineChoices::6cylEngine  (declared)
    'Variation Configuration'::vehicle6Cyl::transmission = Variation Usages::vehicleFamily::transmission::manualTransmission  (declared)
  In the conflict: 'Variation Configuration'::vehicle6Cyl::engine = Variation Definitions::EngineChoices::6cylEngine, 'Variation Configuration'::vehicle6Cyl::transmission = Variation Usages::vehicleFamily::transmission::manualTransmission
  Against: required condition { engine == engine::4cylEngine and transmission == transmission::manualTransmission xor engine == engine::6cylEngine and transmission == transmission::automaticTransmission } of vehicleFamily at examples/sysml-v2-training/36. Variability/Variation Usages.sysml:18:4
  standing: unsatisfiable (proved over inputs: 1 query by solve)
```

The query is the conjunction of every assertion *about the named part's
objects* — its own and inherited asserted constraints, the requirements it
carries and the satisfactions it states, the same collection `%validate`
checks. What a nested part asserts about *its* objects is not in it, and the
report says so. 7b's `anyVehicleConfig` ties an engine, a transmission and a
performance requirement together, and owns a `rearAxleAssembly` whose own
`rearWheelChoice` is asserted one level down:

```text
sysml> %configure '7b-Variant Configurations'::VariabilityModel::anyVehicleConfig all
✓ Part anyVehicleConfig permits 2 selections of variants, which are all of them (z3, 15ms)
  1.
    '7b-Variant Configurations'::VariabilityModel::anyVehicleConfig::engineChoice = '7b-Variant Configurations'::VariabilityModel::anyVehicleConfig::engineChoice::'4cylEngine'
    '7b-Variant Configurations'::VariabilityModel::anyVehicleConfig::engineRqtChoice = '7b-Variant Configurations'::VariabilityModel::anyVehicleConfig::engineRqtChoice::normalPerformanceRequirement
    '7b-Variant Configurations'::VariabilityModel::anyVehicleConfig::transmissionChoice = '7b-Variant Configurations'::VariabilityModel::anyVehicleConfig::transmissionChoice::manualTransmission
  2.
    '7b-Variant Configurations'::VariabilityModel::anyVehicleConfig::engineChoice = '7b-Variant Configurations'::VariabilityModel::anyVehicleConfig::engineChoice::'6cylEngine'
    '7b-Variant Configurations'::VariabilityModel::anyVehicleConfig::engineRqtChoice = '7b-Variant Configurations'::VariabilityModel::anyVehicleConfig::engineRqtChoice::highPerformanceRequirement
    '7b-Variant Configurations'::VariabilityModel::anyVehicleConfig::transmissionChoice = '7b-Variant Configurations'::VariabilityModel::anyVehicleConfig::transmissionChoice::automaticTransmission
  Not in the query: what rearAxleAssembly asserts about its own objects; %instantiate and %validate an object to check it
  standing: satisfiable (witnessed: 1 query by solve)
```

The two selections are complete *for the three variation points the
family's own assertions mention*. The rear-axle choice is a question about
the axle's objects, so it is not solved here; the advisory names the part
and the way to check it — instantiate a configuration and `%validate` it,
which evaluates every nested assertion on the objects actually created.
`VehicleVariabilityModel.sysml` gives the same advisory for `engine`, whose
six-cylinder variant asserts over its own port and bore.

The same solver is reachable from an agent as the MCP `configure` tool
([sysml-mcp reference](../reference/sysml-mcp.md)), and from a notebook
through the [Jupyter kernel](12-jupyter.md). It has no flag on the `sysml`
command line: `bin/sysml -help` lists `-validate` but no `-configure` or
`-solve`, so a script drives it through the REPL's standard input or through
MCP.

What you have learned: `%configure <family>` finds one consistent selection,
`<variation>=<variant>` fixes some and solves the rest or names the refusing
assertion, `all` enumerates with a completeness verdict, a configuration is
solved over the values it declares, and the query stops at the named part's
own objects — the advisory tells you where `%validate` has to take over.

## Seeing variability

A view over the family shows the choices as rows. Put this beside the three
training files (as a fourth file on the command line) and render it:

```sysml
package 'Variation Views' {
    private import 'Variation Definitions'::*;
    private import 'Variation Usages'::*;
    private import Views::*;

    view familyTree {
        expose EngineChoices::**;
        expose vehicleFamily::**;
        render asTreeDiagram;
    }
    view familyInterconnection {
        expose vehicleFamily::**;
        render asInterconnectionDiagram;
    }
}
```

```text
sysml> %render 'Variation Views'::familyTree
'Variation Views'::familyTree - tree rendering (render asTreeDiagram)

variation part def EngineChoices
  variant '4cylEngine'
    part cylinder
      attribute diameter
  variant '6cylEngine'
    part cylinder
      attribute diameter
  part cylinder
    attribute diameter
part vehicleFamily
  part engine
    part cylinder
      attribute diameter
  variation part transmission
    variant manualTransmission
    variant automaticTransmission
  assert constraint

relationships:
  vehicleFamily *-- EngineChoices: engine
```

The text form writes the keywords; the diagram forms (`mermaid`, `dot`,
`plantuml`, `svg`) write them as stereotype rows, `«variation part def»` and
`«variant»`, on the box, and the [editor diagram panel](08-editors.md) renders
the same views. `cylinder` appears under `EngineChoices` and under
each variant because every engine has cylinders and each variant redefines
how many. The interconnection rendering keeps the variation rows and draws
the connections between the exposed parts; this family declares none, so it
is the parts alone:

```text
sysml> %render 'Variation Views'::familyInterconnection
'Variation Views'::familyInterconnection - interconnection rendering (render asInterconnectionDiagram)

part vehicleFamily
  part engine
    part cylinder
      attribute diameter
  variation part transmission
    variant manualTransmission
    variant automaticTransmission
  assert constraint
```

The same views render from the command line with `-render <view>` and
`-render-form <form>` ([CLI reference](../reference/cli.md)), which is how a
slide gets its SVG; the forms and what each draws are listed with
[`%render`](../reference/repl-commands.md).

What you have learned: a view that exposes the family draws every variation
point and its variants as rows marked `variation` and `variant`, in text or as
a diagram, so a class can see the superset model it has been reading.

## What the specification says that tools disagree on

Annex A.12 of SysML v2.1 Beta 2 writes the transmission variation point of
its vehicle family as

```sysml
variation part transmission : TransmissionChoices;
```

a variation usage typed by a variation definition. That declaration violates
the specification's own constraint `validateUsageVariationSpecialization`,
the rule of §7.6.7 that a variation must not specialize another variation;
OpenSysML rejects it with the `variation-specialization` diagnostic shown
[above](#what-the-validator-enforces), and the OMG pilot implementation
rejects it too. It is a known inconsistency between the specification's
example and its normative text, recorded in the
[specification–pilot gap register](../project/spec-pilot-gap-register.md)
and the [specification compliance](../project/spec-compliance.md) tables.
The training files and `VehicleVariabilityModel.sysml` avoid it by declaring
the variants of `transmission` in place, which is the form to teach.

What you have learned: when an example in the specification and a validator
disagree, the constraint in the normative text decides, and both
implementations here apply it.
