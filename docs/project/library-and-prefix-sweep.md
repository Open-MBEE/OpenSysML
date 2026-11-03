# Library and prefix sweep

Three systematic sweeps of what the runtime can *evaluate* among what it parses and validates:
every public Systems and Domain library feature read on an object, every Kernel Function Library
function called from an expression, and every grammar prefix crossed with every usage kind. Each
row is one probe driven through the runtime over the bundled standard library
(`internal/workspace/libs/stdlib/`); correctness is judged from the KerML 1.0 / SysML v2
specification and the library text, with the pinned pilot evaluator
(`scripts/download-pilot-evaluator.sh`) as referee where it can evaluate the expression.

Outcome buckets: **answers** (a value; judged against the library text in the verdict column),
**`<undetermined>`**, **typed error**, **error (`ErrNoSuchFeature`)**, **validate rejects**,
**panic** (none observed).

The Kernel Semantic Library tier is out of scope here: two sibling branches,
`fix/kernel-composite-collection-reads` and `fix/kernel-member-reads`, own every Kernel member
read on an object, and the rows below that fall to them say so. The Domain analysis libraries
(SampledFunctions, TradeStudies, StateSpaceRepresentation) are measured by the existing
[analysis-library census](analysis-library-census.json) and are not re-measured.

## Gap classes

| Class | Evidence | Spec / library basis | Proposed fix |
|---|---|---|---|
| **Usage kinds left out of the object shape.** A `case`, `use case`, `view`, `viewpoint`, `concern`, `rendering`, `enum`, `satisfy requirement` or `include use case` usage nested in a definition is `ErrNoSuchFeature` on the object; the library members of those kinds (`RequirementCheck::concerns`, `Case::subcases`, `UseCase::includedUseCases`/`subUseCases`, `View::subviews`/`viewRendering`/`viewpointConformance`/`viewpointSatisfactions`, `Rendering::subrenderings`, `Viewpoint::concerns`, `StateAction::stateSequencing`) are too. An `analysis`/`verification` usage is read as a calc usage ("computes output features") while a `requirement` usage is an object. | sweep 1: `Probe::p1.caq`, `.uq`, `.vwq`, `.vpq`, `.conq`, `.rdq`, `.eq`, `.sr`, `.iu`; sweep 3: every `enum` cell | SysML v2 §7.19–7.26: each of these is an OccurrenceUsage (or, for `enum`, an AttributeUsage) and so a feature of the type that owns it; the shape (`semantics.IsShapeFeature`) admits a fixed list of symbol kinds | one fix: the shape admits every usage kind that is a feature of its owner (not variants, not parameters), and the case family reads as one kind |
| **Unbound reference read refused with `errUninitializedFeatureValue`.** A `ref part x : P2;`, `ref item`, an unbound `ref` of any occurrence kind, an attribute of a structured value type (`attribute v : VectorQuantityValue;`), a `subject` with no binding (`new R1().s`), `Interface::participant [2..*]`, a state's `entryAction` written `entry;`, a substate, and every `::>` reference-subsetting usage read on its object are refused, while an unbound scalar `attribute n : Integer;` is `<undetermined>` of `[1]`. | sweep 1: `Probe::p1.other`, `.otherItem`, `.vq3`, `new Probe::R1().s`, `new Probe::If1().participant`, `Probe::p1.st.entryAction`, `.on1`; sweep 3: every `ref (unbound)` and `::>` cell | KerML 1.0 §7.3.4.1 / §7.4.12: a multiplicity bounds the number of values and chooses none; nothing in the library distinguishes a valueless reference from a valueless attribute. `instance_library_requirement_features.expected.json` pins the refusal for `subj`, so this is a recorded choice, not an accident | one rule: a feature nothing binds reads as `<undetermined>` of its multiplicity whatever its kind; `::>` additionally reads the referenced feature's values when the multiplicities make them the only ones admissible |
| **`::>` reference subsetting does not read through.** `ref part y ::> x;` on an object holding `x` is refused rather than reading `x`. | sweep 3: `::> reference subsetting` × every kind but `calc` | KerML 1.0 §7.3.4.2 ReferenceSubsetting: `y`'s values are among `x`'s; with `x [1]` and `y [1]` they are `x`'s | covered by the previous class's rule |
| **`@@` metadata test answers `false` for a user metadata definition.** `Probe::Tagged @@ Probe::M` over `#M part def Tagged;` is `false`; the pilot answers `true`. | sweep 2 row for `BaseFunctions::'@@'` | KerML 1.0 §7.4.9.6 ClassificationExpression: `x @@ M` tests whether the metaobjects annotating `x` are classified by `M`; §9.4.2 `BaseFunctions::'@@'` | the metadata test reads the annotating metadata usages of the named element, not only the metaclass |
| **`#` over a Collection-typed operand indexes the holder, not its elements.** `os#(1)` with `os : OrderedSet { :>> elements = (3, 1, 2); }` answers the `os` object; the pilot answers `3`. | sweep 2 row for `CollectionFunctions::'#'` | KerML 1.0 §9.4.5 `CollectionFunctions::'#'`: `(col.elements as Anything)#(index)`, selected for an `OrderedCollection` operand by the operator's overload resolution | the `#` operator dispatches on a value of a Collection type to `CollectionFunctions::'#'` as the qualified call already does |
| **`->reduce` with a function reference is refused at evaluation.** `(1, 2, 3)->reduce '+'` and `->reduce IntegerFunctions::'+'` validate clean and are refused with "requires a body expression, got function"; the body form `->reduce {in a; in b; a + b}` answers `6`. The pilot leaves both unevaluated (no verdict). | sweep 2 rows for `ControlFunctions::reduce` | KerML 1.0 §7.4.9.5: a reduce's body argument may be a reference to a Function; the library itself writes `coll->reduce '+' ?? zero` (`VectorFunctions::sum0`) | the collection operators take a Function reference as their body argument and invoke it per element pair |
| **A Domain default that names a library usage is not evaluable.** `SpatialItems::SpatialItem::coordinateFrame default universalCartesianSpatial3dCoordinateFrame` is "no value for feature universalCartesianSpatial3dCoordinateFrame" on a `SpatialItem` object. | sweep 1: `Probe::spat.coordinateFrame` | Geometry/SpatialItems.sysml line 37; ISQ declares the frame as an `attribute` usage with a bound value | reading a default that names a namespace-level usage evaluates that usage as a model-level read does |
| **`result`/`verdict` parameters are `ErrNoSuchFeature` on a requirement, case or viewpoint object.** `new R1().result`, `new V1().verdict`, `new Ca1().result`; the sibling branch answers Kernel frame members but leaves parameters out (`kernelFrameMember` excludes `IsParameter`). | sweep 1 rows with tier `systems FRAME` and member `result`/`verdict` | Requirements.sysml `RequirementCheck::result`, VerificationCases.sysml `VerificationCase::verdict`: return parameters of the predicate the object enacts | after the sibling lands: a return parameter read on an object answers the object's evaluation (`<undetermined>` where its inputs are unbound); listed here, not fixed, until the sibling's rule is in |

