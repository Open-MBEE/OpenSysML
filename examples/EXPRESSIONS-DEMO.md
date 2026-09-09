# Expressions demo

[`expressions-demo.sysml`](expressions-demo.sysml) is a small instrument
payload — two cameras and a spectrometer with masses, heritage annotations, a
downlink budget, radio bands and a stress field on the mount — written so that
each of the expression forms below has something concrete to answer:

| Form | Where it appears | What it answers |
| --- | --- | --- |
| `x as T` | `WholeReadings`, `CamerasOf`, `PayloadCasts` | the values `T` classifies, and nothing else |
| `*` | `DownlinkBudget` | a bound with no upper limit, compared but never added |
| `.metadata` | `HeritageReport` | the annotations on an element and the values they bind |
| a calc as a value | `Apply`, `Squared`, `SquaresOf`, `Amplifier` | passing, holding, comparing and invoking a calculation |
| `Set` | `RadioBands` | a collection with no order and no repeats |
| `TensorQuantityValue` | `MountStress` | a rank-three tensor indexed by three positions |
| collection bodies | `MassRollup` | `collect`/`select`/`reduce` results typed by their bodies |

Everything below runs with no external tools. Load the model at the prompt:

```bash
./bin/sysml examples/expressions-demo.sysml
```

Each section instantiates one `part def` and reads its features; `%features`
also lists the inherited `Part` features (`ownedPorts`, `subparts`, …), which
are left out of the transcripts here.

## `x as T` — a cast selects, it does not convert

A cast keeps the values its target classifies and drops the rest; it never
changes a value. `4.0 as Integer` is `4.0`, because a whole Real *is* an
Integer in the `ScalarValues` hierarchy, and `2.5 as Integer` is the empty
sequence. The functions that convert — `ToInteger`, `ToString` and their kin
— are in the library, and are not what `as` does.

```
%instantiate PayloadCasts
%features PayloadCasts
```

```
Instance: ExpressionsDemo::PayloadCasts (ID: 1)
Features:
  wholeOnly = [1.0, 3.0]
  cameras = [Instance(ID: 2), Instance(ID: 4)]
    mass = 4.0 [kg]
    mass = 6.5 [kg]
  cameraCount = 2
  asInstrument = Instance(ID: 2)
    mass = 4.0 [kg]
  notACamera = []
```

- `wholeOnly` is `WholeReadings((1.0, 2.5, 3.0, 4.75))`: a sequence casts
  element by element, in order, keeping only the whole numbers.
- `cameras` is `CamerasOf((navCam, spectrometer, sciCam))`: an object is kept
  by every classifier it is an instance of, so the two cameras pass and the
  spectrometer does not.
- `asInstrument` widens `navCam` to its general type and keeps it;
  `notACamera` asks whether the spectrometer is a `Camera`, and it is not.

A feature holding a cast result should be declared `[0..1]` or `[0..*]`: a
cast that selects nothing yields the empty sequence, which a feature of
multiplicity `[1]` cannot hold.

The same expressions work at the prompt, where the cast's target is written
with its qualified name:

```
(1, 2.5, 3) as ScalarValues::Integer
2.5 as ScalarValues::Integer
ExpressionsDemo::navCam as ExpressionsDemo::Spectrometer
```

```
✓ (1, 2.5, 3) as ScalarValues::Integer
  = [1, 3]
✓ 2.5 as ScalarValues::Integer
  = []
✓ ExpressionsDemo::navCam as ExpressionsDemo::Spectrometer
  = []
```

The checker warns ahead of time when a cast can select nothing because the
operand's type and the target are unrelated — `navCam as Spectrometer` draws
"cast argument is typed by Camera, unrelated to the target Spectrometer" — so
a cast that always comes back empty does not have to be found at run time.

## `*` — the unbounded value

`*` is a value of its own, not a large number: it exceeds every finite number,
equals itself, and prints as `*`.

```
%instantiate DownlinkBudget
%features DownlinkBudget
```

```
Instance: ExpressionsDemo::DownlinkBudget (ID: 5)
Features:
  passLimit = *
  plannedPasses = 40
  withinLimit = true
  limitIsUnbounded = true
```

Comparisons work at the prompt too, and arithmetic over `*` is refused with an
error naming the operator rather than answered with a finite result:

```
3 < *
* == *
* + 1
```

```
✓ 3 < *
  = true
✓ * == *
  = true
error: evaluation failed: type mismatch: operator '+' is not defined for the unbounded value '*': * + 1
```

## `.metadata` — what annotates an element

`elem.metadata` is the sequence of metadata annotating `elem`, one object per
annotation in the order they are written, each carrying the values its body
binds over the defaults its `metadata def` declares. `navCam` is annotated
`@Heritage { mission = "Cassini"; }`, the spectrometer sets `flown = false`,
and `sciCam` carries no annotation at all.

```
%instantiate HeritageReport
%features HeritageReport
```

```
Instance: ExpressionsDemo::HeritageReport (ID: 6)
Features:
  navCamAnnotations = [Instance(ID: 7)]
    mission = "Cassini"
    flown = true
  navCamMission = "Cassini"
  navCamFlown = true
  spectrometerFlown = false
  sciCamAnnotations = []
```

`navCamFlown` is `true` without `navCam` saying so: the annotation did not bind
`flown`, so the object carries the default from `Heritage`. Indexing is
one-based, as everywhere in SysML: `navCam.metadata#(1)` is the first
annotation.

## A calculation as a value

A `calc def`, a `calc` usage or an `in calc` parameter named where a value is
expected is a *function value*: the calculation itself, together with whatever
it closes over. `Apply` takes one as its `in calc f` parameter and invokes it
as `f(a)`:

