### KerML Function Library (KerML §9.4 Function Library)

The library declares these functions abstractly — a signature and no body — so
the runtime supplies the implementation. Dispatch is by the declaration a call
resolves to, and only a declaration the library documents make is answered by
an implementation, so a model's own `calc sqrt` — under `RealFunctions` or
anywhere else, with a body or without one — is the model's: evaluated from its
body, or `ErrNoValue` (its result is never bound). A call resolves as the checker resolves it: an unqualified `sqrt(2.0)` denotes
`RealFunctions::sqrt` only where the model imports it, and a qualified name
reaches the library whatever the model imports.

Arguments follow the vendored signatures: a `Real` parameter accepts an Integer
(`ScalarValues` declares `Integer :> Rational :> Real`), an `Integer` parameter
rejects a Real rather than truncating it, and a `Natural` parameter rejects a
negative value. A result that is not a finite value of the declared type — the
square root of a negative, an inverse sine outside `[-1.0, 1.0]`, a `floor` of
an infinity — is reported at evaluation rather than returned as a
NaN, an infinity or a wrapped integer. That is a tool decision, not the clause: KerML 1.1
Beta 2 §9.3.2.2.9 defines `Real` as the extended reals, ±∞ included, while this implementation
holds a `Real` to finite IEEE 754 binary64 values — `*` is its only unbounded value, in
multiplicity and `LiteralInfinity` positions — so a result the clause would admit as an
infinity is a typed error here (⚠️ Approximate; the `Real` row below). An Integer result has
no bound but the Integer size budget (`OPENSYSML_MAX_INTEGER_BITS`).

Beyond the scalars, a **Complex** is one scalar value of its own kind
(`runtime/value.go` `ValComplex`, a `complex128`), which only
`ComplexFunctions::rect`, `polar`, `i` and the Complex operations construct: a
numeric pair `(re, im)` is a two-element sequence, never read as a Complex. A
Real is a Complex with a zero imaginary part (`ScalarValues` declares `Real :>
Complex`), so a number binds to a Complex parameter, and a Complex on the real
axis equals the Real it is and classifies as that Real does in the scalar
lattice (`complexPrimType`: `rect(2.0, 0.0)` is an Integer, `rect(2.5, 0.0)` a
Real, `rect(0.0, 1.0)` a Complex), while it stays one value however it is typed.
A collection of Complex values is a sequence of them, so
`ComplexFunctions::sum`/`product` fold it, keeping Real elements Real as the
library's `reduce '+'` does (`aggregateComplex`; fixture `calc_library_complex_sum_real_axis`).

**Structured values** — an Array, a vector and a vector quantity — are three
value kinds of their own (`runtime/array.go`; `runtime/value.go` `ValArray`,
`ValVector`, `ValVectorQuantity`), each one value however many elements it
holds, so that a sequence of them keeps its grouping and none is mistaken for
the flat sequence of its elements:

- **`ValArray`** carries `Dimensions []int64` and the `Elements []Value` in
  row-major order — the last index varies fastest, as `Collections.kerml`
  documents `Array::elements` — and is what a `Collections::Array`, and any
  specialization of it a model declares (`attribute def Matrix :> Array`),
  evaluates to. The OMG library declares only `Array` itself: no `Matrix`,
  `Tensor` or `*Array` family ships, and a model that wants one specializes
  `Array` ([handbook](../guide/05-checking.md#arrays-sets-and-tensors)). Its
  derived features read out of the value: `rank` is the number of dimensions,
  `flattenedSize` their product, `dimensions` and `elements` the sequences
  carried; a rank-0 array holds one element. It prints as `Array(2, 3)[1, 2, 3,
  4, 5, 6]`, dimensions first, so it is never read as the six-element sequence.
  A model declares one by shaping the usage's own features — `attribute a :
  Array { :>> dimensions = (2, 3); :>> elements = (1, 2, 3, 4, 5, 6); }` — and
  the object that usage names *is* the Array value wherever it is read
  (`arrayOfObject`, `objectValue`, `EvalDeclaredValue`): as an operand, through
  a feature chain (`a.rank`), and at the REPL (`%eval a`). The value keeps the
  object it was read from (`Array.Object`), so its direct type stays the
  specialization (`LabeledGrid`, not `Array`, for write conformance) and a
  member the Array does not declare (`grid.label` on an `attribute def
  LabeledGrid :> Array`) is read from that object (`chainMemberValue`) whether
  the value is named directly or came back from a calc, a parameter or another
  attribute; equality and hashing compare dimensions and elements only. A usage
  stating `dimensions` or `elements` — even as `()` — is judged an Array: one
  whose `elements` does not fill its `dimensions` (`dimensions = ();
  elements = ();`, whose `flattenedSize` is 1) is `ErrMultiplicityViolation`
  naming both counts, and dimensions whose product, or an index offset, is beyond `int64` — an Integer
  still, but one that addresses no element — are `errIntegerUnaddressable`
  rather than a wrapped size. The shape is the pinned pilot evaluator's: asked about
  that usage, `eval-sysml` answers `grid` itself as the unevaluated
  `AttributeUsage` (it prints no Array value), `grid.rank` as `2` and
  `CollectionFunctions::'array#'(grid, (2, 1))` as `4` — row-major, one-based —
  which is what this runtime answers.
- **`ValVector`** carries `Elements []semantics.Value`, the numeric components,
  and is what `VectorFunctions::VectorOf`, `CartesianVectorOf`,
  `CartesianThreeVectorOf` and every vector operation return. `dimension` is
  its number of elements (the one dimension the `NumericalVectorValue :> Array`
  specialization declares), `elements` the sequence of them. An object of a
  `NumericalVectorValue` specialization (`attribute t : Tagged { :>> dimension =
  2; :>> elements = (1, 2); }`) reads as the vector its elements shape and keeps
  the object (`Vector.Object`, as `Array.Object` does), so the specialization's
  own members answer on it — directly, through a calc parameter, and on a calc's
  result (`t.tag`, `tagOf(t)`, `pass(t).tag`) — while `dimension`, `dimensions`,
  `elements`, `rank` and `flattenedSize` answer as the member of the object's type
  declares them, by the `Collections::Array` feature it redefines rather than by
  name (`arrayFeatureNamed`, `structuredMember`); a specialization whose
  dimension the library fixes (`ThreeVectorValue::dimension = 3`) takes it
  without restating it (`fixedDimensions`), and an object whose elements do not
  fill that shape, or whose `dimensions` state a rank above one, is
  `ErrMultiplicityViolation` rather than a vector
  (`vectorOfObject`; conformance `calc_library_vector_specialization_members`,
  `calc_library_vector_specialization_fixed_dimension`,
  `TestVectorObjectMemberReadsFollowTheDeclaration`,
  `TestAbandonedVectorObjectIsForgottenByItsHolders`). The object behind an
  array or vector a run wrote is carried into a re-analysis with its holder
  (`adopt.go` `carriedObject`, `walkValue`, `rewrite`;
  `TestAdoptCarriesTheObjectsBehindWrittenArraysAndVectors`), so those members
  still answer after `%load` re-reads the model. A vector built by value
  (`attribute w : Tagged = VectorOf((1, 2));`) has no object and so no members
  beyond the vector's own: `w.tag` is `ErrTypeMismatch`. It prints between
  angle brackets, `⟨1, 2, 3⟩`, distinct from a sequence's `[1, 2, 3]`; the pilot
  prints no vector at all (its `VectorOf` has no evaluable body), so the
  notation is a decision recorded here. It conforms to `VectorValue`,
  `NumericalVectorValue`, `CartesianVectorValue`, and to
  `CartesianThreeVectorValue` when its dimension is 3 (`structuredValueType`,
  `write_conformance.go`), so it binds to a feature typed by any of these and
  `CartesianThreeVectorOf` with two components is `ErrMultiplicityViolation`.
  A structured value written to a feature typed by any other *specialization*
  of its kind's base type — `Collections::Array` for an Array (the type of the
  object it was read from, for one that keeps its own members),
  `NumericalVectorValue` for a vector, `VectorQuantityValue` for a vector
  quantity (`structuredBaseType`) — is admitted only when the shape that type
  fixes holds: a `dimension`, `dimensions`, `rank` or `flattenedSize` it
  redefines to a constant, the multiplicity of its `dimensions`, and the type
  its `elements` are declared of (`structuredConforms`, `shapeRefusal`,
  `elementsConform`). So a two-component vector does not bind to
  `CartesianThreeVectorValue` or `ThreeVectorValue`, nor a 2×3 Array to a type
  fixing `dimensions = (2, 2)`, and the refusal names the declaration it fails
  (`it declares dimension = 3`); while `VectorOf((1, 2, 3))` binds to a model's
  own `attribute def IntegerVector :> NumericalVectorValue { :>> elements :
  Integer; }` — a sibling of the Cartesian types, not a specialization — and
  `VectorOf((1, 2.5, 3))` does not
  (`calc_library_vector_custom_specialization*` conformance,
  `testStructuredValueOutsideTheDeclaredShape`).
  A sequence of numbers written where a vector parameter is declared
  (`inner((1, 2), (3, 4))`) is still read as a vector — the pilot's
  `VectorOf((1,2,3))` argument form — while a sequence given several vectors'
  worth of numbers is not silently split.
- **`ValVectorQuantity`** carries `Num []semantics.Value` and one `Unit` per
  component, and is what a scalar quantity times a vector is
  (`2 [m] * VectorOf((1.0, 2.0))`, `VectorCalculations::scalarQuantityVectorMult`,
  `vectorScalarQuantityMult`, `vectorScalarQuantityDiv`, and the `*`/`/`
  operators). Its `num` is the vector of magnitudes; the unit composes through
  the scalar quantity's own canonicalization (`quantity.go` `scaleQuantities`,
  `composedQuantity`: `[m] * [m]` is `[SI::'m²']`, `[m] / [s]` is `[SI::'m/s']`) rather
  than a second rule. It prints as the vector followed by the unit, `⟨2.0, 4.0⟩ [m]`,
  and conforms to `Quantities::VectorQuantityValue`. Its `inner`, `norm` and
  `angle` are the `Number` the library declares them — the magnitude computed
  over the `num` components, the unit **dropped by declaration** (`return :
  Number[1]`, where `QuantityCalculations::'*'`/`'/'`/`sqrt` keep a
  `ScalarQuantityValue`; the inconsistency is drafted in
  [omg-issues.md](omg-issues.md)) — so a `Number` feature takes them and the
  checker and runtime agree (conformance `calc_library_vector_quantity_norm`).
  `CartesianVectorOf` takes `Real[*]`, but a quantity's `num`
  is `Number[1..*]` (`Quantities::TensorQuantityValue`) and the unit lives on an
  axis, so a vector of no components takes no unit: `CartesianVectorOf(()) [m]`,
  and scaling the empty vector by a scalar quantity, are `ErrMultiplicityViolation`
  (`vectorQuantityValue`, `scaleVectorQuantity`; conformance
  `calc_library_vector_quantity_empty`) rather than a unitless empty value. Its `mRef`
  answers the one measurement reference every axis shares (`(2 [m] * VectorOf((1.0, 2.0))).mRef`
  is `m`); a vector written over a coordinate frame (`(1.0, 2.0, 3.0) [spatialCF]`,
  `VectorCalculations::'['`) carries the frame and answers **it** as `mRef`
  (`p.mRef == spatialCF`; `ValCoordinateFrame`, below), so axes in *different*
  units are answered by the frame that declares them; a vector whose axes differ
  and whose frame is not held — the components of a `transform` result are
  always in a frame, so only a value the wire or a client assembled — is
  `errUnevaluableLibraryFunction` naming `VectorQuantityValue::mRef`, never a
  unit dressed up as a frame.
- **`ValMeasurementRef`** (`measurement_ref.go`) is a **measurement reference as
  a value**: the unit a quantity is measured in, on its own. It carries the same
  `semantics.Unit` a quantity does — the spelling written, the `UnitProduct`
  naming the declarations it is composed of, and the `UnitTerm` reduction to a
  scale over base units — so `SI::m`, `SI::'m/s'` and `MeasurementReferences::one`
  each evaluate to one (`Context.MeasurementUnitValue`; the REPL evaluates a
  unit's name the same way), `m * s`, `m / s` and `m ** 2` compose one
  (`composeMeasurementRefs`, the `UnitProduct` composing and the term reducing
  exactly as `[m] * [s]` does for a quantity), and it prints as the unit a
  quantity prints after its magnitude: `m`, `'m/s'`, `m*s`, `m**2`. Two
  references are equal, and hash alike, when they reduce to the same term
  (`SI::'m/s' == m / s`, `km != m`, `km / m == m / mm`); a reference naming a
  unit of dimension one is only itself (`rad != sr`, `one != m / m`), as a
  quantity in it stays a quantity where a cancelled ratio is a number. It is typed by the unit
  definition declaring it (`m` is a `LengthUnit`, hence a `MeasurementUnit`,
  `ScalarMeasurementReference`, `TensorMeasurementReference`); a composed unit is
  a `DerivedUnit` — the library's "unit that depends on powers of other
  measurement units" — so it binds to `DerivedUnit` (`kpl : DerivedUnit = km / L`
  in the OMG vehicle example) and to a unit definition of its dimension (`m * m`
  to `AreaUnit`, `km / h` to `SpeedUnit`; `Model.MeasurementRefConforms`, judged
  the same by the checker — `judgedAsMeasurementRef`,
  `MeasurementRefExprConformance` — and the runtime's write conformance). A
  unit raised to a Real the checker cannot fold (`m ** e`, `e : Real`) is a
  `DerivedUnit` whose dimension only the runtime knows, so the checker binds it
  to any unit definition and the runtime judges the written value (`m ** 2.0` to
  `LengthUnit` is refused at the write; `Model.MeasurementRefExprType`,
  `TestMeasurementUnitPowerOfAnUnknownExponent`); an untyped feature valued by
  a unit expression (`attribute area = m * m;`) is typed by it
  (`Model.ExprResultType`, `MeasurementRefFeatureType`), so `ToString(area)`
  selects `MeasurementRefCalculations::ToString` and `ConvertQuantity(q, area)`
  is accepted, by the checker and the runtime alike
  (`TestInferredMeasurementUnitFeature`, `TestMeasurementRefValues`). A
  reference of another dimension (`m / s` to `LengthUnit`) or a number written to
  a unit-typed feature, a reference written to a quantity-typed one (`m` to
  `LengthValue`, `m * m` to `AreaValue` — the checker judges a composed unit as
  the `DerivedUnit` it is before it compares dimensions, which alone would pass
  it), and a unit written to a feature typed by a measurement *scale* (`s`, or
  `h * s / min`, to `Time::TimeScale` or `IntervalScale`: the dimensional
  fallback is for unit-definition targets only, `Model.IsMeasurementUnit`, a
  scale of the same dimension being another kind of reference) are refused by
  both. A measurement scale itself — `Time::UTC`, `SI::'°C_abs'`, a
  `ScalarMeasurementReference` that is not a `MeasurementUnit`
  (`Model.IsMeasurementScale`) — is a **`ValCoordinateFrame`** of one axis
  (below): `MeasurementReferences::IntervalScale :> MeasurementScale,
  CoordinateFrame`, so the value carries the scale's `unit`, its `transformation`
  placing it on another reference and its `quantityValueMapping`, and
  `Time::UTC`, `3 [Time::UTC]`, `UTC.unit` and `ConvertQuantity(300.0 [K],
  SI::'°C_abs')` evaluate; the checker binds one to its declared type
  (`t : Time::TimeScale = Time::UTC`) and refuses it elsewhere (`SI::'°C_abs'`
  to `ThermodynamicTemperatureUnit`), as the runtime does.
  A quantity written to a unit-typed feature
  (`hp : PowerUnit = 745.7 [W]` in the OMG individuals example) is not judged,
  as the pilot implementation accepts it; the feature then holds the quantity,
  and `200 [hp]` is `errNotAQuantity`, `hp` naming no unit. A quantity's `num` and `mRef`
  read (`q.mRef == SI::m`), and the quantity functions the library declares over
  a reference compute: `QuantityCalculations::'['(3.0, m)` is `3.0 [m]`,
  `ConvertQuantity(3 [km], m)` is `3000.0 [m]` (`Quantity.ConvertTo`, through
  the two reductions; `ConvertQuantity(3 [m], s)` is `ErrIncommensurableUnits`),
  `MeasurementRefCalculations::'*'`, `'/'`, `'**'`, `'^'` and `ToString`
  (`"m/s"`); a vector quantity's `mRef` reads when its axes share one unit
  (`VectorOf((1.0, 2.0)) [m]` answers `m`) or when it was written over a frame
  (`((1.0, 2.0, 3.0) [spatialCF]).mRef` answers `spatialCF`). Arithmetic
  the library does not declare — `m * 3`, `m + m`, `-m`, `m < s`,
  `m ** m` — is `ErrTypeMismatch`, not a quantity of magnitude 1. What the value
  itself fixes (`isBound`, `mRefs`, `isOrthogonal`, `dimensions`, `rank`, `order`) it
  answers directly; a reference **naming a declaration** (`UnitProduct` identity:
  `SI::km`, a model's own `furlong`) answers the declaration's other members from the
  object that declaration materializes as, the one `%features SI::km` shows
  (`measurementRefFeature`, `declarationMember` over `occurrenceOf`), with the library's
  redefinitions and defaults followed: `km.unitConversion.conversionFactor` is `1000.0`
  (`ConversionByPrefix::conversionFactor = prefix.conversionFactor`),
  `km.unitConversion.referenceUnit` is `m` — a reference, so
  `ConvertQuantity(3 [km], km.unitConversion.referenceUnit)` is `3000.0 [m]` —
  `km.unitConversion.isExact` is `true` (`UnitConversion::isExact default true`),
  `m.quantityDimension.quantityPowerFactors#(1).exponent` is `1`,
  `m.unitPowerFactors#(1).unit` is `m` (`SimpleUnit`'s `unit = self`, the object read as
  the reference it declares), `K.definitionalQuantityValues#(1).num` is `[273.16]` (`DefinitionalQuantityValue::num` is `Number[1..*]`), and a
  base unit's `unitConversion` is the empty `[0..1]`. A unit **composed** at runtime
  (`m / s`, `m ** 2`; a `DerivedUnit`) names no declaration, so `unitConversion`,
  `quantityDimension`, `unitPowerFactors` and `definitionalQuantityValues` of one are
  `errUnevaluableLibraryFunction` naming `MeasurementReferences::DerivedUnit` and the
  reduction (`TestMeasurementRefDeclarationMembers`, `TestMeasurementUnitDeclarationMembers`
  — the checker types the same chains by the library's records; conformance
  `instance_measurement_reference_members`, `instance_measurement_reference_failures`;
  `repl/measurement_ref_test.go`). Which library attribute definitions are held **as
  values** and which materialize **as objects** is decided by specialization, in one place
  (`semantics/shape.go` `Model.ValueHeld`, read by `FrameFeature` through `HeldByValue`): a
  scalar, an enumeration, a `Quantities::TensorQuantityValue` or a
  `MeasurementReferences::TensorMeasurementReference` — so `IntervalScale`, `TimeScale` and
  `CoordinateFrame`, which descend from `VectorMeasurementReference`, stay values — is held
  as a value and its own members (`num`, `mRef`, `mRefs`, `isBound` stated by the value)
  stay out of an object's shape; every other one (`UnitConversion`, `ConversionByPrefix`,
  `UnitPrefix`, `QuantityDimension`, `QuantityPowerFactor`, `UnitPowerFactor`,
  `DefinitionalQuantityValue`, `QuantityValueMapping`, `SystemOfQuantities`) is a record
  materialized as an object with its library members, so a model's own
  `attribute myConv : ConversionByPrefix { :>> prefix = kilo; :>> referenceUnit = m; }`
  lists and answers `conversionFactor = 1000.0`, and a model's own
  `TensorMeasurementReference` usage that does not restate `isBound` answers the inherited
  `default false` (`TestValueHeldIsDecidedBySpecialization`,
  `TestHeldByValueKeepsTheValuesOwnMembersOutOfTheShape`,
  `repl/measurement_ref_test.go:TestFeaturesOfModelOwnedLibraryRecords`). A base quantity
  the library states without a magnitude (`QuantityPowerFactor::quantity = isq.L`) reads
  as the valueless value it is (`<unset>`), as any valueless feature of a value type does. A
  reference embedded in a written value is rebound across re-analysis like the
  objects behind an array are (`adopt.go` `unitsOf`, `planUnit`, `rewriteUnit`: a
  model's own `furlong` follows the re-read declaration, in the product and in
  the reduction alike, so a base unit the model declares without a conversion —
  which reduces to itself — is carried over and still equals a fresh `chain`
  (`TestAdoptRebindsAModelsOwnBaseUnit`); a unit the new model no longer
  declares, or reduces differently — `furlong` re-declared as `220 m` — refuses
  adoption naming the unit and both reductions, since a magnitude written under
  the old factor would read as another quantity under the new;
  `TestAdoptRefusesAUnitWhoseReductionChanged`; the unit an empty quantity
  collection remembers for the zero its sum yields is judged the same way,
  `TestAdoptRebindsTheUnitOfAnEmptyQuantitySequence`); the solver refuses to pin one
  (`ErrNotPinnable`), a document query binds a single unit but not an operator
  over units, and over gRPC and Connect it crosses whole as the `measurement_ref`
  arm of the wire `Value` — the unit as written, its `UnitTerm` reduction, and
  the canonical id of the declaration a named unit is (`SI::metre` for `m` and
  for the alias `SI::m`), which a composed unit (`m / s`, a `DerivedUnit` naming
  no one declaration) omits — and comes back as the same `ValMeasurementRef`, a
  named one resolved against the model's own declaration and refused when the
  id names nothing, names no unit, or the reduction sent disagrees with the
  declaration's own (`grpc/convert.go` `MeasurementRefToProto`,
  `ProtoToMeasurementRef`, `declaredMeasurementRef`;
  `convert_measurement_ref_test.go` `TestMeasurementRefRoundTrip`,
  `TestMalformedMeasurementRefsAreRejected`, `TestMeasurementRefCrossesEveryValueSurface`).
  The arm is advertised as the `measurement_refs` capability, on its own rather
  than under `structured_values`: a client built against the three structured
  arms keeps reading a bare reference as the unsupported null it read before
  (`unsupported: measurement reference m`), and a service withholding the
  capability refuses one sent to it with `UNIMPLEMENTED`
  (`capability_response.go`; `TestMeasurementRefCapability`, `TestValueCarriesMeasurementRef`).