Status on `develop`: every class above but the last is fixed — the usage kinds held as object features (#1237), the unbound and `::>` reference reads (#1239), the `@@` metadata test (#1240), the Domain default naming a library usage (#1241), `#` over an `OrderedCollection` (#1242) and `->reduce` with a function reference (#1243); the `result`/`verdict` parameters remain open. The tables below record the measurement as it was taken; a probe whose class is fixed reads today as that fix's conformance fixture pins it.

Observations recorded but not treated as gaps:

- **Model-level open count.** A library collection read on a package-level `part p1 : P1;` answers what the model declares, exactly as on `new P1()`: `voids`, `shape`, `envelopingShapes`, `boundingShapes`, `subactions`, `substates`, `subports`, `exhibitedStates`, `interfacingPorts` are empty, `performedActions` beside a `perform action act` is `[act]`, `suboccurrences` is the subsetters' objects, and `isSolid = isEmpty(voids)` is `true` — the Systems and Kernel tiers alike, abstract or not. The earlier rule (a `[0..*]` library collection at model level is `<undetermined>`) is withdrawn: the pilot evaluates such a read as the declaration node and cannot referee it, and KerML 1.0 §7.3.4.4 with §7.3.4 fixes the answer as the declared contributions, a lower bound of 0 met by none. A usage the model itself declares reads by the same rule: `opt : P2[0..1]` and `ref part others : P2[0..*]` are empty, as on `new P1()`, a usage subsetting an open collection (`part x : P2[0..*]; part y : P2 :> x;`) makes `x` the collection holding `y`, and a library collection of a nested usage reads what that usage declares — `p1.act.subactions` is the one `action nested : A2;` of `A1`, as `new P1().act.subactions` already was. `<undetermined>` remains for an unbound input (`attribute n : Integer`), a parameter of a behavior usage nothing has invoked whatever its multiplicity (`act.i`, `act.o`; SysML v2 §7.17.2, KerML 1.0 §7.4.7 — a part's `in action x : A1` is no such parameter and is empty), and a collection whose lower bound of 1 or more nothing declared satisfies (spec-compliance: *A collection a usage values with nothing holds what the model declares*).
- **Package-level usages of non-object kinds.** `port pt : Pt;`, `action act : A1;`, `state`, `requirement`, `constraint`, `case`, `use case`, `view`, `connection`, `interface`, `allocation`, `flow` declared in a package evaluate to `<undetermined>` where `part`, `item`, `occurrence` and an `attribute def`-typed usage are objects; nested in a part they all materialize. Member reads through the undetermined usage stay `<undetermined>` of the member's multiplicity (never `ErrNoSuchFeature`).
- **State sub-actions.** On a state object no performance has run, `doAction` and `exitAction` (`Action[0..1]`) are materialized `Actions::Action` objects while `entryAction` redefined by `entry;` is refused (previous table).
- **Directed occurrence usages** (`in action x`, `in state x`, `in requirement x`, `in constraint x`, `in occurrence x`) are empty on the object (a part's directed usage is no behavior parameter: nothing invokes a part, so a lower bound of 0 nothing values is met by none), while `in part x`, `in item x`, `in port x` materialize an object. `in connection x` fails with "connector end cannot be attached".
- `ShapeItems::Box` edge and vertex bindings fail with "binding end cannot be resolved" — already pinned in `instance_library_geometry_box.expected.json`.
- Bare function names (`size(xs)`, `xs->collect{…}`, `abs(-3)`, `sqrt(4.0)`) resolve only under an import of their package; the pilot refuses them the same way without the import, and every one answers correctly with it.
- Qualified invocations whose argument is a type (`BaseFunctions::'istype'(1, Integer)`, `'all'(E)`, `'meta'(P1, SysML::PartDefinition)`, `'['(2.0, SI::m)`) are refused with a typed error where the operator form answers; the pilot refuses them too (`Must correspond to one input parameter`).
- `ComplexFunctions::ToComplex(String)` is a typed refusal (the library defines no notation); the pilot leaves it unevaluated. `TrigFunctions` take radians (`sin(90.0)` = `0.894`, pilot agrees); `RealFunctions::arg` is `0.0` by its library text.

## Sweep 1 — Systems and Domain library features read on an instance

Probe model: one `part def P1` with nested usages of every kind, a port with items, a requirement with `subject`/`actor`/`stakeholder`, a constraint with parameters, plus package-level usages of every kind, Geometry (`ShapeItems::Box`, `Sphere`, `SpatialItem`), `Time::Clock`, Cause and Effect, Metadata and Quantities usages, and `new <Def>()` for every definition. Tier `kernel` members are skipped (sibling branches); `FRAME` marks a member the semantics class as the Kernel frame.

### Objects probed

| Expression | Evaluates to |
|---|---|
| `Probe::p1` | object:Probe::p1 |
| `Probe::p2` | object:Probe::p2 |
| `Probe::i1` | object:Probe::i1 |
| `Probe::pt` | undetermined <undetermined> |
| `Probe::act` | undetermined <undetermined> |
| `Probe::st` | undetermined <undetermined> |
| `Probe::r1` | undetermined <undetermined> |
| `Probe::c1` | undetermined <undetermined> |
| `Probe::k1` | function Probe::k1 |
| `Probe::ca1` | undetermined <undetermined> |
| `Probe::an1` | error no value: calc usage Probe::an1 computes output features (no output feature); read one of them |
| `Probe::v1` | error no value: calc usage Probe::v1 computes output features (no output feature); read one of them |
| `Probe::u1` | undetermined <undetermined> |
| `Probe::vw1` | undetermined <undetermined> |
| `Probe::vp1` | undetermined <undetermined> |
| `Probe::con1` | undetermined <undetermined> |
| `Probe::rd1` | undetermined <undetermined> |
| `Probe::cn1` | undetermined <undetermined> |
| `Probe::if1` | undetermined <undetermined> |
| `Probe::al1` | undetermined <undetermined> |
| `Probe::fl1` | undetermined <undetermined> |
| `Probe::o1` | object:Probe::o1 |
| `Probe::e1` | enumeration literal E1::a |
| `Probe::at1` | object:Probe::at1 |
| `Probe::mass1` | quantity 2.0 [kg] |
| `Probe::len1` | quantity 2.0 [m] |
| `Probe::q` | undetermined <undetermined> |
| `Probe::box` | object:Probe::box |
| `Probe::sph` | object:Probe::sph |
| `Probe::clk` | object:Probe::clk |
| `Probe::spat` | object:Probe::spat |
| `Probe::sys` | object:Probe::sys |
| `Probe::der` | undetermined <undetermined> |
| `Probe::risk` | error unresolved reference: RiskMetadata::RiskLevel::high |
| `Probe::dsub` | object:Probe::dsub |
| `Probe::psi` | object:Probe::psi |
| `Probe::p1.sub` | collection(2) [instance(7), instance(8)] |
| `Probe::p1.opt` | undetermined <undetermined> |
| `Probe::p1.cargo` | object:Probe::I1 |
| `Probe::p1.pt` | object:Probe::Pt |
| `Probe::p1.act` | object:Probe::A1 |
| `Probe::p1.st` | object:Probe::S1 |
| `Probe::p1.other` | error uninitialized feature value: other |
| `Probe::p1.otherItem` | error uninitialized feature value: otherItem |
| `Probe::p1.others` | undetermined <undetermined> |
| `Probe::p1.rq` | object:Probe::R1 |
| `Probe::p1.cq` | object:Probe::P1::cq |
| `Probe::p1.kq` | constant 6.0 |
| `Probe::p1.caq` | ERR_NOSUCHFEATURE object has no such feature: member caq not found in instance |
| `Probe::p1.anq` | error no value: calc usage anq computes output features (no output feature); read one of them |
| `Probe::p1.vq` | error no value: calc usage vq computes output features (no output feature); read one of them |
| `Probe::p1.uq` | ERR_NOSUCHFEATURE object has no such feature: member uq not found in instance |
| `Probe::p1.vwq` | ERR_NOSUCHFEATURE object has no such feature: member vwq not found in instance |
| `Probe::p1.vpq` | ERR_NOSUCHFEATURE object has no such feature: member vpq not found in instance |
| `Probe::p1.conq` | ERR_NOSUCHFEATURE object has no such feature: member conq not found in instance |
| `Probe::p1.rdq` | ERR_NOSUCHFEATURE object has no such feature: member rdq not found in instance |
| `Probe::p1.cnq` | object:Probe::Cn1 |
| `Probe::p1.ifq` | object:Probe::If1 |
| `Probe::p1.alq` | object:Probe::Al1 |
| `Probe::p1.flq` | object:Probe::Fl1 |
| `Probe::p1.oq` | object:Probe::O1 |
| `Probe::p1.eq` | ERR_NOSUCHFEATURE object has no such feature: member eq not found in instance |
| `Probe::p1.atq` | object:Probe::At1 |
| `Probe::p1.mq` | quantity 2.0 [kg] |
| `Probe::p1.vq3` | error uninitialized feature value: vq3 |
| `Probe::p1.snap` | object:Probe::P1::snap |
| `Probe::p1.slice` | object:Probe::P1::slice |
| `Probe::p1.ev` | object:Probe::P1::ev |
| `Probe::p1.pa` | object:Probe::A2 |
| `Probe::p1.es` | object:Probe::S1 |
| `Probe::p1.sr` | ERR_NOSUCHFEATURE object has no such feature: member sr not found in instance |
| `Probe::p1.ac` | object:Probe::P1::ac |
| `Probe::p1.iu` | ERR_NOSUCHFEATURE object has no such feature: member iu not found in instance |
| `Probe::p1.mass` | collection(1) [3.0 [kg]] |
| `new Probe::P1()` | object:Probe::P1 |
| `new Probe::P2()` | object:Probe::P2 |
| `new Probe::I1()` | object:Probe::I1 |
| `new Probe::Pt()` | object:Probe::Pt |
| `new Probe::A1()` | object:Probe::A1 |
| `new Probe::S1()` | object:Probe::S1 |
| `new Probe::R1()` | object:Probe::R1 |
| `new Probe::C1()` | object:Probe::C1 |
| `new Probe::K1()` | object:Probe::K1 |
| `new Probe::Ca1()` | object:Probe::Ca1 |
| `new Probe::An1()` | object:Probe::An1 |
| `new Probe::V1()` | object:Probe::V1 |
| `new Probe::U1()` | object:Probe::U1 |
| `new Probe::Vw1()` | object:Probe::Vw1 |
| `new Probe::Vp1()` | object:Probe::Vp1 |
| `new Probe::Con1()` | object:Probe::Con1 |
| `new Probe::Rd1()` | object:Probe::Rd1 |
| `new Probe::Cn1()` | object:Probe::Cn1 |
| `new Probe::If1()` | object:Probe::If1 |
| `new Probe::Al1()` | object:Probe::Al1 |
| `new Probe::Fl1()` | object:Probe::Fl1 |
| `new Probe::O1()` | object:Probe::O1 |
| `new Probe::E1()` | object:Probe::E1 |
| `new Probe::M1()` | object:Probe::M1 |
| `new Probe::At1()` | object:Probe::At1 |

### Member reads

| Object | Member | Declared by | Outcome | Bucket | Verdict |
|---|---|---|---|---|---|
| `Probe::p1` | `a` | `Probe::P1::a` (model) | constant 2.5 | answers | ok |
| `Probe::p1` | `ac` | `Probe::P1::ac` (model) | object:Probe::P1::ac | answers | ok |
| `Probe::p1` | `act` | `Probe::P1::act` (model) | object:Probe::A1 | answers | ok |
| `Probe::p1` | `alq` | `Probe::P1::alq` (model) | object:Probe::Al1 | answers | ok |
| `Probe::p1` | `anq` | `Probe::P1::anq` (model) | error no value: calc usage anq computes output features (no output feature); read one of them | typed error | gap: case usage read as a calc |
| `Probe::p1` | `atq` | `Probe::P1::atq` (model) | object:Probe::At1 | answers | ok |
| `Probe::p1` | `boundingShapes` | `Items::Item::boundingShapes` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1` | `caq` | `Probe::P1::caq` (model) | no such feature: caq | error (`ErrNoSuchFeature`) | gap: usage kind left out of the shape |
| `Probe::p1` | `cargo` | `Probe::P1::cargo` (model) | object:Probe::I1 | answers | ok |
| `Probe::p1` | `checkedConstraints` | `Items::Item::checkedConstraints` (systems) | collection(3) [instance(11), instance(12), instance(4)] | answers | ok |
| `Probe::p1` | `cnq` | `Probe::P1::cnq` (model) | object:Probe::Cn1 | answers | ok |
| `Probe::p1` | `conq` | `Probe::P1::conq` (model) | no such feature: conq | error (`ErrNoSuchFeature`) | gap: usage kind left out of the shape |
| `Probe::p1` | `cq` | `Probe::P1::cq` (model) | object:Probe::P1::cq | answers | ok |
| `Probe::p1` | `done` | `Parts::Part::done` (systemsFRAME) | no such feature: done | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::p1` | `envelopingShapes` | `Items::Item::envelopingShapes` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1` | `eq` | `Probe::P1::eq` (model) | no such feature: eq | error (`ErrNoSuchFeature`) | gap: usage kind left out of the shape |
| `Probe::p1` | `es` | `Probe::P1::es` (model) | object:Probe::S1 | answers | ok |
| `Probe::p1` | `ev` | `Probe::P1::ev` (model) | object:Probe::P1::ev | answers | ok |
| `Probe::p1` | `exhibitedStates` | `Parts::Part::exhibitedStates` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1` | `flq` | `Probe::P1::flq` (model) | object:Probe::Fl1 | answers | ok |
| `Probe::p1` | `ifq` | `Probe::P1::ifq` (model) | object:Probe::If1 | answers | ok |
| `Probe::p1` | `isSolid` | `Items::Item::isSolid` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1` | `iu` | `Probe::P1::iu` (model) | no such feature: iu | error (`ErrNoSuchFeature`) | gap: usage kind left out of the shape |
| `Probe::p1` | `kq` | `Probe::P1::kq` (model) | constant 6.0 | answers | ok |
| `Probe::p1` | `mass` | `Probe::P1::mass` (model) | collection(1) [3.0 [kg]] | answers | ok |
| `Probe::p1` | `mq` | `Probe::P1::mq` (model) | quantity 2.0 [kg] | answers | ok |
| `Probe::p1` | `n` | `Probe::P1::n` (model) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1` | `opt` | `Probe::P1::opt` (model) | empty | answers | ok |
| `Probe::p1` | `oq` | `Probe::P1::oq` (model) | object:Probe::O1 | answers | ok |
| `Probe::p1` | `other` | `Probe::P1::other` (model) | uninitialized feature value: other | typed error (`errUninitializedFeatureValue`) | gap: unbound reference read refused |
| `Probe::p1` | `otherItem` | `Probe::P1::otherItem` (model) | uninitialized feature value: otherItem | typed error (`errUninitializedFeatureValue`) | gap: unbound reference read refused |
| `Probe::p1` | `others` | `Probe::P1::others` (model) | empty | answers | ok |
| `Probe::p1` | `ownedActions` | `Parts::Part::ownedActions` (systems) | collection(3) [instance(5), instance(15), instance(22)] | answers | ok |
| `Probe::p1` | `ownedPorts` | `Parts::Part::ownedPorts` (systems) | collection(1) [instance(16)] | answers | ok |
| `Probe::p1` | `ownedStates` | `Parts::Part::ownedStates` (systems) | collection(1) [instance(22)] | answers | ok |
| `Probe::p1` | `pa` | `Probe::P1::pa` (model) | object:Probe::A2 | answers | ok |
| `Probe::p1` | `performedActions` | `Parts::Part::performedActions` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1` | `pt` | `Probe::P1::pt` (model) | object:Probe::Pt | answers | ok |
| `Probe::p1` | `rdq` | `Probe::P1::rdq` (model) | no such feature: rdq | error (`ErrNoSuchFeature`) | gap: usage kind left out of the shape |
| `Probe::p1` | `rq` | `Probe::P1::rq` (model) | object:Probe::R1 | answers | ok |
| `Probe::p1` | `self` | `Parts::Part::self` (systemsFRAME) | no such feature: self | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::p1` | `shape` | `Items::Item::shape` (systems) | empty | answers | ok |
| `Probe::p1` | `slice` | `Probe::P1::slice` (model) | object:Probe::P1::slice | answers | ok |
| `Probe::p1` | `snap` | `Probe::P1::snap` (model) | object:Probe::P1::snap | answers | ok |
| `Probe::p1` | `sr` | `Probe::P1::sr` (model) | no such feature: sr | error (`ErrNoSuchFeature`) | gap: usage kind left out of the shape |
| `Probe::p1` | `st` | `Probe::P1::st` (model) | object:Probe::S1 | answers | ok |
| `Probe::p1` | `start` | `Parts::Part::start` (systemsFRAME) | no such feature: start | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::p1` | `sub` | `Probe::P1::sub` (model) | collection(2) [instance(7), instance(8)] | answers | ok |
| `Probe::p1` | `subitems` | `Items::Item::subitems` (systems) | collection(6) [instance(10), instance(7), instance(8), instance(13), instance(19), instance(6)] | answers | ok |
| `Probe::p1` | `subparts` | `Items::Item::subparts` (systems) | collection(5) [instance(7), instance(8), instance(13), instance(19), instance(6)] | answers | ok |
| `Probe::p1` | `uq` | `Probe::P1::uq` (model) | no such feature: uq | error (`ErrNoSuchFeature`) | gap: usage kind left out of the shape |
| `Probe::p1` | `voids` | `Items::Item::voids` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1` | `vpq` | `Probe::P1::vpq` (model) | no such feature: vpq | error (`ErrNoSuchFeature`) | gap: usage kind left out of the shape |
| `Probe::p1` | `vq` | `Probe::P1::vq` (model) | error no value: calc usage vq computes output features (no output feature); read one of them | typed error | gap: case usage read as a calc |
| `Probe::p1` | `vq3` | `Probe::P1::vq3` (model) | uninitialized feature value: vq3 | typed error (`errUninitializedFeatureValue`) | gap: unbound reference read refused |
| `Probe::p1` | `vwq` | `Probe::P1::vwq` (model) | no such feature: vwq | error (`ErrNoSuchFeature`) | gap: usage kind left out of the shape |
| `Probe::p2` | `boundingShapes` | `Items::Item::boundingShapes` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p2` | `checkedConstraints` | `Items::Item::checkedConstraints` (systems) | empty | answers | ok |
| `Probe::p2` | `done` | `Parts::Part::done` (systemsFRAME) | no such feature: done | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::p2` | `envelopingShapes` | `Items::Item::envelopingShapes` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p2` | `exhibitedStates` | `Parts::Part::exhibitedStates` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p2` | `isSolid` | `Items::Item::isSolid` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p2` | `ownedActions` | `Parts::Part::ownedActions` (systems) | empty | answers | ok |
| `Probe::p2` | `ownedPorts` | `Parts::Part::ownedPorts` (systems) | empty | answers | ok |
| `Probe::p2` | `ownedStates` | `Parts::Part::ownedStates` (systems) | empty | answers | ok |
| `Probe::p2` | `performedActions` | `Parts::Part::performedActions` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p2` | `self` | `Parts::Part::self` (systemsFRAME) | no such feature: self | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::p2` | `shape` | `Items::Item::shape` (systems) | empty | answers | ok |
| `Probe::p2` | `start` | `Parts::Part::start` (systemsFRAME) | no such feature: start | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::p2` | `subitems` | `Items::Item::subitems` (systems) | empty | answers | ok |
| `Probe::p2` | `subparts` | `Items::Item::subparts` (systems) | empty | answers | ok |
| `Probe::p2` | `voids` | `Items::Item::voids` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::i1` | `boundingShapes` | `Items::Item::boundingShapes` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::i1` | `checkedConstraints` | `Items::Item::checkedConstraints` (systems) | empty | answers | ok |
| `Probe::i1` | `done` | `Items::Item::done` (systemsFRAME) | no such feature: done | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::i1` | `envelopingShapes` | `Items::Item::envelopingShapes` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::i1` | `isSolid` | `Items::Item::isSolid` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::i1` | `self` | `Items::Item::self` (systemsFRAME) | no such feature: self | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::i1` | `shape` | `Items::Item::shape` (systems) | empty | answers | ok |
| `Probe::i1` | `start` | `Items::Item::start` (systemsFRAME) | no such feature: start | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::i1` | `subitems` | `Items::Item::subitems` (systems) | empty | answers | ok |
| `Probe::i1` | `subparts` | `Items::Item::subparts` (systems) | empty | answers | ok |
| `Probe::i1` | `voids` | `Items::Item::voids` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::i1` | `w` | `Probe::I1::w` (model) | constant 1.0 | answers | ok |
| `Probe::at1` | `v` | `Probe::At1::v` (model) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::box` | `bf` | `ShapeItems::RectangularCuboid::bf` (domain) | object:ShapeItems::RectangularCuboid::bf | answers | ok |
| `Probe::box` | `bfe` | `ShapeItems::CuboidOrTriangularPrism::bfe` (domain) | error binding end cannot be resolved: box.bfe is bound by `binding [1] bind [0..1] bf.edges = [0..1] bfe`, whi… | typed error | known: pinned by `instance_library_geometry_box` fixtures |
| `Probe::box` | `bflv` | `ShapeItems::CuboidOrTriangularPrism::bflv` (domain) | error binding end cannot be resolved: box.bfe is bound by `binding [1] bind [0..1] bf.edges = [0..1] bfe`, whi… | typed error | known: pinned by `instance_library_geometry_box` fixtures |
| `Probe::box` | `bfrv` | `ShapeItems::CuboidOrTriangularPrism::bfrv` (domain) | error binding end cannot be resolved: box.bfe is bound by `binding [1] bind [0..1] bf.edges = [0..1] bfe`, whi… | typed error | known: pinned by `instance_library_geometry_box` fixtures |
| `Probe::box` | `boundingShapes` | `Items::Item::boundingShapes` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::box` | `bre` | `ShapeItems::CuboidOrTriangularPrism::bre` (domain) | error binding end cannot be resolved: box.bre is bound by `binding [1] bind [0..1] bf.edges = [0..1] bre`, whi… | typed error | known: pinned by `instance_library_geometry_box` fixtures |
| `Probe::box` | `brlv` | `ShapeItems::CuboidOrTriangularPrism::brlv` (domain) | error binding end cannot be resolved: box.bre is bound by `binding [1] bind [0..1] bf.edges = [0..1] bre`, whi… | typed error | known: pinned by `instance_library_geometry_box` fixtures |
| `Probe::box` | `brrv` | `ShapeItems::CuboidOrTriangularPrism::brrv` (domain) | error binding end cannot be resolved: box.bre is bound by `binding [1] bind [0..1] bf.edges = [0..1] bre`, whi… | typed error | known: pinned by `instance_library_geometry_box` fixtures |
| `Probe::box` | `bsle` | `ShapeItems::CuboidOrTriangularPrism::bsle` (domain) | error binding end cannot be resolved: box.bsle is bound by `binding [1] bind [0..1] bf.edges = [0..1] bsle`, w… | typed error | known: pinned by `instance_library_geometry_box` fixtures |
| `Probe::box` | `bsre` | `ShapeItems::CuboidOrTriangularPrism::bsre` (domain) | error binding end cannot be resolved: box.bsre is bound by `binding [1] bind [0..1] bf.edges = [0..1] bsre`, w… | typed error | known: pinned by `instance_library_geometry_box` fixtures |
| `Probe::box` | `checkedConstraints` | `Items::Item::checkedConstraints` (systems) | empty | answers | ok |
| `Probe::box` | `done` | `Items::Item::done` (systemsFRAME) | no such feature: done | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::box` | `edges` | `ShapeItems::Cuboid::edges` (domain) | collection(24) [instance(37), instance(38), instance(39), instance(40), instance(41), instance(42), instance(4… | answers | ok |
| `Probe::box` | `envelopingShapes` | `Items::Item::envelopingShapes` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::box` | `faces` | `ShapeItems::Cuboid::faces` (domain) | collection(6) [instance(34), instance(31), instance(32), instance(33), instance(35), instance(36)] | answers | ok |
| `Probe::box` | `ff` | `ShapeItems::RectangularCuboid::ff` (domain) | object:ShapeItems::RectangularCuboid::ff | answers | ok |
| `Probe::box` | `genus` | `ShapeItems::Polyhedron::genus` (domain) | constant 0 | answers | ok |
| `Probe::box` | `height` | `Probe::box::height` (model) | quantity 3.0 [m] | answers | ok |
| `Probe::box` | `isClosed` | `ShapeItems::Polyhedron::isClosed` (domain) | constant true | answers | ok |
| `Probe::box` | `isSolid` | `Items::Item::isSolid` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::box` | `length` | `Probe::box::length` (model) | quantity 1.0 [m] | answers | ok |
| `Probe::box` | `outerSpaceDimension` | `ShapeItems::Polyhedron::outerSpaceDimension` (domain) | constant 3 | answers | ok |
| `Probe::box` | `rf` | `ShapeItems::RectangularCuboid::rf` (domain) | object:ShapeItems::RectangularCuboid::rf | answers | ok |
| `Probe::box` | `self` | `Items::Item::self` (systemsFRAME) | no such feature: self | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::box` | `shape` | `Items::Item::shape` (systems) | empty | answers | ok |
| `Probe::box` | `slf` | `ShapeItems::RectangularCuboid::slf` (domain) | object:ShapeItems::RectangularCuboid::slf | answers | ok |
| `Probe::box` | `srf` | `ShapeItems::RectangularCuboid::srf` (domain) | object:ShapeItems::RectangularCuboid::srf | answers | ok |
| `Probe::box` | `start` | `Items::Item::start` (systemsFRAME) | no such feature: start | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::box` | `subitems` | `Items::Item::subitems` (systems) | empty | answers | ok |
| `Probe::box` | `subparts` | `Items::Item::subparts` (systems) | empty | answers | ok |
| `Probe::box` | `tf` | `ShapeItems::RectangularCuboid::tf` (domain) | object:ShapeItems::RectangularCuboid::tf | answers | ok |
| `Probe::box` | `tfe` | `ShapeItems::CuboidOrTriangularPrism::tfe` (domain) | error binding end cannot be resolved: box.tfe is bound by `binding [1] bind [0..1] tf.edges = [0..1] tfe`, whi… | typed error | known: pinned by `instance_library_geometry_box` fixtures |
| `Probe::box` | `tflv` | `ShapeItems::CuboidOrTriangularPrism::tflv` (domain) | error binding end cannot be resolved: box.tfe is bound by `binding [1] bind [0..1] tf.edges = [0..1] tfe`, whi… | typed error | known: pinned by `instance_library_geometry_box` fixtures |
| `Probe::box` | `tfrv` | `ShapeItems::CuboidOrTriangularPrism::tfrv` (domain) | error binding end cannot be resolved: box.tfe is bound by `binding [1] bind [0..1] tf.edges = [0..1] tfe`, whi… | typed error | known: pinned by `instance_library_geometry_box` fixtures |
| `Probe::box` | `tre` | `ShapeItems::CuboidOrTriangularPrism::tre` (domain) | error binding end cannot be resolved: box.tre is bound by `binding [1] bind [0..1] tf.edges = [0..1] tre`, whi… | typed error | known: pinned by `instance_library_geometry_box` fixtures |
| `Probe::box` | `trlv` | `ShapeItems::CuboidOrTriangularPrism::trlv` (domain) | error binding end cannot be resolved: box.tre is bound by `binding [1] bind [0..1] tf.edges = [0..1] tre`, whi… | typed error | known: pinned by `instance_library_geometry_box` fixtures |
| `Probe::box` | `trrv` | `ShapeItems::CuboidOrTriangularPrism::trrv` (domain) | error binding end cannot be resolved: box.tre is bound by `binding [1] bind [0..1] tf.edges = [0..1] tre`, whi… | typed error | known: pinned by `instance_library_geometry_box` fixtures |
| `Probe::box` | `tsle` | `ShapeItems::CuboidOrTriangularPrism::tsle` (domain) | error binding end cannot be resolved: box.tsle is bound by `binding [1] bind [0..1] tf.edges = [0..1] tsle`, w… | typed error | known: pinned by `instance_library_geometry_box` fixtures |
| `Probe::box` | `tsre` | `ShapeItems::CuboidOrTriangularPrism::tsre` (domain) | error binding end cannot be resolved: box.tsre is bound by `binding [1] bind [0..1] tf.edges = [0..1] tsre`, w… | typed error | known: pinned by `instance_library_geometry_box` fixtures |
| `Probe::box` | `ufle` | `ShapeItems::CuboidOrTriangularPrism::ufle` (domain) | error binding end cannot be resolved: box.ufle is bound by `binding [1] bind [0..1] ff.edges = [0..1] ufle`, w… | typed error | known: pinned by `instance_library_geometry_box` fixtures |
| `Probe::box` | `ufre` | `ShapeItems::CuboidOrTriangularPrism::ufre` (domain) | error binding end cannot be resolved: box.ufre is bound by `binding [1] bind [0..1] ff.edges = [0..1] ufre`, w… | typed error | known: pinned by `instance_library_geometry_box` fixtures |
| `Probe::box` | `urle` | `ShapeItems::CuboidOrTriangularPrism::urle` (domain) | error binding end cannot be resolved: box.urle is bound by `binding [1] bind [0..1] rf.edges = [0..1] urle`, w… | typed error | known: pinned by `instance_library_geometry_box` fixtures |
| `Probe::box` | `urre` | `ShapeItems::CuboidOrTriangularPrism::urre` (domain) | error binding end cannot be resolved: box.urre is bound by `binding [1] bind [0..1] rf.edges = [0..1] urre`, w… | typed error | known: pinned by `instance_library_geometry_box` fixtures |
| `Probe::box` | `vertices` | `ShapeItems::Cuboid::vertices` (domain) | error subsetting feature tflv of vertices: binding end cannot be resolved: box.tfe is bound by `binding [1] bi… | typed error | known: pinned by `instance_library_geometry_box` fixtures |
| `Probe::box` | `voids` | `Items::Item::voids` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::box` | `width` | `Probe::box::width` (model) | quantity 2.0 [m] | answers | ok |
| `Probe::sph` | `boundingShapes` | `Items::Item::boundingShapes` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::sph` | `checkedConstraints` | `Items::Item::checkedConstraints` (systems) | empty | answers | ok |
| `Probe::sph` | `done` | `Items::Item::done` (systemsFRAME) | no such feature: done | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::sph` | `edges` | `ShapeItems::ConicSurface::edges` (domain) | empty | answers | ok |
| `Probe::sph` | `envelopingShapes` | `Items::Item::envelopingShapes` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::sph` | `faces` | `ShapeItems::Ellipsoid::faces` (domain) | object:ShapeItems::Ellipsoid::faces | answers | ok |
| `Probe::sph` | `genus` | `ShapeItems::ConicSurface::genus` (domain) | constant 0 | answers | ok |
| `Probe::sph` | `isSolid` | `Items::Item::isSolid` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::sph` | `radius` | `Probe::sph::radius` (model) | quantity 1.0 [m] | answers | ok |
| `Probe::sph` | `self` | `Items::Item::self` (systemsFRAME) | no such feature: self | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::sph` | `semiAxis1` | `ShapeItems::Sphere::semiAxis1` (domain) | quantity 1.0 [m] | answers | ok |
| `Probe::sph` | `semiAxis2` | `ShapeItems::Sphere::semiAxis2` (domain) | quantity 1.0 [m] | answers | ok |
| `Probe::sph` | `semiAxis3` | `ShapeItems::Sphere::semiAxis3` (domain) | quantity 1.0 [m] | answers | ok |
| `Probe::sph` | `shape` | `Items::Item::shape` (systems) | empty | answers | ok |
| `Probe::sph` | `start` | `Items::Item::start` (systemsFRAME) | no such feature: start | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::sph` | `subitems` | `Items::Item::subitems` (systems) | empty | answers | ok |
| `Probe::sph` | `subparts` | `Items::Item::subparts` (systems) | empty | answers | ok |
| `Probe::sph` | `vertices` | `ShapeItems::ConicSurface::vertices` (domain) | empty | answers | ok |
| `Probe::sph` | `voids` | `Items::Item::voids` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::clk` | `boundingShapes` | `Items::Item::boundingShapes` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::clk` | `checkedConstraints` | `Items::Item::checkedConstraints` (systems) | empty | answers | ok |
| `Probe::clk` | `currentTime` | `Time::Clock::currentTime` (domain) | quantity 0.0 [s] | answers | ok |
| `Probe::clk` | `done` | `Parts::Part::done` (systemsFRAME) | no such feature: done | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::clk` | `envelopingShapes` | `Items::Item::envelopingShapes` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::clk` | `exhibitedStates` | `Parts::Part::exhibitedStates` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::clk` | `isSolid` | `Items::Item::isSolid` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::clk` | `ownedActions` | `Parts::Part::ownedActions` (systems) | empty | answers | ok |
| `Probe::clk` | `ownedPorts` | `Parts::Part::ownedPorts` (systems) | empty | answers | ok |
| `Probe::clk` | `ownedStates` | `Parts::Part::ownedStates` (systems) | empty | answers | ok |
| `Probe::clk` | `performedActions` | `Parts::Part::performedActions` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::clk` | `self` | `Parts::Part::self` (systemsFRAME) | no such feature: self | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::clk` | `shape` | `Items::Item::shape` (systems) | empty | answers | ok |
| `Probe::clk` | `start` | `Parts::Part::start` (systemsFRAME) | no such feature: start | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::clk` | `subitems` | `Items::Item::subitems` (systems) | empty | answers | ok |
| `Probe::clk` | `subparts` | `Items::Item::subparts` (systems) | empty | answers | ok |
| `Probe::clk` | `voids` | `Items::Item::voids` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::spat` | `boundingShapes` | `Items::Item::boundingShapes` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::spat` | `checkedConstraints` | `Items::Item::checkedConstraints` (systems) | empty | answers | ok |
| `Probe::spat` | `componentItems` | `SpatialItems::SpatialItem::componentItems` (domain) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::spat` | `componentParts` | `SpatialItems::SpatialItem::componentParts` (domain) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::spat` | `coordinateFrame` | `SpatialItems::SpatialItem::coordinateFrame` (domain) | error feature value spat.coordinateFrame: no value for feature universalCartesianSpatial3dCoordinateFrame | typed error | gap: Domain default not evaluable |
| `Probe::spat` | `done` | `Items::Item::done` (systemsFRAME) | no such feature: done | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::spat` | `envelopingShapes` | `Items::Item::envelopingShapes` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::spat` | `isSolid` | `Items::Item::isSolid` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::spat` | `localClock` | `SpatialItems::SpatialItem::localClock` (domain) | object:Time::universalClock | answers | ok |
| `Probe::spat` | `originPoint` | `SpatialItems::SpatialItem::originPoint` (domain) | object:Objects::Point | answers | ok |
| `Probe::spat` | `originPointConstraint` | `SpatialItems::SpatialItem::originPointConstraint` (domain) | uninitialized feature value: originPointConstraint | typed error (`errUninitializedFeatureValue`) | gap: unbound reference read refused |
| `Probe::spat` | `self` | `Items::Item::self` (systemsFRAME) | no such feature: self | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::spat` | `shape` | `Items::Item::shape` (systems) | empty | answers | ok |
| `Probe::spat` | `start` | `Items::Item::start` (systemsFRAME) | no such feature: start | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::spat` | `subSpatialItems` | `SpatialItems::SpatialItem::subSpatialItems` (domain) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::spat` | `subSpatialParts` | `SpatialItems::SpatialItem::subSpatialParts` (domain) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::spat` | `subitems` | `Items::Item::subitems` (systems) | collection(1) [instance(67)] | answers | ok |
| `Probe::spat` | `subparts` | `Items::Item::subparts` (systems) | collection(1) [instance(67)] | answers | ok |
| `Probe::spat` | `voids` | `Items::Item::voids` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::sys` | `boundingShapes` | `Items::Item::boundingShapes` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::sys` | `checkedConstraints` | `Items::Item::checkedConstraints` (systems) | empty | answers | ok |
| `Probe::sys` | `done` | `Parts::Part::done` (systemsFRAME) | no such feature: done | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::sys` | `envelopingShapes` | `Items::Item::envelopingShapes` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::sys` | `exhibitedStates` | `Parts::Part::exhibitedStates` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::sys` | `isSolid` | `Items::Item::isSolid` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::sys` | `ownedActions` | `Parts::Part::ownedActions` (systems) | empty | answers | ok |
| `Probe::sys` | `ownedPorts` | `Parts::Part::ownedPorts` (systems) | empty | answers | ok |
| `Probe::sys` | `ownedStates` | `Parts::Part::ownedStates` (systems) | empty | answers | ok |
| `Probe::sys` | `performedActions` | `Parts::Part::performedActions` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::sys` | `self` | `Parts::Part::self` (systemsFRAME) | no such feature: self | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::sys` | `shape` | `Items::Item::shape` (systems) | empty | answers | ok |
| `Probe::sys` | `start` | `Parts::Part::start` (systemsFRAME) | no such feature: start | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::sys` | `subitems` | `Items::Item::subitems` (systems) | empty | answers | ok |
| `Probe::sys` | `subparts` | `Items::Item::subparts` (systems) | empty | answers | ok |
| `Probe::sys` | `voids` | `Items::Item::voids` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::dsub` | `boundingShapes` | `Items::Item::boundingShapes` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::dsub` | `checkedConstraints` | `Items::Item::checkedConstraints` (systems) | empty | answers | ok |
| `Probe::dsub` | `done` | `Parts::Part::done` (systemsFRAME) | no such feature: done | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::dsub` | `envelopingShapes` | `Items::Item::envelopingShapes` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::dsub` | `exhibitedStates` | `Parts::Part::exhibitedStates` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::dsub` | `isSolid` | `Items::Item::isSolid` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::dsub` | `ownedActions` | `Parts::Part::ownedActions` (systems) | empty | answers | ok |
| `Probe::dsub` | `ownedPorts` | `Parts::Part::ownedPorts` (systems) | empty | answers | ok |
| `Probe::dsub` | `ownedStates` | `Parts::Part::ownedStates` (systems) | empty | answers | ok |
| `Probe::dsub` | `performedActions` | `Parts::Part::performedActions` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::dsub` | `self` | `Parts::Part::self` (systemsFRAME) | no such feature: self | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::dsub` | `shape` | `Items::Item::shape` (systems) | empty | answers | ok |
| `Probe::dsub` | `start` | `Parts::Part::start` (systemsFRAME) | no such feature: start | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::dsub` | `subitems` | `Items::Item::subitems` (systems) | empty | answers | ok |
| `Probe::dsub` | `subparts` | `Items::Item::subparts` (systems) | empty | answers | ok |
| `Probe::dsub` | `voids` | `Items::Item::voids` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::psi` | `annotatedElement` | `ParametersOfInterestMetadata::MeasureOfEffectiveness::annotatedElement` (domain) | object:SysML::Systems::Usage | answers | ok |
| `Probe::psi` | `baseType` | `ParametersOfInterestMetadata::MeasureOfEffectiveness::baseType` (domain) | metaobject meta(ParametersOfInterestMetadata::measuresOfEffectiveness : SysML::Systems::AttributeUsage) | answers | ok |
| `Probe::psi` | `boundingShapes` | `Items::Item::boundingShapes` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::psi` | `checkedConstraints` | `Items::Item::checkedConstraints` (systems) | empty | answers | ok |
| `Probe::psi` | `done` | `Parts::Part::done` (systemsFRAME) | no such feature: done | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::psi` | `envelopingShapes` | `Items::Item::envelopingShapes` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::psi` | `exhibitedStates` | `Parts::Part::exhibitedStates` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::psi` | `isSolid` | `Items::Item::isSolid` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::psi` | `ownedActions` | `Parts::Part::ownedActions` (systems) | empty | answers | ok |
| `Probe::psi` | `ownedPorts` | `Parts::Part::ownedPorts` (systems) | empty | answers | ok |
| `Probe::psi` | `ownedStates` | `Parts::Part::ownedStates` (systems) | empty | answers | ok |
| `Probe::psi` | `performedActions` | `Parts::Part::performedActions` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::psi` | `self` | `Parts::Part::self` (systemsFRAME) | no such feature: self | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::psi` | `shape` | `Items::Item::shape` (systems) | empty | answers | ok |
| `Probe::psi` | `start` | `Parts::Part::start` (systemsFRAME) | no such feature: start | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::psi` | `subitems` | `Items::Item::subitems` (systems) | empty | answers | ok |
| `Probe::psi` | `subparts` | `Items::Item::subparts` (systems) | empty | answers | ok |
| `Probe::psi` | `voids` | `Items::Item::voids` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1.cargo` | `boundingShapes` | `Items::Item::boundingShapes` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1.cargo` | `checkedConstraints` | `Items::Item::checkedConstraints` (systems) | empty | answers | ok |
| `Probe::p1.cargo` | `done` | `Items::Item::done` (systemsFRAME) | no such feature: done | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::p1.cargo` | `envelopingShapes` | `Items::Item::envelopingShapes` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1.cargo` | `isSolid` | `Items::Item::isSolid` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1.cargo` | `self` | `Items::Item::self` (systemsFRAME) | no such feature: self | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::p1.cargo` | `shape` | `Items::Item::shape` (systems) | empty | answers | ok |
| `Probe::p1.cargo` | `start` | `Items::Item::start` (systemsFRAME) | no such feature: start | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::p1.cargo` | `subitems` | `Items::Item::subitems` (systems) | empty | answers | ok |
| `Probe::p1.cargo` | `subparts` | `Items::Item::subparts` (systems) | empty | answers | ok |
| `Probe::p1.cargo` | `voids` | `Items::Item::voids` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1.cargo` | `w` | `Probe::I1::w` (model) | constant 1.0 | answers | ok |
| `Probe::p1.pt` | `inflow` | `Probe::Pt::inflow` (model) | object:Probe::I1 | answers | ok |
| `Probe::p1.pt` | `interfacingPorts` | `Ports::Port::interfacingPorts` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1.pt` | `outflow` | `Probe::Pt::outflow` (model) | object:Probe::I1 | answers | ok |
| `Probe::p1.pt` | `outgoingTransfersFromSelf` | `Ports::Port::outgoingTransfersFromSelf` (systemsFRAME) | no such feature: outgoingTransfersFromSelf | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::p1.pt` | `self` | `Ports::Port::self` (systemsFRAME) | no such feature: self | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::p1.pt` | `subports` | `Ports::Port::subports` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1.pt` | `x` | `Probe::Pt::x` (model) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1.act` | `acceptSubactions` | `Actions::Action::acceptSubactions` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1.act` | `assignments` | `Actions::Action::assignments` (systems) | empty | answers | ok |
| `Probe::p1.act` | `controls` | `Actions::Action::controls` (systems) | empty | answers | ok |
| `Probe::p1.act` | `decisionTransitions` | `Actions::Action::decisionTransitions` (systems) | empty | answers | ok |
| `Probe::p1.act` | `decisions` | `Actions::Action::decisions` (systems) | empty | answers | ok |
| `Probe::p1.act` | `done` | `Actions::Action::done` (systemsFRAME) | no such feature: done | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::p1.act` | `forLoops` | `Actions::Action::forLoops` (systems) | empty | answers | ok |
| `Probe::p1.act` | `forks` | `Actions::Action::forks` (systems) | empty | answers | ok |
| `Probe::p1.act` | `i` | `Probe::A1::i` (model) | undetermined <undetermined> | `<undetermined>` | by design: unbound parameter |
| `Probe::p1.act` | `ifSubactions` | `Actions::Action::ifSubactions` (systems) | empty | answers | ok |
| `Probe::p1.act` | `incomingTransfers` | `Actions::Action::incomingTransfers` (systemsFRAME) | no such feature: incomingTransfers | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::p1.act` | `joins` | `Actions::Action::joins` (systems) | empty | answers | ok |
| `Probe::p1.act` | `loops` | `Actions::Action::loops` (systems) | empty | answers | ok |
| `Probe::p1.act` | `merges` | `Actions::Action::merges` (systems) | empty | answers | ok |
| `Probe::p1.act` | `nested` | `Probe::A1::nested` (model) | object:Probe::A2 | answers | ok |
| `Probe::p1.act` | `o` | `Probe::A1::o` (model) | undetermined <undetermined> | `<undetermined>` | by design: unbound parameter |
| `Probe::p1.act` | `self` | `Actions::Action::self` (systemsFRAME) | no such feature: self | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::p1.act` | `sendSubactions` | `Actions::Action::sendSubactions` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1.act` | `start` | `Actions::Action::start` (systemsFRAME) | no such feature: start | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::p1.act` | `subactions` | `Actions::Action::subactions` (systems) | collection(1) [object:Probe::A2#1] | answers | ok |
| `Probe::p1.act` | `terminateSubactions` | `Actions::Action::terminateSubactions` (systems) | empty | answers | ok |
| `Probe::p1.act` | `transitions` | `Actions::Action::transitions` (systems) | empty | answers | ok |
| `Probe::p1.act` | `whileLoops` | `Actions::Action::whileLoops` (systems) | empty | answers | ok |
| `Probe::p1.st` | `acceptSubactions` | `Actions::Action::acceptSubactions` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1.st` | `assignments` | `Actions::Action::assignments` (systems) | empty | answers | ok |
| `Probe::p1.st` | `controls` | `Actions::Action::controls` (systems) | empty | answers | ok |
| `Probe::p1.st` | `decisionTransitions` | `Actions::Action::decisionTransitions` (systems) | empty | answers | ok |
| `Probe::p1.st` | `decisions` | `Actions::Action::decisions` (systems) | empty | answers | ok |
| `Probe::p1.st` | `doAction` | `States::StateAction::doAction` (systems) | object:Actions::Action | answers | ok |
| `Probe::p1.st` | `done` | `States::StateAction::done` (systemsFRAME) | no such feature: done | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::p1.st` | `entryAction` | `States::StateAction::entryAction` (systems) | uninitialized feature value: entryAction | typed error (`errUninitializedFeatureValue`) | gap: unbound reference read refused |
| `Probe::p1.st` | `exclusiveStates` | `States::StateAction::exclusiveStates` (systems) | empty | answers | ok |
| `Probe::p1.st` | `exitAction` | `States::StateAction::exitAction` (systems) | object:Actions::Action | answers | ok |
| `Probe::p1.st` | `forLoops` | `Actions::Action::forLoops` (systems) | empty | answers | ok |
| `Probe::p1.st` | `forks` | `Actions::Action::forks` (systems) | empty | answers | ok |
| `Probe::p1.st` | `ifSubactions` | `Actions::Action::ifSubactions` (systems) | empty | answers | ok |
| `Probe::p1.st` | `incomingTransfers` | `Actions::Action::incomingTransfers` (systemsFRAME) | no such feature: incomingTransfers | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::p1.st` | `isTriggerDuring` | `States::StateAction::isTriggerDuring` (systems) | constant true | answers | ok |
| `Probe::p1.st` | `joins` | `Actions::Action::joins` (systems) | empty | answers | ok |
| `Probe::p1.st` | `loops` | `Actions::Action::loops` (systems) | empty | answers | ok |
| `Probe::p1.st` | `merges` | `Actions::Action::merges` (systems) | empty | answers | ok |
| `Probe::p1.st` | `on1` | `Probe::S1::on1` (model) | uninitialized feature value: on1 | typed error (`errUninitializedFeatureValue`) | gap: unbound reference read refused |
| `Probe::p1.st` | `self` | `States::StateAction::self` (systemsFRAME) | no such feature: self | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::p1.st` | `sendSubactions` | `Actions::Action::sendSubactions` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1.st` | `start` | `States::StateAction::start` (systemsFRAME) | no such feature: start | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::p1.st` | `stateSequencing` | `States::StateAction::stateSequencing` (systems) | no such feature: stateSequencing | error (`ErrNoSuchFeature`) | gap: usage kind left out of the shape |
| `Probe::p1.st` | `stateTransitions` | `States::StateAction::stateTransitions` (systems) | empty | answers | ok |
| `Probe::p1.st` | `subactions` | `States::StateAction::subactions` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1.st` | `substates` | `States::StateAction::substates` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1.st` | `terminateSubactions` | `Actions::Action::terminateSubactions` (systems) | empty | answers | ok |
| `Probe::p1.st` | `transitions` | `Actions::Action::transitions` (systems) | empty | answers | ok |
| `Probe::p1.st` | `whileLoops` | `Actions::Action::whileLoops` (systems) | empty | answers | ok |
| `Probe::p1.rq` | `actors` | `Requirements::RequirementCheck::actors` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1.rq` | `assumptions` | `Requirements::RequirementCheck::assumptions` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1.rq` | `concerns` | `Requirements::RequirementCheck::concerns` (systems) | no such feature: concerns | error (`ErrNoSuchFeature`) | gap: usage kind left out of the shape |
| `Probe::p1.rq` | `constraints` | `Requirements::RequirementCheck::constraints` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1.rq` | `lim` | `Probe::R1::lim` (model) | constant 5.0 | answers | ok |
| `Probe::p1.rq` | `result` | `Requirements::RequirementConstraintCheck::result` (systemsFRAME) | no such feature: result | error (`ErrNoSuchFeature`) | gap: result parameter read on its object (sibling leaves parameters out) |
| `Probe::p1.rq` | `s` | `Probe::R1::s` (model) | uninitialized feature value: s | typed error (`errUninitializedFeatureValue`) | gap: unbound reference read refused |
| `Probe::p1.rq` | `self` | `Requirements::RequirementCheck::self` (systemsFRAME) | no such feature: self | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::p1.rq` | `sh` | `Probe::R1::sh` (model) | object:Probe::P2 | answers | ok |
| `Probe::p1.rq` | `stakeholders` | `Requirements::RequirementCheck::stakeholders` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1.rq` | `subrequirements` | `Requirements::RequirementCheck::subrequirements` (systems) | empty | answers | ok |
| `Probe::p1.rq` | `who` | `Probe::R1::who` (model) | object:Probe::P2 | answers | ok |
| `Probe::p1.cq` | `self` | `Constraints::ConstraintCheck::self` (systemsFRAME) | no such feature: self | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::p1.cq` | `x` | `Probe::P1::cq::x` (model) | collection(1) [1.0] | answers | ok |
| `Probe::p1.cq` | `y` | `Probe::P1::cq::y` (model) | collection(1) [2.0] | answers | ok |
| `Probe::p1.cnq` | `boundingShapes` | `Items::Item::boundingShapes` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1.cnq` | `checkedConstraints` | `Items::Item::checkedConstraints` (systems) | empty | answers | ok |
| `Probe::p1.cnq` | `done` | `Parts::Part::done` (systemsFRAME) | no such feature: done | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::p1.cnq` | `e1` | `Probe::Cn1::e1` (model) | object:Probe::P2 | answers | ok |
| `Probe::p1.cnq` | `e2` | `Probe::Cn1::e2` (model) | object:Probe::P2 | answers | ok |
| `Probe::p1.cnq` | `envelopingShapes` | `Items::Item::envelopingShapes` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1.cnq` | `exhibitedStates` | `Parts::Part::exhibitedStates` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1.cnq` | `isSolid` | `Items::Item::isSolid` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1.cnq` | `ownedActions` | `Parts::Part::ownedActions` (systems) | empty | answers | ok |
| `Probe::p1.cnq` | `ownedPorts` | `Parts::Part::ownedPorts` (systems) | empty | answers | ok |
| `Probe::p1.cnq` | `ownedStates` | `Parts::Part::ownedStates` (systems) | empty | answers | ok |
| `Probe::p1.cnq` | `performedActions` | `Parts::Part::performedActions` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1.cnq` | `self` | `Parts::Part::self` (systemsFRAME) | no such feature: self | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::p1.cnq` | `shape` | `Items::Item::shape` (systems) | empty | answers | ok |
| `Probe::p1.cnq` | `source` | `Connections::BinaryConnection::source` (systems) | object:Probe::P2 | answers | ok |
| `Probe::p1.cnq` | `start` | `Parts::Part::start` (systemsFRAME) | no such feature: start | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::p1.cnq` | `subitems` | `Items::Item::subitems` (systems) | empty | answers | ok |
| `Probe::p1.cnq` | `subparts` | `Items::Item::subparts` (systems) | empty | answers | ok |
| `Probe::p1.cnq` | `target` | `Connections::BinaryConnection::target` (systems) | object:Probe::P2 | answers | ok |
| `Probe::p1.cnq` | `voids` | `Items::Item::voids` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1.ifq` | `boundingShapes` | `Items::Item::boundingShapes` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1.ifq` | `checkedConstraints` | `Items::Item::checkedConstraints` (systems) | empty | answers | ok |
| `Probe::p1.ifq` | `done` | `Parts::Part::done` (systemsFRAME) | no such feature: done | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::p1.ifq` | `envelopingShapes` | `Items::Item::envelopingShapes` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1.ifq` | `exhibitedStates` | `Parts::Part::exhibitedStates` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1.ifq` | `isSolid` | `Items::Item::isSolid` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1.ifq` | `ownedActions` | `Parts::Part::ownedActions` (systems) | empty | answers | ok |
| `Probe::p1.ifq` | `ownedPorts` | `Parts::Part::ownedPorts` (systems) | empty | answers | ok |
| `Probe::p1.ifq` | `ownedStates` | `Parts::Part::ownedStates` (systems) | empty | answers | ok |
| `Probe::p1.ifq` | `p1` | `Probe::If1::p1` (model) | object:Probe::Pt | answers | ok |
| `Probe::p1.ifq` | `p2` | `Probe::If1::p2` (model) | object:Probe::Pt | answers | ok |
| `Probe::p1.ifq` | `participant` | `Interfaces::BinaryInterface::participant` (systems) | uninitialized feature value: participant | typed error (`errUninitializedFeatureValue`) | gap: unbound reference read refused |
| `Probe::p1.ifq` | `performedActions` | `Parts::Part::performedActions` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1.ifq` | `self` | `Parts::Part::self` (systemsFRAME) | no such feature: self | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::p1.ifq` | `shape` | `Items::Item::shape` (systems) | empty | answers | ok |
| `Probe::p1.ifq` | `source` | `Interfaces::BinaryInterface::source` (systems) | object:Probe::Pt | answers | ok |
| `Probe::p1.ifq` | `start` | `Parts::Part::start` (systemsFRAME) | no such feature: start | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::p1.ifq` | `subitems` | `Items::Item::subitems` (systems) | empty | answers | ok |
| `Probe::p1.ifq` | `subparts` | `Items::Item::subparts` (systems) | empty | answers | ok |
| `Probe::p1.ifq` | `target` | `Interfaces::BinaryInterface::target` (systems) | object:Probe::Pt | answers | ok |
| `Probe::p1.ifq` | `voids` | `Items::Item::voids` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1.alq` | `a1` | `Probe::Al1::a1` (model) | object:Probe::P2 | answers | ok |
| `Probe::p1.alq` | `a2` | `Probe::Al1::a2` (model) | object:Probe::P2 | answers | ok |
| `Probe::p1.alq` | `boundingShapes` | `Items::Item::boundingShapes` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1.alq` | `checkedConstraints` | `Items::Item::checkedConstraints` (systems) | empty | answers | ok |
| `Probe::p1.alq` | `done` | `Parts::Part::done` (systemsFRAME) | no such feature: done | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::p1.alq` | `envelopingShapes` | `Items::Item::envelopingShapes` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1.alq` | `exhibitedStates` | `Parts::Part::exhibitedStates` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1.alq` | `isSolid` | `Items::Item::isSolid` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1.alq` | `ownedActions` | `Parts::Part::ownedActions` (systems) | empty | answers | ok |
| `Probe::p1.alq` | `ownedPorts` | `Parts::Part::ownedPorts` (systems) | empty | answers | ok |
| `Probe::p1.alq` | `ownedStates` | `Parts::Part::ownedStates` (systems) | empty | answers | ok |
| `Probe::p1.alq` | `performedActions` | `Parts::Part::performedActions` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1.alq` | `self` | `Parts::Part::self` (systemsFRAME) | no such feature: self | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::p1.alq` | `shape` | `Items::Item::shape` (systems) | empty | answers | ok |
| `Probe::p1.alq` | `source` | `Allocations::Allocation::source` (systems) | object:Probe::P2 | answers | ok |
| `Probe::p1.alq` | `start` | `Parts::Part::start` (systemsFRAME) | no such feature: start | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::p1.alq` | `subitems` | `Items::Item::subitems` (systems) | empty | answers | ok |
| `Probe::p1.alq` | `subparts` | `Items::Item::subparts` (systems) | empty | answers | ok |
| `Probe::p1.alq` | `target` | `Allocations::Allocation::target` (systems) | object:Probe::P2 | answers | ok |
| `Probe::p1.alq` | `voids` | `Items::Item::voids` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1.flq` | `acceptSubactions` | `Actions::Action::acceptSubactions` (systems) | empty | answers | ok |
| `Probe::p1.flq` | `assignments` | `Actions::Action::assignments` (systems) | empty | answers | ok |
| `Probe::p1.flq` | `controls` | `Actions::Action::controls` (systems) | empty | answers | ok |
| `Probe::p1.flq` | `decisionTransitions` | `Actions::Action::decisionTransitions` (systems) | empty | answers | ok |
| `Probe::p1.flq` | `decisions` | `Actions::Action::decisions` (systems) | empty | answers | ok |
| `Probe::p1.flq` | `done` | `Actions::Action::done` (systemsFRAME) | no such feature: done | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::p1.flq` | `forLoops` | `Actions::Action::forLoops` (systems) | empty | answers | ok |
| `Probe::p1.flq` | `forks` | `Actions::Action::forks` (systems) | empty | answers | ok |
| `Probe::p1.flq` | `ifSubactions` | `Actions::Action::ifSubactions` (systems) | empty | answers | ok |
| `Probe::p1.flq` | `incomingTransfers` | `Actions::Action::incomingTransfers` (systemsFRAME) | no such feature: incomingTransfers | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::p1.flq` | `inflow` | `Probe::Fl1::inflow` (model) | object:Probe::I1 | answers | ok |
| `Probe::p1.flq` | `joins` | `Actions::Action::joins` (systems) | empty | answers | ok |
| `Probe::p1.flq` | `loops` | `Actions::Action::loops` (systems) | empty | answers | ok |
| `Probe::p1.flq` | `merges` | `Actions::Action::merges` (systems) | empty | answers | ok |
| `Probe::p1.flq` | `payload` | `Flows::MessageAction::payload` (systems) | empty | answers | ok |
| `Probe::p1.flq` | `self` | `Actions::Action::self` (systemsFRAME) | no such feature: self | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::p1.flq` | `sendSubactions` | `Actions::Action::sendSubactions` (systems) | empty | answers | ok |
| `Probe::p1.flq` | `start` | `Actions::Action::start` (systemsFRAME) | no such feature: start | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::p1.flq` | `subactions` | `Actions::Action::subactions` (systems) | empty | answers | ok |
| `Probe::p1.flq` | `terminateSubactions` | `Actions::Action::terminateSubactions` (systems) | empty | answers | ok |
| `Probe::p1.flq` | `transitions` | `Actions::Action::transitions` (systems) | empty | answers | ok |
| `Probe::p1.flq` | `whileLoops` | `Actions::Action::whileLoops` (systems) | empty | answers | ok |
| `Probe::p1.atq` | `v` | `Probe::At1::v` (model) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1.pa` | `acceptSubactions` | `Actions::Action::acceptSubactions` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1.pa` | `assignments` | `Actions::Action::assignments` (systems) | empty | answers | ok |
| `Probe::p1.pa` | `controls` | `Actions::Action::controls` (systems) | empty | answers | ok |
| `Probe::p1.pa` | `decisionTransitions` | `Actions::Action::decisionTransitions` (systems) | empty | answers | ok |
| `Probe::p1.pa` | `decisions` | `Actions::Action::decisions` (systems) | empty | answers | ok |
| `Probe::p1.pa` | `done` | `Actions::Action::done` (systemsFRAME) | no such feature: done | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::p1.pa` | `forLoops` | `Actions::Action::forLoops` (systems) | empty | answers | ok |
| `Probe::p1.pa` | `forks` | `Actions::Action::forks` (systems) | empty | answers | ok |
| `Probe::p1.pa` | `ifSubactions` | `Actions::Action::ifSubactions` (systems) | empty | answers | ok |
| `Probe::p1.pa` | `incomingTransfers` | `Actions::Action::incomingTransfers` (systemsFRAME) | no such feature: incomingTransfers | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::p1.pa` | `joins` | `Actions::Action::joins` (systems) | empty | answers | ok |
| `Probe::p1.pa` | `loops` | `Actions::Action::loops` (systems) | empty | answers | ok |
| `Probe::p1.pa` | `merges` | `Actions::Action::merges` (systems) | empty | answers | ok |
| `Probe::p1.pa` | `self` | `Actions::Action::self` (systemsFRAME) | no such feature: self | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::p1.pa` | `sendSubactions` | `Actions::Action::sendSubactions` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1.pa` | `start` | `Actions::Action::start` (systemsFRAME) | no such feature: start | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::p1.pa` | `subactions` | `Actions::Action::subactions` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1.pa` | `terminateSubactions` | `Actions::Action::terminateSubactions` (systems) | empty | answers | ok |
| `Probe::p1.pa` | `transitions` | `Actions::Action::transitions` (systems) | empty | answers | ok |
| `Probe::p1.pa` | `whileLoops` | `Actions::Action::whileLoops` (systems) | empty | answers | ok |
| `Probe::p1.es` | `acceptSubactions` | `Actions::Action::acceptSubactions` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1.es` | `assignments` | `Actions::Action::assignments` (systems) | empty | answers | ok |
| `Probe::p1.es` | `controls` | `Actions::Action::controls` (systems) | empty | answers | ok |
| `Probe::p1.es` | `decisionTransitions` | `Actions::Action::decisionTransitions` (systems) | empty | answers | ok |
| `Probe::p1.es` | `decisions` | `Actions::Action::decisions` (systems) | empty | answers | ok |
| `Probe::p1.es` | `doAction` | `States::StateAction::doAction` (systems) | object:Actions::Action | answers | ok |
| `Probe::p1.es` | `done` | `States::StateAction::done` (systemsFRAME) | no such feature: done | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::p1.es` | `entryAction` | `States::StateAction::entryAction` (systems) | uninitialized feature value: entryAction | typed error (`errUninitializedFeatureValue`) | gap: unbound reference read refused |
| `Probe::p1.es` | `exclusiveStates` | `States::StateAction::exclusiveStates` (systems) | empty | answers | ok |
| `Probe::p1.es` | `exitAction` | `States::StateAction::exitAction` (systems) | object:Actions::Action | answers | ok |
| `Probe::p1.es` | `forLoops` | `Actions::Action::forLoops` (systems) | empty | answers | ok |
| `Probe::p1.es` | `forks` | `Actions::Action::forks` (systems) | empty | answers | ok |
| `Probe::p1.es` | `ifSubactions` | `Actions::Action::ifSubactions` (systems) | empty | answers | ok |
| `Probe::p1.es` | `incomingTransfers` | `Actions::Action::incomingTransfers` (systemsFRAME) | no such feature: incomingTransfers | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::p1.es` | `isTriggerDuring` | `States::StateAction::isTriggerDuring` (systems) | constant true | answers | ok |
| `Probe::p1.es` | `joins` | `Actions::Action::joins` (systems) | empty | answers | ok |
| `Probe::p1.es` | `loops` | `Actions::Action::loops` (systems) | empty | answers | ok |
| `Probe::p1.es` | `merges` | `Actions::Action::merges` (systems) | empty | answers | ok |
| `Probe::p1.es` | `on1` | `Probe::S1::on1` (model) | uninitialized feature value: on1 | typed error (`errUninitializedFeatureValue`) | gap: unbound reference read refused |
| `Probe::p1.es` | `self` | `States::StateAction::self` (systemsFRAME) | no such feature: self | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::p1.es` | `sendSubactions` | `Actions::Action::sendSubactions` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1.es` | `start` | `States::StateAction::start` (systemsFRAME) | no such feature: start | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::p1.es` | `stateSequencing` | `States::StateAction::stateSequencing` (systems) | no such feature: stateSequencing | error (`ErrNoSuchFeature`) | gap: usage kind left out of the shape |
| `Probe::p1.es` | `stateTransitions` | `States::StateAction::stateTransitions` (systems) | empty | answers | ok |
| `Probe::p1.es` | `subactions` | `States::StateAction::subactions` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1.es` | `substates` | `States::StateAction::substates` (systems) | undetermined <undetermined> | `<undetermined>` | by design: model-level open count |
| `Probe::p1.es` | `terminateSubactions` | `Actions::Action::terminateSubactions` (systems) | empty | answers | ok |
| `Probe::p1.es` | `transitions` | `Actions::Action::transitions` (systems) | empty | answers | ok |
| `Probe::p1.es` | `whileLoops` | `Actions::Action::whileLoops` (systems) | empty | answers | ok |
| `Probe::p1.ac` | `self` | `Constraints::ConstraintCheck::self` (systemsFRAME) | no such feature: self | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `Probe::p1.ac` | `x` | `Probe::P1::ac::x` (model) | collection(1) [1.0] | answers | ok |
| `Probe::p1.ac` | `y` | `Probe::P1::ac::y` (model) | collection(1) [2.0] | answers | ok |
| `new Probe::P1()` | `a` | `Probe::P1::a` (model) | constant 2.5 | answers | ok |
| `new Probe::P1()` | `ac` | `Probe::P1::ac` (model) | object:Probe::P1::ac | answers | ok |
| `new Probe::P1()` | `act` | `Probe::P1::act` (model) | object:Probe::A1 | answers | ok |
| `new Probe::P1()` | `alq` | `Probe::P1::alq` (model) | object:Probe::Al1 | answers | ok |
| `new Probe::P1()` | `anq` | `Probe::P1::anq` (model) | error no value: calc usage anq computes output features (no output feature); read one of them | typed error | gap: case usage read as a calc |
| `new Probe::P1()` | `atq` | `Probe::P1::atq` (model) | object:Probe::At1 | answers | ok |
| `new Probe::P1()` | `boundingShapes` | `Items::Item::boundingShapes` (systems) | empty | answers | ok |
| `new Probe::P1()` | `caq` | `Probe::P1::caq` (model) | no such feature: caq | error (`ErrNoSuchFeature`) | gap: usage kind left out of the shape |
| `new Probe::P1()` | `cargo` | `Probe::P1::cargo` (model) | object:Probe::I1 | answers | ok |
| `new Probe::P1()` | `checkedConstraints` | `Items::Item::checkedConstraints` (systems) | collection(3) [instance(122), instance(123), instance(124)] | answers | ok |
| `new Probe::P1()` | `cnq` | `Probe::P1::cnq` (model) | object:Probe::Cn1 | answers | ok |
| `new Probe::P1()` | `conq` | `Probe::P1::conq` (model) | no such feature: conq | error (`ErrNoSuchFeature`) | gap: usage kind left out of the shape |
| `new Probe::P1()` | `cq` | `Probe::P1::cq` (model) | object:Probe::P1::cq | answers | ok |
| `new Probe::P1()` | `done` | `Parts::Part::done` (systemsFRAME) | no such feature: done | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::P1()` | `envelopingShapes` | `Items::Item::envelopingShapes` (systems) | empty | answers | ok |
| `new Probe::P1()` | `eq` | `Probe::P1::eq` (model) | no such feature: eq | error (`ErrNoSuchFeature`) | gap: usage kind left out of the shape |
| `new Probe::P1()` | `es` | `Probe::P1::es` (model) | object:Probe::S1 | answers | ok |
| `new Probe::P1()` | `ev` | `Probe::P1::ev` (model) | object:Probe::P1::ev | answers | ok |
| `new Probe::P1()` | `exhibitedStates` | `Parts::Part::exhibitedStates` (systems) | empty | answers | ok |
| `new Probe::P1()` | `flq` | `Probe::P1::flq` (model) | object:Probe::Fl1 | answers | ok |
| `new Probe::P1()` | `ifq` | `Probe::P1::ifq` (model) | object:Probe::If1 | answers | ok |
| `new Probe::P1()` | `isSolid` | `Items::Item::isSolid` (systems) | constant true | answers | ok |
| `new Probe::P1()` | `iu` | `Probe::P1::iu` (model) | no such feature: iu | error (`ErrNoSuchFeature`) | gap: usage kind left out of the shape |
| `new Probe::P1()` | `kq` | `Probe::P1::kq` (model) | constant 6.0 | answers | ok |
| `new Probe::P1()` | `mass` | `Probe::P1::mass` (model) | collection(1) [3.0 [kg]] | answers | ok |
| `new Probe::P1()` | `mq` | `Probe::P1::mq` (model) | quantity 2.0 [kg] | answers | ok |
| `new Probe::P1()` | `n` | `Probe::P1::n` (model) | object:ScalarValues::Integer | answers | ok |
| `new Probe::P1()` | `opt` | `Probe::P1::opt` (model) | empty | answers | ok |
| `new Probe::P1()` | `oq` | `Probe::P1::oq` (model) | object:Probe::O1 | answers | ok |
| `new Probe::P1()` | `other` | `Probe::P1::other` (model) | uninitialized feature value: other | typed error (`errUninitializedFeatureValue`) | gap: unbound reference read refused |
| `new Probe::P1()` | `otherItem` | `Probe::P1::otherItem` (model) | uninitialized feature value: otherItem | typed error (`errUninitializedFeatureValue`) | gap: unbound reference read refused |
| `new Probe::P1()` | `others` | `Probe::P1::others` (model) | empty | answers | ok |
| `new Probe::P1()` | `ownedActions` | `Parts::Part::ownedActions` (systems) | collection(3) [instance(207), instance(208), instance(212)] | answers | ok |
| `new Probe::P1()` | `ownedPorts` | `Parts::Part::ownedPorts` (systems) | collection(1) [instance(216)] | answers | ok |
| `new Probe::P1()` | `ownedStates` | `Parts::Part::ownedStates` (systems) | collection(1) [instance(220)] | answers | ok |
| `new Probe::P1()` | `pa` | `Probe::P1::pa` (model) | object:Probe::A2 | answers | ok |
| `new Probe::P1()` | `performedActions` | `Parts::Part::performedActions` (systems) | empty | answers | ok |
| `new Probe::P1()` | `pt` | `Probe::P1::pt` (model) | object:Probe::Pt | answers | ok |
| `new Probe::P1()` | `rdq` | `Probe::P1::rdq` (model) | no such feature: rdq | error (`ErrNoSuchFeature`) | gap: usage kind left out of the shape |
| `new Probe::P1()` | `rq` | `Probe::P1::rq` (model) | object:Probe::R1 | answers | ok |
| `new Probe::P1()` | `self` | `Parts::Part::self` (systemsFRAME) | no such feature: self | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::P1()` | `shape` | `Items::Item::shape` (systems) | empty | answers | ok |
| `new Probe::P1()` | `slice` | `Probe::P1::slice` (model) | object:Probe::P1::slice | answers | ok |
| `new Probe::P1()` | `snap` | `Probe::P1::snap` (model) | object:Probe::P1::snap | answers | ok |
| `new Probe::P1()` | `sr` | `Probe::P1::sr` (model) | no such feature: sr | error (`ErrNoSuchFeature`) | gap: usage kind left out of the shape |
| `new Probe::P1()` | `st` | `Probe::P1::st` (model) | object:Probe::S1 | answers | ok |
| `new Probe::P1()` | `start` | `Parts::Part::start` (systemsFRAME) | no such feature: start | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::P1()` | `sub` | `Probe::P1::sub` (model) | collection(2) [instance(265), instance(266)] | answers | ok |
| `new Probe::P1()` | `subitems` | `Items::Item::subitems` (systems) | collection(6) [instance(270), instance(271), instance(272), instance(273), instance(274), instance(276)] | answers | ok |
| `new Probe::P1()` | `subparts` | `Items::Item::subparts` (systems) | collection(5) [instance(280), instance(281), instance(282), instance(283), instance(285)] | answers | ok |
| `new Probe::P1()` | `uq` | `Probe::P1::uq` (model) | no such feature: uq | error (`ErrNoSuchFeature`) | gap: usage kind left out of the shape |
| `new Probe::P1()` | `voids` | `Items::Item::voids` (systems) | empty | answers | ok |
| `new Probe::P1()` | `vpq` | `Probe::P1::vpq` (model) | no such feature: vpq | error (`ErrNoSuchFeature`) | gap: usage kind left out of the shape |
| `new Probe::P1()` | `vq` | `Probe::P1::vq` (model) | error no value: calc usage vq computes output features (no output feature); read one of them | typed error | gap: case usage read as a calc |
| `new Probe::P1()` | `vq3` | `Probe::P1::vq3` (model) | uninitialized feature value: vq3 | typed error (`errUninitializedFeatureValue`) | gap: unbound reference read refused |
| `new Probe::P1()` | `vwq` | `Probe::P1::vwq` (model) | no such feature: vwq | error (`ErrNoSuchFeature`) | gap: usage kind left out of the shape |
| `new Probe::P2()` | `boundingShapes` | `Items::Item::boundingShapes` (systems) | empty | answers | ok |
| `new Probe::P2()` | `checkedConstraints` | `Items::Item::checkedConstraints` (systems) | empty | answers | ok |
| `new Probe::P2()` | `done` | `Parts::Part::done` (systemsFRAME) | no such feature: done | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::P2()` | `envelopingShapes` | `Items::Item::envelopingShapes` (systems) | empty | answers | ok |
| `new Probe::P2()` | `exhibitedStates` | `Parts::Part::exhibitedStates` (systems) | empty | answers | ok |
| `new Probe::P2()` | `isSolid` | `Items::Item::isSolid` (systems) | constant true | answers | ok |
| `new Probe::P2()` | `ownedActions` | `Parts::Part::ownedActions` (systems) | empty | answers | ok |
| `new Probe::P2()` | `ownedPorts` | `Parts::Part::ownedPorts` (systems) | empty | answers | ok |
| `new Probe::P2()` | `ownedStates` | `Parts::Part::ownedStates` (systems) | empty | answers | ok |
| `new Probe::P2()` | `performedActions` | `Parts::Part::performedActions` (systems) | empty | answers | ok |
| `new Probe::P2()` | `self` | `Parts::Part::self` (systemsFRAME) | no such feature: self | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::P2()` | `shape` | `Items::Item::shape` (systems) | empty | answers | ok |
| `new Probe::P2()` | `start` | `Parts::Part::start` (systemsFRAME) | no such feature: start | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::P2()` | `subitems` | `Items::Item::subitems` (systems) | empty | answers | ok |
| `new Probe::P2()` | `subparts` | `Items::Item::subparts` (systems) | empty | answers | ok |
| `new Probe::P2()` | `voids` | `Items::Item::voids` (systems) | empty | answers | ok |
| `new Probe::I1()` | `boundingShapes` | `Items::Item::boundingShapes` (systems) | empty | answers | ok |
| `new Probe::I1()` | `checkedConstraints` | `Items::Item::checkedConstraints` (systems) | empty | answers | ok |
| `new Probe::I1()` | `done` | `Items::Item::done` (systemsFRAME) | no such feature: done | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::I1()` | `envelopingShapes` | `Items::Item::envelopingShapes` (systems) | empty | answers | ok |
| `new Probe::I1()` | `isSolid` | `Items::Item::isSolid` (systems) | constant true | answers | ok |
| `new Probe::I1()` | `self` | `Items::Item::self` (systemsFRAME) | no such feature: self | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::I1()` | `shape` | `Items::Item::shape` (systems) | empty | answers | ok |
| `new Probe::I1()` | `start` | `Items::Item::start` (systemsFRAME) | no such feature: start | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::I1()` | `subitems` | `Items::Item::subitems` (systems) | empty | answers | ok |
| `new Probe::I1()` | `subparts` | `Items::Item::subparts` (systems) | empty | answers | ok |
| `new Probe::I1()` | `voids` | `Items::Item::voids` (systems) | empty | answers | ok |
| `new Probe::I1()` | `w` | `Probe::I1::w` (model) | constant 1.0 | answers | ok |
| `new Probe::Pt()` | `inflow` | `Probe::Pt::inflow` (model) | object:Probe::I1 | answers | ok |
| `new Probe::Pt()` | `interfacingPorts` | `Ports::Port::interfacingPorts` (systems) | empty | answers | ok |
| `new Probe::Pt()` | `outflow` | `Probe::Pt::outflow` (model) | object:Probe::I1 | answers | ok |
| `new Probe::Pt()` | `outgoingTransfersFromSelf` | `Ports::Port::outgoingTransfersFromSelf` (systemsFRAME) | no such feature: outgoingTransfersFromSelf | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::Pt()` | `self` | `Ports::Port::self` (systemsFRAME) | no such feature: self | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::Pt()` | `subports` | `Ports::Port::subports` (systems) | empty | answers | ok |
| `new Probe::Pt()` | `x` | `Probe::Pt::x` (model) | object:ScalarValues::Integer | answers | ok |
| `new Probe::A1()` | `acceptSubactions` | `Actions::Action::acceptSubactions` (systems) | empty | answers | ok |
| `new Probe::A1()` | `assignments` | `Actions::Action::assignments` (systems) | empty | answers | ok |
| `new Probe::A1()` | `controls` | `Actions::Action::controls` (systems) | empty | answers | ok |
| `new Probe::A1()` | `decisionTransitions` | `Actions::Action::decisionTransitions` (systems) | empty | answers | ok |
| `new Probe::A1()` | `decisions` | `Actions::Action::decisions` (systems) | empty | answers | ok |
| `new Probe::A1()` | `done` | `Actions::Action::done` (systemsFRAME) | no such feature: done | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::A1()` | `forLoops` | `Actions::Action::forLoops` (systems) | empty | answers | ok |
| `new Probe::A1()` | `forks` | `Actions::Action::forks` (systems) | empty | answers | ok |
| `new Probe::A1()` | `i` | `Probe::A1::i` (model) | empty | answers | ok |
| `new Probe::A1()` | `ifSubactions` | `Actions::Action::ifSubactions` (systems) | empty | answers | ok |
| `new Probe::A1()` | `incomingTransfers` | `Actions::Action::incomingTransfers` (systemsFRAME) | no such feature: incomingTransfers | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::A1()` | `joins` | `Actions::Action::joins` (systems) | empty | answers | ok |
| `new Probe::A1()` | `loops` | `Actions::Action::loops` (systems) | empty | answers | ok |
| `new Probe::A1()` | `merges` | `Actions::Action::merges` (systems) | empty | answers | ok |
| `new Probe::A1()` | `nested` | `Probe::A1::nested` (model) | object:Probe::A2 | answers | ok |
| `new Probe::A1()` | `o` | `Probe::A1::o` (model) | empty | answers | ok |
| `new Probe::A1()` | `self` | `Actions::Action::self` (systemsFRAME) | no such feature: self | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::A1()` | `sendSubactions` | `Actions::Action::sendSubactions` (systems) | empty | answers | ok |
| `new Probe::A1()` | `start` | `Actions::Action::start` (systemsFRAME) | no such feature: start | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::A1()` | `subactions` | `Actions::Action::subactions` (systems) | collection(1) [instance(367)] | answers | ok |
| `new Probe::A1()` | `terminateSubactions` | `Actions::Action::terminateSubactions` (systems) | empty | answers | ok |
| `new Probe::A1()` | `transitions` | `Actions::Action::transitions` (systems) | empty | answers | ok |
| `new Probe::A1()` | `whileLoops` | `Actions::Action::whileLoops` (systems) | empty | answers | ok |
| `new Probe::S1()` | `acceptSubactions` | `Actions::Action::acceptSubactions` (systems) | empty | answers | ok |
| `new Probe::S1()` | `assignments` | `Actions::Action::assignments` (systems) | empty | answers | ok |
| `new Probe::S1()` | `controls` | `Actions::Action::controls` (systems) | empty | answers | ok |
| `new Probe::S1()` | `decisionTransitions` | `Actions::Action::decisionTransitions` (systems) | empty | answers | ok |
| `new Probe::S1()` | `decisions` | `Actions::Action::decisions` (systems) | empty | answers | ok |
| `new Probe::S1()` | `doAction` | `States::StateAction::doAction` (systems) | object:Actions::Action | answers | ok |
| `new Probe::S1()` | `done` | `States::StateAction::done` (systemsFRAME) | no such feature: done | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::S1()` | `entryAction` | `States::StateAction::entryAction` (systems) | uninitialized feature value: entryAction | typed error (`errUninitializedFeatureValue`) | gap: unbound reference read refused |
| `new Probe::S1()` | `exclusiveStates` | `States::StateAction::exclusiveStates` (systems) | empty | answers | ok |
| `new Probe::S1()` | `exitAction` | `States::StateAction::exitAction` (systems) | object:Actions::Action | answers | ok |
| `new Probe::S1()` | `forLoops` | `Actions::Action::forLoops` (systems) | empty | answers | ok |
| `new Probe::S1()` | `forks` | `Actions::Action::forks` (systems) | empty | answers | ok |
| `new Probe::S1()` | `ifSubactions` | `Actions::Action::ifSubactions` (systems) | empty | answers | ok |
| `new Probe::S1()` | `incomingTransfers` | `Actions::Action::incomingTransfers` (systemsFRAME) | no such feature: incomingTransfers | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::S1()` | `isTriggerDuring` | `States::StateAction::isTriggerDuring` (systems) | constant true | answers | ok |
| `new Probe::S1()` | `joins` | `Actions::Action::joins` (systems) | empty | answers | ok |
| `new Probe::S1()` | `loops` | `Actions::Action::loops` (systems) | empty | answers | ok |
| `new Probe::S1()` | `merges` | `Actions::Action::merges` (systems) | empty | answers | ok |
| `new Probe::S1()` | `on1` | `Probe::S1::on1` (model) | uninitialized feature value: on1 | typed error (`errUninitializedFeatureValue`) | gap: unbound reference read refused |
| `new Probe::S1()` | `self` | `States::StateAction::self` (systemsFRAME) | no such feature: self | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::S1()` | `sendSubactions` | `Actions::Action::sendSubactions` (systems) | empty | answers | ok |
| `new Probe::S1()` | `start` | `States::StateAction::start` (systemsFRAME) | no such feature: start | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::S1()` | `stateSequencing` | `States::StateAction::stateSequencing` (systems) | no such feature: stateSequencing | error (`ErrNoSuchFeature`) | gap: usage kind left out of the shape |
| `new Probe::S1()` | `stateTransitions` | `States::StateAction::stateTransitions` (systems) | empty | answers | ok |
| `new Probe::S1()` | `subactions` | `States::StateAction::subactions` (systems) | empty | answers | ok |
| `new Probe::S1()` | `substates` | `States::StateAction::substates` (systems) | empty | answers | ok |
| `new Probe::S1()` | `terminateSubactions` | `Actions::Action::terminateSubactions` (systems) | empty | answers | ok |
| `new Probe::S1()` | `transitions` | `Actions::Action::transitions` (systems) | empty | answers | ok |
| `new Probe::S1()` | `whileLoops` | `Actions::Action::whileLoops` (systems) | empty | answers | ok |
| `new Probe::R1()` | `actors` | `Requirements::RequirementCheck::actors` (systems) | empty | answers | ok |
| `new Probe::R1()` | `assumptions` | `Requirements::RequirementCheck::assumptions` (systems) | empty | answers | ok |
| `new Probe::R1()` | `concerns` | `Requirements::RequirementCheck::concerns` (systems) | no such feature: concerns | error (`ErrNoSuchFeature`) | gap: usage kind left out of the shape |
| `new Probe::R1()` | `constraints` | `Requirements::RequirementCheck::constraints` (systems) | empty | answers | ok |
| `new Probe::R1()` | `lim` | `Probe::R1::lim` (model) | constant 5.0 | answers | ok |
| `new Probe::R1()` | `result` | `Requirements::RequirementConstraintCheck::result` (systemsFRAME) | no such feature: result | error (`ErrNoSuchFeature`) | gap: result parameter read on its object (sibling leaves parameters out) |
| `new Probe::R1()` | `s` | `Probe::R1::s` (model) | uninitialized feature value: s | typed error (`errUninitializedFeatureValue`) | gap: unbound reference read refused |
| `new Probe::R1()` | `self` | `Requirements::RequirementCheck::self` (systemsFRAME) | no such feature: self | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::R1()` | `sh` | `Probe::R1::sh` (model) | object:Probe::P2 | answers | ok |
| `new Probe::R1()` | `stakeholders` | `Requirements::RequirementCheck::stakeholders` (systems) | empty | answers | ok |
| `new Probe::R1()` | `subrequirements` | `Requirements::RequirementCheck::subrequirements` (systems) | empty | answers | ok |
| `new Probe::R1()` | `who` | `Probe::R1::who` (model) | object:Probe::P2 | answers | ok |
| `new Probe::C1()` | `self` | `Constraints::ConstraintCheck::self` (systemsFRAME) | no such feature: self | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::C1()` | `x` | `Probe::C1::x` (model) | empty | answers | ok |
| `new Probe::C1()` | `y` | `Probe::C1::y` (model) | empty | answers | ok |
| `new Probe::K1()` | `acceptSubactions` | `Actions::Action::acceptSubactions` (systems) | empty | answers | ok |
| `new Probe::K1()` | `assignments` | `Actions::Action::assignments` (systems) | empty | answers | ok |
| `new Probe::K1()` | `controls` | `Actions::Action::controls` (systems) | empty | answers | ok |
| `new Probe::K1()` | `decisionTransitions` | `Actions::Action::decisionTransitions` (systems) | empty | answers | ok |
| `new Probe::K1()` | `decisions` | `Actions::Action::decisions` (systems) | empty | answers | ok |
| `new Probe::K1()` | `done` | `Actions::Action::done` (systemsFRAME) | no such feature: done | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::K1()` | `forLoops` | `Actions::Action::forLoops` (systems) | empty | answers | ok |
| `new Probe::K1()` | `forks` | `Actions::Action::forks` (systems) | empty | answers | ok |
| `new Probe::K1()` | `ifSubactions` | `Actions::Action::ifSubactions` (systems) | empty | answers | ok |
| `new Probe::K1()` | `incomingTransfers` | `Actions::Action::incomingTransfers` (systemsFRAME) | no such feature: incomingTransfers | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::K1()` | `joins` | `Actions::Action::joins` (systems) | empty | answers | ok |
| `new Probe::K1()` | `loops` | `Actions::Action::loops` (systems) | empty | answers | ok |
| `new Probe::K1()` | `merges` | `Actions::Action::merges` (systems) | empty | answers | ok |
| `new Probe::K1()` | `self` | `Calculations::Calculation::self` (systemsFRAME) | error no value: calc usage self computes output features (no output feature); read one of them | typed error | gap: case usage read as a calc |
| `new Probe::K1()` | `sendSubactions` | `Actions::Action::sendSubactions` (systems) | empty | answers | ok |
| `new Probe::K1()` | `start` | `Actions::Action::start` (systemsFRAME) | no such feature: start | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::K1()` | `subactions` | `Actions::Action::subactions` (systems) | empty | answers | ok |
| `new Probe::K1()` | `subcalculations` | `Calculations::Calculation::subcalculations` (systems) | error no value: calc usage subcalculations computes output features (no output feature); read one of them | typed error | gap: case usage read as a calc |
| `new Probe::K1()` | `terminateSubactions` | `Actions::Action::terminateSubactions` (systems) | empty | answers | ok |
| `new Probe::K1()` | `transitions` | `Actions::Action::transitions` (systems) | empty | answers | ok |
| `new Probe::K1()` | `whileLoops` | `Actions::Action::whileLoops` (systems) | empty | answers | ok |
| `new Probe::K1()` | `x` | `Probe::K1::x` (model) | empty | answers | ok |
| `new Probe::Ca1()` | `acceptSubactions` | `Actions::Action::acceptSubactions` (systems) | empty | answers | ok |
| `new Probe::Ca1()` | `actors` | `Cases::Case::actors` (systems) | empty | answers | ok |
| `new Probe::Ca1()` | `assignments` | `Actions::Action::assignments` (systems) | empty | answers | ok |
| `new Probe::Ca1()` | `controls` | `Actions::Action::controls` (systems) | empty | answers | ok |
| `new Probe::Ca1()` | `decisionTransitions` | `Actions::Action::decisionTransitions` (systems) | empty | answers | ok |
| `new Probe::Ca1()` | `decisions` | `Actions::Action::decisions` (systems) | empty | answers | ok |
| `new Probe::Ca1()` | `done` | `Actions::Action::done` (systemsFRAME) | no such feature: done | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::Ca1()` | `forLoops` | `Actions::Action::forLoops` (systems) | empty | answers | ok |
| `new Probe::Ca1()` | `forks` | `Actions::Action::forks` (systems) | empty | answers | ok |
| `new Probe::Ca1()` | `ifSubactions` | `Actions::Action::ifSubactions` (systems) | empty | answers | ok |
| `new Probe::Ca1()` | `incomingTransfers` | `Actions::Action::incomingTransfers` (systemsFRAME) | no such feature: incomingTransfers | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::Ca1()` | `joins` | `Actions::Action::joins` (systems) | empty | answers | ok |
| `new Probe::Ca1()` | `loops` | `Actions::Action::loops` (systems) | empty | answers | ok |
| `new Probe::Ca1()` | `merges` | `Actions::Action::merges` (systems) | empty | answers | ok |
| `new Probe::Ca1()` | `result` | `Cases::Case::result` (systemsFRAME) | no such feature: result | error (`ErrNoSuchFeature`) | gap: result parameter read on its object (sibling leaves parameters out) |
| `new Probe::Ca1()` | `s` | `Probe::Ca1::s` (model) | uninitialized feature value: s | typed error (`errUninitializedFeatureValue`) | gap: unbound reference read refused |
| `new Probe::Ca1()` | `self` | `Cases::Case::self` (systemsFRAME) | no such feature: self | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::Ca1()` | `sendSubactions` | `Actions::Action::sendSubactions` (systems) | empty | answers | ok |
| `new Probe::Ca1()` | `start` | `Actions::Action::start` (systemsFRAME) | no such feature: start | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::Ca1()` | `subactions` | `Actions::Action::subactions` (systems) | empty | answers | ok |
| `new Probe::Ca1()` | `subcalculations` | `Calculations::Calculation::subcalculations` (systems) | error no value: calc usage subcalculations computes output features (no output feature); read one of them | typed error | gap: case usage read as a calc |
| `new Probe::Ca1()` | `subcases` | `Cases::Case::subcases` (systems) | no such feature: subcases | error (`ErrNoSuchFeature`) | gap: usage kind left out of the shape |
| `new Probe::Ca1()` | `terminateSubactions` | `Actions::Action::terminateSubactions` (systems) | empty | answers | ok |
| `new Probe::Ca1()` | `transitions` | `Actions::Action::transitions` (systems) | empty | answers | ok |
| `new Probe::Ca1()` | `whileLoops` | `Actions::Action::whileLoops` (systems) | empty | answers | ok |
| `new Probe::An1()` | `acceptSubactions` | `Actions::Action::acceptSubactions` (systems) | empty | answers | ok |
| `new Probe::An1()` | `actors` | `Cases::Case::actors` (systems) | empty | answers | ok |
| `new Probe::An1()` | `assignments` | `Actions::Action::assignments` (systems) | empty | answers | ok |
| `new Probe::An1()` | `controls` | `Actions::Action::controls` (systems) | empty | answers | ok |
| `new Probe::An1()` | `decisionTransitions` | `Actions::Action::decisionTransitions` (systems) | empty | answers | ok |
| `new Probe::An1()` | `decisions` | `Actions::Action::decisions` (systems) | empty | answers | ok |
| `new Probe::An1()` | `done` | `Actions::Action::done` (systemsFRAME) | no such feature: done | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::An1()` | `forLoops` | `Actions::Action::forLoops` (systems) | empty | answers | ok |
| `new Probe::An1()` | `forks` | `Actions::Action::forks` (systems) | empty | answers | ok |
| `new Probe::An1()` | `ifSubactions` | `Actions::Action::ifSubactions` (systems) | empty | answers | ok |
| `new Probe::An1()` | `incomingTransfers` | `Actions::Action::incomingTransfers` (systemsFRAME) | no such feature: incomingTransfers | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::An1()` | `joins` | `Actions::Action::joins` (systems) | empty | answers | ok |
| `new Probe::An1()` | `loops` | `Actions::Action::loops` (systems) | empty | answers | ok |
| `new Probe::An1()` | `merges` | `Actions::Action::merges` (systems) | empty | answers | ok |
| `new Probe::An1()` | `result` | `Cases::Case::result` (systemsFRAME) | no such feature: result | error (`ErrNoSuchFeature`) | gap: result parameter read on its object (sibling leaves parameters out) |
| `new Probe::An1()` | `s` | `Probe::An1::s` (model) | uninitialized feature value: s | typed error (`errUninitializedFeatureValue`) | gap: unbound reference read refused |
| `new Probe::An1()` | `self` | `AnalysisCases::AnalysisCase::self` (systemsFRAME) | error no value: calc usage self computes output features (no output feature); read one of them | typed error | gap: case usage read as a calc |
| `new Probe::An1()` | `sendSubactions` | `Actions::Action::sendSubactions` (systems) | empty | answers | ok |
| `new Probe::An1()` | `start` | `Actions::Action::start` (systemsFRAME) | no such feature: start | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::An1()` | `subAnalysisCases` | `AnalysisCases::AnalysisCase::subAnalysisCases` (systems) | error no value: calc usage subAnalysisCases computes output features (no output feature); read one of them | typed error | gap: case usage read as a calc |
| `new Probe::An1()` | `subactions` | `Actions::Action::subactions` (systems) | empty | answers | ok |
| `new Probe::An1()` | `subcalculations` | `Calculations::Calculation::subcalculations` (systems) | error no value: calc usage subcalculations computes output features (no output feature); read one of them | typed error | gap: case usage read as a calc |
| `new Probe::An1()` | `subcases` | `Cases::Case::subcases` (systems) | no such feature: subcases | error (`ErrNoSuchFeature`) | gap: usage kind left out of the shape |
| `new Probe::An1()` | `terminateSubactions` | `Actions::Action::terminateSubactions` (systems) | empty | answers | ok |
| `new Probe::An1()` | `transitions` | `Actions::Action::transitions` (systems) | empty | answers | ok |
| `new Probe::An1()` | `whileLoops` | `Actions::Action::whileLoops` (systems) | empty | answers | ok |
| `new Probe::V1()` | `acceptSubactions` | `Actions::Action::acceptSubactions` (systems) | empty | answers | ok |
| `new Probe::V1()` | `actors` | `Cases::Case::actors` (systems) | empty | answers | ok |
| `new Probe::V1()` | `assignments` | `Actions::Action::assignments` (systems) | empty | answers | ok |
| `new Probe::V1()` | `controls` | `Actions::Action::controls` (systems) | empty | answers | ok |
| `new Probe::V1()` | `decisionTransitions` | `Actions::Action::decisionTransitions` (systems) | empty | answers | ok |
| `new Probe::V1()` | `decisions` | `Actions::Action::decisions` (systems) | empty | answers | ok |
| `new Probe::V1()` | `done` | `Actions::Action::done` (systemsFRAME) | no such feature: done | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::V1()` | `forLoops` | `Actions::Action::forLoops` (systems) | empty | answers | ok |
| `new Probe::V1()` | `forks` | `Actions::Action::forks` (systems) | empty | answers | ok |
| `new Probe::V1()` | `ifSubactions` | `Actions::Action::ifSubactions` (systems) | empty | answers | ok |
| `new Probe::V1()` | `incomingTransfers` | `Actions::Action::incomingTransfers` (systemsFRAME) | no such feature: incomingTransfers | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::V1()` | `joins` | `Actions::Action::joins` (systems) | empty | answers | ok |
| `new Probe::V1()` | `loops` | `Actions::Action::loops` (systems) | empty | answers | ok |
| `new Probe::V1()` | `merges` | `Actions::Action::merges` (systems) | empty | answers | ok |
| `new Probe::V1()` | `r1` | `Probe::V1::r1` (model) | object:Probe::R1 | answers | ok |
| `new Probe::V1()` | `requirementVerifications` | `VerificationCases::VerificationCase::requirementVerifications` (systems) | empty | answers | ok |
| `new Probe::V1()` | `s` | `Probe::V1::s` (model) | uninitialized feature value: s | typed error (`errUninitializedFeatureValue`) | gap: unbound reference read refused |
| `new Probe::V1()` | `self` | `VerificationCases::VerificationCase::self` (systemsFRAME) | error no value: calc usage self computes output features (no output feature); read one of them | typed error | gap: case usage read as a calc |
| `new Probe::V1()` | `sendSubactions` | `Actions::Action::sendSubactions` (systems) | empty | answers | ok |
| `new Probe::V1()` | `start` | `Actions::Action::start` (systemsFRAME) | no such feature: start | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::V1()` | `subVerificationCases` | `VerificationCases::VerificationCase::subVerificationCases` (systems) | error no value: calc usage subVerificationCases computes output features (no output feature); read one of them | typed error | gap: case usage read as a calc |
| `new Probe::V1()` | `subactions` | `Actions::Action::subactions` (systems) | empty | answers | ok |
| `new Probe::V1()` | `subcalculations` | `Calculations::Calculation::subcalculations` (systems) | error no value: calc usage subcalculations computes output features (no output feature); read one of them | typed error | gap: case usage read as a calc |
| `new Probe::V1()` | `subcases` | `Cases::Case::subcases` (systems) | no such feature: subcases | error (`ErrNoSuchFeature`) | gap: usage kind left out of the shape |
| `new Probe::V1()` | `terminateSubactions` | `Actions::Action::terminateSubactions` (systems) | empty | answers | ok |
| `new Probe::V1()` | `transitions` | `Actions::Action::transitions` (systems) | empty | answers | ok |
| `new Probe::V1()` | `verdict` | `VerificationCases::VerificationCase::verdict` (systemsFRAME) | no such feature: verdict | error (`ErrNoSuchFeature`) | gap: result parameter read on its object (sibling leaves parameters out) |
| `new Probe::V1()` | `whileLoops` | `Actions::Action::whileLoops` (systems) | empty | answers | ok |
| `new Probe::U1()` | `acceptSubactions` | `Actions::Action::acceptSubactions` (systems) | empty | answers | ok |
| `new Probe::U1()` | `actors` | `Cases::Case::actors` (systems) | empty | answers | ok |
| `new Probe::U1()` | `assignments` | `Actions::Action::assignments` (systems) | empty | answers | ok |
| `new Probe::U1()` | `controls` | `Actions::Action::controls` (systems) | empty | answers | ok |
| `new Probe::U1()` | `decisionTransitions` | `Actions::Action::decisionTransitions` (systems) | empty | answers | ok |
| `new Probe::U1()` | `decisions` | `Actions::Action::decisions` (systems) | empty | answers | ok |
| `new Probe::U1()` | `done` | `UseCases::UseCase::done` (systemsFRAME) | no such feature: done | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::U1()` | `forLoops` | `Actions::Action::forLoops` (systems) | empty | answers | ok |
| `new Probe::U1()` | `forks` | `Actions::Action::forks` (systems) | empty | answers | ok |
| `new Probe::U1()` | `ifSubactions` | `Actions::Action::ifSubactions` (systems) | empty | answers | ok |
| `new Probe::U1()` | `includedUseCases` | `UseCases::UseCase::includedUseCases` (systems) | no such feature: includedUseCases | error (`ErrNoSuchFeature`) | gap: usage kind left out of the shape |
| `new Probe::U1()` | `incomingTransfers` | `Actions::Action::incomingTransfers` (systemsFRAME) | no such feature: incomingTransfers | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::U1()` | `joins` | `Actions::Action::joins` (systems) | empty | answers | ok |
| `new Probe::U1()` | `loops` | `Actions::Action::loops` (systems) | empty | answers | ok |
| `new Probe::U1()` | `merges` | `Actions::Action::merges` (systems) | empty | answers | ok |
| `new Probe::U1()` | `obj` | `UseCases::UseCase::obj` (systems) | object:Requirements::RequirementCheck | answers | ok |
| `new Probe::U1()` | `result` | `Cases::Case::result` (systemsFRAME) | no such feature: result | error (`ErrNoSuchFeature`) | gap: result parameter read on its object (sibling leaves parameters out) |
| `new Probe::U1()` | `s` | `Probe::U1::s` (model) | uninitialized feature value: s | typed error (`errUninitializedFeatureValue`) | gap: unbound reference read refused |
| `new Probe::U1()` | `self` | `UseCases::UseCase::self` (systemsFRAME) | no such feature: self | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::U1()` | `sendSubactions` | `Actions::Action::sendSubactions` (systems) | empty | answers | ok |
| `new Probe::U1()` | `start` | `UseCases::UseCase::start` (systemsFRAME) | no such feature: start | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::U1()` | `subUseCases` | `UseCases::UseCase::subUseCases` (systems) | no such feature: subUseCases | error (`ErrNoSuchFeature`) | gap: usage kind left out of the shape |
| `new Probe::U1()` | `subactions` | `Actions::Action::subactions` (systems) | empty | answers | ok |
| `new Probe::U1()` | `subcalculations` | `Calculations::Calculation::subcalculations` (systems) | error no value: calc usage subcalculations computes output features (no output feature); read one of them | typed error | gap: case usage read as a calc |
| `new Probe::U1()` | `subcases` | `Cases::Case::subcases` (systems) | no such feature: subcases | error (`ErrNoSuchFeature`) | gap: usage kind left out of the shape |
| `new Probe::U1()` | `terminateSubactions` | `Actions::Action::terminateSubactions` (systems) | empty | answers | ok |
| `new Probe::U1()` | `transitions` | `Actions::Action::transitions` (systems) | empty | answers | ok |
| `new Probe::U1()` | `whileLoops` | `Actions::Action::whileLoops` (systems) | empty | answers | ok |
| `new Probe::U1()` | `who` | `Probe::U1::who` (model) | object:Probe::P2 | answers | ok |
| `new Probe::Vw1()` | `boundingShapes` | `Items::Item::boundingShapes` (systems) | empty | answers | ok |
| `new Probe::Vw1()` | `checkedConstraints` | `Items::Item::checkedConstraints` (systems) | empty | answers | ok |
| `new Probe::Vw1()` | `done` | `Parts::Part::done` (systemsFRAME) | no such feature: done | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::Vw1()` | `envelopingShapes` | `Items::Item::envelopingShapes` (systems) | empty | answers | ok |
| `new Probe::Vw1()` | `exhibitedStates` | `Parts::Part::exhibitedStates` (systems) | empty | answers | ok |
| `new Probe::Vw1()` | `isSolid` | `Items::Item::isSolid` (systems) | constant true | answers | ok |
| `new Probe::Vw1()` | `ownedActions` | `Parts::Part::ownedActions` (systems) | empty | answers | ok |
| `new Probe::Vw1()` | `ownedPorts` | `Parts::Part::ownedPorts` (systems) | empty | answers | ok |
| `new Probe::Vw1()` | `ownedStates` | `Parts::Part::ownedStates` (systems) | empty | answers | ok |
| `new Probe::Vw1()` | `performedActions` | `Parts::Part::performedActions` (systems) | empty | answers | ok |
| `new Probe::Vw1()` | `self` | `Views::View::self` (systemsFRAME) | no such feature: self | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::Vw1()` | `shape` | `Items::Item::shape` (systems) | empty | answers | ok |
| `new Probe::Vw1()` | `start` | `Parts::Part::start` (systemsFRAME) | no such feature: start | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::Vw1()` | `subitems` | `Items::Item::subitems` (systems) | empty | answers | ok |
| `new Probe::Vw1()` | `subparts` | `Items::Item::subparts` (systems) | empty | answers | ok |
| `new Probe::Vw1()` | `subviews` | `Views::View::subviews` (systems) | no such feature: subviews | error (`ErrNoSuchFeature`) | gap: usage kind left out of the shape |
| `new Probe::Vw1()` | `viewRendering` | `Views::View::viewRendering` (systems) | no such feature: viewRendering | error (`ErrNoSuchFeature`) | gap: usage kind left out of the shape |
| `new Probe::Vw1()` | `viewpointConformance` | `Views::View::viewpointConformance` (systems) | no such feature: viewpointConformance | error (`ErrNoSuchFeature`) | gap: usage kind left out of the shape |
| `new Probe::Vw1()` | `viewpointSatisfactions` | `Views::View::viewpointSatisfactions` (systems) | no such feature: viewpointSatisfactions | error (`ErrNoSuchFeature`) | gap: usage kind left out of the shape |
| `new Probe::Vw1()` | `voids` | `Items::Item::voids` (systems) | empty | answers | ok |
| `new Probe::Vp1()` | `actors` | `Requirements::RequirementCheck::actors` (systems) | empty | answers | ok |
| `new Probe::Vp1()` | `assumptions` | `Requirements::RequirementCheck::assumptions` (systems) | empty | answers | ok |
| `new Probe::Vp1()` | `con1` | `Probe::Vp1::con1` (model) | no such feature: con1 | error (`ErrNoSuchFeature`) | gap: usage kind left out of the shape |
| `new Probe::Vp1()` | `concerns` | `Requirements::RequirementCheck::concerns` (systems) | no such feature: concerns | error (`ErrNoSuchFeature`) | gap: usage kind left out of the shape |
| `new Probe::Vp1()` | `constraints` | `Requirements::RequirementCheck::constraints` (systems) | empty | answers | ok |
| `new Probe::Vp1()` | `result` | `Requirements::RequirementConstraintCheck::result` (systemsFRAME) | no such feature: result | error (`ErrNoSuchFeature`) | gap: result parameter read on its object (sibling leaves parameters out) |
| `new Probe::Vp1()` | `self` | `Views::ViewpointCheck::self` (systemsFRAME) | no such feature: self | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::Vp1()` | `stakeholders` | `Requirements::RequirementCheck::stakeholders` (systems) | empty | answers | ok |
| `new Probe::Vp1()` | `subj` | `Views::ViewpointCheck::subj` (systems) | uninitialized feature value: subj | typed error (`errUninitializedFeatureValue`) | gap: unbound reference read refused |
| `new Probe::Vp1()` | `subrequirements` | `Requirements::RequirementCheck::subrequirements` (systems) | empty | answers | ok |
| `new Probe::Con1()` | `actors` | `Requirements::RequirementCheck::actors` (systems) | empty | answers | ok |
| `new Probe::Con1()` | `assumptions` | `Requirements::RequirementCheck::assumptions` (systems) | empty | answers | ok |
| `new Probe::Con1()` | `concerns` | `Requirements::RequirementCheck::concerns` (systems) | no such feature: concerns | error (`ErrNoSuchFeature`) | gap: usage kind left out of the shape |
| `new Probe::Con1()` | `constraints` | `Requirements::RequirementCheck::constraints` (systems) | empty | answers | ok |
| `new Probe::Con1()` | `result` | `Requirements::RequirementConstraintCheck::result` (systemsFRAME) | no such feature: result | error (`ErrNoSuchFeature`) | gap: result parameter read on its object (sibling leaves parameters out) |
| `new Probe::Con1()` | `self` | `Requirements::ConcernCheck::self` (systemsFRAME) | no such feature: self | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::Con1()` | `sh` | `Probe::Con1::sh` (model) | object:Probe::P2 | answers | ok |
| `new Probe::Con1()` | `stakeholders` | `Requirements::RequirementCheck::stakeholders` (systems) | empty | answers | ok |
| `new Probe::Con1()` | `subj` | `Requirements::RequirementCheck::subj` (systems) | uninitialized feature value: subj | typed error (`errUninitializedFeatureValue`) | gap: unbound reference read refused |
| `new Probe::Con1()` | `subrequirements` | `Requirements::RequirementCheck::subrequirements` (systems) | empty | answers | ok |
| `new Probe::Rd1()` | `boundingShapes` | `Items::Item::boundingShapes` (systems) | empty | answers | ok |
| `new Probe::Rd1()` | `checkedConstraints` | `Items::Item::checkedConstraints` (systems) | empty | answers | ok |
| `new Probe::Rd1()` | `done` | `Parts::Part::done` (systemsFRAME) | no such feature: done | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::Rd1()` | `envelopingShapes` | `Items::Item::envelopingShapes` (systems) | empty | answers | ok |
| `new Probe::Rd1()` | `exhibitedStates` | `Parts::Part::exhibitedStates` (systems) | empty | answers | ok |
| `new Probe::Rd1()` | `isSolid` | `Items::Item::isSolid` (systems) | constant true | answers | ok |
| `new Probe::Rd1()` | `ownedActions` | `Parts::Part::ownedActions` (systems) | empty | answers | ok |
| `new Probe::Rd1()` | `ownedPorts` | `Parts::Part::ownedPorts` (systems) | empty | answers | ok |
| `new Probe::Rd1()` | `ownedStates` | `Parts::Part::ownedStates` (systems) | empty | answers | ok |
| `new Probe::Rd1()` | `performedActions` | `Parts::Part::performedActions` (systems) | empty | answers | ok |
| `new Probe::Rd1()` | `self` | `Views::Rendering::self` (systemsFRAME) | no such feature: self | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::Rd1()` | `shape` | `Items::Item::shape` (systems) | empty | answers | ok |
| `new Probe::Rd1()` | `start` | `Parts::Part::start` (systemsFRAME) | no such feature: start | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::Rd1()` | `subitems` | `Items::Item::subitems` (systems) | empty | answers | ok |
| `new Probe::Rd1()` | `subparts` | `Items::Item::subparts` (systems) | empty | answers | ok |
| `new Probe::Rd1()` | `subrenderings` | `Views::Rendering::subrenderings` (systems) | no such feature: subrenderings | error (`ErrNoSuchFeature`) | gap: usage kind left out of the shape |
| `new Probe::Rd1()` | `voids` | `Items::Item::voids` (systems) | empty | answers | ok |
| `new Probe::Cn1()` | `boundingShapes` | `Items::Item::boundingShapes` (systems) | empty | answers | ok |
| `new Probe::Cn1()` | `checkedConstraints` | `Items::Item::checkedConstraints` (systems) | empty | answers | ok |
| `new Probe::Cn1()` | `done` | `Parts::Part::done` (systemsFRAME) | no such feature: done | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::Cn1()` | `e1` | `Probe::Cn1::e1` (model) | object:Probe::P2 | answers | ok |
| `new Probe::Cn1()` | `e2` | `Probe::Cn1::e2` (model) | object:Probe::P2 | answers | ok |
| `new Probe::Cn1()` | `envelopingShapes` | `Items::Item::envelopingShapes` (systems) | empty | answers | ok |
| `new Probe::Cn1()` | `exhibitedStates` | `Parts::Part::exhibitedStates` (systems) | empty | answers | ok |
| `new Probe::Cn1()` | `isSolid` | `Items::Item::isSolid` (systems) | constant true | answers | ok |
| `new Probe::Cn1()` | `ownedActions` | `Parts::Part::ownedActions` (systems) | empty | answers | ok |
| `new Probe::Cn1()` | `ownedPorts` | `Parts::Part::ownedPorts` (systems) | empty | answers | ok |
| `new Probe::Cn1()` | `ownedStates` | `Parts::Part::ownedStates` (systems) | empty | answers | ok |
| `new Probe::Cn1()` | `performedActions` | `Parts::Part::performedActions` (systems) | empty | answers | ok |
| `new Probe::Cn1()` | `self` | `Parts::Part::self` (systemsFRAME) | no such feature: self | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::Cn1()` | `shape` | `Items::Item::shape` (systems) | empty | answers | ok |
| `new Probe::Cn1()` | `source` | `Connections::BinaryConnection::source` (systems) | object:Probe::P2 | answers | ok |
| `new Probe::Cn1()` | `start` | `Parts::Part::start` (systemsFRAME) | no such feature: start | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::Cn1()` | `subitems` | `Items::Item::subitems` (systems) | empty | answers | ok |
| `new Probe::Cn1()` | `subparts` | `Items::Item::subparts` (systems) | empty | answers | ok |
| `new Probe::Cn1()` | `target` | `Connections::BinaryConnection::target` (systems) | object:Probe::P2 | answers | ok |
| `new Probe::Cn1()` | `voids` | `Items::Item::voids` (systems) | empty | answers | ok |
| `new Probe::If1()` | `boundingShapes` | `Items::Item::boundingShapes` (systems) | empty | answers | ok |
| `new Probe::If1()` | `checkedConstraints` | `Items::Item::checkedConstraints` (systems) | empty | answers | ok |
| `new Probe::If1()` | `done` | `Parts::Part::done` (systemsFRAME) | no such feature: done | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::If1()` | `envelopingShapes` | `Items::Item::envelopingShapes` (systems) | empty | answers | ok |
| `new Probe::If1()` | `exhibitedStates` | `Parts::Part::exhibitedStates` (systems) | empty | answers | ok |
| `new Probe::If1()` | `isSolid` | `Items::Item::isSolid` (systems) | constant true | answers | ok |
| `new Probe::If1()` | `ownedActions` | `Parts::Part::ownedActions` (systems) | empty | answers | ok |
| `new Probe::If1()` | `ownedPorts` | `Parts::Part::ownedPorts` (systems) | empty | answers | ok |
| `new Probe::If1()` | `ownedStates` | `Parts::Part::ownedStates` (systems) | empty | answers | ok |
| `new Probe::If1()` | `p1` | `Probe::If1::p1` (model) | object:Probe::Pt | answers | ok |
| `new Probe::If1()` | `p2` | `Probe::If1::p2` (model) | object:Probe::Pt | answers | ok |
| `new Probe::If1()` | `participant` | `Interfaces::BinaryInterface::participant` (systems) | uninitialized feature value: participant | typed error (`errUninitializedFeatureValue`) | gap: unbound reference read refused |
| `new Probe::If1()` | `performedActions` | `Parts::Part::performedActions` (systems) | empty | answers | ok |
| `new Probe::If1()` | `self` | `Parts::Part::self` (systemsFRAME) | no such feature: self | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::If1()` | `shape` | `Items::Item::shape` (systems) | empty | answers | ok |
| `new Probe::If1()` | `source` | `Interfaces::BinaryInterface::source` (systems) | object:Probe::Pt | answers | ok |
| `new Probe::If1()` | `start` | `Parts::Part::start` (systemsFRAME) | no such feature: start | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::If1()` | `subitems` | `Items::Item::subitems` (systems) | empty | answers | ok |
| `new Probe::If1()` | `subparts` | `Items::Item::subparts` (systems) | empty | answers | ok |
| `new Probe::If1()` | `target` | `Interfaces::BinaryInterface::target` (systems) | object:Probe::Pt | answers | ok |
| `new Probe::If1()` | `voids` | `Items::Item::voids` (systems) | empty | answers | ok |
| `new Probe::Al1()` | `a1` | `Probe::Al1::a1` (model) | object:Probe::P2 | answers | ok |
| `new Probe::Al1()` | `a2` | `Probe::Al1::a2` (model) | object:Probe::P2 | answers | ok |
| `new Probe::Al1()` | `boundingShapes` | `Items::Item::boundingShapes` (systems) | empty | answers | ok |
| `new Probe::Al1()` | `checkedConstraints` | `Items::Item::checkedConstraints` (systems) | empty | answers | ok |
| `new Probe::Al1()` | `done` | `Parts::Part::done` (systemsFRAME) | no such feature: done | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::Al1()` | `envelopingShapes` | `Items::Item::envelopingShapes` (systems) | empty | answers | ok |
| `new Probe::Al1()` | `exhibitedStates` | `Parts::Part::exhibitedStates` (systems) | empty | answers | ok |
| `new Probe::Al1()` | `isSolid` | `Items::Item::isSolid` (systems) | constant true | answers | ok |
| `new Probe::Al1()` | `ownedActions` | `Parts::Part::ownedActions` (systems) | empty | answers | ok |
| `new Probe::Al1()` | `ownedPorts` | `Parts::Part::ownedPorts` (systems) | empty | answers | ok |
| `new Probe::Al1()` | `ownedStates` | `Parts::Part::ownedStates` (systems) | empty | answers | ok |
| `new Probe::Al1()` | `performedActions` | `Parts::Part::performedActions` (systems) | empty | answers | ok |
| `new Probe::Al1()` | `self` | `Parts::Part::self` (systemsFRAME) | no such feature: self | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::Al1()` | `shape` | `Items::Item::shape` (systems) | empty | answers | ok |
| `new Probe::Al1()` | `source` | `Allocations::Allocation::source` (systems) | object:Probe::P2 | answers | ok |
| `new Probe::Al1()` | `start` | `Parts::Part::start` (systemsFRAME) | no such feature: start | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::Al1()` | `subitems` | `Items::Item::subitems` (systems) | empty | answers | ok |
| `new Probe::Al1()` | `subparts` | `Items::Item::subparts` (systems) | empty | answers | ok |
| `new Probe::Al1()` | `target` | `Allocations::Allocation::target` (systems) | object:Probe::P2 | answers | ok |
| `new Probe::Al1()` | `voids` | `Items::Item::voids` (systems) | empty | answers | ok |
| `new Probe::Fl1()` | `acceptSubactions` | `Actions::Action::acceptSubactions` (systems) | empty | answers | ok |
| `new Probe::Fl1()` | `assignments` | `Actions::Action::assignments` (systems) | empty | answers | ok |
| `new Probe::Fl1()` | `controls` | `Actions::Action::controls` (systems) | empty | answers | ok |
| `new Probe::Fl1()` | `decisionTransitions` | `Actions::Action::decisionTransitions` (systems) | empty | answers | ok |
| `new Probe::Fl1()` | `decisions` | `Actions::Action::decisions` (systems) | empty | answers | ok |
| `new Probe::Fl1()` | `done` | `Actions::Action::done` (systemsFRAME) | no such feature: done | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::Fl1()` | `forLoops` | `Actions::Action::forLoops` (systems) | empty | answers | ok |
| `new Probe::Fl1()` | `forks` | `Actions::Action::forks` (systems) | empty | answers | ok |
| `new Probe::Fl1()` | `ifSubactions` | `Actions::Action::ifSubactions` (systems) | empty | answers | ok |
| `new Probe::Fl1()` | `incomingTransfers` | `Actions::Action::incomingTransfers` (systemsFRAME) | no such feature: incomingTransfers | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::Fl1()` | `inflow` | `Probe::Fl1::inflow` (model) | object:Probe::I1 | answers | ok |
| `new Probe::Fl1()` | `joins` | `Actions::Action::joins` (systems) | empty | answers | ok |
| `new Probe::Fl1()` | `loops` | `Actions::Action::loops` (systems) | empty | answers | ok |
| `new Probe::Fl1()` | `merges` | `Actions::Action::merges` (systems) | empty | answers | ok |
| `new Probe::Fl1()` | `payload` | `Flows::MessageAction::payload` (systems) | empty | answers | ok |
| `new Probe::Fl1()` | `self` | `Actions::Action::self` (systemsFRAME) | no such feature: self | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::Fl1()` | `sendSubactions` | `Actions::Action::sendSubactions` (systems) | empty | answers | ok |
| `new Probe::Fl1()` | `start` | `Actions::Action::start` (systemsFRAME) | no such feature: start | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::Fl1()` | `subactions` | `Actions::Action::subactions` (systems) | empty | answers | ok |
| `new Probe::Fl1()` | `terminateSubactions` | `Actions::Action::terminateSubactions` (systems) | empty | answers | ok |
| `new Probe::Fl1()` | `transitions` | `Actions::Action::transitions` (systems) | empty | answers | ok |
| `new Probe::Fl1()` | `whileLoops` | `Actions::Action::whileLoops` (systems) | empty | answers | ok |
| `new Probe::E1()` | `a` | `Probe::E1::a` (model) | no such feature: a | error (`ErrNoSuchFeature`) | gap: usage kind left out of the shape |
| `new Probe::E1()` | `b` | `Probe::E1::b` (model) | no such feature: b | error (`ErrNoSuchFeature`) | gap: usage kind left out of the shape |
| `new Probe::M1()` | `boundingShapes` | `Items::Item::boundingShapes` (systems) | empty | answers | ok |
| `new Probe::M1()` | `checkedConstraints` | `Items::Item::checkedConstraints` (systems) | empty | answers | ok |
| `new Probe::M1()` | `done` | `Items::Item::done` (systemsFRAME) | no such feature: done | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::M1()` | `envelopingShapes` | `Items::Item::envelopingShapes` (systems) | empty | answers | ok |
| `new Probe::M1()` | `isSolid` | `Items::Item::isSolid` (systems) | constant true | answers | ok |
| `new Probe::M1()` | `self` | `Metadata::MetadataItem::self` (systemsFRAME) | no such feature: self | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::M1()` | `shape` | `Items::Item::shape` (systems) | empty | answers | ok |
| `new Probe::M1()` | `start` | `Items::Item::start` (systemsFRAME) | no such feature: start | error (`ErrNoSuchFeature`) | owned by `fix/kernel-member-reads` (frame member) |
| `new Probe::M1()` | `subitems` | `Items::Item::subitems` (systems) | empty | answers | ok |
| `new Probe::M1()` | `subparts` | `Items::Item::subparts` (systems) | empty | answers | ok |
| `new Probe::M1()` | `tag` | `Probe::M1::tag` (model) | object:ScalarValues::String | answers | ok |
| `new Probe::M1()` | `voids` | `Items::Item::voids` (systems) | empty | answers | ok |
| `new Probe::At1()` | `v` | `Probe::At1::v` (model) | object:ScalarValues::Real | answers | ok |

Verdict counts: ok: 546, owned by `fix/kernel-member-reads` (frame member): 129, by design: model-level open count: 120, gap: usage kind left out of the shape: 39, gap: unbound reference read refused: 23, known: pinned by `instance_library_geometry_box` fixtures: 21, gap: case usage read as a calc: 14, gap: result parameter read on its object (sibling leaves parameters out): 8, gap: Domain default not evaluable: 1.

## Sweep 2 — Kernel Function Library callability

Every function the seventeen Kernel Function Library files declare, called with spec-valid arguments in its qualified form and, where the library binds one, its operator or short form. Abstract functions have no body to call; they are probed through the concrete operator that specializes them and listed in the notes above. Probes evaluate in a package importing `ScalarValues::*` and `Collections::*`; a bare name that does not resolve there is re-tried under an import of every function package (*with imports*). The pilot column is the pinned evaluator's answer for the same expression in the same package.

| Function | Probe | Expected | Outcome | Bucket | Pilot |
|---|---|---|---|---|---|
| `BaseFunctions::'=='` | `1 == 1` | `true` | constant true | answers | LiteralBoolean true |
| `BaseFunctions::'=='` | `BaseFunctions::'=='(1, 2)` | `false` | constant false | answers | LiteralBoolean false |
| `BaseFunctions::'!='` | `1 != 2` | `true` | constant true | answers | LiteralBoolean true |
| `BaseFunctions::'!='` | `BaseFunctions::'!='(1, 1)` | `false` | constant false | answers | LiteralBoolean false |
| `BaseFunctions::'==='` | `Probe::p1 === Probe::p1` | `true` | constant true | answers | LiteralBoolean true |
| `BaseFunctions::'==='` | `BaseFunctions::'==='(1, 1)` | `true` | constant true | answers | LiteralBoolean true |
| `BaseFunctions::'!=='` | `Probe::p1 !== Probe::p2` | `true` | constant true | answers | LiteralBoolean true |
| `BaseFunctions::'!=='` | `BaseFunctions::'!=='(1, 1)` | `false` | constant false | answers | LiteralBoolean false |
| `BaseFunctions::ToString` | `BaseFunctions::ToString(42)` | `"42"` | string "42" | answers | no verdict (unevaluated) |
| `BaseFunctions::'['` | `2.0 [SI::m]` | `2.0 [m]` | quantity 2.0 [SI::m] | answers | no verdict (unevaluated) |
| `BaseFunctions::'['` | `BaseFunctions::'['(2.0, SI::m)` | `2.0 [m]` | error library function is not evaluable: BaseFunctions::'[': '[' is abstract: the runtime evaluates its operator notatio… | typed error | no verdict (unevaluated) |
| `BaseFunctions::'#'` | `(10, 20, 30)#(2)` | `20` | constant 20 | answers | LiteralInteger 20 |
| `BaseFunctions::'#'` | `BaseFunctions::'#'((10, 20, 30), 2)` | `20` | constant 20 | answers | LiteralInteger 20 |
| `BaseFunctions::','` | `(1, 2)` | `(1, 2)` | collection(2) [1, 2] | answers | LiteralInteger 1 |
| `BaseFunctions::','` | `BaseFunctions::','(1, 2)` | `(1, 2)` | collection(2) [1, 2] | answers | LiteralInteger 1 |
| `BaseFunctions::'all'` | `size(all Probe::E)` | `2` | error unresolved reference: size\|\| WITH IMPORTS: constant 2 | answers with imports | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'size'. (32.sysm…) |
| `BaseFunctions::'all'` | `size(BaseFunctions::'all'(Probe::E))` | `2` | error cannot evaluate definition Probe::E | typed error | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'size'. (34.sysm…) |
| `BaseFunctions::'istype'` | `1 istype Integer` | `true` | constant true | answers | no verdict (pilot: ERROR:Couldn't resolve reference to Type 'Integer'. (36.sysm…) |
| `BaseFunctions::'istype'` | `1 istype String` | `false` | constant false | answers | no verdict (pilot: ERROR:Couldn't resolve reference to Type 'String'. (38.sysml…) |
| `BaseFunctions::'istype'` | `BaseFunctions::'istype'(1, Integer)` | `true` | error cannot evaluate definition Integer | typed error | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'Integer'. (40.s…) |
| `BaseFunctions::'hastype'` | `1 hastype Integer` | `true` | constant true | answers | no verdict (pilot: ERROR:Couldn't resolve reference to Type 'Integer'. (42.sysm…) |
| `BaseFunctions::'hastype'` | `1 hastype Real` | `false` | constant false | answers | no verdict (pilot: ERROR:Couldn't resolve reference to Type 'Real'. (44.sysml l…) |
| `BaseFunctions::'hastype'` | `BaseFunctions::'hastype'(1, Integer)` | `true` | error cannot evaluate definition Integer | typed error | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'Integer'. (46.s…) |
| `BaseFunctions::'@'` | `1 @ Integer` | `true` | constant true | answers | no verdict (pilot: ERROR:Couldn't resolve reference to Type 'Integer'. (48.sysm…) |
| `BaseFunctions::'@'` | `BaseFunctions::'@'(1, Integer)` | `true` | error cannot evaluate definition Integer | typed error | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'Integer'. (50.s…) |
| `BaseFunctions::'@@'` | `Probe::Tagged @@ Probe::M` | `true` | constant false | answers **wrong** | LiteralBoolean true |
| `BaseFunctions::'@@'` | `Probe::P1 @@ Probe::M` | `false` | constant false | answers | LiteralBoolean false |
| `BaseFunctions::'as'` | `1 as Real` | `1` | constant 1 | answers | no verdict (pilot: ERROR:Couldn't resolve reference to Type 'Real'. (56.sysml l…) |
| `BaseFunctions::'as'` | `BaseFunctions::'as'(1, Real)` | `1` | error cannot evaluate definition Real | typed error | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'Real'. (58.sysm…) |
| `BaseFunctions::'as'` | `(1, "a", 2) as Integer` | `(1, 2)` | collection(2) [1, 2] | answers | no verdict (pilot: ERROR:Couldn't resolve reference to Type 'Integer'. (60.sysm…) |
| `BaseFunctions::'meta'` | `Probe::P1 meta SysML::PartDefinition` | `metaobject` | collection(1) [meta(Probe::P1 : SysML::Systems::PartDefinition)] | answers | MetadataFeature PartDefinition P1 |
| `BaseFunctions::'meta'` | `BaseFunctions::'meta'(Probe::P1, SysML::PartDefinition)` | `metaobject` | error cannot evaluate definition Probe::P1 | typed error | no verdict (pilot: ERROR:Must correspond to one input parameter of the invoked …) |
| `BooleanFunctions::'not'` | `not true` | `false` | constant false | answers | LiteralBoolean false |
| `BooleanFunctions::'not'` | `BooleanFunctions::'not'(false)` | `true` | constant true | answers | no verdict (unevaluated) |
| `BooleanFunctions::'xor'` | `true xor false` | `true` | constant true | answers | LiteralBoolean true |
| `BooleanFunctions::'xor'` | `BooleanFunctions::'xor'(true, true)` | `false` | constant false | answers | no verdict (unevaluated) |
| `BooleanFunctions::'\|'` | `true \| false` | `true` | constant true | answers | LiteralBoolean true |
| `BooleanFunctions::'\|'` | `BooleanFunctions::'\|'(false, false)` | `false` | constant false | answers | no verdict (unevaluated) |
| `BooleanFunctions::'&'` | `true & false` | `false` | constant false | answers | LiteralBoolean false |
| `BooleanFunctions::'&'` | `BooleanFunctions::'&'(true, true)` | `true` | constant true | answers | no verdict (unevaluated) |
| `BooleanFunctions::'=='` | `true == true` | `true` | constant true | answers | LiteralBoolean true |
| `BooleanFunctions::'=='` | `BooleanFunctions::'=='(true, false)` | `false` | constant false | answers | no verdict (unevaluated) |
| `BooleanFunctions::ToString` | `BooleanFunctions::ToString(true)` | `"true"` | string "true" | answers | no verdict (unevaluated) |
| `BooleanFunctions::ToBoolean` | `BooleanFunctions::ToBoolean("true")` | `true` | constant true | answers | no verdict (unevaluated) |
| `BooleanFunctions::ToBoolean` | `BooleanFunctions::ToBoolean("false")` | `false` | constant false | answers | no verdict (unevaluated) |
| `CollectionFunctions::'=='` | `CollectionFunctions::'=='(Probe::lst, Probe::lst)` | `true` | constant true | answers | LiteralBoolean true |
| `CollectionFunctions::'=='` | `Probe::lst == Probe::os` | `false` | constant false | answers | LiteralBoolean false |
| `CollectionFunctions::size` | `CollectionFunctions::size(Probe::lst)` | `3` | constant 3 | answers | LiteralInteger 3 |
| `CollectionFunctions::isEmpty` | `CollectionFunctions::isEmpty(Probe::lst)` | `false` | constant false | answers | LiteralBoolean false |
| `CollectionFunctions::notEmpty` | `CollectionFunctions::notEmpty(Probe::lst)` | `true` | constant true | answers | LiteralBoolean true |
| `CollectionFunctions::contains` | `CollectionFunctions::contains(Probe::lst, 2)` | `true` | constant true | answers | LiteralBoolean true |
| `CollectionFunctions::contains` | `CollectionFunctions::contains(Probe::lst, 9)` | `false` | constant false | answers | LiteralBoolean false |
| `CollectionFunctions::containsAll` | `CollectionFunctions::containsAll(Probe::lst, Probe::os)` | `true` | constant true | answers | LiteralBoolean true |
| `CollectionFunctions::head` | `CollectionFunctions::head(Probe::os)` | `3` | constant 3 | answers | LiteralInteger 3 |
| `CollectionFunctions::tail` | `CollectionFunctions::tail(Probe::os)` | `(1, 2)` | collection(2) [1, 2] | answers | LiteralInteger 1 |
| `CollectionFunctions::last` | `CollectionFunctions::last(Probe::os)` | `2` | constant 2 | answers | LiteralInteger 2 |
| `CollectionFunctions::'#'` | `CollectionFunctions::'#'(Probe::os, 1)` | `3` | constant 3 | answers | LiteralInteger 3 |
| `CollectionFunctions::'#'` | `Probe::os#(1)` | `3` | object:Probe::os | answers **wrong** | LiteralInteger 3 |
| `CollectionFunctions::'array#'` | `CollectionFunctions::'array#'(Probe::arr, (2, 1))` | `3` | constant 3 | answers | LiteralInteger 3 |
| `CollectionFunctions::'array#'` | `Probe::arr#(2, 1)` | `3` | constant 3 | answers | LiteralInteger 3 |
| `ComplexFunctions::rect` | `ComplexFunctions::rect(1.0, 2.0)` | `1.0 + 2.0i` | complex number 1.0 + 2.0i | answers | no verdict (unevaluated) |
| `ComplexFunctions::polar` | `ComplexFunctions::re(ComplexFunctions::polar(2.0, 0.0))` | `2.0` | constant 2.0 | answers | no verdict (unevaluated) |
| `ComplexFunctions::re` | `ComplexFunctions::re(ComplexFunctions::rect(1.0, 2.0))` | `1.0` | constant 1.0 | answers | no verdict (unevaluated) |
| `ComplexFunctions::im` | `ComplexFunctions::im(ComplexFunctions::rect(1.0, 2.0))` | `2.0` | constant 2.0 | answers | no verdict (unevaluated) |
| `ComplexFunctions::isZero` | `ComplexFunctions::isZero(ComplexFunctions::rect(0.0, 0.0))` | `true` | constant true | answers | LiteralBoolean false |
| `ComplexFunctions::isUnit` | `ComplexFunctions::isUnit(ComplexFunctions::rect(1.0, 0.0))` | `true` | constant true | answers | LiteralBoolean false |
| `ComplexFunctions::abs` | `ComplexFunctions::abs(ComplexFunctions::rect(3.0, 4.0))` | `5.0` | constant 5.0 | answers | no verdict (unevaluated) |
| `ComplexFunctions::arg` | `ComplexFunctions::arg(ComplexFunctions::rect(1.0, 0.0))` | `0.0` | constant 0.0 | answers | no verdict (unevaluated) |
| `ComplexFunctions::'+'` | `ComplexFunctions::'+'(ComplexFunctions::rect(1.0, 2.0), ComplexFunctions::rect(1.0, 1.0))` | `2.0 + 3.0i` | complex number 2.0 + 3.0i | answers | no verdict (unevaluated) |
| `ComplexFunctions::'+'` | `ComplexFunctions::rect(1.0, 2.0) + ComplexFunctions::rect(1.0, 1.0)` | `2.0 + 3.0i` | complex number 2.0 + 3.0i | answers | no verdict (unevaluated) |
| `ComplexFunctions::'-'` | `ComplexFunctions::rect(1.0, 2.0) - ComplexFunctions::rect(1.0, 1.0)` | `0.0 + 1.0i` | complex number 0.0 + 1.0i | answers | no verdict (unevaluated) |
| `ComplexFunctions::'*'` | `ComplexFunctions::rect(0.0, 1.0) * ComplexFunctions::rect(0.0, 1.0)` | `-1.0 + 0.0i` | complex number -1.0 + 0.0i | answers | no verdict (unevaluated) |
| `ComplexFunctions::'/'` | `ComplexFunctions::rect(2.0, 0.0) / ComplexFunctions::rect(1.0, 0.0)` | `2.0 + 0.0i` | complex number 2.0 + 0.0i | answers | no verdict (unevaluated) |
| `ComplexFunctions::'**'` | `ComplexFunctions::rect(0.0, 1.0) ** 2` | `-1.0 + 0.0i` | complex number -1.0 + 0.0i | answers | no verdict (unevaluated) |
| `ComplexFunctions::'^'` | `ComplexFunctions::rect(0.0, 1.0) ^ 2` | `-1.0 + 0.0i` | complex number -1.0 + 0.0i | answers | no verdict (unevaluated) |
| `ComplexFunctions::'=='` | `ComplexFunctions::rect(1.0, 2.0) == ComplexFunctions::rect(1.0, 2.0)` | `true` | constant true | answers | LiteralBoolean false |
| `ComplexFunctions::ToString` | `ComplexFunctions::ToString(ComplexFunctions::rect(1.0, 2.0))` | `string` | error library function is not evaluable: ComplexFunctions::ToString: no string notation for a Complex value is defined | typed error | no verdict (unevaluated) |
| `ComplexFunctions::ToComplex` | `ComplexFunctions::ToComplex("1.0 + 2.0i")` | `1.0 + 2.0i` | error library function is not evaluable: ComplexFunctions::ToComplex: no string notation for a Complex value is defined | typed error | no verdict (unevaluated) |
| `ComplexFunctions::sum` | `ComplexFunctions::sum((ComplexFunctions::rect(1.0, 0.0), ComplexFunctions::rect(0.0, 1.0)))` | `1.0 + 1.0i` | complex number 1.0 + 1.0i | answers | — |
| `ComplexFunctions::product` | `ComplexFunctions::product((ComplexFunctions::rect(2.0, 0.0), ComplexFunctions::rect(0.0, 1.0)))` | `0.0 + 2.0i` | complex number 0.0 + 2.0i | answers | — |
| `ControlFunctions::'.'` | `Probe::p1.sub` | `collection(2)` | collection(2) [instance(6), instance(7)] | answers | PartUsage sub |
| `ControlFunctions::'.'` | `(Probe::p1, Probe::p1).sub` | `collection(4)` | collection(4) [instance(6), instance(7), instance(6), instance(7)] | answers | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'sub'. (164.sysm…) |
| `ControlFunctions::'if'` | `if true ? 1 else 2` | `1` | constant 1 | answers | LiteralInteger 1 |
| `ControlFunctions::'if'` | `if false ? 1 else 2` | `2` | constant 2 | answers | LiteralInteger 2 |
| `ControlFunctions::'if'` | `ControlFunctions::'if'(true, 1, 2)` | `1` | constant 1 | answers | LiteralInteger 1 |
| `ControlFunctions::'if'` | `if true ? 1 else 1/0` | `1` | constant 1 | answers | LiteralInteger 1 |
| `ControlFunctions::'??'` | `null ?? 3` | `3` | constant 3 | answers | LiteralInteger 3 |
| `ControlFunctions::'??'` | `Probe::empty ?? 3` | `3` | constant 3 | answers | LiteralInteger 3 |
| `ControlFunctions::'??'` | `2 ?? 3` | `2` | constant 2 | answers | LiteralInteger 2 |
| `ControlFunctions::'??'` | `ControlFunctions::'??'((), 5)` | `5` | constant 5 | answers | LiteralInteger 5 |
| `ControlFunctions::'??'` | `1 ?? 1/0` | `1` | constant 1 | answers | LiteralInteger 1 |
| `ControlFunctions::'and'` | `true and false` | `false` | constant false | answers | LiteralBoolean false |
| `ControlFunctions::'and'` | `false and (1/0 == 0)` | `false` | constant false | answers | LiteralBoolean false |
| `ControlFunctions::'and'` | `ControlFunctions::'and'(true, true)` | `true` | constant true | answers | LiteralBoolean true |
| `ControlFunctions::'or'` | `true or (1/0 == 0)` | `true` | constant true | answers | LiteralBoolean true |
| `ControlFunctions::'or'` | `false or false` | `false` | constant false | answers | LiteralBoolean false |
| `ControlFunctions::'or'` | `ControlFunctions::'or'(false, true)` | `true` | constant true | answers | LiteralBoolean true |
| `ControlFunctions::'implies'` | `false implies (1/0 == 0)` | `true` | constant true | answers | LiteralBoolean true |
| `ControlFunctions::'implies'` | `true implies false` | `false` | constant false | answers | LiteralBoolean false |
| `ControlFunctions::'implies'` | `ControlFunctions::'implies'(true, true)` | `true` | constant true | answers | LiteralBoolean true |
| `ControlFunctions::collect` | `(1, 2, 3)->collect{in x; x * 2}` | `(2, 4, 6)` | error unresolved reference: collect; \|\| WITH IMPORTS: collection(3) [2, 4, 6] | answers with imports | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'collect'. (202.…) |
| `ControlFunctions::collect` | `ControlFunctions::collect((1, 2, 3), {in x; x * 2})` | `(2, 4, 6)` | collection(3) [2, 4, 6] | answers | LiteralInteger 2 |
| `ControlFunctions::select` | `(1, 2, 3)->select{in x; x > 1}` | `(2, 3)` | error unresolved reference: select; \|\| WITH IMPORTS: collection(2) [2, 3] | answers with imports | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'select'. (206.s…) |
| `ControlFunctions::select` | `ControlFunctions::select((1, 2, 3), {in x; x > 1})` | `(2, 3)` | collection(2) [2, 3] | answers | LiteralInteger 2 |
| `ControlFunctions::selectOne` | `(1, 2, 3)->selectOne{in x; x > 1}` | `2` | error unresolved reference: selectOne; \|\| WITH IMPORTS: constant 2 | answers with imports | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'selectOne'. (21…) |
| `ControlFunctions::selectOne` | `ControlFunctions::selectOne((1, 2, 3), {in x; x > 1})` | `2` | constant 2 | answers | LiteralInteger 2 |
| `ControlFunctions::reject` | `(1, 2, 3)->reject{in x; x > 1}` | `1` | error unresolved reference: reject; \|\| WITH IMPORTS: collection(1) [1] | answers with imports | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'reject'. (214.s…) |
| `ControlFunctions::reject` | `ControlFunctions::reject((1, 2, 3), {in x; x > 1})` | `1` | collection(1) [1] | answers | LiteralInteger 1 |
| `ControlFunctions::reduce` | `(1, 2, 3)->reduce{in a; in b; a + b}` | `6` | error unresolved reference: reduce; \|\| WITH IMPORTS: constant 6 | answers with imports | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'reduce'. (218.s…) |
| `ControlFunctions::reduce` | `(1, 2, 3)->reduce '+'` | `6` | error unresolved reference: + \|\| WITH IMPORTS: error type mismatch: ControlFunctions::reduce requires a body expression,… | typed error with imports | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'reduce'. (220.s…) |
| `ControlFunctions::reduce` | `ControlFunctions::reduce((1, 2, 3), {in a; in b; a + b})` | `6` | constant 6 | answers | LiteralInteger 6 |
| `ControlFunctions::forAll` | `(1, 2, 3)->forAll{in x; x > 0}` | `true` | error unresolved reference: forAll; \|\| WITH IMPORTS: constant true | answers with imports | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'forAll'. (224.s…) |
| `ControlFunctions::forAll` | `ControlFunctions::forAll((1, 2, 3), {in x; x > 1})` | `false` | constant false | answers | LiteralBoolean false |
| `ControlFunctions::exists` | `(1, 2, 3)->exists{in x; x > 2}` | `true` | error unresolved reference: exists; \|\| WITH IMPORTS: constant true | answers with imports | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'exists'. (228.s…) |
| `ControlFunctions::exists` | `ControlFunctions::exists((1, 2, 3), {in x; x > 3})` | `false` | constant false | answers | LiteralBoolean false |
| `ControlFunctions::allTrue` | `ControlFunctions::allTrue((true, true))` | `true` | constant true | answers | LiteralBoolean true |
| `ControlFunctions::allTrue` | `(true, false)->allTrue()` | `false` | error unresolved reference: allTrue; \|\| WITH IMPORTS: constant false | answers with imports | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'allTrue'. (234.…) |
| `ControlFunctions::anyTrue` | `ControlFunctions::anyTrue((false, true))` | `true` | constant true | answers | LiteralBoolean true |
| `ControlFunctions::anyTrue` | `(false, false)->anyTrue()` | `false` | error unresolved reference: anyTrue; \|\| WITH IMPORTS: constant false | answers with imports | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'anyTrue'. (238.…) |
| `ControlFunctions::minimize` | `(3, 1, 2)->minimize{in x; x}` | `1` | error unresolved reference: minimize; \|\| WITH IMPORTS: constant 1 | answers with imports | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'minimize'. (240…) |
| `ControlFunctions::minimize` | `ControlFunctions::minimize((3, 1, 2), {in x; -x})` | `3` | constant -3 | answers | LiteralInteger -3 |
| `ControlFunctions::maximize` | `(3, 1, 2)->maximize{in x; x}` | `3` | error unresolved reference: maximize; \|\| WITH IMPORTS: constant 3 | answers with imports | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'maximize'. (244…) |
| `ControlFunctions::maximize` | `ControlFunctions::maximize((3, 1, 2), {in x; -x})` | `1` | constant -1 | answers | LiteralInteger -1 |
| `DataFunctions::'==='` | `DataFunctions::'==='(1, 1)` | `true` | constant true | answers | LiteralBoolean true |
| `DataFunctions::'==='` | `1 === 2` | `false` | constant false | answers | LiteralBoolean false |
| `IntegerFunctions::abs` | `IntegerFunctions::abs(-3)` | `3` | constant 3 | answers | no verdict (unevaluated) |
| `IntegerFunctions::abs` | `abs(-3)` | `3` | error unresolved reference: abs\|\| WITH IMPORTS: constant 3 | answers with imports | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'abs'. (254.sysm…) |
| `IntegerFunctions::'+'` | `1 + 2` | `3` | constant 3 | answers | LiteralInteger 3 |
| `IntegerFunctions::'+'` | `IntegerFunctions::'+'(1, 2)` | `3` | constant 3 | answers | no verdict (unevaluated) |
| `IntegerFunctions::'+'` | `IntegerFunctions::'+'(1)` | `1` | constant 1 | answers | no verdict (unevaluated) |
| `IntegerFunctions::'-'` | `5 - 2` | `3` | constant 3 | answers | LiteralInteger 3 |
| `IntegerFunctions::'-'` | `IntegerFunctions::'-'(5)` | `-5` | constant -5 | answers | no verdict (unevaluated) |
| `IntegerFunctions::'-'` | `-5` | `-5` | constant -5 | answers | LiteralInteger -5 |
| `IntegerFunctions::'*'` | `2 * 3` | `6` | constant 6 | answers | LiteralInteger 6 |
| `IntegerFunctions::'*'` | `IntegerFunctions::'*'(2, 3)` | `6` | constant 6 | answers | no verdict (unevaluated) |
| `IntegerFunctions::'/'` | `7 / 2` | `7/2` | constant 3.5 | answers | LiteralRational 3.5 |
| `IntegerFunctions::'/'` | `IntegerFunctions::'/'(6, 2)` | `3` | constant 3.0 | answers | no verdict (unevaluated) |
| `IntegerFunctions::'**'` | `2 ** 3` | `8` | constant 8 | answers | LiteralRational 8.0 |
| `IntegerFunctions::'**'` | `IntegerFunctions::'**'(2, 3)` | `8` | constant 8 | answers | no verdict (unevaluated) |
| `IntegerFunctions::'^'` | `2 ^ 3` | `8` | constant 8 | answers | LiteralRational 8.0 |
| `IntegerFunctions::'^'` | `IntegerFunctions::'^'(2, 3)` | `8` | constant 8 | answers | no verdict (unevaluated) |
| `IntegerFunctions::'%'` | `7 % 2` | `1` | constant 1 | answers | LiteralInteger 1 |
| `IntegerFunctions::'%'` | `IntegerFunctions::'%'(-7, 2)` | `-1` | constant -1 | answers | no verdict (unevaluated) |
| `IntegerFunctions::'<'` | `1 < 2` | `true` | constant true | answers | LiteralBoolean true |
| `IntegerFunctions::'<'` | `IntegerFunctions::'<'(2, 1)` | `false` | constant false | answers | no verdict (unevaluated) |
| `IntegerFunctions::'>'` | `2 > 1` | `true` | constant true | answers | LiteralBoolean true |
| `IntegerFunctions::'>'` | `IntegerFunctions::'>'(1, 2)` | `false` | constant false | answers | no verdict (unevaluated) |
| `IntegerFunctions::'<='` | `2 <= 2` | `true` | constant true | answers | LiteralBoolean true |
| `IntegerFunctions::'<='` | `IntegerFunctions::'<='(3, 2)` | `false` | constant false | answers | no verdict (unevaluated) |
| `IntegerFunctions::'>='` | `2 >= 2` | `true` | constant true | answers | LiteralBoolean true |
| `IntegerFunctions::'>='` | `IntegerFunctions::'>='(1, 2)` | `false` | constant false | answers | no verdict (unevaluated) |
| `IntegerFunctions::max` | `IntegerFunctions::max(1, 2)` | `2` | constant 2 | answers | no verdict (unevaluated) |
| `IntegerFunctions::max` | `max(1, 2)` | `2` | error unresolved reference: max\|\| WITH IMPORTS: constant 2 | answers with imports | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'max'. (306.sysm…) |
| `IntegerFunctions::min` | `IntegerFunctions::min(1, 2)` | `1` | constant 1 | answers | no verdict (unevaluated) |
| `IntegerFunctions::min` | `min(1, 2)` | `1` | error unresolved reference: min\|\| WITH IMPORTS: constant 1 | answers with imports | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'min'. (310.sysm…) |
| `IntegerFunctions::'=='` | `IntegerFunctions::'=='(1, 1)` | `true` | constant true | answers | no verdict (unevaluated) |
| `IntegerFunctions::'..'` | `1..3` | `(1, 2, 3)` | collection(3) [1, 2, 3] | answers | LiteralInteger 1 |
| `IntegerFunctions::'..'` | `IntegerFunctions::'..'(1, 3)` | `(1, 2, 3)` | collection(3) [1, 2, 3] | answers | no verdict (unevaluated) |
| `IntegerFunctions::'..'` | `3..1` | `()` | empty | answers | — |
| `IntegerFunctions::ToString` | `IntegerFunctions::ToString(7)` | `"7"` | string "7" | answers | no verdict (unevaluated) |
| `IntegerFunctions::ToNatural` | `IntegerFunctions::ToNatural(7)` | `7` | constant 7 | answers | no verdict (unevaluated) |
| `IntegerFunctions::ToInteger` | `IntegerFunctions::ToInteger("7")` | `7` | constant 7 | answers | no verdict (unevaluated) |
| `IntegerFunctions::sum` | `IntegerFunctions::sum((1, 2, 3))` | `6` | constant 6 | answers | no verdict (unevaluated) |
| `IntegerFunctions::sum` | `sum((1, 2, 3))` | `6` | error unresolved reference: sum\|\| WITH IMPORTS: constant 6 | answers with imports | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'sum'. (328.sysm…) |
| `IntegerFunctions::sum` | `IntegerFunctions::sum(())` | `0` | constant 0 | answers | no verdict (unevaluated) |
| `IntegerFunctions::product` | `IntegerFunctions::product((1, 2, 3))` | `6` | constant 6 | answers | no verdict (unevaluated) |
| `IntegerFunctions::product` | `product((2, 3))` | `6` | error unresolved reference: product\|\| WITH IMPORTS: constant 6 | answers with imports | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'product'. (334.…) |
| `IntegerFunctions::product` | `IntegerFunctions::product(())` | `1` | constant 1 | answers | no verdict (unevaluated) |
| `NaturalFunctions::'+'` | `NaturalFunctions::'+'(1, 2)` | `3` | constant 3 | answers | no verdict (unevaluated) |
| `NaturalFunctions::'*'` | `NaturalFunctions::'*'(2, 3)` | `6` | constant 6 | answers | no verdict (unevaluated) |
| `NaturalFunctions::'/'` | `NaturalFunctions::'/'(7, 2)` | `3` | error arithmetic domain error: function NaturalFunctions::'/' has no Natural result for 7 / 2; the quotient is 3.5 | typed error | no verdict (unevaluated) |
| `NaturalFunctions::'%'` | `NaturalFunctions::'%'(7, 2)` | `1` | constant 1 | answers | no verdict (unevaluated) |
| `NaturalFunctions::'<'` | `NaturalFunctions::'<'(1, 2)` | `true` | constant true | answers | no verdict (unevaluated) |
| `NaturalFunctions::'>'` | `NaturalFunctions::'>'(1, 2)` | `false` | constant false | answers | no verdict (unevaluated) |
| `NaturalFunctions::'<='` | `NaturalFunctions::'<='(2, 2)` | `true` | constant true | answers | no verdict (unevaluated) |
| `NaturalFunctions::'>='` | `NaturalFunctions::'>='(1, 2)` | `false` | constant false | answers | no verdict (unevaluated) |
| `NaturalFunctions::max` | `NaturalFunctions::max(1, 2)` | `2` | constant 2 | answers | no verdict (unevaluated) |
| `NaturalFunctions::min` | `NaturalFunctions::min(1, 2)` | `1` | constant 1 | answers | no verdict (unevaluated) |
| `NaturalFunctions::'=='` | `NaturalFunctions::'=='(1, 1)` | `true` | constant true | answers | no verdict (unevaluated) |
| `NaturalFunctions::ToString` | `NaturalFunctions::ToString(7)` | `"7"` | string "7" | answers | no verdict (unevaluated) |
| `NaturalFunctions::ToNatural` | `NaturalFunctions::ToNatural("7")` | `7` | constant 7 | answers | no verdict (unevaluated) |
| `NumericalFunctions::sum0` | `NumericalFunctions::sum0((1, 2), 0)` | `3` | constant 3 | answers | no verdict (unevaluated) |
| `NumericalFunctions::sum0` | `NumericalFunctions::sum0((), 0)` | `0` | constant 0 | answers | no verdict (unevaluated) |
| `NumericalFunctions::product1` | `NumericalFunctions::product1((2, 3), 1)` | `6` | constant 6 | answers | no verdict (unevaluated) |
| `NumericalFunctions::product1` | `NumericalFunctions::product1((), 1)` | `1` | constant 1 | answers | no verdict (unevaluated) |
| `OccurrenceFunctions::'==='` | `OccurrenceFunctions::'==='(Probe::p1, Probe::p1)` | `true` | constant true | answers | LiteralBoolean false |
| `OccurrenceFunctions::'==='` | `OccurrenceFunctions::'==='(Probe::p1, Probe::p2)` | `false` | constant false | answers | LiteralBoolean false |
| `OccurrenceFunctions::isDuring` | `OccurrenceFunctions::isDuring(Probe::p1)` | `boolean` | constant true | answers | LiteralBoolean true |
| `OccurrenceFunctions::create` | `OccurrenceFunctions::create(Probe::p1)` | `object` | error occurrence lifetime does not admit the change: function OccurrenceFunctions::create: object #1 (p1) began at 1, be… | typed error | PartUsage p1 |
| `OccurrenceFunctions::create` | `OccurrenceFunctions::create(new Probe::P2())` | `object` | object:Probe::P2 | answers | Feature |
| `OccurrenceFunctions::destroy` | `OccurrenceFunctions::destroy(new Probe::P2())` | `object` | object:Probe::P2 | answers | Feature |
| `OccurrenceFunctions::addNew` | `size(OccurrenceFunctions::addNew(Probe::p1.sub, new Probe::P2()))` | `3` | error unresolved reference: size\|\| WITH IMPORTS: constant 3 | answers with imports | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'size'. (384.sys…) |
| `OccurrenceFunctions::addNewAt` | `size(OccurrenceFunctions::addNewAt(Probe::p1.sub, new Probe::P2(), 1))` | `3` | error unresolved reference: size\|\| WITH IMPORTS: constant 3 | answers with imports | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'size'. (386.sys…) |
| `RationalFunctions::rat` | `RationalFunctions::rat(1, 2)` | `1/2` | constant 0.5 | answers | no verdict (unevaluated) |
| `RationalFunctions::numer` | `RationalFunctions::numer(RationalFunctions::rat(2, 4))` | `1` | constant 1 | answers | no verdict (unevaluated) |
| `RationalFunctions::denom` | `RationalFunctions::denom(RationalFunctions::rat(2, 4))` | `2` | constant 2 | answers | no verdict (unevaluated) |
| `RationalFunctions::abs` | `RationalFunctions::abs(RationalFunctions::rat(-1, 2))` | `1/2` | constant 0.5 | answers | no verdict (unevaluated) |
| `RationalFunctions::'+'` | `RationalFunctions::rat(1, 2) + RationalFunctions::rat(1, 2)` | `1` | constant 1.0 | answers | no verdict (unevaluated) |
| `RationalFunctions::'+'` | `RationalFunctions::'+'(RationalFunctions::rat(1, 3), RationalFunctions::rat(1, 3))` | `2/3` | constant 2/3 | answers | no verdict (unevaluated) |
| `RationalFunctions::'-'` | `RationalFunctions::rat(1, 2) - RationalFunctions::rat(1, 4)` | `1/4` | constant 0.25 | answers | no verdict (unevaluated) |
| `RationalFunctions::'*'` | `RationalFunctions::rat(1, 2) * RationalFunctions::rat(1, 2)` | `1/4` | constant 0.25 | answers | no verdict (unevaluated) |
| `RationalFunctions::'/'` | `RationalFunctions::rat(1, 2) / RationalFunctions::rat(1, 4)` | `2` | constant 2.0 | answers | no verdict (unevaluated) |
| `RationalFunctions::'**'` | `RationalFunctions::rat(1, 2) ** 2` | `1/4` | constant 0.25 | answers | no verdict (unevaluated) |
| `RationalFunctions::'^'` | `RationalFunctions::rat(1, 2) ^ 2` | `1/4` | constant 0.25 | answers | no verdict (unevaluated) |
| `RationalFunctions::'<'` | `RationalFunctions::rat(1, 2) < RationalFunctions::rat(3, 4)` | `true` | constant true | answers | no verdict (unevaluated) |
| `RationalFunctions::'>'` | `RationalFunctions::rat(1, 2) > RationalFunctions::rat(3, 4)` | `false` | constant false | answers | no verdict (unevaluated) |
| `RationalFunctions::'<='` | `RationalFunctions::rat(1, 2) <= RationalFunctions::rat(1, 2)` | `true` | constant true | answers | no verdict (unevaluated) |
| `RationalFunctions::'>='` | `RationalFunctions::rat(1, 2) >= RationalFunctions::rat(3, 4)` | `false` | constant false | answers | no verdict (unevaluated) |
| `RationalFunctions::max` | `RationalFunctions::max(RationalFunctions::rat(1, 2), RationalFunctions::rat(3, 4))` | `3/4` | constant 0.75 | answers | no verdict (unevaluated) |
| `RationalFunctions::min` | `RationalFunctions::min(RationalFunctions::rat(1, 2), RationalFunctions::rat(3, 4))` | `1/2` | constant 0.5 | answers | no verdict (unevaluated) |
| `RationalFunctions::'=='` | `RationalFunctions::rat(1, 2) == RationalFunctions::rat(2, 4)` | `true` | constant true | answers | LiteralBoolean false |
| `RationalFunctions::gcd` | `RationalFunctions::gcd(12, 18)` | `6` | constant 6 | answers | no verdict (unevaluated) |
| `RationalFunctions::floor` | `RationalFunctions::floor(RationalFunctions::rat(7, 2))` | `3` | constant 3 | answers | no verdict (unevaluated) |
| `RationalFunctions::round` | `RationalFunctions::round(RationalFunctions::rat(7, 2))` | `4` | constant 4 | answers | no verdict (unevaluated) |
| `RationalFunctions::ToString` | `RationalFunctions::ToString(RationalFunctions::rat(1, 2))` | `string` | string "0.5" | answers | no verdict (unevaluated) |
| `RationalFunctions::ToInteger` | `RationalFunctions::ToInteger(RationalFunctions::rat(4, 2))` | `2` | constant 2 | answers | no verdict (unevaluated) |
| `RationalFunctions::ToRational` | `RationalFunctions::ToRational("1/2")` | `1/2` | constant 0.5 | answers | no verdict (unevaluated) |
| `RationalFunctions::sum` | `RationalFunctions::sum((RationalFunctions::rat(1, 2), RationalFunctions::rat(1, 2)))` | `1` | constant 1.0 | answers | no verdict (unevaluated) |
| `RationalFunctions::product` | `RationalFunctions::product((RationalFunctions::rat(1, 2), RationalFunctions::rat(1, 2)))` | `1/4` | constant 0.25 | answers | no verdict (unevaluated) |
| `RealFunctions::re` | `RealFunctions::re(2.5)` | `2.5` | constant 2.5 | answers | LiteralRational 2.5 |
| `RealFunctions::im` | `RealFunctions::im(2.5)` | `0.0` | constant 0.0 | answers | LiteralRational 0.0 |
| `RealFunctions::abs` | `RealFunctions::abs(-2.5)` | `2.5` | constant 2.5 | answers | no verdict (unevaluated) |
| `RealFunctions::arg` | `RealFunctions::arg(-1.0)` | `pi` | constant 0.0 | answers | LiteralRational 0.0 |
| `RealFunctions::arg` | `RealFunctions::arg(1.0)` | `0.0` | constant 0.0 | answers | LiteralRational 0.0 |
| `RealFunctions::'+'` | `1.5 + 1.0` | `2.5` | constant 2.5 | answers | LiteralRational 2.5 |
| `RealFunctions::'-'` | `1.5 - 1.0` | `0.5` | constant 0.5 | answers | LiteralRational 0.5 |
| `RealFunctions::'*'` | `1.5 * 2.0` | `3.0` | constant 3.0 | answers | LiteralRational 3.0 |
| `RealFunctions::'/'` | `3.0 / 2.0` | `1.5` | constant 1.5 | answers | LiteralRational 1.5 |
| `RealFunctions::'**'` | `2.0 ** 3` | `8.0` | constant 8.0 | answers | LiteralRational 8.0 |
| `RealFunctions::'^'` | `2.0 ^ 3` | `8.0` | constant 8.0 | answers | LiteralRational 8.0 |
| `RealFunctions::'**'` | `RealFunctions::'**'(2.0, 0.5)` | `1.414` | constant 1.4142135623730951 | answers | no verdict (unevaluated) |
| `RealFunctions::'<'` | `1.0 < 2.0` | `true` | constant true | answers | LiteralBoolean true |
| `RealFunctions::'>'` | `1.0 > 2.0` | `false` | constant false | answers | LiteralBoolean false |
| `RealFunctions::'<='` | `2.0 <= 2.0` | `true` | constant true | answers | LiteralBoolean true |
| `RealFunctions::'>='` | `1.0 >= 2.0` | `false` | constant false | answers | LiteralBoolean false |
| `RealFunctions::max` | `RealFunctions::max(1.0, 2.0)` | `2.0` | constant 2.0 | answers | no verdict (unevaluated) |
| `RealFunctions::min` | `RealFunctions::min(1.0, 2.0)` | `1.0` | constant 1.0 | answers | no verdict (unevaluated) |
| `RealFunctions::'=='` | `1.0 == 1.0` | `true` | constant true | answers | LiteralBoolean true |
| `RealFunctions::sqrt` | `RealFunctions::sqrt(4.0)` | `2.0` | constant 2.0 | answers | no verdict (unevaluated) |
| `RealFunctions::sqrt` | `sqrt(4.0)` | `2.0` | error unresolved reference: sqrt\|\| WITH IMPORTS: constant 2.0 | answers with imports | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'sqrt'. (480.sys…) |
| `RealFunctions::floor` | `RealFunctions::floor(2.7)` | `2` | constant 2 | answers | no verdict (unevaluated) |
| `RealFunctions::round` | `RealFunctions::round(2.5)` | `3` | constant 3 | answers | no verdict (unevaluated) |
| `RealFunctions::round` | `RealFunctions::round(-2.5)` | `-3` | constant -3 | answers | no verdict (unevaluated) |
| `RealFunctions::ToString` | `RealFunctions::ToString(2.5)` | `"2.5"` | string "2.5" | answers | no verdict (unevaluated) |
| `RealFunctions::ToInteger` | `RealFunctions::ToInteger(2.7)` | `2` | constant 2 | answers | no verdict (unevaluated) |
| `RealFunctions::ToRational` | `RealFunctions::ToRational(0.5)` | `1/2` | constant 0.5 | answers | no verdict (unevaluated) |
| `RealFunctions::ToReal` | `RealFunctions::ToReal("2.5")` | `2.5` | constant 2.5 | answers | no verdict (unevaluated) |
| `RealFunctions::sum` | `RealFunctions::sum((1.0, 2.5))` | `3.5` | constant 3.5 | answers | no verdict (unevaluated) |
| `RealFunctions::sum` | `RealFunctions::sum(())` | `0.0` | constant 0.0 | answers | no verdict (unevaluated) |
| `RealFunctions::product` | `RealFunctions::product((2.0, 2.5))` | `5.0` | constant 5.0 | answers | no verdict (unevaluated) |
| `RealFunctions::product` | `RealFunctions::product(())` | `1.0` | constant 1.0 | answers | no verdict (unevaluated) |
| `SequenceFunctions::'#'` | `SequenceFunctions::'#'((10, 20), 2)` | `20` | constant 20 | answers | no verdict (unevaluated) |
| `SequenceFunctions::equals` | `SequenceFunctions::equals((1, 2), (1, 2))` | `true` | constant true | answers | LiteralBoolean true |
| `SequenceFunctions::equals` | `(1, 2)->equals((2, 1))` | `false` | error unresolved reference: equals; \|\| WITH IMPORTS: constant false | answers with imports | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'equals'. (508.s…) |
| `SequenceFunctions::same` | `SequenceFunctions::same((Probe::p1, Probe::p2), (Probe::p1, Probe::p2))` | `true` | constant true | answers | LiteralBoolean true |
| `SequenceFunctions::same` | `SequenceFunctions::same((1, 2), (1, 2))` | `true` | constant true | answers | LiteralBoolean true |
| `SequenceFunctions::size` | `SequenceFunctions::size((1, 2, 3))` | `3` | constant 3 | answers | LiteralInteger 3 |
| `SequenceFunctions::size` | `size((1, 2, 3))` | `3` | error unresolved reference: size\|\| WITH IMPORTS: constant 3 | answers with imports | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'size'. (516.sys…) |
| `SequenceFunctions::size` | `(1, 2, 3)->size()` | `3` | error unresolved reference: size\|\| WITH IMPORTS: constant 3 | answers with imports | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'size'. (518.sys…) |
| `SequenceFunctions::size` | `size(())` | `0` | error unresolved reference: size\|\| WITH IMPORTS: constant 0 | answers with imports | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'size'. (520.sys…) |
| `SequenceFunctions::isEmpty` | `SequenceFunctions::isEmpty(())` | `true` | constant true | answers | LiteralBoolean true |
| `SequenceFunctions::isEmpty` | `isEmpty((1))` | `false` | error unresolved reference: isEmpty\|\| WITH IMPORTS: constant false | answers with imports | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'isEmpty'. (524.…) |
| `SequenceFunctions::isEmpty` | `()->isEmpty()` | `true` | error unresolved reference: isEmpty\|\| WITH IMPORTS: constant true | answers with imports | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'isEmpty'. (526.…) |
| `SequenceFunctions::notEmpty` | `SequenceFunctions::notEmpty((1))` | `true` | constant true | answers | LiteralBoolean true |
| `SequenceFunctions::notEmpty` | `notEmpty(())` | `false` | error unresolved reference: notEmpty\|\| WITH IMPORTS: constant false | answers with imports | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'notEmpty'. (530…) |
| `SequenceFunctions::includes` | `SequenceFunctions::includes((1, 2), 2)` | `true` | constant true | answers | LiteralBoolean true |
| `SequenceFunctions::includes` | `(1, 2)->includes(3)` | `false` | error unresolved reference: includes; \|\| WITH IMPORTS: constant false | answers with imports | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'includes'. (534…) |
| `SequenceFunctions::includes` | `includes((1, 2), (1, 2))` | `true` | error unresolved reference: includes; \|\| WITH IMPORTS: constant true | answers with imports | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'includes'. (536…) |
| `SequenceFunctions::includesOnly` | `SequenceFunctions::includesOnly((1, 1), 1)` | `true` | constant true | answers | LiteralBoolean true |
| `SequenceFunctions::includesOnly` | `SequenceFunctions::includesOnly((1, 2), 1)` | `false` | constant false | answers | LiteralBoolean false |
| `SequenceFunctions::excludes` | `SequenceFunctions::excludes((1, 2), 3)` | `true` | constant true | answers | LiteralBoolean true |
| `SequenceFunctions::excludes` | `(1, 2)->excludes(2)` | `false` | error unresolved reference: excludes; \|\| WITH IMPORTS: constant false | answers with imports | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'excludes'. (544…) |
| `SequenceFunctions::union` | `SequenceFunctions::union((1, 2), (2, 3))` | `(1, 2, 2, 3)` | collection(4) [1, 2, 2, 3] | answers | LiteralInteger 1 |
| `SequenceFunctions::union` | `(1, 2)->union((3))` | `(1, 2, 3)` | error unresolved reference: union; \|\| WITH IMPORTS: collection(3) [1, 2, 3] | answers with imports | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'union'. (548.sy…) |
| `SequenceFunctions::intersection` | `SequenceFunctions::intersection((1, 2), (2, 3))` | `2` | collection(1) [2] | answers | LiteralInteger 2 |
| `SequenceFunctions::intersection` | `(1, 2, 2)->intersection((2))` | `(2, 2)` | error unresolved reference: intersection; \|\| WITH IMPORTS: collection(2) [2, 2] | answers with imports | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'intersection'. …) |
| `SequenceFunctions::including` | `SequenceFunctions::including((1, 2), 3)` | `(1, 2, 3)` | collection(3) [1, 2, 3] | answers | LiteralInteger 1 |
| `SequenceFunctions::including` | `(1, 2)->including(3)` | `(1, 2, 3)` | error unresolved reference: including; \|\| WITH IMPORTS: collection(3) [1, 2, 3] | answers with imports | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'including'. (55…) |
| `SequenceFunctions::includingAt` | `SequenceFunctions::includingAt((1, 3), 2, 2)` | `(1, 2, 3)` | collection(3) [1, 2, 3] | answers | LiteralInteger 1 |
| `SequenceFunctions::includingAt` | `(1, 3)->includingAt(2, 2)` | `(1, 2, 3)` | error unresolved reference: includingAt; \|\| WITH IMPORTS: collection(3) [1, 2, 3] | answers with imports | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'includingAt'. (…) |
| `SequenceFunctions::excluding` | `SequenceFunctions::excluding((1, 2, 3), 2)` | `(1, 3)` | collection(2) [1, 3] | answers | LiteralInteger 1 |
| `SequenceFunctions::excluding` | `(1, 2, 2, 3)->excluding(2)` | `(1, 3)` | error unresolved reference: excluding; \|\| WITH IMPORTS: collection(2) [1, 3] | answers with imports | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'excluding'. (56…) |
| `SequenceFunctions::excludingAt` | `SequenceFunctions::excludingAt((1, 2, 3), 2)` | `(1, 3)` | collection(2) [1, 3] | answers | LiteralInteger 1 |
| `SequenceFunctions::excludingAt` | `(1, 2, 3)->excludingAt(2)` | `(1, 3)` | error unresolved reference: excludingAt; \|\| WITH IMPORTS: collection(2) [1, 3] | answers with imports | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'excludingAt'. (…) |
| `SequenceFunctions::subsequence` | `SequenceFunctions::subsequence((1, 2, 3, 4), 2, 3)` | `(2, 3)` | collection(2) [2, 3] | answers | LiteralInteger 2 |
| `SequenceFunctions::subsequence` | `(1, 2, 3, 4)->subsequence(2, 3)` | `(2, 3)` | error unresolved reference: subsequence; \|\| WITH IMPORTS: collection(2) [2, 3] | answers with imports | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'subsequence'. (…) |
| `SequenceFunctions::head` | `SequenceFunctions::head((1, 2, 3))` | `1` | constant 1 | answers | LiteralInteger 1 |
| `SequenceFunctions::head` | `(1, 2, 3)->head()` | `1` | error unresolved reference: head\|\| WITH IMPORTS: constant 1 | answers with imports | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'head'. (576.sys…) |
| `SequenceFunctions::head` | `head(())` | `()` | error unresolved reference: head\|\| WITH IMPORTS: null | answers with imports | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'head'. (578.sys…) |
| `SequenceFunctions::tail` | `SequenceFunctions::tail((1, 2, 3))` | `(2, 3)` | collection(2) [2, 3] | answers | LiteralInteger 2 |
| `SequenceFunctions::tail` | `(1, 2, 3)->tail()` | `(2, 3)` | error unresolved reference: tail\|\| WITH IMPORTS: collection(2) [2, 3] | answers with imports | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'tail'. (582.sys…) |
| `SequenceFunctions::tail` | `tail((1))` | `()` | error unresolved reference: tail\|\| WITH IMPORTS: empty | answers with imports | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'tail'. (584.sys…) |
| `SequenceFunctions::last` | `SequenceFunctions::last((1, 2, 3))` | `3` | constant 3 | answers | LiteralInteger 3 |
| `SequenceFunctions::last` | `(1, 2, 3)->last()` | `3` | error unresolved reference: last\|\| WITH IMPORTS: constant 3 | answers with imports | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'last'. (588.sys…) |
| `SequenceFunctions::last` | `last(())` | `()` | error unresolved reference: last\|\| WITH IMPORTS: null | answers with imports | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'last'. (590.sys…) |
| `StringFunctions::'+'` | `"a" + "b"` | `"ab"` | string "ab" | answers | LiteralString ab |
| `StringFunctions::'+'` | `StringFunctions::'+'("a", "b")` | `"ab"` | string "ab" | answers | no verdict (unevaluated) |
| `StringFunctions::Length` | `StringFunctions::Length("abc")` | `3` | constant 3 | answers | LiteralInteger 3 |
| `StringFunctions::Length` | `Length("")` | `0` | error unresolved reference: Length; \|\| WITH IMPORTS: constant 0 | answers with imports | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'Length'. (598.s…) |
| `StringFunctions::Substring` | `StringFunctions::Substring("abcd", 2, 3)` | `"bc"` | string "bc" | answers | LiteralString bc |
| `StringFunctions::Substring` | `Substring("abcd", 1, 1)` | `"a"` | error unresolved reference: Substring; \|\| WITH IMPORTS: string "a" | answers with imports | no verdict (pilot: ERROR:Couldn't resolve reference to Element 'Substring'. (60…) |
| `StringFunctions::'<'` | `"a" < "b"` | `true` | constant true | answers | LiteralBoolean true |
| `StringFunctions::'>'` | `"a" > "b"` | `false` | constant false | answers | LiteralBoolean false |
| `StringFunctions::'<='` | `"a" <= "a"` | `true` | constant true | answers | LiteralBoolean true |
| `StringFunctions::'>='` | `"a" >= "b"` | `false` | constant false | answers | LiteralBoolean false |
| `StringFunctions::'=='` | `"a" == "a"` | `true` | constant true | answers | LiteralBoolean true |
| `StringFunctions::ToString` | `StringFunctions::ToString("a")` | `"a"` | string "a" | answers | LiteralString a |
| `TrigFunctions::deg` | `TrigFunctions::deg(TrigFunctions::pi)` | `180.0` | constant 180.0 | answers | LiteralRational 180.0 |
| `TrigFunctions::rad` | `TrigFunctions::rad(180.0)` | `pi` | constant 3.141592653589793 | answers | LiteralRational 3.141592653589793 |
| `TrigFunctions::sin` | `TrigFunctions::sin(0.0)` | `0.0` | constant 0.0 | answers | LiteralRational 0.0 |
| `TrigFunctions::sin` | `TrigFunctions::sin(90.0)` | `1.0` | constant 0.893996663600558 | answers | LiteralRational 0.8939966636005579 |
| `TrigFunctions::cos` | `TrigFunctions::cos(0.0)` | `1.0` | constant 1.0 | answers | LiteralRational 1.0 |
| `TrigFunctions::cos` | `TrigFunctions::cos(180.0)` | `-1.0` | constant -0.5984600690578581 | answers | LiteralRational -0.5984600690578581 |
| `TrigFunctions::tan` | `TrigFunctions::tan(0.0)` | `0.0` | constant 0.0 | answers | LiteralRational 0.0 |
| `TrigFunctions::tan` | `TrigFunctions::tan(45.0)` | `1.0` | constant 1.6197751905438613 | answers | LiteralRational 1.6197751905438615 |
| `TrigFunctions::cot` | `TrigFunctions::cot(45.0)` | `1.0` | constant 0.6173696237835552 | answers | LiteralRational 0.6173696237835551 |
| `TrigFunctions::arcsin` | `TrigFunctions::arcsin(1.0)` | `90.0` | constant 1.5707963267948966 | answers | LiteralRational 1.5707963267948966 |
| `TrigFunctions::arccos` | `TrigFunctions::arccos(1.0)` | `0.0` | constant 0.0 | answers | LiteralRational 0.0 |
| `TrigFunctions::arctan` | `TrigFunctions::arctan(1.0)` | `45.0` | constant 0.7853981633974483 | answers | LiteralRational 0.7853981633974483 |
| `VectorFunctions::VectorOf` | `VectorFunctions::VectorOf((1, 2, 3)).elements` | `(1, 2, 3)` | collection(3) [1, 2, 3] | answers | LiteralInteger 1 |
| `VectorFunctions::CartesianVectorOf` | `VectorFunctions::CartesianVectorOf((1.0, 2.0)).elements` | `(1.0, 2.0)` | collection(2) [1.0, 2.0] | answers | LiteralRational 1.0 |
| `VectorFunctions::CartesianThreeVectorOf` | `VectorFunctions::CartesianThreeVectorOf((1.0, 2.0, 3.0)).elements` | `(1.0, 2.0, 3.0)` | collection(3) [1.0, 2.0, 3.0] | answers | LiteralRational 1.0 |
| `VectorFunctions::isCartesianZeroVector` | `VectorFunctions::isCartesianZeroVector(VectorFunctions::CartesianVectorOf((0.0, 0.0)))` | `true` | constant true | answers | LiteralBoolean true |
| `VectorFunctions::'cartesian+'` | `VectorFunctions::'cartesian+'(VectorFunctions::CartesianVectorOf((1.0, 2.0)), VectorFunctions::CartesianVectorOf((1.0, 1.0))).elements` | `(2.0, 3.0)` | collection(2) [2.0, 3.0] | answers | — |
| `VectorFunctions::'cartesian-'` | `VectorFunctions::'cartesian-'(VectorFunctions::CartesianVectorOf((1.0, 2.0)), VectorFunctions::CartesianVectorOf((1.0, 1.0))).elements` | `(0.0, 1.0)` | collection(2) [0.0, 1.0] | answers | Feature elements |
| `VectorFunctions::cartesianScalarVectorMult` | `VectorFunctions::cartesianScalarVectorMult(2.0, VectorFunctions::CartesianVectorOf((1.0, 2.0))).elements` | `(2.0, 4.0)` | collection(2) [2.0, 4.0] | answers | Feature elements |
| `VectorFunctions::cartesianVectorScalarMult` | `VectorFunctions::cartesianVectorScalarMult(VectorFunctions::CartesianVectorOf((1.0, 2.0)), 2.0).elements` | `(2.0, 4.0)` | collection(2) [2.0, 4.0] | answers | Feature elements |
| `VectorFunctions::cartesianInner` | `VectorFunctions::cartesianInner(VectorFunctions::CartesianVectorOf((1.0, 2.0)), VectorFunctions::CartesianVectorOf((3.0, 4.0)))` | `11.0` | constant 11.0 | answers | no verdict (unevaluated) |
| `VectorFunctions::cartesianNorm` | `VectorFunctions::cartesianNorm(VectorFunctions::CartesianVectorOf((3.0, 4.0)))` | `5.0` | constant 5.0 | answers | no verdict (unevaluated) |
| `VectorFunctions::cartesianAngle` | `VectorFunctions::cartesianAngle(VectorFunctions::CartesianVectorOf((1.0, 0.0)), VectorFunctions::CartesianVectorOf((0.0, 1.0)))` | `90.0` | constant 1.5707963267948966 | answers | no verdict (unevaluated) |
| `VectorFunctions::sum` | `VectorFunctions::sum((VectorFunctions::CartesianThreeVectorOf((1.0, 0.0, 0.0)), VectorFunctions::CartesianThreeVectorOf((0.0, 1.0, 0.0)))).elements` | `(1.0, 1.0, 0.0)` | collection(3) [1.0, 1.0, 0.0] | answers | — |
| `VectorFunctions::'+'` | `VectorFunctions::CartesianVectorOf((1.0, 2.0)) + VectorFunctions::CartesianVectorOf((1.0, 1.0))` | `vector(2.0, 3.0)` | vector ⟨2.0, 3.0⟩ | answers | no verdict (unevaluated) |
| `VectorFunctions::'*'` | `VectorFunctions::CartesianVectorOf((1.0, 2.0)) * 2.0` | `vector(2.0, 4.0)` | vector ⟨2.0, 4.0⟩ | answers | no verdict (unevaluated) |

333 probes; wrong values: `Probe::Tagged @@ Probe::M` (false, pilot true), `Probe::os#(1)` (the object, pilot 3); typed refusals that validate clean: `->reduce` with a function reference.

## Sweep 3 — grammar prefixes × usage kinds at runtime

Each cell is one model: `part def D { <siblings> <prefix> <kind> x : T …; } part d : D;` read as `Probe::d.x` (and `d.y`, `d2.x` where the prefix needs a second usage), with `bin/sysml -validate` on the same file. A cell the validator rejects is an invalid probe and carries no runtime verdict. Usage kinds whose only runtime meaning is their owner's (`perform action`, `exhibit state`, `satisfy requirement`, `assert constraint`, `include use case`, `snapshot`, `timeslice`, `event occurrence`) are probed as nested usages in sweep 1.

| Prefix | Kind | Declaration | Reads | Bucket | Validate |
|---|---|---|---|---|---|
| (none) | `attribute Integer` | `attribute x : Integer;` | Probe::d.x => undetermined <undetermined> | `<undetermined>` | clean |
| (none) | `attribute At1` | `attribute x : At1;` | Probe::d.x => undetermined <undetermined> | `<undetermined>` | clean |
| (none) | `enum E1` | `enum x : E1;` | Probe::d.x => ERR_NOSUCHFEATURE object has no such feature: member x not found in instance | error (`ErrNoSuchFeature`) | clean |
| (none) | `calc K1` | `calc x : K1;` | Probe::d.x => constant 4 | answers | clean |
| (none) | `part P2` | `part x : P2;` | Probe::d.x => object:Probe::P2 | answers | clean |
| (none) | `item I1` | `item x : I1;` | Probe::d.x => object:Probe::I1 | answers | clean |
| (none) | `port Pt` | `port x : Pt;` | Probe::d.x => object:Probe::Pt | answers | clean |
| (none) | `action A1` | `action x : A1;` | Probe::d.x => object:Probe::A1 | answers | clean |
| (none) | `state S1` | `state x : S1;` | Probe::d.x => object:Probe::S1 | answers | clean |
| (none) | `requirement R1` | `requirement x : R1;` | Probe::d.x => object:Probe::R1 | answers | clean |
| (none) | `constraint C1` | `constraint x : C1;` | Probe::d.x => object:Probe::C1 | answers | clean |
| (none) | `occurrence O1` | `occurrence x : O1;` | Probe::d.x => object:Probe::O1 | answers | clean |
| (none) | `connection Cn1` | `connection x : Cn1 connect sub1 to sub2;` | Probe::d.x => object:Probe::Cn1 | answers | clean |
| in | `attribute Integer` | `in attribute x : Integer;` | Probe::d.x => undetermined <undetermined> | `<undetermined>` | clean |
| in | `attribute At1` | `in attribute x : At1;` | Probe::d.x => undetermined <undetermined> | `<undetermined>` | clean |
| in | `enum E1` | `in enum x : E1;` | Probe::d.x => ERR_NOSUCHFEATURE object has no such feature: member x not found in instance | error (`ErrNoSuchFeature`) | clean |
| in | `calc K1` | `in calc x : K1;` | Probe::d.x => constant 4 | answers | clean |
| in | `part P2` | `in part x : P2;` | Probe::d.x => object:Probe::P2 | answers | clean |
| in | `item I1` | `in item x : I1;` | Probe::d.x => object:Probe::I1 | answers | clean |
| in | `port Pt` | `in port x : Pt;` | Probe::d.x => object:Probe::Pt | answers | clean |
| in | `action A1` | `in action x : A1;` | Probe::d.x => undetermined <undetermined> | `<undetermined>` | clean |
| in | `state S1` | `in state x : S1;` | Probe::d.x => undetermined <undetermined> | `<undetermined>` | clean |
| in | `requirement R1` | `in requirement x : R1;` | Probe::d.x => undetermined <undetermined> | `<undetermined>` | clean |
| in | `constraint C1` | `in constraint x : C1;` | Probe::d.x => undetermined <undetermined> | `<undetermined>` | clean |
| in | `occurrence O1` | `in occurrence x : O1;` | Probe::d.x => undetermined <undetermined> | `<undetermined>` | clean |
| in | `connection Cn1` | `in connection x : Cn1 connect sub1 to sub2;` | Probe::d.x => error connector end cannot be attached: d.x end "x": a connector of more than one object has no set of ends to attach | typed error | clean |
| out | `attribute Integer` | `out attribute x : Integer;` | Probe::d.x => undetermined <undetermined> | `<undetermined>` | clean |
| out | `attribute At1` | `out attribute x : At1;` | Probe::d.x => undetermined <undetermined> | `<undetermined>` | clean |
| out | `enum E1` | `out enum x : E1;` | Probe::d.x => ERR_NOSUCHFEATURE object has no such feature: member x not found in instance | error (`ErrNoSuchFeature`) | clean |
| out | `calc K1` | `out calc x : K1;` | Probe::d.x => constant 4 | answers | clean |
| out | `part P2` | `out part x : P2;` | Probe::d.x => object:Probe::P2 | answers | clean |
| out | `item I1` | `out item x : I1;` | Probe::d.x => object:Probe::I1 | answers | clean |
| out | `port Pt` | `out port x : Pt;` | Probe::d.x => object:Probe::Pt | answers | clean |
| out | `action A1` | `out action x : A1;` | Probe::d.x => undetermined <undetermined> | `<undetermined>` | clean |
| out | `state S1` | `out state x : S1;` | Probe::d.x => undetermined <undetermined> | `<undetermined>` | clean |
| out | `requirement R1` | `out requirement x : R1;` | Probe::d.x => undetermined <undetermined> | `<undetermined>` | clean |
| out | `constraint C1` | `out constraint x : C1;` | Probe::d.x => undetermined <undetermined> | `<undetermined>` | clean |
| out | `occurrence O1` | `out occurrence x : O1;` | Probe::d.x => undetermined <undetermined> | `<undetermined>` | clean |
| out | `connection Cn1` | `out connection x : Cn1 connect sub1 to sub2;` | Probe::d.x => error connector end cannot be attached: d.x end "x": a connector of more than one object has no set of ends to attach | typed error | clean |
| inout | `attribute Integer` | `inout attribute x : Integer;` | Probe::d.x => undetermined <undetermined> | `<undetermined>` | clean |
| inout | `attribute At1` | `inout attribute x : At1;` | Probe::d.x => undetermined <undetermined> | `<undetermined>` | clean |
| inout | `enum E1` | `inout enum x : E1;` | Probe::d.x => ERR_NOSUCHFEATURE object has no such feature: member x not found in instance | error (`ErrNoSuchFeature`) | clean |
| inout | `calc K1` | `inout calc x : K1;` | Probe::d.x => constant 4 | answers | clean |
| inout | `part P2` | `inout part x : P2;` | Probe::d.x => object:Probe::P2 | answers | clean |
| inout | `item I1` | `inout item x : I1;` | Probe::d.x => object:Probe::I1 | answers | clean |
| inout | `port Pt` | `inout port x : Pt;` | Probe::d.x => object:Probe::Pt | answers | clean |
| inout | `action A1` | `inout action x : A1;` | Probe::d.x => undetermined <undetermined> | `<undetermined>` | clean |
| inout | `state S1` | `inout state x : S1;` | Probe::d.x => undetermined <undetermined> | `<undetermined>` | clean |
| inout | `requirement R1` | `inout requirement x : R1;` | Probe::d.x => undetermined <undetermined> | `<undetermined>` | clean |
| inout | `constraint C1` | `inout constraint x : C1;` | Probe::d.x => undetermined <undetermined> | `<undetermined>` | clean |
| inout | `occurrence O1` | `inout occurrence x : O1;` | Probe::d.x => undetermined <undetermined> | `<undetermined>` | clean |
| inout | `connection Cn1` | `inout connection x : Cn1 connect sub1 to sub2;` | Probe::d.x => error connector end cannot be attached: d.x end "x": a connector of more than one object has no set of ends to attach | typed error | clean |
| ref (unbound) | `attribute Integer` | `ref attribute x : Integer;` | Probe::d.x => error uninitialized feature value: x | typed error (`errUninitializedFeatureValue`) | clean |
| ref (unbound) | `attribute At1` | `ref attribute x : At1;` | Probe::d.x => error uninitialized feature value: x | typed error (`errUninitializedFeatureValue`) | clean |
| ref (unbound) | `enum E1` | `ref enum x : E1;` | Probe::d.x => ERR_NOSUCHFEATURE object has no such feature: member x not found in instance | error (`ErrNoSuchFeature`) | clean |
| ref (unbound) | `calc K1` | `ref calc x : K1;` | Probe::d.x => constant 4 | answers | clean |
| ref (unbound) | `part P2` | `ref part x : P2;` | Probe::d.x => error uninitialized feature value: x | typed error (`errUninitializedFeatureValue`) | clean |
| ref (unbound) | `item I1` | `ref item x : I1;` | Probe::d.x => error uninitialized feature value: x | typed error (`errUninitializedFeatureValue`) | clean |
| ref (unbound) | `port Pt` | `ref port x : Pt;` | Probe::d.x => error uninitialized feature value: x | typed error (`errUninitializedFeatureValue`) | clean |
| ref (unbound) | `action A1` | `ref action x : A1;` | Probe::d.x => error uninitialized feature value: x | typed error (`errUninitializedFeatureValue`) | clean |
| ref (unbound) | `state S1` | `ref state x : S1;` | Probe::d.x => error uninitialized feature value: x | typed error (`errUninitializedFeatureValue`) | clean |
| ref (unbound) | `requirement R1` | `ref requirement x : R1;` | Probe::d.x => error uninitialized feature value: x | typed error (`errUninitializedFeatureValue`) | clean |
| ref (unbound) | `constraint C1` | `ref constraint x : C1;` | Probe::d.x => error uninitialized feature value: x | typed error (`errUninitializedFeatureValue`) | clean |
| ref (unbound) | `occurrence O1` | `ref occurrence x : O1;` | Probe::d.x => error uninitialized feature value: x | typed error (`errUninitializedFeatureValue`) | clean |
| ref (unbound) | `connection Cn1` | `ref connection x : Cn1 connect sub1 to sub2;` | Probe::d.x => object:Probe::D::x | answers | clean |
| ref (bound =) | `attribute Integer` | `ref attribute x : Integer = 1;` | Probe::d.x => constant 1 ;; Probe::d.x == Probe::d.sub1 => constant false | answers | clean |
| ref (bound =) | `enum E1` | `ref enum x : E1 = E1::a;` | Probe::d.x => ERR_NOSUCHFEATURE object has no such feature: member x not found in instance ;; Probe::d.x == Probe::d.sub1 => ERR_NOSUCHFEATURE object has no suc… | error (`ErrNoSuchFeature`) | clean |
| ref (bound =) | `part P2` | `ref part x : P2 = sub1;` | Probe::d.x => object:Probe::P2 ;; Probe::d.x == Probe::d.sub1 => constant true | answers | clean |
| ref (bound =) | `item I1` | `ref item x : I1 = it1;` | Probe::d.x => object:Probe::I1 ;; Probe::d.x == Probe::d.sub1 => constant false | answers | clean |
| ref (bound =) | `port Pt` | `ref port x : Pt = pt1;` | Probe::d.x => object:Probe::Pt ;; Probe::d.x == Probe::d.sub1 => constant false | answers | clean |
| ref (bound =) | `action A1` | `ref action x : A1 = act1;` | Probe::d.x => object:Probe::A1 ;; Probe::d.x == Probe::d.sub1 => constant false | answers | clean |
| ref (bound =) | `state S1` | `ref state x : S1 = st1;` | Probe::d.x => object:Probe::S1 ;; Probe::d.x == Probe::d.sub1 => constant false | answers | clean |
| ref (bound =) | `requirement R1` | `ref requirement x : R1 = rq1;` | Probe::d.x => object:Probe::R1 ;; Probe::d.x == Probe::d.sub1 => constant false | answers | clean |
| ref (bound =) | `constraint C1` | `ref constraint x : C1 = cq1;` | Probe::d.x => object:Probe::C1 ;; Probe::d.x == Probe::d.sub1 => constant false | answers | clean |
| ref (bound =) | `occurrence O1` | `ref occurrence x : O1 = oc1;` | Probe::d.x => object:Probe::O1 ;; Probe::d.x == Probe::d.sub1 => constant false | answers | clean |
| abstract [0..*] | `attribute Integer` | `abstract attribute x : Integer[0..*];` | Probe::d.x => empty | answers | clean |
| abstract [0..*] | `attribute At1` | `abstract attribute x : At1[0..*];` | Probe::d.x => empty | answers | clean |
| abstract [0..*] | `enum E1` | `abstract enum x : E1[0..*];` | Probe::d.x => ERR_NOSUCHFEATURE object has no such feature: member x not found in instance | error (`ErrNoSuchFeature`) | clean |
| abstract [0..*] | `calc K1` | `abstract calc x : K1[0..*];` | Probe::d.x => constant 4 | answers | clean |
| abstract [0..*] | `part P2` | `abstract part x : P2[0..*];` | Probe::d.x => empty | answers | clean |
| abstract [0..*] | `item I1` | `abstract item x : I1[0..*];` | Probe::d.x => empty | answers | clean |
| abstract [0..*] | `port Pt` | `abstract port x : Pt[0..*];` | Probe::d.x => empty | answers | clean |
| abstract [0..*] | `action A1` | `abstract action x : A1[0..*];` | Probe::d.x => empty | answers | clean |
| abstract [0..*] | `state S1` | `abstract state x : S1[0..*];` | Probe::d.x => empty | answers | clean |
| abstract [0..*] | `requirement R1` | `abstract requirement x : R1[0..*];` | Probe::d.x => empty | answers | clean |
| abstract [0..*] | `constraint C1` | `abstract constraint x : C1[0..*];` | Probe::d.x => empty | answers | clean |
| abstract [0..*] | `occurrence O1` | `abstract occurrence x : O1[0..*];` | Probe::d.x => empty | answers | clean |
| abstract [0..*] | `connection Cn1` | `abstract connection x : Cn1[0..*] connect sub1 to sub2;` | Probe::d.x => empty | answers | clean |
| abstract [0..*] + subsetter | `attribute Integer` | `abstract attribute x : Integer[0..*]; attribute y : Integer :> x;` | Probe::d.x => undetermined <undetermined> ;; Probe::d.y => undetermined <undetermined> | `<undetermined>` | clean |
| abstract [0..*] + subsetter | `attribute At1` | `abstract attribute x : At1[0..*]; attribute y : At1 :> x;` | Probe::d.x => undetermined <undetermined> ;; Probe::d.y => undetermined <undetermined> | `<undetermined>` | clean |
| abstract [0..*] + subsetter | `enum E1` | `abstract enum x : E1[0..*]; enum y : E1 :> x;` | Probe::d.x => ERR_NOSUCHFEATURE object has no such feature: member x not found in instance ;; Probe::d.y => ERR_NOSUCHFEATURE object has no such feature: member… | error (`ErrNoSuchFeature`) | clean |
| abstract [0..*] + subsetter | `calc K1` | `abstract calc x : K1[0..*]; calc y : K1 :> x;` | Probe::d.x => constant 4 ;; Probe::d.y => constant 4 | answers | clean |
| abstract [0..*] + subsetter | `part P2` | `abstract part x : P2[0..*]; part y : P2 :> x;` | Probe::d.x => collection(1) [instance(2)] ;; Probe::d.y => object:Probe::P2 | answers | clean |
| abstract [0..*] + subsetter | `item I1` | `abstract item x : I1[0..*]; item y : I1 :> x;` | Probe::d.x => collection(1) [instance(2)] ;; Probe::d.y => object:Probe::I1 | answers | clean |
| abstract [0..*] + subsetter | `port Pt` | `abstract port x : Pt[0..*]; port y : Pt :> x;` | Probe::d.x => collection(1) [instance(2)] ;; Probe::d.y => object:Probe::Pt | answers | clean |
| abstract [0..*] + subsetter | `action A1` | `abstract action x : A1[0..*]; action y : A1 :> x;` | Probe::d.x => collection(1) [instance(2)] ;; Probe::d.y => object:Probe::A1 | answers | clean |
| abstract [0..*] + subsetter | `state S1` | `abstract state x : S1[0..*]; state y : S1 :> x;` | Probe::d.x => collection(1) [instance(2)] ;; Probe::d.y => object:Probe::S1 | answers | clean |
| abstract [0..*] + subsetter | `requirement R1` | `abstract requirement x : R1[0..*]; requirement y : R1 :> x;` | Probe::d.x => collection(1) [instance(2)] ;; Probe::d.y => object:Probe::R1 | answers | clean |
| abstract [0..*] + subsetter | `constraint C1` | `abstract constraint x : C1[0..*]; constraint y : C1 :> x;` | Probe::d.x => collection(1) [instance(2)] ;; Probe::d.y => object:Probe::C1 | answers | clean |
| abstract [0..*] + subsetter | `occurrence O1` | `abstract occurrence x : O1[0..*]; occurrence y : O1 :> x;` | Probe::d.x => collection(1) [instance(2)] ;; Probe::d.y => object:Probe::O1 | answers | clean |
| abstract [0..*] + subsetter | `connection Cn1` | `abstract connection x : Cn1[0..*] connect sub1 to sub2; connection y : Cn1 connect sub1 to sub2 :> x;` | Probe::d.x => empty ;; Probe::d.y => object:Probe::Cn1 | validate rejects | error[syntax]: expected '{' or ';' after declaration; |
| variation | `attribute Integer` | `variation attribute x : Integer { variant attribute va : Integer; variant attribute vb : Integer; }` | Probe::d.x => error variation has no variant selected: d.x ;; Probe::d.x.va => variant va | typed error | clean |
| variation | `attribute At1` | `variation attribute x : At1 { variant attribute va : At1; variant attribute vb : At1; }` | Probe::d.x => error variation has no variant selected: d.x ;; Probe::d.x.va => variant va | typed error | clean |
| variation | `enum E1` | `variation enum x : E1 { variant enum va : E1; variant enum vb : E1; }` | Probe::d.x => ERR_NOSUCHFEATURE object has no such feature: member x not found in instance ;; Probe::d.x.va => ERR_NOSUCHFEATURE object has no such feature: mem… | validate rejects | error[variation-specialization]: variation `x` must not be typed by enumeration … |
| variation | `calc K1` | `variation calc x : K1 { variant calc va : K1; variant calc vb : K1; }` | Probe::d.x => constant 4 ;; Probe::d.x.va => error unknown output: calc Probe::D::x declares no output va (it declares: none) | answers | clean |
| variation | `part P2` | `variation part x : P2 { variant part va : P2; variant part vb : P2; }` | Probe::d.x => error variation has no variant selected: d.x ;; Probe::d.x.va => variant va | typed error | clean |
| variation | `item I1` | `variation item x : I1 { variant item va : I1; variant item vb : I1; }` | Probe::d.x => error variation has no variant selected: d.x ;; Probe::d.x.va => variant va | typed error | clean |
| variation | `port Pt` | `variation port x : Pt { variant port va : Pt; variant port vb : Pt; }` | Probe::d.x => error variation has no variant selected: d.x ;; Probe::d.x.va => variant va | typed error | clean |
| variation | `action A1` | `variation action x : A1 { variant action va : A1; variant action vb : A1; }` | Probe::d.x => error variation has no variant selected: d.x ;; Probe::d.x.va => variant va | typed error | clean |
| variation | `state S1` | `variation state x : S1 { variant state va : S1; variant state vb : S1; }` | Probe::d.x => error variation has no variant selected: d.x ;; Probe::d.x.va => variant va | typed error | clean |
| variation | `requirement R1` | `variation requirement x : R1 { variant requirement va : R1; variant requirement vb : R1; }` | Probe::d.x => error variation has no variant selected: d.x ;; Probe::d.x.va => variant va | typed error | clean |
| variation | `constraint C1` | `variation constraint x : C1 { variant constraint va : C1; variant constraint vb : C1; }` | Probe::d.x => error variation has no variant selected: d.x ;; Probe::d.x.va => variant va | typed error | clean |
| variation | `occurrence O1` | `variation occurrence x : O1 { variant occurrence va : O1; variant occurrence vb : O1; }` | Probe::d.x => error variation has no variant selected: d.x ;; Probe::d.x.va => variant va | typed error | clean |
| variation | `connection Cn1` | `variation connection x : Cn1 connect sub1 to sub2 { variant connection va : Cn1 connect sub1 to sub2; variant connection vb : Cn1 connect sub1 to sub2; }` | Probe::d.x => error variation has no variant selected: d.x ;; Probe::d.x.va => variant va | typed error | clean |
| constant | `attribute Integer` | `constant attribute x : Integer = 1;` | Probe::d.x => constant 1 | answers | clean |
| constant | `attribute At1` | `constant attribute x : At1;` | Probe::d.x => undetermined <undetermined> | `<undetermined>` | clean |
| constant | `enum E1` | `constant enum x : E1 = E1::a;` | Probe::d.x => ERR_NOSUCHFEATURE object has no such feature: member x not found in instance | error (`ErrNoSuchFeature`) | clean |
| constant | `calc K1` | `constant calc x : K1;` | Probe::d.x => constant 4 | validate rejects | error[constant-feature-not-variable]: Only a variable feature can be constant; |
| constant | `part P2` | `constant part x : P2 = sub1;` | Probe::d.x => object:Probe::P2 | answers | clean |
| constant | `item I1` | `constant item x : I1 = it1;` | Probe::d.x => object:Probe::I1 | answers | clean |
| constant | `port Pt` | `constant port x : Pt = pt1;` | Probe::d.x => object:Probe::Pt | answers | clean |
| constant | `action A1` | `constant action x : A1 = act1;` | Probe::d.x => object:Probe::A1 | validate rejects | error[constant-feature-not-variable]: Only a variable feature can be constant; |
| constant | `state S1` | `constant state x : S1 = st1;` | Probe::d.x => object:Probe::S1 | validate rejects | error[constant-feature-not-variable]: Only a variable feature can be constant; |
| constant | `requirement R1` | `constant requirement x : R1 = rq1;` | Probe::d.x => object:Probe::R1 | answers | clean |
| constant | `constraint C1` | `constant constraint x : C1 = cq1;` | Probe::d.x => object:Probe::C1 | answers | clean |
| constant | `occurrence O1` | `constant occurrence x : O1 = oc1;` | Probe::d.x => object:Probe::O1 | answers | clean |
| constant | `connection Cn1` | `constant connection x : Cn1 connect sub1 to sub2;` | Probe::d.x => object:Probe::Cn1 | answers | clean |
| readonly | `attribute Integer` | `readonly attribute x : Integer = 1;` | Probe::d.x => constant 1 | validate rejects | error[syntax]: expected a body member; |
| readonly | `attribute At1` | `readonly attribute x : At1;` | Probe::d.x => undetermined <undetermined> | validate rejects | error[syntax]: expected a body member; |
| readonly | `enum E1` | `readonly enum x : E1 = E1::a;` | Probe::d.x => ERR_NOSUCHFEATURE object has no such feature: member x not found in instance | validate rejects | error[syntax]: expected a body member; |
| readonly | `calc K1` | `readonly calc x : K1;` | Probe::d.x => constant 4 | validate rejects | error[syntax]: expected a body member; |
| readonly | `part P2` | `readonly part x : P2 = sub1;` | Probe::d.x => object:Probe::P2 | validate rejects | error[syntax]: expected a body member; |
| readonly | `item I1` | `readonly item x : I1 = it1;` | Probe::d.x => object:Probe::I1 | validate rejects | error[syntax]: expected a body member; |
| readonly | `port Pt` | `readonly port x : Pt = pt1;` | Probe::d.x => object:Probe::Pt | validate rejects | error[syntax]: expected a body member; |
| readonly | `action A1` | `readonly action x : A1 = act1;` | Probe::d.x => object:Probe::A1 | validate rejects | error[syntax]: expected a body member; |
| readonly | `state S1` | `readonly state x : S1 = st1;` | Probe::d.x => object:Probe::S1 | validate rejects | error[syntax]: expected a body member; |
| readonly | `requirement R1` | `readonly requirement x : R1 = rq1;` | Probe::d.x => object:Probe::R1 | validate rejects | error[syntax]: expected a body member; |
| readonly | `constraint C1` | `readonly constraint x : C1 = cq1;` | Probe::d.x => object:Probe::C1 | validate rejects | error[syntax]: expected a body member; |
| readonly | `occurrence O1` | `readonly occurrence x : O1 = oc1;` | Probe::d.x => object:Probe::O1 | validate rejects | error[syntax]: expected a body member; |
| readonly | `connection Cn1` | `readonly connection x : Cn1 connect sub1 to sub2;` | Probe::d.x => object:Probe::Cn1 | validate rejects | error[syntax]: expected a body member; |
| derived | `attribute Integer` | `derived attribute x : Integer = 1;` | Probe::d.x => constant 1 | answers | clean |
| derived | `attribute At1` | `derived attribute x : At1;` | Probe::d.x => undetermined <undetermined> | `<undetermined>` | clean |
| derived | `enum E1` | `derived enum x : E1 = E1::a;` | Probe::d.x => ERR_NOSUCHFEATURE object has no such feature: member x not found in instance | error (`ErrNoSuchFeature`) | clean |
| derived | `calc K1` | `derived calc x : K1;` | Probe::d.x => constant 4 | answers | clean |
| derived | `part P2` | `derived part x : P2 = sub1;` | Probe::d.x => object:Probe::P2 | answers | clean |
| derived | `item I1` | `derived item x : I1 = it1;` | Probe::d.x => object:Probe::I1 | answers | clean |
| derived | `port Pt` | `derived port x : Pt = pt1;` | Probe::d.x => object:Probe::Pt | answers | clean |
| derived | `action A1` | `derived action x : A1 = act1;` | Probe::d.x => object:Probe::A1 | answers | clean |
| derived | `state S1` | `derived state x : S1 = st1;` | Probe::d.x => object:Probe::S1 | answers | clean |
| derived | `requirement R1` | `derived requirement x : R1 = rq1;` | Probe::d.x => object:Probe::R1 | answers | clean |
| derived | `constraint C1` | `derived constraint x : C1 = cq1;` | Probe::d.x => object:Probe::C1 | answers | clean |
| derived | `occurrence O1` | `derived occurrence x : O1 = oc1;` | Probe::d.x => object:Probe::O1 | answers | clean |
| derived | `connection Cn1` | `derived connection x : Cn1 connect sub1 to sub2;` | Probe::d.x => object:Probe::Cn1 | answers | clean |
| portion | `attribute Integer` | `portion attribute x : Integer;` | Probe::d.x => undetermined <undetermined> | `<undetermined>` | clean |
| portion | `attribute At1` | `portion attribute x : At1;` | Probe::d.x => undetermined <undetermined> | `<undetermined>` | clean |
| portion | `enum E1` | `portion enum x : E1;` | Probe::d.x => ERR_NOSUCHFEATURE object has no such feature: member x not found in instance | error (`ErrNoSuchFeature`) | clean |
| portion | `calc K1` | `portion calc x : K1;` | Probe::d.x => constant 4 | answers | clean |
| portion | `part P2` | `portion part x : P2;` | Probe::d.x => object:Probe::P2 | answers | clean |
| portion | `item I1` | `portion item x : I1;` | Probe::d.x => object:Probe::I1 | answers | clean |
| portion | `port Pt` | `portion port x : Pt;` | Probe::d.x => object:Probe::Pt | answers | clean |
| portion | `action A1` | `portion action x : A1;` | Probe::d.x => object:Probe::A1 | answers | clean |
| portion | `state S1` | `portion state x : S1;` | Probe::d.x => object:Probe::S1 | answers | clean |
| portion | `requirement R1` | `portion requirement x : R1;` | Probe::d.x => object:Probe::R1 | answers | clean |
| portion | `constraint C1` | `portion constraint x : C1;` | Probe::d.x => object:Probe::C1 | answers | clean |
| portion | `occurrence O1` | `portion occurrence x : O1;` | Probe::d.x => object:Probe::O1 | answers | clean |
| portion | `connection Cn1` | `portion connection x : Cn1 connect sub1 to sub2;` | Probe::d.x => object:Probe::Cn1 | answers | clean |
| individual | `attribute Integer` | `individual attribute x : Integer;` | Probe::d.x => undetermined <undetermined> | validate rejects | error[individual-typing]: An individual must be typed by one individual definiti… |
| individual | `attribute At1` | `individual attribute x : At1;` | Probe::d.x => undetermined <undetermined> | validate rejects | error[individual-typing]: An individual must be typed by one individual definiti… |
| individual | `enum E1` | `individual enum x : E1;` | Probe::d.x => ERR_NOSUCHFEATURE object has no such feature: member x not found in instance | validate rejects | error[individual-typing]: An individual must be typed by one individual definiti… |
| individual | `calc K1` | `individual calc x : K1;` | Probe::d.x => constant 4 | validate rejects | error[individual-typing]: An individual must be typed by one individual definiti… |
| individual | `part P2` | `individual part x : P2;` | Probe::d.x => object:Probe::P2 | validate rejects | error[individual-typing]: An individual must be typed by one individual definiti… |
| individual | `item I1` | `individual item x : I1;` | Probe::d.x => object:Probe::I1 | validate rejects | error[individual-typing]: An individual must be typed by one individual definiti… |
| individual | `port Pt` | `individual port x : Pt;` | Probe::d.x => object:Probe::Pt | validate rejects | error[individual-typing]: An individual must be typed by one individual definiti… |
| individual | `action A1` | `individual action x : A1;` | Probe::d.x => object:Probe::A1 | validate rejects | error[individual-typing]: An individual must be typed by one individual definiti… |
| individual | `state S1` | `individual state x : S1;` | Probe::d.x => object:Probe::S1 | validate rejects | error[individual-typing]: An individual must be typed by one individual definiti… |
| individual | `requirement R1` | `individual requirement x : R1;` | Probe::d.x => object:Probe::R1 | validate rejects | error[individual-typing]: An individual must be typed by one individual definiti… |
| individual | `constraint C1` | `individual constraint x : C1;` | Probe::d.x => object:Probe::C1 | validate rejects | error[individual-typing]: An individual must be typed by one individual definiti… |
| individual | `occurrence O1` | `individual occurrence x : O1;` | Probe::d.x => object:Probe::O1 | validate rejects | error[individual-typing]: An individual must be typed by one individual definiti… |
| individual | `connection Cn1` | `individual connection x : Cn1 connect sub1 to sub2;` | Probe::d.x => object:Probe::Cn1 | validate rejects | error[individual-typing]: An individual must be typed by one individual definiti… |
| [2..4] | `attribute Integer` | `attribute x : Integer[2..4];` | Probe::d.x => undetermined <undetermined> | `<undetermined>` | clean |
| [2..4] | `attribute At1` | `attribute x : At1[2..4];` | Probe::d.x => undetermined <undetermined> | `<undetermined>` | clean |
| [2..4] | `enum E1` | `enum x : E1[2..4];` | Probe::d.x => ERR_NOSUCHFEATURE object has no such feature: member x not found in instance | error (`ErrNoSuchFeature`) | clean |
| [2..4] | `calc K1` | `calc x : K1[2..4];` | Probe::d.x => constant 4 | answers | clean |
| [2..4] | `part P2` | `part x : P2[2..4];` | Probe::d.x => undetermined <undetermined> | `<undetermined>` | clean |
| [2..4] | `item I1` | `item x : I1[2..4];` | Probe::d.x => undetermined <undetermined> | `<undetermined>` | clean |
| [2..4] | `port Pt` | `port x : Pt[2..4];` | Probe::d.x => undetermined <undetermined> | `<undetermined>` | clean |
| [2..4] | `action A1` | `action x : A1[2..4];` | Probe::d.x => undetermined <undetermined> | `<undetermined>` | clean |
| [2..4] | `state S1` | `state x : S1[2..4];` | Probe::d.x => undetermined <undetermined> | `<undetermined>` | clean |
| [2..4] | `requirement R1` | `requirement x : R1[2..4];` | Probe::d.x => undetermined <undetermined> | `<undetermined>` | clean |
| [2..4] | `constraint C1` | `constraint x : C1[2..4];` | Probe::d.x => undetermined <undetermined> | `<undetermined>` | clean |
| [2..4] | `occurrence O1` | `occurrence x : O1[2..4];` | Probe::d.x => undetermined <undetermined> | `<undetermined>` | clean |
| [2..4] | `connection Cn1` | `connection x : Cn1[2..4] connect sub1 to sub2;` | Probe::d.x => error connector end cannot be attached: d.x end "x": a connector of more than one object has no set of ends to attach | typed error | clean |
| [*] = (3,1,3) (unique default) | `attribute Integer` | `attribute x : Integer[*] = (3, 1, 3);` | Probe::d.x => error feature value d.x: uniqueness violation: 3 (an Integer) is written at positions 1 and 3 of a unique feature | validate rejects | error[unique-feature-values]: 3 (an Integer) is written at positions 1 and 3 of … |
| [*] ordered nonunique = (3,1,3) | `attribute Integer` | `attribute x : Integer[*] ordered nonunique = (3, 1, 3);` | Probe::d.x => collection(3) [3, 1, 3] | answers | clean |
| [*] nonunique = (3,1,3) | `attribute Integer` | `attribute x : Integer[*] nonunique = (3, 1, 3);` | Probe::d.x => collection(3) [3, 1, 3] | answers | clean |
| [2] = (1,2,3) (too many) | `attribute Integer` | `attribute x : Integer[2] = (1, 2, 3);` | Probe::d.x => error feature value d.x: multiplicity violation: 3 value(s) bound to a feature with multiplicity upper bound 2 | validate rejects | error[feature-value-count]: 3 value(s) bound to a feature with multiplicity uppe… |
| default | `attribute Integer` | `attribute x : Integer default 1;` | Probe::d.x => constant 1 | answers | clean |
| default | `enum E1` | `enum x : E1 default E1::a;` | Probe::d.x => ERR_NOSUCHFEATURE object has no such feature: member x not found in instance | error (`ErrNoSuchFeature`) | clean |
| default | `part P2` | `part x : P2 default sub1;` | Probe::d.x => object:Probe::P2 | answers | clean |
| default | `item I1` | `item x : I1 default it1;` | Probe::d.x => object:Probe::I1 | answers | clean |
| default | `port Pt` | `port x : Pt default pt1;` | Probe::d.x => object:Probe::Pt | answers | clean |
| default | `action A1` | `action x : A1 default act1;` | Probe::d.x => object:Probe::A1 | answers | clean |
| default | `state S1` | `state x : S1 default st1;` | Probe::d.x => object:Probe::S1 | answers | clean |
| default | `requirement R1` | `requirement x : R1 default rq1;` | Probe::d.x => object:Probe::R1 | answers | clean |
| default | `constraint C1` | `constraint x : C1 default cq1;` | Probe::d.x => object:Probe::C1 | answers | clean |
| default | `occurrence O1` | `occurrence x : O1 default oc1;` | Probe::d.x => object:Probe::O1 | answers | clean |
| default + :>> override | `attribute Integer` | `attribute x : Integer default 1;` | Probe::d.x => constant 1 ;; Probe::d2.x => constant 2 ;; Probe::d2.x == Probe::d2.sub2 => constant false | answers | clean |
| default + :>> override | `enum E1` | `enum x : E1 default E1::a;` | Probe::d.x => ERR_NOSUCHFEATURE object has no such feature: member x not found in instance ;; Probe::d2.x => enumeration literal E1::b ;; Probe::d2.x == Probe::… | error (`ErrNoSuchFeature`) | clean |
| default + :>> override | `part P2` | `part x : P2 default sub1;` | Probe::d.x => object:Probe::P2 ;; Probe::d2.x => object:Probe::P2 ;; Probe::d2.x == Probe::d2.sub2 => constant true | answers | clean |
| default + :>> override | `item I1` | `item x : I1 default it1;` | Probe::d.x => object:Probe::I1 ;; Probe::d2.x => object:Probe::I1 ;; Probe::d2.x == Probe::d2.sub2 => constant false | answers | clean |
| default + :>> override | `port Pt` | `port x : Pt default pt1;` | Probe::d.x => object:Probe::Pt ;; Probe::d2.x => object:Probe::Pt ;; Probe::d2.x == Probe::d2.sub2 => constant false | answers | clean |
| default + :>> override | `action A1` | `action x : A1 default act1;` | Probe::d.x => object:Probe::A1 ;; Probe::d2.x => object:Probe::A1 ;; Probe::d2.x == Probe::d2.sub2 => constant false | answers | clean |
| default + :>> override | `state S1` | `state x : S1 default st1;` | Probe::d.x => object:Probe::S1 ;; Probe::d2.x => object:Probe::S1 ;; Probe::d2.x == Probe::d2.sub2 => constant false | answers | clean |
| default + :>> override | `requirement R1` | `requirement x : R1 default rq1;` | Probe::d.x => object:Probe::R1 ;; Probe::d2.x => object:Probe::R1 ;; Probe::d2.x == Probe::d2.sub2 => constant false | answers | clean |
| default + :>> override | `constraint C1` | `constraint x : C1 default cq1;` | Probe::d.x => object:Probe::C1 ;; Probe::d2.x => object:Probe::C1 ;; Probe::d2.x == Probe::d2.sub2 => constant false | answers | clean |
| default + :>> override | `occurrence O1` | `occurrence x : O1 default oc1;` | Probe::d.x => object:Probe::O1 ;; Probe::d2.x => object:Probe::O1 ;; Probe::d2.x == Probe::d2.sub2 => constant false | answers | clean |
| := initial | `attribute Integer` | `attribute x : Integer := 1;` | Probe::d.x => constant 1 | answers | clean |
| := initial | `enum E1` | `enum x : E1 := E1::a;` | Probe::d.x => ERR_NOSUCHFEATURE object has no such feature: member x not found in instance | error (`ErrNoSuchFeature`) | clean |
| := initial | `part P2` | `part x : P2 := sub1;` | Probe::d.x => object:Probe::P2 | answers | clean |
| := initial | `item I1` | `item x : I1 := it1;` | Probe::d.x => object:Probe::I1 | answers | clean |
| := initial | `port Pt` | `port x : Pt := pt1;` | Probe::d.x => object:Probe::Pt | answers | clean |
| := initial | `action A1` | `action x : A1 := act1;` | Probe::d.x => object:Probe::A1 | validate rejects | error[initial-value-not-variable]: Initialized feature must be variable; |
| := initial | `state S1` | `state x : S1 := st1;` | Probe::d.x => object:Probe::S1 | validate rejects | error[initial-value-not-variable]: Initialized feature must be variable; |
| := initial | `requirement R1` | `requirement x : R1 := rq1;` | Probe::d.x => object:Probe::R1 | answers | clean |
| := initial | `constraint C1` | `constraint x : C1 := cq1;` | Probe::d.x => object:Probe::C1 | answers | clean |
| := initial | `occurrence O1` | `occurrence x : O1 := oc1;` | Probe::d.x => object:Probe::O1 | answers | clean |
| :>> redefinition masks | `attribute Integer` | `attribute x : Integer = 1;` | Probe::d.x => constant 1 ;; Probe::d2.x => constant 2 ;; Probe::d2.x == Probe::d2.sub2 => constant false | validate rejects | error[feature-value-overriding]: cannot override the binding value of Probe::D::… |
| :>> redefinition masks | `enum E1` | `enum x : E1 = E1::a;` | Probe::d.x => ERR_NOSUCHFEATURE object has no such feature: member x not found in instance ;; Probe::d2.x => ERR_NOSUCHFEATURE object has no such feature: membe… | validate rejects | error[feature-value-overriding]: cannot override the binding value of Probe::D::… |
| :>> redefinition masks | `part P2` | `part x : P2 = sub1;` | Probe::d.x => object:Probe::P2 ;; Probe::d2.x => object:Probe::P2 ;; Probe::d2.x == Probe::d2.sub2 => constant true | validate rejects | error[feature-value-overriding]: cannot override the binding value of Probe::D::… |
| :>> redefinition masks | `item I1` | `item x : I1 = it1;` | Probe::d.x => object:Probe::I1 ;; Probe::d2.x => object:Probe::I1 ;; Probe::d2.x == Probe::d2.sub2 => constant false | validate rejects | error[feature-value-overriding]: cannot override the binding value of Probe::D::… |
| :>> redefinition masks | `port Pt` | `port x : Pt = pt1;` | Probe::d.x => object:Probe::Pt ;; Probe::d2.x => object:Probe::Pt ;; Probe::d2.x == Probe::d2.sub2 => constant false | validate rejects | error[feature-value-overriding]: cannot override the binding value of Probe::D::… |
| :>> redefinition masks | `action A1` | `action x : A1 = act1;` | Probe::d.x => object:Probe::A1 ;; Probe::d2.x => object:Probe::A1 ;; Probe::d2.x == Probe::d2.sub2 => constant false | validate rejects | error[feature-value-overriding]: cannot override the binding value of Probe::D::… |
| :>> redefinition masks | `state S1` | `state x : S1 = st1;` | Probe::d.x => object:Probe::S1 ;; Probe::d2.x => object:Probe::S1 ;; Probe::d2.x == Probe::d2.sub2 => constant false | validate rejects | error[feature-value-overriding]: cannot override the binding value of Probe::D::… |
| :>> redefinition masks | `requirement R1` | `requirement x : R1 = rq1;` | Probe::d.x => object:Probe::R1 ;; Probe::d2.x => object:Probe::R1 ;; Probe::d2.x == Probe::d2.sub2 => constant false | validate rejects | error[feature-value-overriding]: cannot override the binding value of Probe::D::… |
| :>> redefinition masks | `constraint C1` | `constraint x : C1 = cq1;` | Probe::d.x => object:Probe::C1 ;; Probe::d2.x => object:Probe::C1 ;; Probe::d2.x == Probe::d2.sub2 => constant false | validate rejects | error[feature-value-overriding]: cannot override the binding value of Probe::D::… |
| :>> redefinition masks | `occurrence O1` | `occurrence x : O1 = oc1;` | Probe::d.x => object:Probe::O1 ;; Probe::d2.x => object:Probe::O1 ;; Probe::d2.x == Probe::d2.sub2 => constant false | validate rejects | error[feature-value-overriding]: cannot override the binding value of Probe::D::… |
| :> subsetting | `attribute Integer` | `attribute x : Integer[0..*]; attribute y : Integer :> x;` | Probe::d.x => undetermined <undetermined> ;; Probe::d.y => undetermined <undetermined> | `<undetermined>` | clean |
| :> subsetting | `attribute At1` | `attribute x : At1[0..*]; attribute y : At1 :> x;` | Probe::d.x => undetermined <undetermined> ;; Probe::d.y => undetermined <undetermined> | `<undetermined>` | clean |
| :> subsetting | `enum E1` | `enum x : E1[0..*]; enum y : E1 :> x;` | Probe::d.x => ERR_NOSUCHFEATURE object has no such feature: member x not found in instance ;; Probe::d.y => ERR_NOSUCHFEATURE object has no such feature: member… | error (`ErrNoSuchFeature`) | clean |
| :> subsetting | `calc K1` | `calc x : K1[0..*]; calc y : K1 :> x;` | Probe::d.x => constant 4 ;; Probe::d.y => constant 4 | answers | clean |
| :> subsetting | `part P2` | `part x : P2[0..*]; part y : P2 :> x;` | Probe::d.x => undetermined <undetermined> ;; Probe::d.y => object:Probe::P2 | `<undetermined>` | clean |
| :> subsetting | `item I1` | `item x : I1[0..*]; item y : I1 :> x;` | Probe::d.x => undetermined <undetermined> ;; Probe::d.y => object:Probe::I1 | `<undetermined>` | clean |
| :> subsetting | `port Pt` | `port x : Pt[0..*]; port y : Pt :> x;` | Probe::d.x => undetermined <undetermined> ;; Probe::d.y => object:Probe::Pt | `<undetermined>` | clean |
| :> subsetting | `action A1` | `action x : A1[0..*]; action y : A1 :> x;` | Probe::d.x => undetermined <undetermined> ;; Probe::d.y => object:Probe::A1 | `<undetermined>` | clean |
| :> subsetting | `state S1` | `state x : S1[0..*]; state y : S1 :> x;` | Probe::d.x => undetermined <undetermined> ;; Probe::d.y => object:Probe::S1 | `<undetermined>` | clean |
| :> subsetting | `requirement R1` | `requirement x : R1[0..*]; requirement y : R1 :> x;` | Probe::d.x => undetermined <undetermined> ;; Probe::d.y => object:Probe::R1 | `<undetermined>` | clean |
| :> subsetting | `constraint C1` | `constraint x : C1[0..*]; constraint y : C1 :> x;` | Probe::d.x => undetermined <undetermined> ;; Probe::d.y => object:Probe::C1 | `<undetermined>` | clean |
| :> subsetting | `occurrence O1` | `occurrence x : O1[0..*]; occurrence y : O1 :> x;` | Probe::d.x => undetermined <undetermined> ;; Probe::d.y => object:Probe::O1 | `<undetermined>` | clean |
| :> subsetting | `connection Cn1` | `connection x : Cn1[0..*] connect sub1 to sub2; connection y : Cn1 connect sub1 to sub2 :> x;` | Probe::d.x => error connector end cannot be attached: d.x end "x": a connector of more than one object has no set of ends to attach ;; Probe::d.y => object:Prob… | validate rejects | error[syntax]: expected '{' or ';' after declaration; |
| ::> reference subsetting | `attribute Integer` | `attribute x : Integer; ref attribute y ::> x;` | Probe::d.x => undetermined <undetermined> ;; Probe::d.y => error uninitialized feature value: y ;; Probe::d.y == Probe::d.x => error uninitialized feature value… | `<undetermined>` | clean |
| ::> reference subsetting | `attribute At1` | `attribute x : At1; ref attribute y ::> x;` | Probe::d.x => undetermined <undetermined> ;; Probe::d.y => error uninitialized feature value: y ;; Probe::d.y == Probe::d.x => error uninitialized feature value… | `<undetermined>` | clean |
| ::> reference subsetting | `enum E1` | `enum x : E1; ref enum y ::> x;` | Probe::d.x => ERR_NOSUCHFEATURE object has no such feature: member x not found in instance ;; Probe::d.y => ERR_NOSUCHFEATURE object has no such feature: member… | error (`ErrNoSuchFeature`) | clean |
| ::> reference subsetting | `calc K1` | `calc x : K1; ref calc y ::> x;` | Probe::d.x => constant 4 ;; Probe::d.y => constant 4 ;; Probe::d.y == Probe::d.x => constant true | answers | clean |
| ::> reference subsetting | `part P2` | `part x : P2; ref part y ::> x;` | Probe::d.x => object:Probe::P2 ;; Probe::d.y => error uninitialized feature value: y ;; Probe::d.y == Probe::d.x => error uninitialized feature value: y | answers | clean |
| ::> reference subsetting | `item I1` | `item x : I1; ref item y ::> x;` | Probe::d.x => object:Probe::I1 ;; Probe::d.y => error uninitialized feature value: y ;; Probe::d.y == Probe::d.x => error uninitialized feature value: y | answers | clean |
| ::> reference subsetting | `port Pt` | `port x : Pt; ref port y ::> x;` | Probe::d.x => object:Probe::Pt ;; Probe::d.y => error uninitialized feature value: y ;; Probe::d.y == Probe::d.x => error uninitialized feature value: y | answers | clean |
| ::> reference subsetting | `action A1` | `action x : A1; ref action y ::> x;` | Probe::d.x => object:Probe::A1 ;; Probe::d.y => error uninitialized feature value: y ;; Probe::d.y == Probe::d.x => error uninitialized feature value: y | answers | clean |
| ::> reference subsetting | `state S1` | `state x : S1; ref state y ::> x;` | Probe::d.x => object:Probe::S1 ;; Probe::d.y => error uninitialized feature value: y ;; Probe::d.y == Probe::d.x => error uninitialized feature value: y | answers | clean |
| ::> reference subsetting | `requirement R1` | `requirement x : R1; ref requirement y ::> x;` | Probe::d.x => object:Probe::R1 ;; Probe::d.y => error uninitialized feature value: y ;; Probe::d.y == Probe::d.x => error uninitialized feature value: y | answers | clean |
| ::> reference subsetting | `constraint C1` | `constraint x : C1; ref constraint y ::> x;` | Probe::d.x => object:Probe::C1 ;; Probe::d.y => error uninitialized feature value: y ;; Probe::d.y == Probe::d.x => error uninitialized feature value: y | answers | clean |
| ::> reference subsetting | `occurrence O1` | `occurrence x : O1; ref occurrence y ::> x;` | Probe::d.x => object:Probe::O1 ;; Probe::d.y => error uninitialized feature value: y ;; Probe::d.y == Probe::d.x => error uninitialized feature value: y | answers | clean |
| ::> reference subsetting | `connection Cn1` | `connection x : Cn1 connect sub1 to sub2; ref connection y ::> x;` | Probe::d.x => object:Probe::Cn1 ;; Probe::d.y => error uninitialized feature value: y ;; Probe::d.y == Probe::d.x => error uninitialized feature value: y | validate rejects | error[related-elements]: Must have at least two related elements; |

Reading of the sweep 3 matrix:

- `derived`, `constant`, `readonly`-as-`constant`, `default`, `=`, `:=`, `:>>` (masking a `default`), `abstract` with and without subsetters, `[*] ordered nonunique` (keeps `(3, 1, 3)`), `[*]` unique (refuses the duplicate as the validator does), `[2]` over three values (multiplicity violation on both surfaces), and `variation` (not instantiable: "variation has no variant selected", its variants readable) behave as the specification states. Writes to `constant`/`derived` features are pinned by the existing `assign_*_refused` conformance fixtures and were not re-probed.
- `[2..4]` on a package-level object is `<undetermined>` by the model-level rule; the instantiated-object contract materializes the lower bound.
- `in`/`out`/`inout` make an occurrence usage a parameter and so `<undetermined>`, except `part`/`item`/`port`, which materialize.
- `ref (unbound)` and `::>` are the unbound-reference class above; `enum` is the shape-kind class above.
- `readonly` is not a SysML v2 prefix (syntax error, as expected); `individual` requires an individual definition and `constant` a variable feature — the validator rejects those cells correctly.