```
%calc Squared(3.0)
%calc Halved(3.0)
%calc Apply(Halve, 9.0)
```

```
✓ Squared(3.0)
  = 9.0
✓ Halved(3.0)
  = 1.5
✓ Apply(Halve, 9.0)
  = 4.5
```

Reading a calculation on its own answers the function, named by its
declaration; the analysis library's `SampledFunctions::Sample` takes one and
tabulates it over a domain:

```
ExpressionsDemo::Square
%calc SquaresOf((1.0, 2.0, 3.0))
```

```
✓ ExpressionsDemo::Square
  = ExpressionsDemo::Square
✓ SquaresOf((1.0, 2.0, 3.0))
  = [1.0, 4.0, 9.0]
```

A nested calc closes over the features around it. `Amplifier::amplify`
multiplies by the part's `gain`, so the function held in `transfer` carries
that `gain` with it, and two reads of the same calc in one object are the same
function:

```
%instantiate Amplifier
%features Amplifier
```

```
Instance: ExpressionsDemo::Amplifier (ID: 9)
Features:
  gain = 1.5
  transfer = ExpressionsDemo::Amplifier::amplify
  sameTransfer = true
  atThree = 4.5
  squaredAtThree = 9.0
```

A definition is not a feature, so `attribute transfer = Square;` is refused by
the checker ("Must be a valid feature") where `attribute transfer = amplify;`
— a calc usage — is fine; a `calc def` is passed as an argument, as
`Apply(Square, 3.0)` does, or held through a usage of it.

## `Set` — no order, no repeats

The library declares a `Collections::Set`'s elements unique and unordered, so
a `Set` holds a set: the elements it was given with every repeat dropped, and
no order of its own. Two sets given the same elements in different orders are
equal, and so are their `elements`.

```
%instantiate RadioBands
%features RadioBands
```

```
Instance: ExpressionsDemo::RadioBands (ID: 10)
Features:
  requested = Instance(ID: 11)
    elements = Set{"Ka", "S", "X"}
  licensed = Instance(ID: 12)
    elements = Set{"Ka", "S", "X"}
  distinctBands = 3
  sameBands = true
  hasKa = true
  asList = ["Ka", "S", "X"]
```

A set prints as `Set{…}` in a canonical order, which is not the order it was
written in. Flowing its elements into an ordered feature (`asList`) gives a
sequence in that same canonical order. `Bag` and `OrderedSet` are not sets:
the library keeps their order or their repeats, and so does the runtime.

## A rank-three tensor

`TensorCalculations::'['` pairs a flat sequence of numbers with a
`TensorMeasurementReference` whose `dimensions` give the shape, in row-major
order with the last index varying fastest. `#` then takes one index per
dimension, and the arithmetic keeps the shape component by component:

```
%instantiate MountStress
%features MountStress
```

```
Instance: ExpressionsDemo::MountStress (ID: 13)
Features:
  field = Tensor(2, 2, 2)[1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0] [Pa]
  rank = 3
  dims = [2, 2, 2]
  corner = 1.0 [Pa]
  opposite = 8.0 [Pa]
  doubled = Tensor(2, 2, 2)[2.0, 4.0, 6.0, 8.0, 10.0, 12.0, 14.0, 16.0] [Pa]
  doubledCorner = 10.0 [Pa]
```

`doubledCorner` is `doubled#(2, 1, 1)`: the first index selects the second
2×2 slab, whose first component is `2 * 5.0`. An index outside the shape, or
the wrong number of indices for the rank, is an error rather than a wrapped
or truncated read.

## Collection bodies — the result has the body's type

`collect`, `select` and `reduce` take a body whose parameter is bound to each
element in turn. The checker types a `collect` by what its body returns, not
by the element type of the collection it ran over, so `instruments->collect {
in i : Instrument; i.mass }` is a `MassValue[0..*]` and can be declared as one
— and can be reduced, compared and aggregated as masses:

```
%instantiate MassRollup
%features MassRollup
```

```
Instance: ExpressionsDemo::MassRollup (ID: 15)
Features:
  instruments = [Instance(ID: 2), Instance(ID: 4), Instance(ID: 3)]
    mass = 4.0 [kg]
    mass = 6.5 [kg]
    mass = 12.0 [kg]
  masses = [4.0 [kg], 6.5 [kg], 12.0 [kg]]
  total = 22.5 [kg]
  heaviest = 12.0 [kg]
  heavy = [Instance(ID: 4), Instance(ID: 3)]
    mass = 6.5 [kg]
    mass = 12.0 [kg]
  heavyCount = 2
```

Because the result type is the body's, a mismatch is caught before anything
runs. Declaring the same `collect` as `String[0..*]` is refused when the model
is loaded:

```
error: cannot bind a value of type MassValue to a feature typed by String
        attribute names : String[0..*] = instruments->collect { in i : Instrument; i.mass };
```

The body's parameter is declared with its type, `in i : Instrument`, so that
`i.mass` resolves to a feature of `Instrument` and the body's result is typed
by it.

## Where these are specified

- Casts: KerML 1.1 §8.3.4.9, *CastExpression*.
- `*`: KerML 1.1 §8.3.3.1, *LiteralInfinity*.
- `.metadata`: KerML 1.1 §8.3.3.1, *MetadataAccessExpression*.
- Sets: `Collections::Set` in the SysML v2 Systems Library, whose `elements`
  redefine `UniqueCollection::elements` as unordered.
- Tensors: `Quantities::TensorQuantityValue` and `TensorCalculations` in the
  Quantities and Units Domain Library.

The guide chapter on [expressions, calculations, constraints and
requirements](../docs/guide/05-checking.md) introduces each of these forms;
this demo is its worked example.
