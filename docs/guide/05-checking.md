# 5. Expressions, calculations, constraints and requirements

This chapter describes what the runtime evaluates and how each kind of check reports its result.
A constraint declared on a definition is checked against the object that carries it, so
instantiate the definition first if you want a verdict about a concrete value rather than a
default. When several objects the session holds carry it — two `%instantiate`s of one name leave
the first object reachable as `#<id>`, and a multi-valued part holds one carrier per element — the
check names them (`car.wheels[1]`, `car.wheels[2]`, `#1.wheels[1]`, …) and asks you to pick one,
with `%eval in car.wheels[2] : ...` or `%eval in #1 : ...`.

## Expressions

**Literals:**
```sysml
attribute x = 42;              // Integer
attribute y = 3.14;            // Real
attribute flag = true;         // Boolean
attribute name = "System";     // String
```

**Operators:**
```sysml
attribute sum = 10 + 5;        // Arithmetic
attribute product = 3 * 7;
attribute comparison = x > 10; // Relational
attribute logic = flag and true; // Boolean
```

**Feature References:**
```sysml
part def Wheel {
    attribute diameter = 16.0;
}

part Vehicle {
    part wheel : Wheel;
    attribute wheelDiameter = wheel.diameter; // Feature chain
}
```

## Composite structures

```sysml
private import ScalarValues::*;

part def Engine {
    attribute power : Real default = 200.0;
}

part def Car {
    part engine : Engine {
        :>> power = 250.0;  // Redefine nested feature
    }
}
```

Instantiate and inspect:
```sysml
sysml> %instantiate Car
✓ Created instance of Car
  ID: 1
  Use %features Car to inspect

sysml> %features Car
Instance: Car (ID: 1)
Features:
  engine = Instance(ID: 2)
    power = 250.0
```

The nested engine is an object of its own. Reach it by a path from the object that holds it, or
by the id it was given, and read a value from it the same way:
```sysml
sysml> %features Car.engine
Instance: Car.engine (ID: 2)
Features:
  power = 250.0

sysml> %eval in #2 : power * 2
✓ power * 2 (on #2 ID: 2)
  = 500.0
```