- **`ValTensorQuantity`** (`tensor.go`, `tensor_functions.go`) is a **tensor
  quantity**: the rank-*n* generalization of a vector quantity, and the value
  `Quantities::TensorQuantityValue :> Array` describes — `dimensions` in row-major
  order as an `Array`'s, one magnitude per component (`num`), one measurement
  reference per component (the `mRefs` of its `TensorMeasurementReference`), and
  `isBound`. `TensorCalculations::'['(elements, mRef)` builds one over a
  `TensorMeasurementReference` the model declares with `:>> dimensions` and
  `:>> mRefs` (a 2×2 stress tensor over `(Pa, Pa, Pa, Pa)`, a 3×3 inertia tensor
  over `kg*m**2`): the elements must number `mRef.flattenedSize`
  (`ErrMultiplicityViolation` naming the count, the dimensions and the size
  otherwise), each pairs with the reference at its row-major position, and one
  reference does **not** broadcast to four components — `mRefs` is declared
  `ScalarMeasurementReference[1..*] nonunique :>> elements`, so as `Array::elements`
  it must fill the dimensions, and a one-element `mRefs` over `(2, 2)` is the
  `Array` shape error. It prints as its shape and row-major components with the
  unit they share (`Tensor(2, 2)[1.0, 2.0, 3.0, 4.0] [Pa]`) or with each
  component's own (`Tensor(2, 2)[1.0 [Pa], 2.0 [kPa], …]`), and conforms to
  `TensorQuantityValue` and `Collections::Array` and to no scalar or vector type.
  Its `dimensions`, `rank`/`order`, `flattenedSize`, `elements`, `num`, `isBound`
  and `mRef` read (`mRef` answers the model's reference usage when the value was
  built over one, and otherwise the array of the components' units); `#` and
  `array#` index it exactly as an `Array` — one index per dimension, the last
  varying fastest, rank and range errors typed (`componentArray`, reusing
  `Array.at`). `contravariantOrder` and `covariantOrder` are never fabricated: the
  library gives them no default, `'['` sets neither, and only their sum is
  constrained (`orderSum`), so reading one is `errUnevaluableLibraryFunction`
  quoting that constraint; `isBound` of a tensor a bodiless calculation produced is
  the same, since the declaration says nothing about its result's boundness.
  `'+'`/`'-'` (and the operators) are componentwise over two tensors of one shape
  — the right component converted into the left's unit, `ErrIncommensurableUnits`
  when it cannot be, `ErrMultiplicityViolation` naming both shapes when the
  dimensions differ — `scalarTensorMult`/`TensorScalarMult` scale every
  component by a `Number`, `scalarQuantityTensorMult`/`TensorScalarQuantityMult`
  by a scalar quantity whose unit composes with each component's by the scalar
  rule (`2 [m] * Tensor(2, 2)[…] [Pa]` is in `Pa*m`); `isZeroTensorQuantity` holds
  when every magnitude is zero, and `isUnitTensorQuantity` decides a square
  order-two tensor against the identity and is `errUnevaluableLibraryFunction`
  naming the shape it needs for any other, the library defining no identity there.
  Every one of these accepts a scalar or vector quantity where it declares a
  `TensorQuantityValue` (`ScalarQuantityValue :> VectorQuantityValue :>
  TensorQuantityValue`) and answers a value of the operands' rank: a scalar for
  order 0, a vector quantity for order 1, a tensor above (`tensorResult`). A
  result whose component units all cancel follows the scalar rule above — a
  ratio of like quantities is a number of no unit (`3 [Pa] * 2 [Pa**-1]` is `6`),
  and a vector quantity so reduced is a plain vector (`vectorResult`) — so
  `TensorScalarQuantityMult(Tensor(2, 2)[…] [Pa], 2 [Pa**-1])` is the plain
  `Array(2, 2)[…]`, which, like the number and the vector, the `TensorCalculations`
  functions do not take back as a `TensorQuantityValue`; what a quantity of
  dimension one carries is one decision for every rank and is not made here. What
  remains unevaluable is what the text does not determine: `tensorVectorMult`,
  `vectorTensorMult`, `tensorTensorMult` (no contraction convention),
  `VectorCalculations::outer` (declared to return a `VectorQuantityValue`, which
  an order-two product is not), and `TensorCalculations::transform` (needs a
  coordinate frame); each names itself and the reason. It is carried over by
  `Adopt` with every component's unit rebound (`TestAdoptRebindsATensorsComponentUnits`),
  refused by the solver as a non-scalar and refused as a document-query binding.
  **Rank is unbounded**: a `TensorMeasurementReference` with `:>> dimensions =
  (2, 2, 2)` and eight `mRefs` builds a rank-three tensor, `#` takes exactly
  three indexes (`cube#(2, 1, 2)` is the sixth row-major component), and the
  shape checks are the `Array`'s at every rank — too few or too many indexes
  `ErrMultiplicityViolation` naming the count and the rank, an index below 1 or
  above its dimension `ErrIndexOutOfRange` naming the index and the range, a
  non-Integer index `ErrTypeMismatch`, a component count off `flattenedSize`
  `ErrMultiplicityViolation`, and `'+'`/`'-'` between two shapes
  `ErrMultiplicityViolation` naming both. Arithmetic and printing keep the shape
  (`Tensor(2, 2, 2)[…] [Pa]`), and the trace renders the same text
  (`FormatTraceValue`). It **crosses gRPC whole** as the `tensor_quantity` arm —
  `dimensions` and one `Quantity` per row-major component, advertised as the
  `tensor_values` capability; a rank-one tensor stays a `tensor_quantity`, not a
  `vector_quantity` (*Values* in the gRPC rows below). It is **not compiled
  natively**: `sysml -compile` refuses a calc that declares or builds one with
  `codegen.UnsupportedError` (`type Quantities::TensorQuantityValue is not
  Integer, Real or Boolean`), and it needs **no RDF literal form**, as no value kind has one —
  the mapping writes the model, never an evaluation, so a tensor-valued feature of any
  rank is its `TensorCalculations::'['` invocation and an indexing its `#` tree, standard
  expression shapes that round trip exactly with the source text stripped and whose
  load-bearing structural predicates (`operator`, `function`, `referent`, and
  the operands by `ParameterMembership` or legacy `argument`) carry the round trip
  ([the mapping](../reference/rdf-mapping.md#expressions)).
  Conformance `instance_tensor_quantity`, `instance_tensor_quantity_failures`,
  `instance_tensor_rank_three`, `instance_tensor_rank_three_failures`;
  robustness `tensor_quantity_failure_modes` (every rank-three failure);
  `TestTensorQuantity*`, `TestTensorQuantityRankThree`,
  `TestTensorCalculationResultTypes`, `TestTensorCalculationsBindToTheirDeclaredTypes`;
  `tests/codegen/compiled_calcs_test.go:TestCompileRefusesWhatItCannotCompile` (`TensorParam`,
  `TensorBuilt`); `export/set_tensor_rdf_test.go:TestSetAndTensorValuesRoundTripAsExpressions`,
  `TestSetAndTensorGraphsAreStandardShaped`, `TestSetAndTensorStructuralPredicatesCarryTheRoundTrip`,
  `TestSetOrderAndTensorShapeSurviveTheHop`.
- **`ValCoordinateFrame`** (`coordinate_frame.go`, `frame.go`, `frame_read.go`)
  is a **coordinate frame or a measurement scale as a value**. The OMG corpus
  this project pins populates frames — Annex A's SimpleVehicleModel
  (`attribute spatialCF : CartesianSpatial3dCoordinateFrame[1] { :>> mRefs = (m, m, m); }`,
  `velocityCF = spatialCF/s`), the geometry example's `datum` with
  `(x, y, z)[datum]` literals and a `TranslationRotationSequence`, the
  validation suite's `MissionElapsedTimeScale` — and the library itself places
  `SI::'°C_abs' : IntervalScale` on `K`. A model-owned usage typed
  `CoordinateFrame` (or a subtype) with its `:>> mRefs` evaluates to the frame it
  declares (`referenceValueOfObject`, `readFrame`), the geometry example's
  `datum :>> coordinateFrame { :>> mRefs = (mm, mm, mm); }` included: a renamed
  redefinition whose body describes the value governs over the `default` it
  inherits as a same-named one does (`subsetting.go` `sharedRedefinitionName`,
  `declarationValues`; `TestRenamedRedefinitionBodyGovernsAnInheritedValue`),
  while that example's component frames, whose `mRefs` default to
  `(that.that as SpatialItem).coordinateFrame.mRefs`, stay a typed error: the
  `as` cast is evaluated now (`cast.go` `evalCast`), but `that.that` is not — an
  object features no `that`, so the chain reports itself before the cast sees a
  value. The value
  carries the declaration, its type, the `dimensions` the type fixes (`'3dCoordinateFrame'`
  states `3`; a scalar reference `()`, one axis), one `ValMeasurementRef` per
  axis read from `mRefs` (the flattened size of `dimensions` gates the count: a
  frame stating no `mRefs` is `ErrNoValue` naming
  `TensorMeasurementReference::mRefs[1..*]`, a count off the flattened size
  `ErrMultiplicityViolation`), and the `transformation` the usage states (below).
  A scale is the one-axis case: `IntervalScale :> MeasurementScale, CoordinateFrame`,
  so `SI::'°C_abs'`, `Time::UTC` and a model's `TimeScale` carry their `unit` as
  the one axis, plus the `quantityValueMapping` and the placement on another
  reference when the declaration states them (`readScale`). A frame prints as
  its name followed by its axis units — `spatialCF [m, m, m]`, `velocityCF
  [m/s, m/s, m/s]`, a composed one as the expression that made it, `datum / s
  [mm/s, mm/s, mm/s]` — a scale as its name, `SI::'°C_abs'`; `describe` says
  *coordinate frame* or *measurement scale*; `%features` shows `mRefs`,
  `dimensions`, `transformation`, and for a scale `unit`,
  `transformation.origin` (`structuredFeature`). The object a frame usage
  materializes carries the `MeasurementReferences` and `Collections` members
  that describe it (`semantics/shape.go` `DescribesReference`); the library's
  `transformation { :>> target = that; }` is not one of them, since the runtime
  answers the target of a frame's own transformation as that frame
  (`frame_read.go` `readTarget`), and a `%features` listing of the object still
  reports `flattenedSize` as an error, `dimensions->reduce '*'` naming a reducer
  the runtime does not evaluate (the *Found, not fixed* row below), while
  `frame.flattenedSize` reads from the value. It conforms to its declared
  type and that type's generals (`CartesianSpatial3dCoordinateFrame`,
  `'3dCoordinateFrame'`, `CoordinateFrame`, `VectorMeasurementReference`,
  `TensorMeasurementReference`; a scale to `IntervalScale`, `MeasurementScale`,
  `ScalarMeasurementReference`), judged alike by the checker and the runtime
  (`semantics/coordinate_frame.go`; `passes/typecheck_dimension.go`
  `judgedAsMeasurementRef`; `TestCoordinateFrameExpr`, `TestComposedFrameConforms`,
  `TestBoundComposedCoordinateFrame`). Two frames are equal, and hash alike,
  when they are the same object — the frame a usage declares is one object,
  however many times it is read — or, for a composed frame, when their
  dimensions, axes, scale and transformation agree (`spatialCF / s == velocityCF`;
  `CoordinateFrame.equal`, `key`). Frame arithmetic:
  `MeasurementRefCalculations::'CoordinateFrame*'`/`'CoordinateFrame/'`, and the
  `*`/`/` operators over a frame and a unit, compose every axis with the unit
  (`spatialCF / s` is a frame of axes `m/s, m/s, m/s`, dimensions unchanged),
  typed by the calc's `CoordinateFrame` — so `velocityCF :
  CartesianVelocity3dCoordinateFrame[1] = spatialCF / s` binds, as the pilot's
  checker accepts it; a scale composes with nothing (`'°C_abs' * s` is
  `errUnevaluableLibraryFunction`: its points are on the scale, not in a unit
  to compose). A vector
  literal over a frame, `VectorCalculations::'['((1.0, 2.0, 3.0), spatialCF)`,
  is a `ValVectorQuantity` with the frame's unit per axis and **the frame as
  `mRef`** (`p.mRef == spatialCF`, `p.num` the three numbers); an element count
  off `mRef.flattenedSize` is `ErrMultiplicityViolation`; the frame may be named
  by a feature chain (`(1.0, 2.0, 3.0) [vehicle.body]`). Vector arithmetic keeps
  the frame: `p + p`, `p - q`, `-p`, `2 * p` and `p / 2` are over `p`'s frame, and
  `p / 1 [s]` is over the frame `CoordinateFrame/` composes (`(p / 1 [s]).mRef ==
  velocityCF`), while `'+'`, `'-'`, `inner` and `angle` between vectors over two
  frames, or over a frame and over none, are `ErrTypeMismatch` naming both — the
  library gives such a pair no common coordinates, and `transform` is the way
  from one frame to the other. A vector over a frame equals no vector over
  another frame or over none, so restating a feature's vector over another frame
  is a change its dependents follow. A quantity on a
  scale, `21.5 [SI::'°C_abs']` or `0 [Time::UTC]`, is a scalar quantity whose
  magnitude is in the scale's unit and whose `mRef` is the scale.
  `ConvertQuantity` converts **through the scale's placement**: a scale `S`
  with `transformation : CoordinateFramePlacement { source = R; origin = o }`
  maps `x [S]` to `x · f + o` on `R` and `q` on `R` to `(q − o) / f` on `S`,
  `f` the factor of `S.unit` relative to `R.unit` and `o` converted to
  `R.unit`, so `ConvertQuantity(300.0 [K], SI::'°C_abs')` is `26.85 ['°C_abs']`
  (`26.850000000000023` in binary64, as `300.0 - 273.15` is) and
  `ConvertQuantity(26.85 [SI::'°C_abs'], K)` is `300.0 [K]`; two scales
  convert through a common source (`scale_conversion.go`). Where the scale
  also states a `quantityValueMapping` (`°C_abs`: `0.01 ['°C']` ↔
  `273.16 [K]`) the mapping must agree with the placement, and a disagreement
  is a typed error naming both offsets rather than a choice between them. A
  scale that states neither (`Time::UTC`, every `OrdinalScale`,
  `CyclicRatioScale`, `LogarithmicScale`) is a value, but converting to or from
  it is `errUnevaluableLibraryFunction` naming the scale and the missing
  transformation or mapping; a placement whose `origin` is not a quantity on
  the source (the validation suite's `definitionalEpochInUTC`, an
  `Iso8601DateTime`), or whose `basisDirections` is not the identity `1 [R]`,
  is a typed error naming that; a chain of placements that returns to a scale
  it passed (a scale placed on itself, or two on each other) is
  `ErrCyclicFeatureValue` naming the scale, not a loop. `transform(transformation, sourceVector)`
  (`frame_transform.go`) re-expresses a vector quantity over a frame in the
  transformation's target: `sourceVector.mRef` must be the transformation's
  `source` (`ErrTypeMismatch` naming both otherwise, a uniform-unit vector with
  no frame included), and the result is a vector quantity over the target with
  the target's axis units. The library fixes a placement's `origin` as *the
  location of the origin of the target frame as a vector in the source frame*
  and `basisDirections` as the target's basis in the source, so a placement
  maps target coordinates into source ones, `v_source = origin + B v_target`,
  and `transform` applies the **inverse**, `B⁻¹ (v − origin)`, with the
  directions normalized (they are *directions*; a zero or a linearly
  dependent direction is a typed error). A `TranslationRotationSequence`
  applies its `elements` in order, a `Translation` shifting the origin by its
  `translationVector` converted to the source axis units
  (`ErrIncommensurableUnits` otherwise) and a `Rotation` turning about
  `axisDirection` by `angle` — an angular measure converted through its unit,
  `90 ['°']` and `1.5707963267948966 [rad]` agreeing to the library's seven-digit
  degree — intrinsically (about the axis as the earlier elements moved it, the
  default) or extrinsically (`isIntrinsic = false`, about the axis fixed in the
  source); an `AffineTransformationMatrix3d` applies its `rotationMatrix` and
  `translationVector` in the 4×4 layout its documentation gives; a
  `NullTransformation` is the identity. A transformation of any other type is
  `errUnevaluableLibraryFunction` naming the declaration and the four shapes; a
  frame whose axes are in different units reoriented by a rotation or a basis
  is `ErrIncommensurableUnits`, the axes having nothing to mix in. Two
  transformations are equal when they are the same object or
  when source, target and steps agree (`lbcf.transformation == trs` in the
  geometry example). The solver refuses to pin a frame (`ErrNotPinnable`), a
  document query traverses one to its declaration, and across re-analysis a
  written frame follows the re-read declaration, its units and objects as a
  unit does (`adopt_frame.go`; `TestAdoptRebindsTheFramesAWrittenValueNames`).
  Over gRPC a frame or a transformation crosses as an unsupported null naming
  the value (`unsupported: coordinate frame spatialCF [m, m, m]`;
  `TestCoordinateFrameCrossesAsUnsupported`): the wire `Value` has no arm for one.
  Pilot referee: the pinned pilot evaluates none of `CoordinateFrame/`, `'['`
  over a frame, `ConvertQuantity` or `transform`, so the probes in
  `tools/referee/exec/testdata/cases/coordinate_frames.cases` are
  pilot-unevaluated and the vendored library text is the authority; every
  reading made — transform direction, normalized directions, intrinsic
  rotations about the moved axes — is drafted in [omg-issues.md](omg-issues.md).
- **`ValCoordinateTransformation`** is the `transformation` a frame or a scale
  states, held as a value of its own so `shifted.transformation` reads,
  compares (`turned.transformation == trs`) and is what `transform` takes; it
  prints as its name with source and target, describes as a *coordinate
  transformation*, and is refused by the solver and the wire as a frame is.

Every structured value is compared and hashed by content (`value_equality.go`
`structuredKey`), described (`describe.go`), traced (`trace.go`), rendered by
the REPL (`%eval`, `%features`), pinned by the solver as a non-scalar refusal
(`solve/pin.go`), and traversed by document queries (`repl/docquery.go`);
`TestEveryValueKindIsDispatched` builds one value of every kind up to
`valueKindCount` and fails on a kind any of these does not name. Over gRPC and
Connect each crosses whole: the wire `Value` has an `array` arm (dimensions plus
the elements in row-major order, each a `Value`), a `vector` arm (the numeric
components as `Value`s, Integer and Real kept apart) and a `vector_quantity`
arm (one `Quantity` per component, unit and reduction included), and a value
sent back as an action input or calc argument decodes to the same runtime value
(`grpc/convert.go` `arrayToProto`/`protoToArray`, `vectorToProto`/`protoToVector`,
`vectorQuantityToProto`/`protoToVectorQuantity`; `convert_structured_test.go`
`TestArrayRoundTrip`, `TestVectorRoundTrip`, `TestVectorQuantityRoundTrip`,
`TestMalformedStructuredValuesAreRejected`). A malformed one is refused with a typed
error rather than read as something else: a non-positive dimension
(`ErrArrayDimensionNotPositive`), elements not filling the dimensions
(`ErrArrayShapeMismatch`), a vector component that is not a number
(`ErrVectorComponentNotNumeric`), a vector quantity with no components
(`ErrVectorQuantityEmpty`), or a component unit named without its reduction
(the `Quantity` rules). The service advertises `structured_values` for the
three arms and `measurement_refs` for the bare-reference arm above; the rows on
the API boundary under *Values* record what each client maps them to.

Not represented, and reported by name rather than approximated: the **tensor
products** — `TensorCalculations::tensorVectorMult`, `vectorTensorMult` and
`tensorTensorMult` declare only their parameter and result types, and neither
the package nor the Kernel states which indices contract or how the operands'
`contravariantOrder` and `covariantOrder` combine, so no product is determined;
`VectorCalculations::outer`, whose declared `VectorQuantityValue` return no outer
product inhabits (drafted in [omg-issues.md](omg-issues.md), unfiled); and
`TensorCalculations::transform`, for which neither package states how a
`CoordinateTransformation` acts on a tensor's indices — the runtime transforms a
vector quantity only. A coordinate frame and a measurement scale *are*
represented (`ValCoordinateFrame` above); what they cannot honestly compute — a
scale with no placement and no mapping, a transformation of a shape the library
gives no meaning, a placement origin that is not a quantity — names the
declaration and what it lacks, and the unit text of a quantity is never
promoted to a frame.

A library **feature** — a named constant a library declares and gives no
evaluable value — takes its value from a seam of its own: `libraryFeatures` maps
the feature's qualified name to a Go thunk, consulted only for a symbol a library
document declared, so ordinary name resolution decides and a model's own `pi`
keeps its own value. The value is recomputed per read rather than cached, which
keeps a vector-valued feature from sharing one sequence between readers and keeps
the value independent of whether the library index cache was warm (a warm cache
restores library symbols without their AST).

| Semantic Rule | Implementation | Test Case | Status |
|--------------|----------------|-----------|--------|
| **A Kernel Function Library declaration the runtime does not evaluate as a function call is refused with the library's reason, before its arguments are read; every such declaration is pinned beside the notation that does evaluate** (KerML §9.4.2 `BaseFunctions`: `'['`, `'istype'`, `'hastype'`, `'@'`, `'as'`, `'meta'`, `'all'` are `abstract function`s whose type parameter is a type reference, evaluated through KerML 8.3.4.8 ClassificationExpression, ExtentExpression and MetadataAccessExpression, and whose `[` is the quantity notation `num [unit]`; §9.4.8 `ComplexFunctions::ToString`/`ToComplex` declare no body and the library defines no Complex string notation; §9.4.12 `NaturalFunctions::'/'` returns `Natural[1]`; `OccurrenceFunctions::create` binds `occ.startShot` to happen during the call and returns `occ`). `BaseFunctions::'istype'(1, Integer)`, `'hastype'`, `'@'`, `'as'(1)`, `'all'(E)`, `'meta'(P1)` and `'['(2.0, SI::m)` are `library function is not evaluable: <function>: <reason>` naming the operator notation, where the argument's evaluation failed first as `cannot evaluate definition Integer`; `1 istype Integer`, `1 hastype Integer`, `1 @ Integer`, `1 as Real`, `size(all E)`, `P1 meta SysML::PartDefinition` and `2.0 [SI::m]` evaluate. `ComplexFunctions::ToString(rect(1.0, 2.0))` and `ToComplex("1.0 + 2.0i")` stay refused naming the missing notation. `NaturalFunctions::'/'(6, 2)` is `3`; `'/'(7, 2)` is an arithmetic domain error naming the `Natural` result the library declares. `OccurrenceFunctions::create(new O())` returns the fresh occurrence; `create(p1)` on an occurrence that began before the call is refused as a lifetime constraint the library states. `(1, 2, 3)->reduce '+'` is the function-reference form of a collection operation, recorded in its own row | `runtime/eval.go` `evalInvocation` (an `unevaluable` library target answers before `evalInvocationArgs`), `runtime/library_unevaluable.go` `registerUnevaluableDeclarations` (the reasons), `runtime/library_functions.go` `registerUnevaluable`, `runtime/library_conversions.go` (Complex), `runtime/library_operators.go` (`NaturalFunctions::'/'`), `runtime/builtins_occurrence.go` (the `create` library function) | conformance `read_unevaluable_library_functions` (each declaration on a real receiver, the refusal's reason and the notation that evaluates) | ✅ By design, per the library text: an abstract function with a type parameter has no evaluable function form, only its operator notation; the Complex string notation is undefined; a Natural quotient that is no Natural has no value of the declared type; `create` constrains, not performs, an occurrence's start |
| `RealFunctions`: `sqrt`, `abs`, `floor`, `round`, `max`, `min`; `floor` and `round` return the exact Integer a whole Real is, of any magnitude (`floor(1.0e20)` is `100000000000000000000`), never a wrapped `int64`; only an infinity or NaN, which no Integer is, is an error | `runtime/library_functions.go` `integerResult` | `TestLibraryFunctionValues`, `TestLibraryFunctionErrors`; `migrate/opaque_migration_test.go:TestTranslatedRoundingsGiveExactIntegers` (a migrated `Math.floor`/`ceil`/`round` at and beyond the `int64` boundary) | ✅ Faithful |
| `RationalFunctions`/`NumericalFunctions`: `abs`, `max`, `min` (kind-preserving), `isZero`, `isUnit` | `runtime/library_functions.go` | `TestLibraryFunctionValues` | ✅ Faithful |
| `IntegerFunctions`: `abs`, `max`, `min`; `NaturalFunctions`: `max`, `min` | `runtime/library_functions.go` | `TestLibraryFunctionValues` | ✅ Faithful |
| `TrigFunctions`: `sin`, `cos`, `tan`, `cot`, `arcsin`, `arccos`, `arctan` | `runtime/library_functions.go` | `TestLibraryFunctionValues` | ✅ Faithful |
| Domain, arity and argument-type failures reported at evaluation | `runtime/library_functions.go` `bindAndApply` | `TestLibraryFunctionErrors` | ✅ Faithful |
| A model's declaration is never answered by a library implementation: with a body it is evaluated from that body, without one its result is unbound (`ErrNoValue`) — for a scalar function (`RealFunctions::ToReal`, `RealFunctions::'+'`, `RealFunctions::sqrt`) exactly as for a collection built-in | `runtime/library_functions.go` `libraryFunctionFor` (`Context.libraryDeclared`, the same test `Context.builtinFor` applies) | `TestLibraryFunctionDoesNotHijackADeclaredBody`, `TestLibraryFunctionNeverAnswersAModelDeclaration`; robustness `bodiless_model_calc_named_as_a_builtin` | ✅ Faithful |
| Named argument binds to the parameter the signature declares (`sin(theta = 0.0)`); a name no parameter carries is reported (`errUnknownParameter`) rather than absorbed by an omitted `[0..1]` parameter | `runtime/library_functions.go` `bindAndApply`, `checkNamedArguments` | `TestLibraryFunctionNamedArguments`, `TestVectorAndComplexNamedArguments`, `TestVectorAndComplexUnknownNamedArgument` | ✅ Faithful |
| `TrigFunctions::deg`/`rad`, whose library bodies (`theta * 180 / pi`, `theta * pi / 180`) had no evaluable `pi` | `runtime/library_functions.go` (`degreesFromRadians`/`radiansFromDegrees`, over `libraryPi`) | `TestTrigDegreesAndRadians`; conformance `calc_library_trig_degrees` (`deg(rad(180.0))`), `calc_library_trig_degrees.trace.golden` | ✅ Faithful |
| `VectorFunctions`: `VectorOf`/`CartesianVectorOf`/`CartesianThreeVectorOf`, `isZeroVector`, `'+'`, `'-'` (binary and the one-argument negation the signature's `[0..1]` second parameter allows), `inner`, `norm`, `angle`, `scalarVectorMult`/`'*'`, `vectorScalarMult`, `vectorScalarDiv`, each with the `cartesian*` specialization the library declares of it, and `sum`/`sum0` over a collection of vectors — the whole vendored package. Each answers a **vector value** (`ValVector`, *Structured values* above), printed `⟨1.0, 2.0⟩`; a sequence of numbers given where the signature declares a vector is read as one, a sequence of vectors keeps its grouping | `runtime/library_functions.go` `registerVectorFunctions`; `runtime/vector_functions.go` `readVector`, `vectorCollection`, `vectorSum`, `vectorSum0`, `Context.vectorArithmetic` | `TestVectorFunctionValues`, `TestVectorFunctionScalarValues`, `TestVectorAndComplexNamedArguments`, `TestVectorAndComplexFunctionErrors`; conformance `calc_library_vector_functions`, `calc_library_vector_norm` (+ golden traces), `calc_library_vector_sum`, `calc_library_vector_sum_flat_sequence`, `calc_library_vector_dimension_mismatch`, pilot-exec-diff `w6d:vector-is-zero` (agrees), `:vector-elements`, `:vector-add`, `:vector-norm-cartesian`, `:vector-norm-abstract`, `:vector-inner`; conformance `calc_vector_elements_chain`, robustness `elements_chain_of_a_non_numeric_collection`, `robustness_library_functions_test.go:library_function_outside_its_domain` | ⚠️ Approximate: `elements` of a vector is the sequence of its components (`structuredFeature`), so `w6d:vector-elements` and `:vector-add` agree with the pinned artifact, while `elements` of any other collection stays an element-wise member lookup. `norm`, `inner`, `angle`, `sum` and the vector notation are self-assessed: the pinned artifact does not evaluate its own library bodies for them (`w6d:vector-norm-*`, `:vector-inner` answer an `InvocationExpression`, `VectorOf` too), so it prints no vector to match. A mismatched dimension, a non-numeric element, an element or inner product outside the range of its kind, Real or Integer (`elementArith`, `intArith`, `checkedNumeric`), a zero vector where an angle or a direction is asked for, a division by zero, a two-component `CartesianThreeVectorOf`, and a flat sequence of numbers or a mixed collection where `sum`/`sum0` want vectors are typed errors |
| `ComplexFunctions`: `rect`, `polar`, `re`, `im`, `isZero`, `isUnit`, `abs`, `arg`, `'+'`, `'-'` (both arities), `'*'`, `'/'`, `'**'`/`'^'`, `'=='`, `sum`, `product` | `runtime/library_functions.go` `registerComplexFunctions`, `aggregateComplex` (also reached by `NumericalFunctions::sum`/`product` over a collection holding a Complex, `runtime/collections.go` `aggregate`) | `TestComplexFunctionValues`, `TestComplexFunctionScalarValues`, `TestLibraryFunctionOptionalOperand`, `TestLibraryFunctionEmptyOptionalOperand` (an empty collection written for a `[0..1]` operand is the same no value as null, `argumentOmitted`); conformance `calc_library_complex_functions` (`i * i` is `-1.0`), pilot-exec-diff `w6d:complex-abs-qualified`, `:complex-mul-qualified`, `:complex-re`, `:complex-abs`, `:complex-is-zero`, `:complex-mul-re`, `:complex-is-zero-qualified` | ⚠️ Approximate: the unqualified call selects the declaration its argument types fit — `abs(rect(3.0, 4.0))` is `ComplexFunctions::abs` and answers `5.0`, `isZero(rect(0.0, 0.0))` answers `true` (`w6d:complex-abs`, `:complex-is-zero`; the selection rule is the Invocation overload row under Static Expression Type Checking, and the runtime dispatches the declaration the checker selected); the operators are dispatched by operand kind, so `'*'` over a Complex computes (`:complex-mul-re` answers `-1.0`). The values themselves are self-assessed: the pinned artifact evaluates none of these (its `re`/`im` have no evaluable body, which is also why its `ComplexFunctions::isZero(rect(0.0, 0.0))` answers `false` — `w6d:complex-is-zero-qualified` is unrefereeable, not a disagreement: the same run answers `false` for `isZero(rect(3.0, 4.0))` too, so the value folds against unevaluated operands rather than deciding zero). A Complex is one value (`ValComplex`), so a Real binds to a Complex parameter, a numeric pair is refused as one (`TestComplexFunctionsRejectNumericPairs`), division by zero is reported, and the operators `+`, `-`, `*`, `/`, `**` and `==` over a Complex operand compute as `ComplexFunctions`' (`runtime/complex.go` `complexOperands`, `complexArithmetic`, `complexEqual`) |
| A library **feature** with a library-supplied value: `TrigFunctions::pi` (which the library fixes by an invariant, not a value), `ComplexFunctions::i` (`rect(0.0, 1.0)`), `VectorFunctions::cartesian3DZeroVector` (`⟨0.0, 0.0, 0.0⟩`), `VectorFunctions::cartesianZeroVector` — the library's one feature of three vectors, the 1-, 2- and 3-dimensional zero vectors, answered as a sequence of three vector values `[⟨0.0⟩, ⟨0.0, 0.0⟩, ⟨0.0, 0.0, 0.0⟩]` that keeps its grouping | `runtime/library_functions.go` `libraryFeatures`, `registerLibraryFeature`, `Context.libraryFeatureValue`, `libraryFeatureByName` | `TestLibraryFeatureValue`, `TestLibraryFeatureValueOverridesADeclaredLibraryValue`, `TestLibraryFeatureValueLeavesAModelsOwnFeatureAlone`, `TestLibraryFeatureZeroVectors`, `TestLibraryFeatureByName`, `TestLibraryFeatureImaginaryUnit`; conformance `calc_library_feature_zero_vector`, `calc_library_feature_zero_vectors` | ✅ Faithful |
| A library feature is **read as a name**, qualified (`TrigFunctions::pi`) or imported (`i` under `import ComplexFunctions::*`), and computes like any other value (`2 * TrigFunctions::pi`); a registered feature with no computable value reports itself (`errUnevaluableLibraryFunction`) rather than answering nothing | `runtime/eval_name.go` `evalName` (both the resolved-scope and the qualified-member paths, consulting `runtime/eval.go` `Context.libraryFeatureValue`) | `TestLibraryFeatureNameReadFromItsLibraryDeclaration`; conformance `calc_library_feature_pi`, `calc_library_feature_imaginary_unit`, `calc_library_feature_zero_vector`, `calc_library_feature_zero_vectors` | ✅ Faithful |
| The seam is consulted **after** the ordinary lookups — frame binding, calc output, evaluated feature, instance feature value, enumeration literal — and answers only for a library-declared symbol, so a model feature shadowing a library name keeps its own value and name resolution decides which is read | `runtime/eval_name.go` `evalName` (seam after `runtime/eval.go` `Lookup`/`lookupOutput`/`selfFeatureValue`, before the declaration's body and the cannot-evaluate failure) | conformance `calc_library_feature_shadowed_by_model`; `TestLibraryFeatureValueLeavesAModelsOwnFeatureAlone` | ✅ Faithful |
| A library declaration is answered by its built-in even where the vendored declaration carries a body, since a warm library index cache restores library symbols without their AST and the result must not depend on the cache | `runtime/library_functions.go` `libraryFunctionFor`, `Context.libraryDeclared` | `TestLibraryFunctionAnswersALibraryDeclarationWithABody`, `TestLibraryFunctionDoesNotHijackADeclaredBody` (a model's own declaration keeps its body) | ✅ Faithful |
| `StringFunctions`: `'+'`, `Length`, `Substring`, `'<'`, `'>'`, `'<='`, `'>='`, `'=='`, `ToString` — the whole vendored package. `Length` counts characters (one per Unicode code point, so `Length("héllo")` is 5 over 6 bytes); `Substring` takes 1-based inclusive character positions and reports a position outside `1..Length(x)` naming the character it indexed (`ErrIndexOutOfRange`, one identity for a sequence index and a string position, worded for both), an upper below lower selecting no character, as `SequenceFunctions::subsequence` answers for such a range; `'=='` declares `String[0..1]` operands, so two omitted operands are equal and an omitted one is not equal to a string | `runtime/library_functions.go` `registerStringFunctions`, `stringLength`, `stringSubstring`, `stringOrdering`, `stringEquals`, `stringToString`, `concatStrings`, `compareStrings` | `TestStringFunctionValues`, `TestStringFunctionErrors`, `TestStringFunctionNamedArguments`, `TestVendoredFunctionsAreAllDispatchable` (gating `StringFunctions`); conformance `string_functions`, `string_empty`, `string_substring_out_of_range` (+ golden traces) | ✅ Faithful |
| The operators `StringFunctions` declares evaluate over two String operands: `'+'` concatenates, and `'<'`, `'>'`, `'<='`, `'>='` order strings by character (UTF-8 orders bytes as it orders code points, so a byte comparison *is* code-point order). An operand of another type is reported, naming the operator and both operand types, and neither operand is coerced | `runtime/eval.go` `evalArithmetic` (`ast.OpAdd` over two `ValString`), `evalComparison` (`runtime/errors.go` `OperandTypeError`) | `TestStringOperators`, `TestStringOperatorErrors`, `robustness_library_functions_test.go:testStringOperandOfTheWrongKind`; conformance `string_operators`, `string_comparison`, `string_compared_with_a_number` (+ golden traces) | ✅ Faithful |
| `StringFunctions::'=='` specializes `DataFunctions::'=='`, which is equality over any two values, so the `==` *operator* answers `false` for a String and an Integer rather than reporting; the explicit call `StringFunctions::'=='(s, 3)` reports the non-String argument, as every String-declared signature does. Two strings are equal by their characters, which is also what a collection membership test asks | `runtime/eval.go` `evalEquality` → `runtime/eval.go` `valueEqual`, `runtime/value_equality.go` `valueKey` (`runtime/value.go` `ValString`); `runtime/library_functions.go` `stringEquals` | `TestStringOperators` (`s == three` is false, `("a", "b")->includes("b")`), `TestStringFunctionErrors` | ✅ Faithful |
| **Domain library** `QuantityCalculations` (28 declarations): every one computes over the quantity value — unit-aware `sqrt`, `abs`, `floor`, `round`, `max`/`min`, `sum`/`product`, the operators, comparisons, predicates and conversions (see *Requirement*, the quantity rows), and `'['` and `ConvertQuantity` over a measurement reference value (`'['(3.0, SI::m)` is `3.0 [m]`; *Structured values* above) | `runtime/quantity_functions.go` `registerQuantityCalculations`; `runtime/measurement_ref_functions.go` `quantityOf`, `convertQuantity` | `TestQuantityCalculations`, `TestQuantityCalculationsReport`, `TestQuantityCalculationsAreAllDispatchable`, `TestMeasurementRef*`; conformance `instance_quantity_calculations`, `instance_quantity_calculation_failures`, `instance_quantity_trigonometry`, `instance_measurement_references`, `instance_measurement_reference_failures`, `calc_quantity_sum_imported`, `calc_quantity_sum_unimported` | ✅ Faithful |
| **Domain library** `VectorCalculations` (16): the ten over a numeric vector compute as their `VectorFunctions` counterparts (`inner`, `norm`, `angle` over vector quantities answer the declared `Number`, the magnitude over the `num` components with the unit dropped by declaration — drafted in `omg-issues.md`), and `scalarQuantityVectorMult`, `vectorScalarQuantityMult`, `vectorScalarQuantityDiv` answer a **vector quantity** (`ValVectorQuantity`, `⟨2.0, 4.0⟩ [m]`) whose unit is composed by the scalar quantities' own rule; the five `MeasurementRefCalculations` over a scalar unit (`'*'`, `'/'`, `'**'`, `'^'`, `ToString`) compute over the measurement reference value; **`TensorCalculations`** (13): `'['` over a model-declared `TensorMeasurementReference`, `'+'`, `'-'`, the four scalar multiplications, `isZeroTensorQuantity` and `isUnitTensorQuantity` (square order two) compute over a **tensor quantity** (`ValTensorQuantity`, `Tensor(2, 2)[1.0, 2.0, 3.0, 4.0] [Pa]`) and over the scalar and vector quantities the subtype chain admits; `'CoordinateFrame*'`/`'CoordinateFrame/'`, `VectorCalculations::'['` and `transform` compute over the **coordinate frame** value (`ValCoordinateFrame`); `outer`, and `TensorCalculations`' three products and `transform`, report themselves by name, each with the reason (*Structured values* above) | `runtime/quantity_functions.go` `registerVectorCalculations`, `registerMeasurementRefCalculations`, `registerTensorCalculations`; `runtime/measurement_ref_functions.go`; `runtime/vector_functions.go` `scaleVectorQuantity` over `quantity.go` `scaleQuantities`; `runtime/tensor.go`, `runtime/tensor_functions.go` | `TestVectorCalculations`, `TestQuantityCalculationsReport`, `TestQuantityCalculationsAreAllDispatchable`, `TestMeasurementRef*`, `TestTensorQuantity*`, `semantics/tensor_type_test.go`, `passes/typecheck_tensor_test.go`; conformance `calc_library_vector_quantity`, `calc_library_vector_quantity_norm`, `instance_measurement_references`, `instance_tensor_quantity`, `instance_tensor_quantity_failures`, `calc_library_unevaluable_function` (`TensorCalculations::tensorTensorMult`); robustness `library_function_outside_its_domain`, `measurement_reference_failure_modes`, `tensor_quantity_failure_modes` | ⚠️ Approximate (below) |
| A declaration this runtime has no representation for the values of reports itself by name (`errUnevaluableLibraryFunction`), never a wrong result | `runtime/library_functions.go` `registerUnevaluable` | `TestUnevaluableLibraryFunctionsNameThemselves`, `TestVendoredFunctionsAreAllDispatchable` (every vendored declaration of these packages either computes or names itself); conformance `calc_library_unevaluable_function` | ✅ Faithful |
| **Every declaration of every vendored Kernel Function Library package is dispatchable by name** — the 17 packages `BaseFunctions`, `BooleanFunctions`, `CollectionFunctions`, `ComplexFunctions`, `ControlFunctions`, `DataFunctions`, `IntegerFunctions`, `NaturalFunctions`, `NumericalFunctions`, `OccurrenceFunctions`, `RationalFunctions`, `RealFunctions`, `ScalarFunctions`, `SequenceFunctions`, `StringFunctions`, `TrigFunctions`, `VectorFunctions`, and `OpenSysMLMathFunctions`. Each calc or function declaration, operator-named ones included, is registered as a value function (with the declared parameter names in declared order), as a builtin, or as unevaluable by its own name with a reason; no package and no name is skipped | `runtime/library_functions.go` `libraryFunctions`, `runtime/builtins.go` `builtins`, `runtime/library_unevaluable.go` | `TestVendoredFunctionsAreAllDispatchable` (reads the vendored `.kerml`, so a declaration the library adds fails the gate until it is registered) | ✅ Faithful |
| Conversions: `BaseFunctions::ToString` (`x: Anything[0..1]`: the notation a value is written with — a Boolean, number or String literal, `*` for the unbounded value, an enumeration literal by name, a quantity with its unit as `1.5 [m]`; an omitted or null `x` reads `"null"`; two or more values are `ErrMultiplicityViolation`; an instance, a variant or a body has no notation and is `ErrTypeMismatch`; a Complex stays unevaluable, see below), `BooleanFunctions::ToString`/`ToBoolean`, `IntegerFunctions::ToString`/`ToInteger`/`ToNatural`, `NaturalFunctions::ToString`/`ToNatural`, `RationalFunctions::ToString`/`ToInteger`/`ToRational`, `RealFunctions::ToString`/`ToInteger`/`ToRational`/`ToReal`. A String is parsed as the type's literal notation — decimal digits for an Integer, `true`/`false` for a Boolean, a decimal with an optional fraction and exponent for a Real or Rational — and a String that is no such notation is `errInvalidNotation`, never 0 or a panic (`"NaN"`, `"Inf"`, a hex float and a padded `" 1.5 "` are refused too — the notation is the whole String, nothing is trimmed). A negative given to `ToNatural` is `semantics.ErrArithmeticDomain`; an Integer is unbounded, so a String of any number of digits or a whole Real beyond `int64` converts to the exact Integer it spells (`ToInteger("123456789012345678901234567890")`, `ToInteger(1.0e20)` is `100000000000000000000`); a Real notation float64 cannot hold — one that overflows to an infinity (`"1e400"`) or a nonzero one that underflows to zero (`"1e-400"`), as a literal or a String — is `semantics.ErrArithmeticOverflow`, never `0.0`. `ToInteger` of a Real or Rational truncates toward zero: the vendored declarations carry no body and the library declares `floor` and `round` beside them, so a rounding conversion would duplicate one of those. `RealFunctions::ToRational` of a number answers the exact Rational it is — an Integer or Rational itself as a Rational (`ToRational(2)` is the Rational `2.0`), a finite binary64 Real the rational number that double is, an infinity or NaN `semantics.ErrArithmeticDomain`; `RationalFunctions::ToInteger` of an Integer keeps it | `semantics/eval.go` `ParseReal` (decimal notation only — `isRealNotation`, any other text `semantics.ErrRealNotation` — shared by the Real literal in `runtime/eval.go` `evalLiteralReal`, `runtime/compile.go`, `codegen/compile.go` and the model-level folder); `runtime/library_conversions.go` `registerConversionFunctions`, `parseReal`, `parseInteger` | `semantics/eval_test.go:TestParseReal`, `:TestParseRealRejectsNonDecimalNotation`; `TestConversionFunctionValues`, `TestConversionFunctionErrors`, `TestRealToStringRoundTrips`; conformance `calc_library_conversions`, `calc_library_to_real_invalid_string`, `calc_library_to_natural_negative`; robustness `named_library_call_that_has_no_value`, `real_literal_that_underflows` | ✅ Faithful |
| **`ToString` of a Real** is the shortest decimal that reads back as the same float64 (`FormatReal`: `2.5` → `"2.5"`, `1.0` → `"1.0"`, `0.1 + 0.2` → `"0.30000000000000004"`, `1e21` → `"1e+21"`), the rendering the REPL already prints a Real with, so `ToReal(ToString(x)) == x` for every finite Real. The pinned pilot evaluator was asked for `ToString(1.0)`, `ToString(0.1 + 0.2)`, `ToString(1e21)` and `ToString(2.5)` and answered each with the unevaluated `InvocationExpression ToString` — its `ToString` has no evaluable body — so there is no pilot rendering to match, and the round-tripping one was chosen | `runtime/library_conversions.go` `numberToString` → `semantic/semantics/quantity.go` `FormatReal` | `TestRealToStringRoundTrips`, `TestConversionFunctionValues`; conformance `calc_library_conversions` (`roundTrip`) | ⚠️ Approximate (self-assessed: the pilot does not evaluate it, so the rendering is a decision, recorded here) |
| `RationalFunctions::floor`/`round` are exact over a Rational, answering the Integer their declarations return (`floor(-7/2)` is `-4`, `round(5/2)` is `3`, a half rounding away from zero as `RealFunctions::round` does; `semantics.RatFloor`, `RatRound`), and over a binary64 Real stay the Real implementations; `RationalFunctions::gcd` is the Integer gcd over whole-valued operands, computed exactly (`math/big`) over their magnitudes so any whole Rational is an operand — `gcd(-2^63, 2)` is `2`, `gcd(1.0e20, 6.0e18)` is `2000000000000000000` — and a divisor beyond `int64` is the Integer it is (`gcd(2^63, 0)` is `9223372036854775808`, `gcd(1.0e19, 1.0e19)` is `10000000000000000000`); `gcd(0, 0)` is `0`, and a non-whole operand `semantics.ErrArithmeticDomain`. `RealFunctions::re`/`im`/`arg` answer for a Real as the Complex it is with a zero imaginary part — `re(x)` is `x`, `im(x)` is `0.0`, `arg(x)` is `0.0` or `pi` — rather than being marked unevaluable | `runtime/library_conversions.go` `registerConversionFunctions` (`rationalGCD`, `realItself`, `realZero`) | `TestConversionFunctionValues`, `TestConversionFunctionErrors`; conformance `calc_library_conversions` | ✅ Faithful |
| `RationalFunctions::rat(numer, denum)` is the exact Rational `numer / denum` in lowest terms (KerML 1.1 §9.3.2.2.8; `rat(Integer, Integer): Rational`), the same value `RationalFunctions::'/'(numer, denum)` and the `/` operator compute: `rat(1, 3)` is `1/3`, `rat(6, 4)` is `1.5`, `rat(6, 3)` the Rational `2.0`, and `rat(1, 0)` is `ErrDivisionByZero` as `1 / 0` is, never an infinity; a non-Integer operand is `ErrTypeMismatch`. `numer(rat)`/`denom(rat)` read the exact numerator and positive denominator in lowest terms — an Integer is itself over `1` — so `numer(rat(1, 3))` is `1`, `denom(0.1)` is `10`, `numer(-0.75)` is `-3`, `numer(2)`/`denom(2)` are `2`/`1`, `denom(0.0)` is `1`, and `rat(numer(x), denom(x)) == x` holds for every finite `x`. A finite binary64 Real is the rational number that double is, so `numer` of a Real `x = 0.1` reads the terms of the double the declaration rounded to (`3602879701896397` over `2^55`); a term beyond `int64` is the exact Integer it is, an infinity or NaN `semantics.ErrArithmeticDomain`, each naming the function | `runtime/library_conversions.go` `integersToRational`, `rationalNumerator`, `rationalDenominator`, `exactRationalTerm` over `semantics/rational.go` `RatArith`, `Value.RatNumer`, `Value.RatDenom` | `TestConversionFunctionValues`, `TestConversionFunctionErrors`, `TestRationalTermsRoundTrip`, `TestVendoredFunctionsAreAllDispatchable`; conformance `calc_library_rational_terms`, `calc_library_rational_zero_denominator`, `calc_library_rational_denominator_beyond_int64`, `calc_exact_rational_arithmetic`; robustness `named_library_call_that_has_no_value`; pilot-exec-diff `rational_terms.cases` (`rat-third`, `numer-tenth`, … all `pilot-unevaluated`: the pilot evaluates none of the three, see [exact-rational-evaluation.md](exact-rational-evaluation.md#rationalfunctionsrat-numer-and-denom)) | ✅ Faithful |
| A decimal or exponent literal (`0.1`, `1.5e3`, `25E-3`) is the exact Rational it denotes (§8.4.4.9.2: a `LiteralRational`'s result is classified in `Rational`), and an Integer-only literal stays an Integer; `0.1 + 0.2 == 0.3` is `true`. An exponent beyond the size budget is `ErrRationalSizeLimit` at evaluation, never an infinity or a rounded double | `semantics/rational.go` `ParseRational`; `semantics/eval.go` (literal folding); `runtime/eval.go` (literal evaluation) | `semantics/rational_test.go:TestParseRationalIsExact`; golden `tests/parser/testdata/parse/exact_rational_literals`; conformance `calc_exact_rational_arithmetic`; robustness `TestRuntimeRobustnessExactRational` (`oversized exponent literal`) | ✅ Faithful |
| Rational arithmetic is exact: `+`, `-`, `*`, `/`, `%` and `**`/`^` with an Integer exponent over Integers and Rationals answer the exact Rational `RationalFunctions` declares, the Integer `/` included (`1 / 3` is `1/3`, `IntegerFunctions::'/'` returning `Rational`); a whole result stays a Rational (`6 / 3` is the Rational `2.0`) unless the declaration returns `Integer` (`+`, `-`, `*`, `%`, `**` over Integers, `floor`, `round`, `numer`, `denom`, `ToInteger`). Comparison and equality are exact (`semantics.CompareRat`); `abs`, `max`, `min`, `sum`, `product` keep exactness. A `*` operand follows the existing `LiteralInfinity` rules | `semantics/rational.go` `RatArith`, `smallRatArith`, `RatPow`, `CompareRat`, `RatNeg`, `RatAbs`; `runtime/eval.go` `arithmeticValues`, `comparisonValues`; `runtime/library_functions.go`, `runtime/collections.go` (aggregation) | `semantics/rational_test.go:TestRationalArithmeticIsExact`, `:TestRationalPower`, `:TestRationalComparisonAndRounding`, `runtime/division_test.go:TestWholeNumberQuotientIsRational`, `:TestQuotientBeyondFloatExactRangeIsExact`; conformance `calc_exact_rational_arithmetic`, `action_exact_rational_accumulation` (trace), `constraint_rational_quotient`, `requirement_rational_quotient`; pilot-exec-diff `exact_rationals.cases` | ✅ Faithful |
| A Rational is held in lowest terms over a positive denominator, inline (`int64` numerator, `uint32` denominator) when it fits and as an immutable `math/big.Rat` otherwise, so each Rational has one representation; a result whose numerator-plus-denominator bits exceed the evaluation's budget (`DefaultMaxIntegerBits`, shared with Integers) is `ErrRationalSizeLimit`, refused before an oversized power is computed and never rounded | `semantics/rational.go` `RatValue`, `FracValue`, `smallRat`, `sizedRat`, `RationalSizeExceeded` | `semantics/rational_test.go:TestRationalNormalization`; robustness `TestRuntimeRobustnessExactRational` (denominator growth, harmonic sum, oversized power) | ✅ Faithful |
| A Rational prints exactly: a terminating decimal as the Real of that value prints (`0.1`, `2.0`, `1.5e-05`), any other as `numer/denom` (`1/3`), in the REPL, JSON, traces and `ToString`; `ToRational` reads both forms. The specification fixes no output form | `semantics/rational.go` `Value.FormatRational`, `formatDecimal`, `ParseRationalText` | `semantics/rational_test.go:TestRationalFormatting`, `:TestParseRationalIsExact` | ✅ Faithful (presentation is an OpenSysML choice) |
| `Real` is held to IEEE 754 binary64 *finite* values — a tool decision, not the clause: KerML 1.1 Beta 2 §9.3.2.2.9 defines `Real` as the mathematical *extended* reals, positive and negative infinity included, and the implementation represents finite binary64 values only, never an infinity, `*` being the only unbounded value and living in multiplicity and `LiteralInfinity` positions (§8.3.4.8.11), not in a `Real`. A value declared `Real` is rounded once to the nearest double, operations with no rational result (`sqrt`, trigonometry, `exp`, `ln`, `**` with a non-Integer exponent, anything typed only `Real`) answer a Real, and mixed Rational/Real arithmetic rounds the Rational once to the nearest double. Mixed Rational/Real *comparison* (`==`, `!=`, `<`, `<=`, `>`, `>=`, set membership, query equality, witness replay, compiled comparisons) is at Real precision: the Rational is rounded once to the nearest double, as in mixed arithmetic, so `x : Real = 0.1; x == 0.1` and `rat(1, 3) == 1.0 / 3.0` are `true` and a binding's two ends (§7.4.9) stay equal across a `Real` declaration; a Rational past the binary64 range compares as the infinity of its sign, NaN as with any number; Rational/Rational and Integer/anything stay exact (`CompareIntReal`). A tool-defined approximation of KerML's extended Real — the clause admits ±∞ and leaves precision open, the implementation settles both — recorded in [exact-rational-evaluation.md](exact-rational-evaluation.md#comparing-a-rational-with-a-binary64-real). On the wire an exact Rational is sent as `rational_value` (one a double holds included) and an inbound `real_value` is always a Real ([the wire rule](exact-rational-evaluation.md#a-rational-a-double-holds-on-input)) | `semantics/quantity_eval.go` `RealOf`; `semantics/rational.go` `CompareReal`, `ratToFloat`, `LowestTermsRational`, `CanonicalRational`; `semantics/eval.go`; `runtime/eval.go` `arithmeticValues`; `runtime/set_order.go` `compareExactReal`; `runtime/value_equality.go`; `runtime/write_conformance.go` `holdAsReal`; `solve/replay.go`; `codegen/rational.go`; `protoconv/convert.go` | `semantics/rational_test.go:TestRationalComparisonAndRounding`, `semantics/quantity_eval_test.go`; `runtime/robustness_rational_real_comparison_test.go:TestRuntimeRobustnessRationalRealComparison`; conformance `calc_exact_rational_arithmetic` (`root`), `instance_real_binding_meets_rational`, `action_real_accumulation_stays_binary64`; `grpc/convert_rational_test.go:TestBinaryExactRationalValueReadsAsARational`; gRPC conformance `execute_action_rational_input_meets_rational`, `execute_action_real_input_meets_rational`, `evaluate_calc_rational_wire`, `evaluate_calc_rational_wire_canonical_answer`; `client/python/tests/test_rational.py` | ⚠️ Approximate — `Real` is finite binary64 where the clause admits ±∞, and a Rational meeting one is compared at its precision |
| Every boundary keeps a Rational exact: gRPC carries one no double holds as `Value.rational_value`/`Quantity.rational_magnitude`/`DocumentValue.rational_value` in lowest terms under the `rational_values` capability (a service without it reports the value unsupported and refuses one sent, as `big_int_values` does; a noncanonical encoding is `ErrRationalNotCanonical`), the JSON engine as `rationalValue`, the Go, Python and Node clients as `Rational`, `fractions.Fraction` and `{numerator, denominator}` bigints, and a Rational a double holds exactly as that Real | `api/proto/sysml.proto` `Rational`; `frontend/protoconv/convert.go` `RationalToProto`, `ProtoToRational`, `ValueCarriesRational`; `frontend/grpc/service.go` `CapabilityRationalValues`; `semantics/rational.go` `CanonicalRational`; `client/opensysml/value.go` `Rational`; `client/python/opensysml/values.py`; `client/node/src/core/values.ts` | `grpc/convert_rational_test.go:TestRationalValueRoundTripsOverTheWire`, `:TestRationalQuantityMagnitudeCrossesExactly`, `:TestRationalValueReadsOnlyLowestTerms`, `:TestEvaluateCalcTakesAndReturnsRationals`, `:TestRationalCapability`; `client/opensysml/edges_test.go:TestRationalArithmeticIsExact`; `client/python/tests/test_rational.py`; `client/node/test/rational.test.ts` | ✅ Faithful |
| RDF keeps a literal's exact value: a decimal exports as `xsd:decimal`, an exponent literal as `xsd:double` only when a double holds it exactly and as the exact `xsd:decimal` otherwise, and an imported `xsd:double`/`xsd:float` is read as the binary value it names | `translate/export/rdf_expr.go` `exactRealLiteral`; `translate/export/api_json_in.go` | `tests/export/result_expression_test.go:TestRationalLiteralValuesAreExact` | ✅ Faithful |
| `sysml -compile` compiles exact arithmetic where one binary64 rounding gives the exact result (whole values within `2^53`, Rationals a double holds), folds exact constants, and refuses any other exact Rational operation with a typed error naming the construct, never emitting code that rounds a Rational the interpreter keeps exact; a declared `Real` stays binary64 | `translate/codegen/rational.go` `exactness`, `translate/codegen/ir.go` | `translate/codegen/library_test.go` | ⚠️ Approximate — exact Rationals outside binary64 are refused, not compiled |
| `NumericalFunctions::sum0`/`product1` (the library declares no `Integer`/`Rational`/`Real` specialization of either) fold a collection as `sum`/`product` do, keeping the elements' kind, and answer the identity argument for an empty collection; an identity that is not `isZero`/`isUnit` as the library's own precondition asserts is `ErrTypeMismatch`, and a non-numeric element or an overflow is reported as for `sum` | `runtime/builtins_named.go` `builtinNumericalSum0`, `builtinNumericalProduct1`, `aggregateWithIdentity` | `TestAggregationsWithIdentity`, `TestAggregationsWithIdentityErrors`; conformance `calc_library_aggregation_identity`, `calc_library_sum0_wrong_identity` | ✅ Faithful |
| Generic `DataFunctions::max`/`min` and `ScalarFunctions::max`/`min` order two values the way the `<` operator does — numbers keeping their kind as the numeric `max`/`min` do, strings by character, quantities by magnitude — answering the operand chosen; a pair the library declares no ordering for (two Booleans, two sequences, a string and a number) is `ErrTypeMismatch` | `runtime/library_operators.go` `registerGenericExtrema`, `genericExtremum` | `TestOperatorFunctionValues`, `TestOperatorFunctionErrors`; conformance `calc_library_operator_call_forms` (`largest`, `smallest`) | ✅ Faithful |
| **Every operator the library declares is callable as a function**, and evaluates by the operator's own evaluator code, never a second arithmetic: `'+'`, `'-'` (with `y` omitted — not given, null or an empty collection — the unary sign, the same evaluation the `+`/`-` operator with an empty second operand reaches, KerML 1.1 §8.4.4.9.6; see the binary-operators row), `'*'`, `'/'`, `'%'`, `'**'`, `'^'`, `'<'`, `'<='`, `'>'`, `'>='` in `DataFunctions`, `ScalarFunctions`, `NumericalFunctions`, `RealFunctions`, `RationalFunctions`, `IntegerFunctions` (`'**'`/`'^'` with a Natural exponent) and `NaturalFunctions`, each package's parameter types imposed on the arguments — a `RealFunctions` parameter is `Real`, so an Integer argument is bound as the Real it equals and the function answers a Real (`RealFunctions::'+'(1, 2)` is `3.0`, `'-'(5)` is `-5.0`, `'*'(2^40, 2^40)` is `2^80` where the Integer product would overflow), as `RealFunctions::max`/`abs` already answer Reals, while `RationalFunctions` bind an Integer as the exact Rational it equals and answer an exact Rational (`RationalFunctions::'+'(1, 2)` is the Rational `3.0`), as `RationalFunctions::abs`/`max`/`min` keep exactness (see [exact-rational-evaluation.md](exact-rational-evaluation.md)); `NaturalFunctions::'/'` is the exception to the shared arithmetic, computing the Natural its declaration returns: `'/'(6, 3)` is the Integer `2`, a quotient no Natural equals (`'/'(7, 2)`) is `semantics.ErrArithmeticDomain` rather than a truncated or Rational answer, and `'/'(6, 0)` stays `ErrDivisionByZero` (the `/` *operator* on Natural operands still answers the Rational the pilot answers, see [omg-issues.md](omg-issues.md)); `'=='` in every package that declares it, each typed package (`Boolean`, `Integer`, `Natural`, `Rational`, `Real`) imposing its operand type on both given operands (`IntegerFunctions::'=='(2, 2.0)` is `ErrTypeMismatch`, where the `==` operator and `BaseFunctions::'=='` answer `true`) while an omitted operand stays admitted as null, and `DataFunctions::'=='`/`'==='` admitting DataValues only (a part or other occurrence is `ErrTypeMismatch` naming `DataValue`; `BaseFunctions::'=='`/`'==='` compare anything), `'!='`, `'==='` and `'!=='` in `BaseFunctions`/`DataFunctions` — every equality and identity form holds its `[0..1]` operands to one value: an empty collection is the same no value as null (`BaseFunctions::'=='((), null)` is `true`, `'!=='((), "")` `true`, as `() == null` is by the operator, `isEmptyValue`), a sole element stands for itself, and two or more values are `ErrMultiplicityViolation` naming the parameter (`singleOperands`); every other operand is declared `[1]` and held to exactly one value before its kind is judged — a one-element sequence or set stands for its element (`IntegerFunctions::'+'(xs->select {in i; i > 2}, 1)` adds that element), while an empty or several-valued operand is `ErrMultiplicityViolation` naming the parameter, never a type mismatch (`soleValue`, `checkOperands`, the generic `max`/`min` and `NaturalFunctions::'/'` included); `'not'`, `'xor'`, `'\|'`, `'&'` in `BooleanFunctions`, `DataFunctions` and `ScalarFunctions` (both operands given, so nothing short-circuits); `'..'` in `DataFunctions`, `ScalarFunctions` and `IntegerFunctions`; `BaseFunctions::'#'` (declared `Positive[1..*]` indexes: one selects from the sequence as `SequenceFunctions::'#'` does, several address an Array and select its element in row-major order as `CollectionFunctions::'array#'` does — over a flat sequence they are `ErrTypeMismatch` — and none is `ErrMultiplicityViolation`, `builtinBaseIndex`; conformance `calc_library_base_index_many`, `calc_library_base_index_many_sequence`, robustness `base_index_with_several_indexes`), `CollectionFunctions::'#'` and `','`; `CollectionFunctions::'=='`. A failure names the function as the model writes it (`function IntegerFunctions::'/': division by zero`) | `runtime/library_operators.go` `registerOperatorFunctions`, `naturalDivision`, `equalityForm` over `runtime/eval.go` `arithmeticValues`, `comparisonValues`, `Context.equalityValues`, `unaryValue`, `combineBooleans`, `valueIdentical`; `runtime/builtins_named.go` `registerNamedOperatorBuiltins`, `runtime/range.go` `rangeBuiltin`; `runtime/library_functions.go` `writtenName` | `TestOperatorFunctionValues`, `TestOperatorFunctionErrors`, `TestOperatorFunctionOperandMultiplicity`, `TestDataOperatorFunctionsRequireDataValues`, `TestSequenceOperatorCallForms`, `TestIndexCallFormErrors`, `TestEval_EqualityNull`; conformance `calc_library_operator_call_forms`, `calc_library_operator_singleton_operands`, `calc_library_operator_many_operands`, `calc_library_empty_equality`, `calc_library_base_index_many`, `calc_library_base_index_many_sequence`; robustness `named_library_call_that_has_no_value`, `data_equality_over_a_part`, `base_index_with_several_indexes` | ✅ Faithful |
| An input parameter whose multiplicity admits no value (`in y: Integer[0..1]`) may go without an argument, and is then null: the static arity check counts it as optional and the runtime binds null to it, so `IntegerFunctions::'-'(5)` and `ControlFunctions::'if'(false, 1)` are accepted and evaluated, and a model's own `calc def Scale { in x: Integer; in factor: Integer[0..1]; ... }` is called as `Scale(5)`. A built-in receives one value per parameter it declares whatever the call wrote, positionally or by name, so a trailing omission is null too: `SequenceFunctions::size()` is `0`, `NumericalFunctions::sum()` `0`, `ControlFunctions::'if'(false)` and `'??'()` null, and `subsequence(seq, 2)` runs to the end as the library's `endIndex` default declares; a required parameter left out, or an argument past the last parameter, is `errCalcArity`. A parameter declaring no multiplicity holds one value and stays required (KerML 1.1 §7.4.7.2) | `semantics/multiplicity.go` `Range.AllowsNone`, `Model.IsOptionalParameter`; `passes/typecheck_expr.go` `checkArguments`; `runtime/invoke_calc.go` `optionalParameter`, `EvalContext.bindCalcParameter`; `runtime/builtins_signature.go` `bindBuiltinArgs`, `fillUnbound` | `passes/typecheck_expr_test.go:TestExprInvocationOptionalParameterMayBeOmitted`; conformance `calc_parameter_optional_omitted`, `calc_library_operator_call_forms`, `calc_library_control_functions`, `calc_library_builtin_omitted_arguments`; `builtins_named_test.go:TestControlFunctionCallForms`, `:TestSequenceOperatorCallForms`; robustness `builtin_named_argument_that_binds_nothing` | ✅ Faithful |
| `ControlFunctions::'if'`, `'and'`, `'or'`, `'implies'` and `'??'` as functions keep the library's short-circuit semantics: the `expr` parameters are deferred, so only the branch `'if'` selects, the second operand `'and'`/`'or'`/`'implies'` needs, and the second value `'??'` falls back to are evaluated (`'if'(true, 1, 1 / 0)` is `1`); a body written for one (`{1 + 1}`), or reached through an `expr` parameter of the calling calc (`Pick(test, { base + 1 }, { 0 - base })`), is applied rather than answered as a body, and one declaring a parameter is `ErrBodyArity`. `'if'` with `elseValue` omitted answers null, as the `[0..1]` parameter allows; `'??'` falls back over an empty first operand — null, `()`, an empty set or a collection filtered down to nothing — in the function form and the `??` operator alike (`coalesceNull`, `isEmptyValue`) | `runtime/builtins_named.go` `EvalContext.evalDeferred`, `deferredCount`, `builtinControlIf`, `builtinControlLogical`, `builtinControlNullCoalesce`, `EvalContext.evalDeferred` | `TestControlFunctionCallForms`, `TestControlFunctionCallFormErrors`; conformance `calc_library_control_functions`, `calc_library_control_body_by_reference`; robustness `named_library_call_that_has_no_value`, `body_by_reference_that_cannot_be_applied` | ✅ Faithful |
| A call is dispatched by the declaration it resolves to, the model's declarations first: a built-in answers only a symbol the library declares, so a model's own `NumericalFunctions::sum0` or `size` — with a body or without one — is the model's calc, evaluated from its body or reported as `ErrNoValue` (its result is never bound), never computed by the library's implementation of that name; a built-in binds named arguments by its declared parameter names in any order (`sum0(zero = 0, collection = xs)`), and a name it does not declare is `errUnknownParameter`, a name bound twice or mixed with positional arguments `errCalcArity`. A direct invocation through the runtime API (`InvokeCalc`, `invokeCalcNamed`) takes the same path, so it binds and computes as the written call does; a body or expression value it hands to an `expr` parameter is applied only when the control function selects it, any other value is the operand's value | `runtime/eval.go` `invocationTarget`, `evalInvocation`; `runtime/builtins.go` `Context.builtinFor` (`runtime/library_functions.go` `Context.libraryDeclared`); `runtime/builtins_signature.go` `bindBuiltin`, `bindBuiltinArgs`, `bindBuiltinValues`; `runtime/invoke_calc.go` `invokeCalcWithSelf`, `invokeBuiltinValues` | conformance `calc_library_builtin_shadowed_by_model`, `calc_library_builtin_named_arguments`; robustness `bodiless_model_calc_named_as_a_builtin`, `builtin_named_argument_that_binds_nothing`; `TestInvokeCalcDispatchesBuiltins`, `TestInvokeCalcNamedDispatchesBuiltins`, `TestInvokeCalcDefersBodyArguments` | ✅ Faithful |
| **Occurrences have a lifetime in the runtime's own execution order** (KerML §9.4.18 `OccurrenceFunctions`, `Occurrences::Occurrence`/`Life`, `Performances::Performance`): every object the runtime materializes, and every action or state performance it runs, is an occurrence with a begin and an end expressed as *activation marks* — the monotone counter the executors already order performances by, never wall-clock time — held in a side table keyed by object identity (`Context.lives`), so no Kernel Semantic Library frame member (`portionOfLife`, `startShot`, `endShot`) is added to an object and `%features` keeps its shape. An object begins when its whole does (a part materialized under a bench began when the bench did), a performed action's or exhibited state's occurrence begins when its performance starts and ends when it completes, and the lifetimes of the objects an evaluation materialized and abandoned are dropped with them. The information is read through `Context.OccurrenceLife(id)` (began, ended, destroyed), `%features` (a destroyed object prints `(destroyed at N, alive since M)` in place of features it no longer holds), `%instances` (`, destroyed`), and the execution trace (`create: T #n`, `destroy: T #n`) | `runtime/lifetimes.go` `life`, `Context.beginLife`, `beginPerformanceLife`, `endPerformanceLife`, `forgetLives`, `OccurrenceLife`; `runtime/instance.go` `materialize`; `runtime/action_executor.go` `initialize`/`retireToken`; `runtime/state_executor.go` `initialize`/`completeIfDone`; `runtime/trace.go` `recordOccurrenceCreated`/`recordOccurrenceDestroyed`; `repl/meta.go` `featureValueWalk.rows` (the `(destroyed at N, alive since M)` and `, destroyed` renderings) | `lifetimes_test.go:TestOccurrenceLifeBeginsWithWhole`, `:TestAbandonedObjectsLoseTheirLives`; conformance `occurrence_is_during_completed_performance`, `occurrence_is_during_in_exhibited_state`, `occurrence_lifecycle_in_performed_action` (+ golden trace); `repl/occurrence_test.go:TestOccurrenceCommandReturnsAtThePrompt`; `library_shape_test.go:TestShapeKeepsSystemsLibraryFeaturesAndLeavesOutTheKernelFrame` passes unchanged | ✅ Faithful |
| `OccurrenceFunctions::'==='` (`x: Occurrence[0..1]`, `y: Occurrence[0..1]`) is same-occurrence identity: `true` iff both operands denote one object, so two separately materialized, structurally equal parts are `!==` while their attributes are `==`; the operators `===`/`!==` answer the same over objects (`evalIdentity`, which compares instances by identity). Two omitted operands are identical (as `BaseFunctions::'==='` answers), one omitted and one object are not; a data value is `errNotAnOccurrence` naming the parameter, two or more values `ErrMultiplicityViolation`. The pinned pilot evaluator (0.61.0) answers `w1 === w1` `true`, `w1 !== w2` `true` and `BaseFunctions::'==='(w1, w1)` `true` but `OccurrenceFunctions::'==='(w1, w1)` **`false`** — it folds the declared body `x.portionOfLife == y.portionOfLife` over features no value has — so the operator agreement, not the pilot's function answer, is what is matched ([omg-issues.md](omg-issues.md#occurrencefunctions-are-evaluated-over-declarations-rather-than-occurrences-pilot-2026-07)) | `runtime/builtins_occurrence.go` `builtinOccurrenceSame`, `occurrenceOf`; `runtime/eval.go` `evalIdentity`, `valueIdentical` | `TestOccurrenceFunctionArity`; conformance `occurrence_same_distinguishes_equal_parts`; robustness `OccurrenceFunctions::'==='(xs, xs)`, `'==='(factor, factor)` | ✅ Faithful |
| `OccurrenceFunctions::isDuring(occ: Occurrence[1])` — the vendored declaration takes the one occurrence and answers `notEmpty(during)`, whether the call *happens during* it — is `true` while `occ` is alive at the evaluation: an object from its materialization until `destroy`, a performed action or exhibited state from its start until it completes, so `isDuring(go)` assigned inside `go`'s body is `true` and read after `go` completed `false` (a performed action stating no flow performs no step, so it takes its inputs — a binding of an `out` parameter or of a name it does not declare failing the performer, as for a flowed action — and is complete at once, `isDuring` of it `false`); an owned part read from a state's entry action is `true`, and `false` after that action destroyed it. A data value is `errNotAnOccurrence`, an empty or several-valued argument `ErrMultiplicityViolation`, an object the context holds no lifetime for `errOccurrenceLifetime` — never a silent `false`. The pilot answers `isDuring(1)` and `isDuring("x")` `true`, folding `during` statically; not matched. **Not computed:** a nested action node named as a value (`isDuring(inner)` in `Outer`'s body) is `errNodePin` (`declares no result to read it as a value by`), as every read of an action node without a result is — the performance is an occurrence, but the node is not a value the evaluator hands out; and forks run interleaved in one step order, so `isDuring` across concurrent branches answers for that order, which is one of the orders the specification admits | `runtime/builtins_occurrence.go` `builtinOccurrenceIsDuring`; `runtime/lifetimes.go` `Context.lifeOf`, `life.alive` | conformance `occurrence_is_during_completed_performance`, `occurrence_is_during_in_exhibited_state`, `occurrence_lifecycle_in_performed_action`; robustness `OccurrenceFunctions::isDuring(xs)`, `isDuring(factor)` (a data value is also a type error the checker reports before execution); `internal/frontend/repl/occurrence_test.go` `isDuring(a.n)`; `TestOccurrenceLifeBeginsWithWhole`, `TestDestroyEndsPortionsAndRefusesReads`, `TestPerformedActionWithoutAFlowCompletes`, `TestPerformedActionWithoutAFlowTakesItsInputs`, robustness `no_flow_performed_action_checks_its_inputs` | ✅ Faithful (the two uncomputed cases above are typed errors) |
| `OccurrenceFunctions::create(inout occ: Occurrence[1])` begins `occ` *during the current performance*: an occurrence the call is the first to reach — a part materialized by evaluating the argument — begins where the call reached it, ahead of the parts and performances reached with it (a state it exhibits or action it performs starts no earlier than its performer), and is answered, journaled so a rolled-back speculative evaluation forgets it, and traced as `create: T #n`; one that began before the call (a part the model read earlier, an object `%instantiate` made) is `errOccurrenceLifetime` naming when it began, since an occurrence begins once; a destroyed one `errOccurrenceDestroyed`. `destroy(inout occ: Occurrence[0..1])` ends `occ` and, with it, every object it holds as a portion of itself (its owned parts, transitively, each once however many names of a redefined feature hold it), answering `occ`; an omitted `occ` answers nothing, as declared. Afterwards `isDuring(occ)` is `false`, every read or write of a feature of the destroyed object is `errOccurrenceDestroyed` (never a stale value) — through a binding too, whichever end names the destroyed object's feature: a live feature bound to it is not read from it, and a live value is not written into it (`binding.go` `resolveBindingLocation`, `TestBindingRefusesADestroyedEnd`, robustness `binding_end_of_a_destroyed_object`) — and the destroyed object performs nothing: an operation invoked on it, an action or state machine performed by it, is `errOccurrenceDestroyed` before it runs, even one touching no feature of it, so it sends no message (`lifetimes.go` `checkPerformer`, from `invokeOperation` and the executor constructors; `TestDestroyedObjectPerformsNothing`, robustness `operation_of_a_destroyed_object`) — `%features` prints the destruction, and a second `destroy` is `errOccurrenceDestroyed` naming the first. A `=` value that read a feature of the destroyed object, `isDuring` of it, or `all T` is derived again when next read rather than answering what it derived before: `destroy` unmaterializes what derived from the ended objects' features (`lifetimes.go` `forgetDerivedFrom`), and the lifetimes are a dependency of their own (`Context.lifetimes`, `readsLives` from `lifeOf` and `evalExtent`, `livesChanged` from every change of a life — creation, a `=` value's own `new T()` included, `destroy`, `terminate`, a performance beginning or ending — sparing only what is deriving as it changes), journaled so a rollback restores what was derived, and carried by a held image — a `=` value that read the lives is derived again in the context it is materialized into once a life there changes, and the objects the image brings are a change of that context's lives: what its own objects derived from `all T` before is derived again over them (`held_image.go` `imagedFeature.readsLives`, `HeldImage.Materialize`; `robustness_object_lifecycle_test.go` `derivations_that_read_lifetimes_derive_again_when_they_change`, `an_imaged_derivation_that_read_lifetimes_follows_the_lives_where_materialized`). Destroying an occurrence ends the behaviors it performs with it: the state machine it exhibits and the actions it performs are terminated where they stand (`terminated with occurrence: <name>` in the trace), their own performance occurrences ended, and a behavior attached but not yet run takes no run; a destroy reached from inside one of those behaviors unwinds it as a `terminate` does and the enclosing performance goes on. The destroyed object is released from `all T` (the extent walk skips it), while a feature that still names it — a `ref` alias, an element of a collection — keeps the value, so `size(cars)` is unchanged and `cars#(2).n` is `errOccurrenceDestroyed`: the library declares `destroy` over the occurrence, not over what refers to it, and a stale reference is an error to read, not an empty one (`isDuring` of it is `false`). A portion that ended before its whole stays ended where it did. A snapshot (`Image`) of a graph whose held objects include a destroyed one carries it destroyed and materializes its behaviors as terminated. The pilot answers `create(w1)` and `destroy(w1)` with `w1` and `destroy(null)` with nothing, no lifecycle being observable in it | `runtime/builtins_occurrence.go` `builtinOccurrenceCreate`, `builtinOccurrenceDestroy`; `runtime/lifetimes.go` `Context.createDuring`, `destroy`, `portionsOf` (what its composite features hold and what it is home to, transitively), `checkNotDestroyed`; `runtime/occurrence_terminate.go` `Context.endBehaviorsWith`, `noteEndingUndo` (a journal under way — a constructor's, a probe's — puts an ended executor back where it stood on rollback), `ActionExecutor.endsWith`/`endTerminated`, `StateExecutor.endsWith`/`endTerminated`; `runtime/classifier_behavior.go` `nextRunnableBehavior` (an ended pending behavior takes no run), `performanceOccurrence` (a held performance occurrence resolves for a destroyed object too); `runtime/extent.go` `Context.objectsOf`; `runtime/instance.go` `GetFeatureValue`/`SetFeatureValue` | `TestCreateOnlyWhatTheCallReaches`, `TestCreateBeginsBeforeWhatItReachesWith`, `TestDestroyEndsPortionsAndRefusesReads`, `TestDestroyEndsAnAliasedPartOnce`, `TestDestroyEndsTheMachinePerformed`, `TestLifetimeChangesRollBackWithTheJournal`; `robustness_object_lifecycle_test.go:TestRuntimeRobustnessObjectLifecycle` (`destroy_through_a_reference_ends_the_held_object`, `destroy_ends_the_created_objects_machine`, `destroy_twice_is_refused`, `destroy_of_the_whole_ends_the_created_parts`, `explore_destroy_race_reaches_both_outcomes`); conformance `object_destroyed_at_runtime` + trace golden; conformance `occurrence_lifecycle_in_performed_action`, `occurrence_lifecycle_errors` (`destroyedTwice`, `readAfter`, `begunBefore`); robustness `OccurrenceFunctions::create(factor)`, `destroy(factor)` | ✅ Faithful |
| `OccurrenceFunctions::addNew(inout group: Occurrence[0..*] nonunique, inout occ: Occurrence[1])` and `addNewAt(…, in index: Positive[1])` create `occ` as `create` does and insert it into `group`: `addNew` at the end, `addNewAt` before the 1-based `index`, `index == size + 1` appending and any other index outside `1..size + 1` `ErrIndexOutOfRange` before anything begins (`insertAt`, the one insertion `SequenceFunctions::includingAt` uses); an element of `group` that is not an occurrence is `errNotAnOccurrence`. **Approximate in its value:** the declaration returns `occ` and mutates `group` through `inout`, but an expression call has no feature to write back to — an argument is a value, and writes belong to `assign` in a behavior body (a calculation body may not write a part's feature at all) — so the call answers the group *after* insertion, as `including`/`includingAt` do, for the body to write (`assign spares := addNew(spares, spare)`); `group` itself is unchanged by the call. The pilot answers `addNew(group, w1)` and `addNewAt(group, w1, 5)` with `w1` and leaves `group` as it was, so neither its insertion nor its bounds are observable there | `runtime/builtins_occurrence.go` `builtinOccurrenceAddNew`, `builtinOccurrenceAddNewAt`, `occurrenceGroup`; `runtime/collections.go` `EvalContext.insertAt` | `TestOccurrenceFunctionArity`, `TestCreateOnlyWhatTheCallReaches`; conformance `occurrence_lifecycle_in_performed_action` (`addNew` then `size`), `occurrence_lifecycle_errors` (`tooFar`); robustness `OccurrenceFunctions::addNew(xs)`, `addNew(occ = xs)`, `addNew(xs, factor)`, `addNewAt(xs, xs)`, `addNewAt(occ = xs, index = 1)`, `addNewAt((), factor, 0)` | ⚠️ Approximate (self-assessed: the value is the group, not `occ`; `inout group` is not written back) |
| A body expression is a value closed over the environment it is written in — the frames and scope in force there — and is applied in that environment wherever it is later called, so a body passed on through an `expr` parameter (`Keep(xs, { in x; x > threshold })`, `Keep` doing `xs->select pred`) reads its writer's `threshold`, a body-local `bound`, or a reducer's `weight`, and a same-named parameter of the applying calc (`Keep`'s own `threshold` or `pred`) never captures it; a name the writer's scope does not reach stays `ErrUnresolvedReference`. The bindings are copied into the closure, so a body a calc returns (`return : expr = { in x; x > threshold }`) still reads its `threshold` after the calc has returned and its frame storage has been reused; the calc evaluation whose outputs the body may name is detached from that storage too, so a body bound to the result (`out threshold = n; out pred : expr = { in x; x > threshold }; bind result = pred;`) or read from a usage nested in the calc's body works those outputs out from the invocation's own parameters, memoized in common with the invocation and with an output named in terms of itself still `errCyclicOutput` | `runtime/value.go` `exprValue`, `newExprValue`, `Value.exprEnv`; `runtime/eval.go` `EvalContext.closure`, `runtime/frame.go` `frame.snapshot`, `runtime/calc_usage.go` `calcRun.detached`; `runtime/collections.go` `bodyOf`, `evalClosure`, `applyBody`; `runtime/builtins_signature.go` `evalArgument` | conformance `calc_library_body_by_reference`, `calc_library_body_keeps_declaring_scope`, `calc_library_body_outlives_its_calc`, `calc_library_body_reads_calc_output`, `calc_library_control_body_by_reference`; `calc_run_detach_test.go:TestEscapedBodyReadsCalcOutputsAfterFrameReuse`, `:TestEscapedBodyReportsOutputCycle`; robustness `body_by_reference_that_cannot_be_applied` | ✅ Faithful |

Found, not fixed — numeric library declarations that remain unevaluable. Each
reports itself by name rather than answering:

| Not implemented | Why |
|---|---|
| `MatrixFunctions` | The vendored Kernel Function Library declares no such package — only `docs/` mention it — so there is nothing to dispatch. Not implemented rather than invented. |
| `ComplexFunctions::ToString`/`ToComplex`, `BaseFunctions::ToString` of a Complex | No string notation for a Complex value is defined; inventing a rendering would make `ToComplex(ToString(x))` a value nothing else in the library agrees on. |
| `BaseFunctions::'['` | Declared abstract, and the operator notation `a[i, j]` is the quantity notation `num [unit]` to the parser and the evaluator (`evalIndexExpr`): over an operand whose bracket names no unit the error (`errNotAQuantity`, `notAQuantityError`) says so, and when the operand is declared an Array, a vector or a feature of more than one value (`declaredCollection`, from the declaration alone — the operand is not evaluated to diagnose it) points at `a#(i, j)`, which `CollectionFunctions::'array#'` and `BaseFunctions::'#'` evaluate. Reading `a[i, j]` as indexing by the operand's static type would be a checker rule as well as a runtime one, and is not made here. |
| `BaseFunctions::as`, `meta`, `istype`, `hastype`, `'@'`, `'@@'`; `ControlFunctions::'.'` | The operator notations `all T`, `x istype T`, `x as T`, `x meta T`, `x @ M`, `x.f` **are** evaluated, from their own expression nodes (`all`/`as`/`meta`/`istype`/`hastype`/`@`/`@@` take a *type*, which the function form would have to receive as a value). The function forms report themselves rather than pretend. |
| `DataFunctions::'~'`, `ScalarFunctions::'~'` | Declared abstract and specialized by no concrete library function, so the complement denotes no operation on any value type — reported as the `~` operator is; adjudicated abstract-only in [bitwise-complement.md](bitwise-complement.md). |
| `QuantityCalculations::ConvertQuantity` to or from a measurement scale that states neither a `CoordinateFramePlacement` nor a `quantityValueMapping` — `Time::UTC`, every `OrdinalScale`, `CyclicRatioScale`, `LogarithmicScale` in the vendored library | The scale is a value (`ValCoordinateFrame`, *Structured values* above) and a quantity on it reads, but the library gives its points no relation to any other reference, so a conversion has nothing to compute by; the reason names the scale and the missing transformation or mapping. `SI::'°C_abs'`, placed on `K`, converts. |
| `VectorCalculations::transform` over a `CoordinateTransformation` that is none of `CoordinateFramePlacement`, `TranslationRotationSequence`, `AffineTransformationMatrix3d`, `NullTransformation` | Those four are the shapes the library documents; a user-defined subtype states no origin, basis, steps or matrix the runtime could apply, so the reason names the declaration and the four shapes. |
| `TensorCalculations::tensorVectorMult`, `vectorTensorMult`, `tensorTensorMult`; `VectorCalculations::outer`; `TensorCalculations::transform` | The three products declare only their parameter and result types: neither `TensorCalculations` nor the Kernel states which indices contract or how the operands' `contravariantOrder` and `covariantOrder` combine, so no product is determined. `outer` declares `return : VectorQuantityValue`, which the order-two outer product of two vectors is not (drafted in `omg-issues.md`, unfiled). `transform` takes a `CoordinateTransformation` the runtime holds (`ValCoordinateTransformation`), but neither package states how it acts on a tensor's contravariant and covariant indices, and `VectorCalculations::transform` over a vector is the only law the library gives. Each names itself and the reason. Not invented; the structural and componentwise `TensorCalculations` compute (*Structured values*, `ValTensorQuantity`). |