An element of a multi-valued part is picked by index counted from 1, `System.wheels[3]`; see
[addressing an object](04-repl.md#addressing-an-object).

## Multiplicity

```sysml
part System {
    part sensors : Sensor[0..10];  // 0 to 10 sensors
    part wheels : Wheel[4];         // Exactly 4 wheels
}
```

## Casts, the unbounded value and metadata

**Casts:** `x as T` selects rather than converts. It yields `x` where `T` classifies the value `x`
is, and the empty sequence where it does not; a sequence is cast element by element, keeping the
elements `T` classifies in their order. A whole Real *is* an Integer in the `ScalarValues`
hierarchy, so `4.0 as Integer` keeps `4.0` (the conversions, `ToInteger` and its kin, are library
functions). An object is kept by every classifier it is an instance of, its type's generalizations
included.

```sysml
sysml> package Payload {
  ...>     private import ScalarValues::*;
  ...>     part def Instrument;
  ...>     part def Camera :> Instrument;
  ...>     part navCam : Camera;
  ...>     part probe : Instrument;
  ...>     ref part cameras : Camera[0..*] = (navCam, probe) as Camera;
  ...>     attribute whole : Integer[0..*] = (1.0, 2.5, 3.0) as Integer;
  ...> }
✓ package Payload

sysml> %eval Payload::whole
✓ Payload::whole
  = [1.0, 3.0]

sysml> %eval Payload::cameras
✓ Payload::cameras
  = [Instance(ID: 1)]

sysml> %eval 2.5 as ScalarValues::Integer
✓ 2.5 as ScalarValues::Integer
  = []
```

Declare a feature that holds a cast result `[0..1]` or `[0..*]`: a cast that selects nothing is
empty, which a feature of multiplicity `[1]` cannot hold. The checker warns where a cast can only
be empty because the operand's type and the target are unrelated.

**The unbounded value:** `*` is a value of its own, not a large number. It exceeds every finite
number, equals itself and prints as `*`; arithmetic over it is refused with an error naming the
operator.

```sysml
sysml> package Budget {
  ...>     private import ScalarValues::*;
  ...>     attribute passLimit : Natural = *;
  ...>     attribute withinLimit : Boolean = 40 < passLimit;
  ...> }
✓ package Budget

sysml> %eval Budget::withinLimit
✓ Budget::withinLimit
  = true

sysml> %eval * + 1
error: evaluation failed: type mismatch: operator '+' is not defined for the unbounded value '*': * + 1
```

**Metadata:** `elem.metadata` is the sequence of metadata annotating `elem`, one object per
annotation in the order written, each carrying the values its body binds over the defaults its
`metadata def` declares. An element with no annotation answers the empty sequence. The library
types the sequence as `Metaobject`, so cast an annotation to its `metadata def` before reading
the values it binds.

```sysml
sysml> package Provenance {
  ...>     private import ScalarValues::*;
  ...>     metadata def Heritage { attribute mission : String; attribute flown : Boolean default true; }
  ...>     part def Camera;
  ...>     part navCam : Camera { @Heritage { mission = "Cassini"; } }
  ...>     part sciCam : Camera;
  ...> }
✓ package Provenance

sysml> %eval (Provenance::navCam.metadata#(1) as Provenance::Heritage).mission
✓ (Provenance::navCam.metadata#(1) as Provenance::Heritage).mission
  = "Cassini"

sysml> %eval (Provenance::navCam.metadata#(1) as Provenance::Heritage).flown
✓ (Provenance::navCam.metadata#(1) as Provenance::Heritage).flown
  = true

sysml> %eval Provenance::sciCam.metadata
✓ Provenance::sciCam.metadata
  = []
```

## Sets and tensors

**Sets:** the library declares the elements of a `Collections::Set` unique and unordered, so a
`Set` holds a set: the elements it was given with every repeat dropped and no order of its own.
Two sets holding the same elements are equal however they were written, and a set prints as
`Set{…}` in a canonical order. `Bag` and `OrderedSet` keep their repeats or their order, and are
sequences.

```sysml
sysml> package Bands {
  ...>     private import ScalarValues::*;
  ...>     private import Collections::*;
  ...>     private import CollectionFunctions::*;
  ...>     attribute requested : Set { :>> elements = ("X", "Ka", "X", "S"); }
  ...>     attribute licensed : Set { :>> elements = ("S", "X", "Ka"); }
  ...>     attribute same : Boolean = requested == licensed;
  ...> }
✓ package Bands

sysml> %instantiate Bands::requested
✓ Created instance of Bands::requested
  ID: 1
  Use %features Bands::requested to inspect

sysml> %features Bands::requested
Instance: Bands::requested (ID: 1)
Features:
  elements = Set{"Ka", "S", "X"}

sysml> %eval Bands::same
✓ Bands::same
  = true

sysml> %eval CollectionFunctions::size(Bands::requested)
✓ CollectionFunctions::size(Bands::requested)
  = 3
```

**Tensors:** a `TensorQuantityValue` of any rank is built by `TensorCalculations::'['` from a flat
sequence of numbers and a `TensorMeasurementReference` whose `dimensions` give the shape, in
row-major order with the last index varying fastest. `#` takes one index per dimension, counted
from 1, and refuses an index outside the shape or the wrong number of them; `+`, `-` and the
scalar multiplications keep the shape component by component.

```sysml
sysml> package Stress {
  ...>     private import ScalarValues::*;
  ...>     private import ISQ::*;
  ...>     private import SI::*;
  ...>     private import Quantities::*;
  ...>     private import MeasurementReferences::*;
  ...>     private import TensorCalculations::*;
  ...>     attribute ref3 : TensorMeasurementReference {
  ...>         :>> dimensions = (2, 2, 2);
  ...>         :>> mRefs = (Pa, Pa, Pa, Pa, Pa, Pa, Pa, Pa);
  ...>     }
  ...>     attribute field : TensorQuantityValue = '['((1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0), ref3);
  ...>     attribute cell = field#(2, 1, 1);
  ...>     attribute rank = field.order;
  ...> }
✓ package Stress

sysml> %eval Stress::field
✓ Stress::field
  = Tensor(2, 2, 2)[1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0] [Pa]

sysml> %eval Stress::cell
✓ Stress::cell
  = 5.0 [Pa]

sysml> %eval Stress::rank
✓ Stress::rank
  = 3
```

## Calculations, constraints and requirements

**Calculations:**
```sysml
sysml> calc distance {
  ...>     in x;
  ...>     in y;
  ...>     x * x + y * y
  ...> }
✓ calc distance

sysml> %calc distance 3 4
✓ distance(3, 4)
  = 25
```

**Library functions:**

The KerML function libraries (`RealFunctions::sqrt`, `SequenceFunctions::size`,
`NumericalFunctions::sum`, …) are ordinary library packages, and an expression reaches one of
their functions by the same rule the checker applies to every name: the qualified name resolves
anywhere, and the bare name resolves only where the model imports the package that declares it.
Evaluation follows the checker, so a call the checker reports as an unresolved reference does not
evaluate either; the error names the qualified spellings the call may have meant, and importing
one of those packages makes it resolve.

```sysml
sysml> package Demo {
  ...>     attribute wheels : ScalarValues::Integer[*] = (1, 2, 3, 4);
  ...>     attribute wheelCount = wheels->size();
  ...> }
3:36: error: unresolved reference: size — did you mean SequenceFunctions::size or CollectionFunctions::size?
    attribute wheelCount = wheels->size();
                                   ^~~~

sysml> %eval Demo::wheelCount
error: evaluation failed: unresolved reference: size — did you mean SequenceFunctions::size or CollectionFunctions::size?

sysml> %eval SequenceFunctions::size(Demo::wheels)
✓ SequenceFunctions::size(Demo::wheels)
  = 4

sysml> package Demo {
  ...>     private import SequenceFunctions::*;
  ...>     attribute wheels : ScalarValues::Integer[*] = (1, 2, 3, 4);
  ...>     attribute wheelCount = wheels->size();
  ...> }
✓ package Demo
note: added to the existing package Demo, replacing attribute wheels, attribute wheelCount

sysml> %eval Demo::wheelCount
✓ Demo::wheelCount
  = 4
```

A `calc` the model declares under a library function's name is what a call resolves to, even where
the library is also imported. `%builtins` lists every function the build evaluates, each with the
package an `import` must name for its bare name to resolve.

**Calculations as values:**

A `calc def`, a `calc` usage or an `in calc` parameter named where a value is expected is a
*function value*: the calculation, together with whatever it closes over. It is passed as an
argument, held in a feature, compared with `==`, and invoked by the parameter that receives it;
reading it on its own answers the function, named by its declaration.

```sysml
sysml> package Gains {
  ...>     private import ScalarValues::*;
  ...>     calc def Square { in v : Real; return : Real = v * v; }
  ...>     calc def Apply { in calc f { in v : Real; return : Real; } in a : Real; return : Real = f(a); }
  ...> }
✓ package Gains

sysml> %calc Gains::Apply(Gains::Square, 3.0)
✓ Gains::Apply(Gains::Square, 3.0)
  = 9.0

sysml> %eval Gains::Square
✓ Gains::Square
  = Gains::Square
```

A `calc def` is a definition, not a feature, so it is passed as an argument or referenced through
a `calc` usage rather than bound directly as a feature's value. A nested `calc` closes over the
features around it, and `SampledFunctions::Sample` from the analysis library takes a function value
and tabulates it over a domain.

**Collection bodies:**

`collect`, `select` and `reduce` (`ControlFunctions`) take a body whose parameter is bound to each
element in turn. A `collect` is typed by what its body returns, not by the element type of the
collection it ran over, so its result can be declared with the body's type and a mismatch is
reported before anything runs.

```sysml
sysml> package Rollup {
  ...>     private import ScalarValues::*;
  ...>     private import ISQ::*;
  ...>     private import SI::*;
  ...>     private import ControlFunctions::*;
  ...>     part def Instrument { attribute mass : MassValue; }
  ...>     part navCam : Instrument { :>> mass = 4.0 [kg]; }
  ...>     part spectrometer : Instrument { :>> mass = 12.0 [kg]; }
  ...>     attribute masses : MassValue[0..*] = (navCam, spectrometer)->collect { in i : Instrument; i.mass };
  ...>     attribute total : MassValue = masses->reduce { in a : MassValue; in b : MassValue; a + b };
  ...> }
✓ package Rollup

sysml> %eval Rollup::total
✓ Rollup::total
  = 16.0 [kg]

sysml> package Rollup {
  ...>     attribute names : String[0..*] = (navCam, spectrometer)->collect { in i : Instrument; i.mass };
  ...> }
1:35: error: cannot bind a value of type MassValue to a feature typed by String
	attribute names : String[0..*] = (navCam, spectrometer)->collect { in i : Instrument; i.mass };
                                  ^~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
note: added to the existing package Rollup (its other members are kept)
```

**Constraints:**
```sysml
sysml> constraint ValidSpeed {
  ...>     65 > 0 and 65 <= 120
  ...> }
✓ constraint ValidSpeed

sysml> %constraint ValidSpeed
✓ Constraint ValidSpeed passed
```

**Requirements:**
```sysml
sysml> requirement SafetyReq {
  ...>     assume constraint { 65 > 0 }
  ...>     require constraint { 100 > 50 }
  ...> }
✓ requirement SafetyReq

sysml> %requirement SafetyReq
✓ Requirement SafetyReq satisfied
```

For more examples, see
[examples/repl-behavioral-demo.sysml](../../examples/repl-behavioral-demo.sysml), and the
[expressions demo](../../examples/EXPRESSIONS-DEMO.md) for casts, `*`, `.metadata`, function
values, sets, tensors and collection bodies worked through one model.

---

Next: [6. Behavior: actions and state machines](06-behavior.md).
