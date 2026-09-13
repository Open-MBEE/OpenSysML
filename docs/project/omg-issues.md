# Bugs in the OMG materials

One place to look for defects found in the OMG-published sources this
implementation consumes. This page records defects in the **vendored specification
libraries** (`internal/core/libs/stdlib/`), in the **published example corpora**, and
in the **OMG pilot implementation** the differential is measured against.

Each row quotes the vendored declaration verbatim so a reviewer can judge it
without opening the library, and names what OpenSysML implements instead. Every
divergence is also a row in [spec-compliance.md](spec-compliance.md).

| Library file | Declaration | What the vendored body says | What we implement | Why |
|---|---|---|---|---|
| `Kernel Libraries/Kernel Function Library/NaturalFunctions.kerml` | `function '/'` | `function '/' specializes IntegerFunctions::'/' { in x: Natural[1]; in y: Natural[1]; return : Natural[1]; }` — a Natural quotient, which `7 / 2` cannot inhabit without truncation | the quotient of two whole numbers is a Rational, never normalised back to a whole number even when exact: `divisionResult` types `Natural/Natural` as `Rational`, and the runtime answers a Real (`runtime/eval.go` `evalArithmetic`) | The pilot's evaluator answers `LiteralRational 2.5` for `5 / 2` even when both operands are `Natural`-typed attributes — it dispatches on value kind, and a whole-number value divides through `RationalFunctions::'/'`, which `IntegerFunctions::'/'` specializes with `return : Rational[1]`. The declared `Natural[1]` return is unimplementable without truncating, which the reference does not do; the draft below asks which of the two the specification intends |
| `Domain Libraries/Quantities and Units/MeasurementReferences.sysml`, `VectorCalculations.sysml` | `attribute def CoordinateFramePlacement`, `attribute def Rotation`, `calc def transform` | `origin specifies the location of the origin of the target frame as a vector in the source frame`; `basisDirections specifies the orientation of the target frame by specifying the directions of the respective basis vectors of the target frame via direction vectors in the source frame`; `transform` has no body — the text fixes what a placement *states* but not which way `transform` applies it, whether a direction vector's magnitude matters, or which axes an intrinsic rotation turns about | a placement maps target coordinates into source ones (`v_source = origin + B v_target`), so `transform(source→target)` applies the inverse `B⁻¹ (v − origin)`; basis directions are normalized; an intrinsic rotation turns about the axis as the earlier elements of the sequence moved it, an extrinsic one about the axis fixed in the source; a `NullTransformation` is the identity | The pinned pilot evaluates `transform` for no input, so the library text is the only authority and each reading is drafted below for clarification rather than asserted as the specification's |
| `Domain Libraries/Quantities and Units/VectorCalculations.sysml` | `calc def inner`, `calc def norm` | `calc def inner :> VectorFunctions::inner { in : VectorQuantityValue[1]; in : VectorQuantityValue[1]; return : Number[1]; }` and `calc def norm :> VectorFunctions::norm { in : VectorQuantityValue[1]; return : Number[1]; }` — a bare `Number`, where `QuantityCalculations::'*'`, `'/'` and `sqrt` over scalar quantities return `ScalarQuantityValue[1]`, so the norm of a length vector has no unit while the square root of a length squared keeps one | the declaration: `inner`, `norm` and `angle` of a vector quantity answer the `Number` computed over the vector's `num` components (`norm(⟨3.0, 4.0⟩ [m])` is `5.0`, `inner` is `25.0`), never a quantity, and a `Number` feature takes them; the unit is dropped by declaration (`runtime/vector_functions.go` `vectorInner`, `vectorNorm`) | The checker already types the calls by their declared `Number` return, so a runtime answering a quantity would disagree with it; the pinned pilot evaluates neither, so there is no reference answer to follow. Recorded as a library inconsistency for review, not as a defect OpenSysML corrects |
| `Domain Libraries/Quantities and Units/VectorCalculations.sysml` | `calc def outer` | `calc def outer { in : VectorQuantityValue[1]; in : VectorQuantityValue[1]; return : VectorQuantityValue[1]; }` — a vector, where the outer product of two vectors of orders one is a tensor of order two (`TensorCalculations::'['` returns `TensorQuantityValue[1]` for exactly that shape) | nothing: `outer` is `ErrUnevaluableLibraryFunction` naming the declaration and its return type (`runtime/quantity_functions.go` `registerVectorCalculations`); the checker types a call by the declared `VectorQuantityValue` | No `VectorQuantityValue` holds an outer product, and answering a `TensorQuantityValue` would disagree with the checker and the declaration; the draft below asks for `return : TensorQuantityValue[1]` |
| `Domain Libraries/Quantities and Units/TensorCalculations.sysml` | `calc def isUnitTensorQuantity` | `calc def isUnitTensorQuantity { in x : TensorQuantityValue[1]; return : Boolean[1]; }` — no body, and no statement of which tensor is the unit one beside `isUnitVectorQuantity` (a vector of norm one) | the identity of a square order-two tensor: `true` when every diagonal component is one and every other zero; a tensor of any other shape (a vector, a 2×3, an order three) is `ErrUnevaluableLibraryFunction` naming the shape it needs (`runtime/tensor_functions.go` `tensorIsUnit`) | The library names no unit tensor; the identity matrix is the one tensor with an established claim to the name, and it exists for square order two alone, so a reading over other shapes would be invented. Recorded so the reading can be checked against a future release that gives the calculation a body |
| `Domain Libraries/Geometry/ShapeItems.sysml` | `item def CuboidOrTriangularPrism` (`Cuboid`, `RectangularCuboid`, `Box`) | `item tfe [2] :> edges;` … `binding [1] bind [0..1] tf.edges = [0..1] tfe;` `binding [1] bind [0..1] ff.edges = [0..1] tfe;` — every named edge group and vertex group (`tfe`…`urre`, `tflv`…`brrv`) is valued only by bindings whose ends link *one unspecified* value each; and `item :>> vertices; assert constraint { size(vertices) == size(edges) }` beside `Quadrilateral`'s `item :>> vertices [8];` and `StructuredSpaceObject`'s `faces.vertices subsets vertices` | the groups and everything read through them (`box.tfe`, `box.tflv`, `box.tfe.length`, `box.vertices`) are the typed `ErrBindingEnd` naming the binding; the runtime never picks a member (`runtime/binding.go` `UndeterminedBindingError`) | No conjunction of the bindings, the `MatesWith` connections and the `size(...)` assertions identifies which of a face's four edges a group holds, so no evaluator can name `tfe`; and a `Cuboid`'s `vertices` cannot be both a superset of its six faces' 48 vertex objects and 24 long. The draft below records both |
| `Kernel Libraries/Kernel Data Type Library/Collections.kerml` | `UniqueCollection::elements`, `OrderedSet::elements`, `Map::elements`, `OrderedMap::elements` | `feature elements[0..*] :>> Collection::elements { doc /* Note: Redefinition of 'elements' is unique by default. */ }` over `feature elements[0..*] nonunique` — a redefinition stating neither `nonunique` nor `ordered`, whose only claim to uniqueness is the note | the four redefinitions are unique, as their notes say, and every collection under them (`Set`, `OrderedSet`, `Map`, `OrderedMap`, a model's `:>> elements` in one) inherits that; a redefinition anywhere else that states neither keyword takes the uniqueness of what it redefines (`semantics.Model.IsUnique`) | KerML 1.0 §7.3.4.4 gives the *default* only to a fresh or subsetting feature; a redefinition has "the same" values as the feature it redefines (§7.3.4.5), and the pinned pilot models it so — `'3dVectorQuantityValue'::num :>> num` and `CartesianSpatial3dCoordinateFrame::mRefs :>> mRefs` restate no keyword and the validator refuses to redefine either `nonunique` (`validateSubsettingUniquenessConformance`), while the library's own `mRefs default (SI::m, SI::m, SI::m)` and every `(0, 0, 1)` direction vector hold repeats it accepts. Reading every unstated redefinition as unique would make those defaults and vectors violations; reading it as inheriting makes `Set` and `Map` nonunique against the note. The note is the library's statement of intent, so it wins for these four declarations only |
| `Kernel Libraries/Kernel Function Library/SequenceFunctions.kerml` | `function includingAt` | `(seq->subsequence(1, index - 1), values, seq->subsequence(index + 1))` — the prefix before `index`, then the values, then the tail from `index + 1`, so the element **at** `index` is dropped from the result | insertion: the values are inserted before the 1-based `index`, the tail from that position shifts right, and the result is longer than `seq` by the values inserted. `index == size + 1` appends; any other index outside `1..size + 1` is `ErrIndexOutOfRange` (`runtime.builtinSequenceIncludingAt`) | The body contradicts the declarations around it in the same file. `excludingAt` is the operation that removes at an index, and the behavior pairs are additive/subtractive: `add` calls `including` as `remove` calls `excluding`, and `addAt` calls `includingAt` (`seq->includingAt(values, index)`) as `removeAt` calls `excludingAt`. A removing `includingAt` would leave the library with two ways to delete at an index and none to insert at one, and would make `addAt` remove. The vendored expression is an off-by-one slip in the tail: the insertion body is `(seq->subsequence(1, index - 1), values, seq->subsequence(index))` |

## `Collections::UniqueCollection::elements` and its kin — unique by a note, over a `nonunique` root

Quoted verbatim from
`internal/core/libs/stdlib/Kernel Libraries/Kernel Data Type Library/Collections.kerml`:

```kerml
abstract datatype Collection {
    feature elements[0..*] nonunique { … }
}
abstract datatype UniqueCollection :> Collection {
    feature elements[0..*] :>> Collection::elements {
        doc
        /* Note: Redefinition of 'elements' is unique by default. */
    }
}
datatype OrderedSet :> OrderedCollection, UniqueCollection {
    feature elements[0..*] ordered :>> OrderedCollection::elements, UniqueCollection::elements {
        doc
        /* Note: Redefinition of elements is unique by default. */
    }
}
datatype Map :> Collection {
    feature elements: KeyValuePair[0..*] :>> Collection::elements {
        doc
        /* Note: Redefinition of elements is unique by default.*/
    }
}
datatype OrderedMap :> Map, OrderedCollection {
    feature elements: KeyValuePair[0..*] ordered :>> Map::elements, OrderedCollection::elements {
        doc
        /* Note: Redefinition of elements is unique by default. */
    }
}
```

KerML 1.0 §7.3.4.2 makes `isUnique` default to true ("the default is that the feature is unique";
`+isUnique : Boolean = true` in the §8.3.3.3.1 overview), and `nonunique` is the one piece of concrete syntax that sets it false
(`MultiplicityPart`, §8.2.4.3.1). §7.3.4.4 says what that default means under specialization —
"if the subsetted feature is non-unique, then the subsetting feature will still be unique by
default, unless specifically flagged as nonunique" — and the four notes above read that sentence
as applying to a redefinition too. The library is not consistent with that reading elsewhere:

- `Quantities::'3dVectorQuantityValue'` declares `:>> num : Real[3]` over
  `TensorQuantityValue::num : Number[1..*] ordered nonunique`, and every direction vector in the
  domain library and the geometry examples, `(0, 0, 1)` and the like, repeats a value.
- `MeasurementReferences::CartesianSpatial3dCoordinateFrame` declares `:>> mRefs : LengthUnit[3]`
  over `TensorMeasurementReference::mRefs : ScalarMeasurementReference[1..*] nonunique`, and the
  library's own `default (SI::m, SI::m, SI::m)` repeats one.

The pinned pilot (`jupyter-sysml-kernel` 0.61.0, release 2026-07) models both redefinitions as
**unique**: a model redefining either `nonunique` (`:>> mRefs nonunique = (m, m, m)`,
`:>> num nonunique = (0, 0, 1)`) is refused with `validateSubsettingUniquenessConformance`
("Subsetting/redefining feature cannot be nonunique if subsetted/redefined feature is unique"),
so no model author can opt out. The pilot never checks values against `isUnique` — its census
names no value-level constraint and its evaluator answers `1, 1, 2` for
`OrderedSet { :>> elements = (1, 1, 2); }` — which is why the contradiction never surfaces there.

OpenSysML enforces `isUnique` on values (KerML §8.4.3.4 item 6, "if a Feature is unique, there
are no values with the same markings"), so it has to pick a reading under which the library's
own declarations and defaults are conforming:

- A **fresh or subsetting** multi-valued feature is unique unless declared `nonunique`
  (§7.3.4.4 as written).
- A **redefinition** stating neither keyword takes the uniqueness of the features it redefines,
  all of them (§7.3.4.5: the redefining feature's values are the redefined feature's values, so a
  restriction the redefined feature does not impose is not imposed by restating it — the same way
  type, multiplicity and ordering already reach a redefinition here). `3dVectorQuantityValue::num`
  and `CartesianSpatial3dCoordinateFrame::mRefs` are then nonunique, as their defaults require.
- The **four `Collections` redefinitions above** are read as unique, as their notes say, against
  what the inheritance reading would give them. This is the erratum: the library states its
  intent in a comment where the notation has a keyword for it, and the keyword — restating
  nothing — points the other way. `Set`, `OrderedSet`, `Map`, `OrderedMap`, and a model's
  `:>> elements` under any of them, inherit uniqueness from these four.

Implementation: `internal/core/semantics/uniqueness.go` (`Model.IsUnique`, `uniqueByLibraryNote`).
Evidence: `semantics/uniqueness_test.go:TestIsUniqueLibraryCollections` pins all six library
collections; the conformance fixtures under `internal/core/runtime/testdata/conformance/`
for ISQ vectors, coordinate frames and geometry run unchanged, and
`library_ordered_set_elements_repeated` / `library_ordered_map_elements_repeated` refuse the repeat.
The row is in [spec-compliance.md](spec-compliance.md) under *Uniqueness through redefinition
and subsetting*. Recorded for review against a future KerML release: either the four
redefinitions should say `unique` in notation the grammar does not yet have, or `§7.3.4.4`
should say which of the two readings a redefinition takes.

## `includingAt` — the vendored declaration

Quoted verbatim from
`internal/core/libs/stdlib/Kernel Libraries/Kernel Function Library/SequenceFunctions.kerml`:

```kerml
function includingAt{ in seq: Anything[0..*] ordered nonunique; in values: Anything[0..*] ordered nonunique;
    in index: Positive[1];
    return : Anything[0..*] ordered nonunique =
        (seq->subsequence(1, index - 1), values, seq->subsequence(index + 1));
}
```

`subsequence(1, index - 1)` is the prefix ending before `index`, and
`subsequence(index + 1)` is the tail starting after `index`; the element at
`index` appears in neither, so evaluating the body as written *replaces* it with
`values` rather than inserting before it. OpenSysML implements insertion
(maintainer ruling), so `includingAt` is a divergence from the
vendored body and is recorded here for review against a future OMG release.

## `NaturalFunctions::'/'` — the declared Natural return against the pilot's Rational answer

**Not filed.** Drafted here for a maintainer to authorise; nothing has been
posted upstream.

Quoted verbatim from
`internal/core/libs/stdlib/Kernel Libraries/Kernel Function Library/NaturalFunctions.kerml`:

```kerml
function '/' specializes IntegerFunctions::'/' { in x: Natural[1]; in y: Natural[1]; return : Natural[1]; }
```

````markdown
**Question, not a bug report:** `NaturalFunctions::'/'` declares
`return : Natural[1]`, but the pinned pilot implementation (`2026-05`,
`jupyter-sysml-kernel` 0.60.1) evaluates `5 / 2` to `LiteralRational 2.5` even
when both operands are `Natural`-typed attributes
(`attribute a : ScalarValues::Natural = 5; attribute b : ScalarValues::Natural = 2;`).
Its evaluator dispatches on the value's kind rather than the declared type, so
the division runs through `RationalFunctions::'/'` — the function
`IntegerFunctions::'/'` specializes with `return : Rational[1]` — and no
truncating Natural division is observable. A `Natural[1]` return would require
the quotient to be truncated or the call to be rejected, and the pilot does
neither. Is the declared return type intended to be `Rational[1]` (matching
`IntegerFunctions::'/'`), or is a conforming evaluator expected to truncate?
````

OpenSysML follows the pilot's observed behavior for the operator: the type
checker (`passes/typecheck_expr.go` `divisionResult`) types `Natural/Natural`
division as `Rational`, and the runtime answers a Real, so a non-whole quotient
bound to a `Natural`-typed feature is reported rather than truncated. The
function called by name, `NaturalFunctions::'/'(x, y)`, is the one place the
declaration itself is the contract: it returns the Natural quotient when `y`
divides `x` and reports a non-whole quotient (`ErrArithmeticDomain`) rather
than truncating or answering a Rational (`runtime/library_operators.go`
`naturalDivision`).

---

## `VectorCalculations::inner`/`norm` — a `Number` where the scalar calculations return a quantity

**Not filed.** Drafted here for a maintainer to authorise; nothing has been
posted upstream.

Quoted verbatim from
`internal/core/libs/stdlib/Domain Libraries/Quantities and Units/VectorCalculations.sysml`:

```sysml
	calc def inner :> VectorFunctions::inner { in : VectorQuantityValue[1]; in : VectorQuantityValue[1]; return : Number[1]; }
	calc def norm :> VectorFunctions::norm { in : VectorQuantityValue[1]; return : Number[1]; }
	calc def angle :> VectorFunctions::angle { in : VectorQuantityValue[1]; in : VectorQuantityValue[1]; return : Number[1]; }
```

and from `QuantityCalculations.sysml` in the same directory:

```sysml
	calc def '*' specializes NumericalFunctions::'*' { in x: ScalarQuantityValue[1]; in y: ScalarQuantityValue[1]; return : ScalarQuantityValue[1]; }
	calc def '/' specializes NumericalFunctions::'/' { in x: ScalarQuantityValue[1]; in y: ScalarQuantityValue[1]; return : ScalarQuantityValue[1]; }
	calc def sqrt{ in x: ScalarQuantityValue[1]; return : ScalarQuantityValue[1]; }
```

````markdown
**Library inconsistency, question rather than bug report:** the scalar
quantity calculations keep the quantity through an operation —
`QuantityCalculations::'*'`, `'/'` and `sqrt` all declare
`return : ScalarQuantityValue[1]`, so `sqrt(q * q)` of a length `q` is a
length — but the vector quantity calculations drop it: `VectorCalculations::inner`
and `norm` declare `return : Number[1]` over `VectorQuantityValue` operands.
The norm of a length vector is therefore a bare number, and the inner product of
two length vectors a bare number too, although each is a quantity of the operands'
unit (or its square) in the same way `q * q` is. Only `angle` is naturally
dimensionless. Is `Number[1]` the intended return for `inner` and `norm`, with
the unit understood to be implied by the operands, or should they return
`ScalarQuantityValue[1]` as the scalar calculations do? (A redefinition of
`VectorFunctions::inner`/`norm`, whose returns are `Number`, could not narrow
to `ScalarQuantityValue` since that is not a `Number`; a resolution would need
the vector calculations declared independently of `VectorFunctions`, as
`QuantityCalculations::'*'` is of `NumericalFunctions::'*'` only by
specialization.)
````

The pinned pilot implementation (`2026-07`, `jupyter-sysml-kernel` 0.61.0)
evaluates none of these: asked through `build/pilot-evaluator/eval-sysml` for
`VectorFunctions::norm(CartesianVectorOf((3.0, 4.0)))`,
`VectorCalculations::norm(...)`, a `Number` attribute bound to the former,
`QuantityCalculations::sqrt(q * q)` and `q * q` over `q : ScalarQuantityValue = 2 [m]`,
it answers the unevaluated `InvocationExpression norm`, `InvocationExpression sqrt`
and `OperatorExpression *` for every case, so it offers no reference for either
reading.

OpenSysML follows the declarations: the checker types `inner`, `norm` and `angle`
of vector quantities as `Number` (a `ScalarQuantityValue` feature bound to one is a
static error, a `Number` feature is accepted), and the runtime answers the bare
number computed over the vector's `num` components — `norm(⟨3.0, 4.0⟩ [m])` is
`5.0`, `inner(⟨1.0, 2.0⟩ [m], ⟨3.0, 4.0⟩ [m])` is `11.0` — the unit dropped by
declaration (`runtime/vector_functions.go`; conformance
`calc_library_vector_quantity_norm`). The scalar calculations keep their quantity
results as declared (`runtime/quantity_functions.go`).

## `VectorCalculations::outer` — a `VectorQuantityValue` return for an order-two product

**Not filed.** Drafted here for a maintainer to authorise; nothing has been
posted upstream.

Quoted verbatim from
`internal/core/libs/stdlib/Domain Libraries/Quantities and Units/VectorCalculations.sysml`:

```sysml
    calc def outer { in : VectorQuantityValue[1]; in : VectorQuantityValue[1]; return : VectorQuantityValue[1]; }
```

and from `TensorCalculations.sysml` in the same directory:

```sysml
    calc def '[' specializes BaseFunctions::'[' { 
    	in elements: Number[1..n] ordered nonunique; 
    	in mRef: TensorMeasurementReference[1]; 
    	return quantity: TensorQuantityValue[1];
    	private attribute n = mRef.flattenedSize;
    }
```

````markdown
**Library defect:** `VectorCalculations::outer` declares
`return : VectorQuantityValue[1]`. The outer product of two vectors of orders
one is a tensor of order two — `dimensions` of two entries, `order` two — and
`Quantities::VectorQuantityValue` is the subtype of `TensorQuantityValue` whose
`dimensions` has at most one entry (`VectorMeasurementReference::dimensions :
Positive[0..1]`), so no value of the declared type can hold the result. The
sibling `TensorCalculations::'['` returns `TensorQuantityValue[1]` for exactly
this shape, and `TensorCalculations::tensorVectorMult`/`vectorTensorMult`
return `VectorQuantityValue` where the contraction does lower the order.
Should `outer` declare `return : TensorQuantityValue[1]`?
````

The pinned pilot implementation (`2026-07`, `jupyter-sysml-kernel` 0.61.0)
evaluates no `VectorCalculations` or `TensorCalculations` call
(`cmd/pilot-exec-diff`, `tensor_quantities.cases`: all twelve probes, `outer` among
them, are pinned `pilot-unevaluated`), so it
offers no reference answer. OpenSysML leaves `outer` unevaluable with a reason
naming the declared return type (`runtime/quantity_functions.go`
`registerVectorCalculations`; robustness `tensor_quantity_failure_modes`), and
types a call by the declaration, as the checker must.

## `TensorCalculations::isUnitTensorQuantity` — the reading OpenSysML takes

Not a defect report; a record of a reading the text does not fix.

Quoted verbatim from
`internal/core/libs/stdlib/Domain Libraries/Quantities and Units/TensorCalculations.sysml`:

```sysml
    calc def isZeroTensorQuantity { 
    	in x : TensorQuantityValue[1]; 
    	return : Boolean[1];
    }
    calc def isUnitTensorQuantity { 
    	in x : TensorQuantityValue[1]; 
    	return : Boolean[1];
    }
```

`isZeroTensorQuantity` has one reading — every component zero — and OpenSysML
answers it for a tensor of any shape. `isUnitTensorQuantity` has a body neither
here nor in a Kernel function it specializes, and `VectorCalculations::isUnitVectorQuantity`
(a vector of norm one) does not generalize: a tensor has no single norm the
library names. The one tensor with an established claim to the name is the
identity matrix, which exists for a square tensor of order two alone (the
library's own matrix, `AffineTransformationMatrix3d`, is an order-two `Array`).
OpenSysML therefore answers
`isUnitTensorQuantity` for a square order-two tensor (`true` exactly when the
diagonal is one and the rest zero) and reports any other shape as
`ErrUnevaluableLibraryFunction` naming the shape it needs
(`runtime/tensor_functions.go` `tensorIsUnit`; conformance
`instance_tensor_quantity`, `instance_tensor_quantity_failures`). Nothing is
invented for a vector, a rectangular or a higher-order tensor.

## `CoordinateFramePlacement`, `Rotation`, `VectorCalculations::transform` — the direction of a transformation, the magnitude of a direction, and the axes of an intrinsic rotation

**Not filed.** Drafted here for a maintainer to authorise; nothing has been
posted upstream.

Quoted verbatim from
`internal/core/libs/stdlib/Domain Libraries/Quantities and Units/MeasurementReferences.sysml`:

```sysml
	attribute def CoordinateFramePlacement :> CoordinateTransformation {
    	doc
    	/*
    	 * CoordinateFramePlacement is a CoordinateTransformation by placement of the target frame in the source frame.
    	 *
    	 * Attribute origin specifies the location of the origin of the target frame as a vector in the source frame.
    	 *
    	 * Attribute basisDirections specifies the orientation of the target frame by specifying the directions of
    	 * the respective basis vectors of the target frame via direction vectors in the source frame. An empty sequence of
    	 * basisDirections signifies no change of orientation of the target coordinate frame.
    	 */
		attribute origin : VectorQuantityValue[1];
		attribute basisDirections : VectorQuantityValue[0..*] ordered nonunique;
```

```sysml
	attribute def Rotation :> TranslationOrRotation {
		/*
		 * Attribute isIntrinsic asserts whether the intermediate coordinate frame moves with the rotation or not,
		 * i.e. whether an instrinsic or extrinsic rotation is specified.
		 *
		 * See https://en.wikipedia.org/wiki/Davenport_chained_rotations for details.
		 */
		attribute axisDirection : VectorQuantityValue[1];
		attribute angle :>> angularMeasure;
		attribute isIntrinsic : Boolean[1] default true;
	}
```

and from `VectorCalculations.sysml` in the same directory:

```sysml
	calc def transform {
	    in transformation : CoordinateTransformation;
	    in sourceVector : VectorQuantityValue { :>> mRef = transformation.source; }
	    return targetVector : VectorQuantityValue { :>> mRef = transformation.target { ... } }
	}
```

````markdown
**Clarification request:** `VectorCalculations::transform` re-expresses a vector
quantity written over `transformation.source` as one over `transformation.target`,
and `CoordinateFramePlacement` describes the target frame from within the source
(`origin` is "the location of the origin of the target frame as a vector in the
source frame"; `basisDirections` are "the directions of the respective basis
vectors of the target frame via direction vectors in the source frame"). Three
things the text leaves to the reader decide the numbers `transform` answers:

1. **Direction.** Read literally, a placement maps *target* coordinates into
   *source* coordinates: a point at target coordinates `t` sits at
   `origin + B t` in the source, `B` the matrix whose columns are the basis
   directions. `transform(source → target)` must then apply the **inverse**,
   `B⁻¹ (v − origin)`, and a `Translation` in a `TranslationRotationSequence`
   *subtracts* its `translationVector` from the source coordinates. Is that the
   intent, or does a placement state the mapping `transform` applies directly
   (`v_target = origin + B v_source`)? The two readings agree only for the
   `NullTransformation`.
2. **Magnitude of a direction.** `basisDirections` are "direction vectors", which
   suggests only their direction is meaningful and `B` is built from the
   normalized vectors; but a `VectorQuantityValue` carries a magnitude, and
   `(0, 2, 0)[datum]` as a basis direction could equally state a target axis of
   twice the source unit. Should a non-unit direction be normalized, scale the
   axis, or be a violation?
3. **Intrinsic rotations.** For a `Rotation` with `isIntrinsic = true` (the
   default), the Davenport reference the doc cites turns about the axis of the
   *intermediate* frame, i.e. `axisDirection` as the earlier elements of the
   sequence have moved it, and an extrinsic one about `axisDirection` fixed in
   the source; the composition order of the two therefore differs. Is
   `axisDirection` written in the source frame in both cases, and does a
   `Translation` between two rotations move the intermediate frame's origin
   without affecting the axes?

A fourth, smaller point: `MeasurementScale`s that specialize `CoordinateFrame`
(`IntervalScale`) state `basisDirections` in one dimension (`1 [UTC]` in the
validation suite's `MissionElapsedTimeScale`). A magnitude of 1 is the identity;
is a magnitude other than 1 a scale factor between the two scales' units, or is
the unit's own `unitConversion` the only factor a scale may apply?
````

The pinned pilot implementation (`2026-07`, `jupyter-sysml-kernel` 0.61.0)
evaluates none of `transform`, `MeasurementRefCalculations::'CoordinateFrame/'`,
`VectorCalculations::'['` over a frame or `ConvertQuantity` to a scale
(`cmd/pilot-exec-diff/testdata/cases/coordinate_frames.cases` pins the probes as
pilot-unevaluated), so it offers no reference answer for any of the four.

OpenSysML follows the literal reading of each: a placement maps target
coordinates into the source and `transform` applies the inverse (a
`TranslationRotationSequence` of a translation and a rotation followed by its
inverse sequence round-trips, conformance `instance_coordinate_frames`); basis
directions are normalized, a zero or a linearly dependent direction being a typed
error; an intrinsic rotation turns about the axis as the earlier elements moved
it and an extrinsic one about the axis fixed in the source; and a one-dimensional
basis direction of magnitude 1 is the identity while any other magnitude is a
typed error naming it, the library giving a scale no other basis
(`runtime/frame_transform.go`, `runtime/scale_conversion.go`; the compliance
record's *Structured values* section names each decision).

## `ShapeItems::CuboidOrTriangularPrism` — edge and vertex groups fixed only by `[0..1]` bindings, and a vertex count its faces exceed

**Not filed.** Drafted here for a maintainer to authorise; nothing has been
posted upstream.

Quoted verbatim from
`internal/core/libs/stdlib/Domain Libraries/Geometry/ShapeItems.sysml`
(`CuboidOrTriangularPrism`; `Cuboid` adds the `srf`, `tsre`, `ufre`, `urre`,
`tfrv`, `trrv` bindings in the same form):

```sysml
		item :>> edges = faces.edges;
		...
		assert constraint { size(edges) == 18 or size(edges) == 24 }

		item tfe  [2]	 :> edges;
		item tre  [2]	 :> edges;
		item tsle [2]	 :> edges;
		...
		item :>> vertices;
		assert constraint { size(vertices) == size(edges) }

		item tflv [3]	 :> vertices;
		...
		/* Bind face edges to specific edges */
		binding [1] bind [0..1] tf.edges = [0..1] tfe;
		binding [1] bind [0..1] tf.edges = [0..1] tre;
		binding [1] bind [0..1] tf.edges = [0..1] tsle;
		...
		binding [1] bind [0..1] ff.edges = [0..1] tfe;
		...
		/* Bind edge vertices to specific vertices */
		binding [1] bind [0..1] tfe.vertices = [0..1] tflv;
		binding [1] bind [0..1] tsle.vertices = [0..1] tflv;
		binding [1] bind [0..1] ufle.vertices = [0..1] tflv;
		...
		/* Meeting edges */
		connection :MatesWith connect [1] tfe to [1] tfe;
		...
		/* Meeting vertices  */
		connection :MatesWith connect [2] tflv to [2] tflv;
```

and from `Polygon` and `Quadrilateral` in the same file and `StructuredSpaceObject`
in `Kernel Libraries/Kernel Semantic Library/Objects.kerml`:

```sysml
	item def Polygon :> Path, PlanarCurve {
		item :>> edges : Line { item :>> vertices [2]; }
		...
	item def Quadrilateral :> Polygon {
		item :>> edges [4] = (e1, e2, e3, e4);
		...
		item :>> vertices [8];
```

```kerml
	portion feature faces : StructuredSurface[0..*] ordered subsets structuredSpaceObjectCells {
		feature redefines that : StructuredSpaceObject;
		feature redefines edges subsets that.edges;
		feature redefines vertices subsets that.vertices;
	}
```

````markdown
**Library question, two parts.**

**1. The edge and vertex groups are underdetermined.** `tfe [2] :> edges` (the two
edge portions where the top and front faces meet) is valued by nothing but
`binding [1] bind [0..1] tf.edges = [0..1] tfe` and
`binding [1] bind [0..1] ff.edges = [0..1] tfe`. The `[0..1]` on each end is a
connector-end multiplicity (KerML 1.0 §7.4.6.2): each binding declares exactly one
`SelfLink` (§8.4.4.6.2 BindingConnector) joining *some* value of `tf.edges` to *some*
value of `tfe`, and constrains how many values of either end take part — it does not say
which of `tf`'s four edges is meant. Nothing else in the model does either:
`tf.edges` and `ff.edges` are disjoint objects, so their bindings pick two different
members of `tfe` without relating them to each other; the other bindings on `tf.edges`
(`tre`, `tsle`, and `tsre` in `Cuboid`) each pick one unspecified edge too, and no
constraint says the four groups partition `tf.edges`; `connection :MatesWith connect
[1] tfe to [1] tfe` relates the two members of the group to one another, which every
choice satisfies alike; and `size(edges) == 24` counts `faces.edges`, which the groups
do not affect. The three `[0..1] tfe.vertices` / `tsle.vertices` / `ufle.vertices`
bindings on `tflv [3]` repeat the pattern one level down, over the four vertices of
each group's two edges. So any two edges of `tf` are as good a `tfe` as any other —
the model determines the groups' sizes, not their members — and an evaluator asked
for `tfe`, `tflv`, `tfe.length` or `vertices` (subsetted by the groups) has nothing
to answer with but a guess. Are the bindings intended as an identification of a
*particular* edge (e.g. `tf.e1` and `ff.e3`), spelled by position as `Quadrilateral`
spells `edges [4] = (e1, e2, e3, e4)`? If so the groups need a value, or the bindings
need to name the members (`bind tf.e1 = tfe#(1)`).

**2. `size(vertices) == size(edges)` is unsatisfiable for a `Cuboid`.** `vertices`
is inherited from `StructuredSpaceObject`, whose `faces.vertices subsets vertices`,
and each `Quadrilateral` face declares `vertices [8]` (two per edge, the endpoints of
consecutive edges being distinct mating occurrences). A `Cuboid` therefore holds at
least 6 × 8 = 48 vertex objects, while `edges = faces.edges` holds 6 × 4 = 24 and the
assertion requires 24 vertices. The eight corner groups `tflv [3]` … `brrv [3]` sum
to exactly 24, so the assertion appears to count those and to intend `vertices` to
be the corner groups alone — which the inherited subsetting forbids. Should the
assertion read over the groups (or over the distinct meeting points), or should
`vertices` be redefined to exclude the faces' own vertices?

(Aside: the face redefinitions in `CuboidOrTriangularPrism`, `TriangularPrism` and
`Cuboid` read `ref :>> Quadrilateral::edges, ConeOrCylinder::faces::edges;` — they
name `ConeOrCylinder`'s faces from a `Polyhedron` that is not one. `faces::edges`
resolves the same feature, as the `ff`/`rf` declarations beside them spell it.)
````

The pinned pilot implementation (`2026-07`) offers no reference here: asked through
`build/pilot-evaluator/eval-sysml` for `box.tfe`, `box.tflv`, `box.vertices` and
`box.edges` over `part box : ShapeItems::Box { :>> length = 2 [m]; :>> width = 1 [m];
:>> height = 3 [m]; }`, it answers the unevaluated `ItemUsage tfe`, `ItemUsage tflv`,
`ItemUsage vertices` and `ItemUsage edges`, and `box.tfe.length` is
`Couldn't resolve reference to Feature 'length'`. Neither validator reports the pattern:
the specification places no conformance rule between a connector end's multiplicity and
the multiplicity of the feature it relates (`[0..1]` ends over `[2]` and `[4]` features
are well-formed): OpenSysML's `-validate` accepts the `Box` model clean, and the pilot
loads the library and the model without complaint before leaving the usages unevaluated.

OpenSysML answers `box.faces` (6), `box.edges` (24), `box.tf.edges` (4) and every
face-local value; the groups and everything read through them are the typed
`ErrBindingEnd` naming the binding and both ends, on `-e`, `%eval`, `-instantiate`
and `%features` alike (`runtime/binding.go` `UndeterminedBindingError`; conformance
`instance_library_geometry_box`):

```text
binding end cannot be resolved: box.tfe is bound by `bind [0..1] tf.edges = [0..1] tfe`, which makes some value of tfe a value of tf.edges without saying which value of either; the model does not state what tfe holds
```

The runtime reads each partial binding on its own and does not solve the conjunction of several, so a group two partial bindings *would*
pin down — two collections sharing exactly one value, bound `[0..1]` to a `[1]`
feature — is reported the same way; that is a limitation recorded in
[spec-compliance.md](spec-compliance.md), not a reading of this library, whose groups
no conjunction pins down.

---

## Defects in published OMG example models

These rows are true positives found by OpenSysML's static analysis in models published with the
OMG example corpora. The pinned pilot is silent on each because it does not perform the
corresponding check.

| Example | Expression as published | Defect | Specification reading | Status |
|---|---|---|---|---|
| `Geometry Examples/VehicleGeometryAndCoordinateFrames.sysml:38` | `22/2*25.4 + 110 [mm]` | `[mm]` binds only to `110`, so `+` combines a dimensionless value with a length | KerML 1.0 §8.2.5.8.1–8.2.5.8.2 makes the bracket construction a primary expression; SysML v2.0 §9.8.9.1 requires addition operands to have the same quantity dimension. The evident intended spelling is `(22/2*25.4 + 110) [mm]`. | **not filed** |
| `Analysis Examples/Turbojet Stage Analysis.sysml:25` | `1/(2 * Cp) * V^2 + T_static`, with `Cp : DimensionOneValue`, `V : VolumeValue`, and `T_static : TemperatureValue` | the declared types make the operands L^6 and Θ | SysML v2.0 §9.8.9.1 requires addition operands to have the same quantity dimension and top-level quantity type. The formula needs dimensionally appropriate parameter declarations or conversion before addition. | **not filed** |
| `Analysis Examples/Dynamics.sysml:13` | `return a : AccelerationValue = tp * dt * tp;`, with `tp : PowerValue` and `dt : TimeValue` | a power squared times a duration has dimension L^4·M^2·T^-5, not the L·T^-2 an `AccelerationValue` is measured in | KerML 7.4.9 makes the expression the return feature's value, so it answers to that feature's type, and the ISQ definitions the file imports fix the dimensions: `AccelerationUnit` is L^1·T^-2, `PowerUnit` is L^2·M^1·T^-3, and `TimeValue` aliases `DurationValue` (T^1). No grouping of the published product is an acceleration. | **not filed** |
| `Individuals Examples/AnalysisIndividualExample.sysml:86` | `individual action :>> fuelConsumption : FuelEconomyAnalysis_1` | the redefining action is typed by the enclosing analysis definition, which does not conform to the redefined feature's `FuelConsumption` | KerML 7.4.9 and 8.3.4.2 make a redefinition a subsetting, so the redefining feature's type must conform to the redefined one's. The file declares `individual action def FuelConsumption_1 :> FuelConsumption` and never uses it: that is the intended type. | **fixed upstream at `2026-07`** — the corpus now publishes `fuelConsumption : FuelConsumption_1`, the type this row named, so the row is closed and its overlay entry retired |

The first two rows form one adjudicated quantity-commensurability family in
[the adjudications record](adjudications.md); the third is the same physics read through a
declared type rather than through an addition, so it is an error and not a warning; and the fourth
was the corpus instance of the subsetting-conformance divergence adjudicated in
[the same record](adjudications.md), which the `2026-07` corpus corrects on its own.
OpenSysML retains the three open diagnostics, and nothing has been posted upstream. All three are
entries of the declared errata overlay — the geometry row with a correction, the turbojet and
dynamics rows without one — quoted verbatim with their derivations in
[the fourth section](#the-errata-overlay-entries-for-these-models).

---

## Defects in the pilot implementation

The first section records a defect in a vendored library body, and the second records defects in
published example models. This section records defects in the **OMG SysML v2 pilot implementation**
(`Systems-Modeling/SysML-v2-Pilot-Implementation`), which
[pilot-differential.md](pilot-differential.md) uses as the reference oracle. A
row lands here only when it is established from the pilot's own artifacts — its
grammar, its `.ecore`, or its loaded object graph probed through its own API —
and not from a disagreement alone.

| Component | Pinned version | Symptom | Adjudication | Status |
|---|---|---|---|---|
| `org.omg.sysml` — `Type::ownedDisjoining` setting delegate | `2026-05` (`jupyter-sysml-kernel` 0.60.1) | every `disjoint from` clause in a type declaration draws EMF's `The opposite features 'owningType' … and 'ownedDisjoining' … do not refer to each other` | [one cause for all six corpus diagnostics](pilot-differential.md#k6-diagnostic-by-diagnostic-f33), reproduced in three lines and probed through the pilot's API | filed upstream as [Systems-Modeling/SysML-v2-Pilot-Implementation#790](https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation/issues/790), **fixed** by [#791](https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation/pull/791) (`basicGet` compared `getTypeDisjoined()` against the delegate rather than the owner) and shipped in `2026-08`, where all six corpus diagnostics are gone; body below |
| `org.omg.sysml` — the `queryx/failing` Xpect fixtures | `2026-05` (`jupyter-sysml-kernel` 0.60.1) | `QPE-Qualifier`, `QPE-Traversal` and `QPE-Wildcard` declare `XPECT noErrors`, yet the pinned validator rejects all three with `no viable alternative at input '/'`, `For input string: "."` and `no viable alternative at input '@'` | [adjudications.md](adjudications.md) — established by running the pinned pilot's own SysML validator on the three fixtures, not from a disagreement | **not filed** — question drafted below, awaiting maintainer authorisation |
| `org.omg.sysml.xtext` — `checkTransitionFeatureMembership` (`validateTransitionFeatureMembershipGuardExpression`) | `2026-07` (`jupyter-sysml-kernel` 0.61.0) | `TransitionUsage_invalid.sysml.xt` expects `Must be a Boolean expression.` at `if "test"`, yet the pinned validator with the full standard library accepts a `String` or arithmetic guard in the same shape | [pilot-rejection.md](pilot-rejection.md#constraints-the-pilot-declares-but-does-not-enforce) — established by running the pinned pilot's own SysML validator on the fixture's shape, not from a disagreement alone | **not filed** — question drafted below, awaiting maintainer authorisation |
| `org.omg.sysml.xtext` — `SysMLValidator.checkControlNode`, `checkDecisionNode`, `checkForkNode`, `checkJoinNode`, `checkMergeNode` | `2026-07` (`jupyter-sysml-kernel` 0.61.0) | a fork or decision node with two incoming successions, a join or merge node with two outgoing, and a succession end whose written multiplicity is not the one SysML v2 §7.17.3 requires all validate clean; only `validateControlNodeOwningType` is reported | established from the pilot's source: eight of the nine constraints are `// TODO: Check validate… (?)` comments in the check methods (`SysMLValidator.xtend:857–888` at `c7fc737`); the reproducers are `cmd/pilot-reject/testdata/negative/semantic/cn01`–`cn04`, `cn06`–`cn09`, run through the pinned batch validator | **not filed** — drafted below, awaiting maintainer authorisation |
| `org.omg.kerml.xtext` — `KerMLValidator.checkFeature`, the `validateFeatureOwnedCrossSubsetting` check | `2026-07` (`jupyter-sysml-kernel` 0.61.0) | a feature with two `crosses` clauses reports `Error executing EValidator` instead of `At most one cross subsetting is allowed`: the loop indexes `refSubsettings` (the reference subsettings, collected for the check above it) with the cross-subsetting index, and throws | established from the pinned `KerMLValidator.xtend` line 649 and reproduced with `cmd/pilot-reject/testdata/negative/semantic/k42-two-cross-subsettings.kerml`; the same file is byte-identical at upstream `master` `13c32ea2` (2026-09-01), so the defect is still present; [pilot-rejection.md](pilot-rejection.md#permissiveness-gaps) records the case as a gap of ours | filed upstream as [Systems-Modeling/SysML-v2-Pilot-Implementation#794](https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation/issues/794) **pending adjudication**, body below |
| `org.omg.kerml.xtext` — `KerMLValidator.checkMultiplicityRange`, the `validateMultiplicityRangeResultTypes` check | `2026-07` (`jupyter-sysml-kernel` 0.61.0) | a multiplicity bound naming a package-level feature typed by `ScalarValues::Natural` or `Integer` (`feature k : Natural; feature d [k];`, both owned by a package) reports `Must have a Natural value`; the same bound inside a type (`class T { feature k : Natural; feature d [k]; }`) is accepted | established from the pinned `KerMLValidator.xtend` lines 1333–1339, `FeatureReferenceExpression_modelLevelEvaluable_InvocationDelegate` and `MultiplicityRange_valueOf_InvocationDelegate`: a reference to a feature with no featuring type and no value is deemed model-level evaluable, its evaluation yields the feature rather than a `LiteralInteger`, `valueOf` returns the `-2` null marker and the check reports it; a reference to a type's member is not evaluable and is judged by its type through `isInteger`. The method carries `// TODO: Correct validateMultiplicityBoundResults OCL from KERML-199`. Reproduced with the model below through `validate-kerml`; [pilot-differential.md](pilot-differential.md#multiplicity-bound-result-types-round) records how OpenSysML judges both spellings by the referent's type | **not filed** — drafted below, awaiting maintainer authorisation |
| `org.omg.sysml.logic` — `Type_multiplicity_SettingDelegate`, behind `KerMLValidator.checkFeature`'s `validateFeatureMultiplicityDomain` check | `2026-07` (`jupyter-sysml-kernel` 0.61.0) | a feature whose body holds an `alias` for a `multiplicity` member of the enclosing class, or whose value references one (`class E { multiplicity em [1..4]; feature k : Integer { alias a for em; } }`), reports `Multiplicity must have same featuring types as it feature`, although the feature owns no multiplicity; the spec-genuine violation, a standalone `featuring of C::k::m by D;` on a feature's owned multiplicity, is accepted | established from the pinned `Type_multiplicity_SettingDelegate.getMultiplicityOf` (lines 43–48 at `c7fc737`), which takes the first `Multiplicity` among the members of every `ownedMembership` — aliases and reference memberships included — where KerML 1.1 8.3.3.1.10 `deriveTypeMultiplicity` reads `ownedMember->selectByKind(Multiplicity)`; and from `Feature_featuringType_SettingDelegate.basicGet` (lines 39–51), which reads `ownedTypeFeaturing` where 8.3.3.3.4 `deriveFeatureFeaturingType` reads every `typeFeaturing`. Reproduced with the models below through `validate-kerml`; [validation-constraints.md](validation-constraints.md) records the census row | **not filed** — question drafted below, awaiting maintainer authorisation |
| `org.omg.sysml.logic` — `ConnectorAdapter.getDefaultSupertype`, with `KerMLValidator.checkConnectorBinarySpecialization` | `2026-07` (`jupyter-sysml-kernel` 0.61.0) | a connector owning two ends that redefine two ends of a three-ended general (`connector m : N { end redefines a references x; end redefines b references y; }`) reports `Cannot have more than two ends`, while the same shape spelled as an association (`assoc B specializes N { end redefines a : T; end redefines b : T; }`) is accepted | established from the pinned `ConnectorAdapter.xtend` (`getDefaultSupertype` counts `TypeUtil.getOwnedEndFeaturesOf(target)`, two here, so the connector is given `Links::binaryLinks`) and `KerMLValidator.xtend` (`checkConnectorBinarySpecialization` then counts three `connectorEnd`s), reproduced with the model below through `validate-kerml`; KerML 1.1 8.3.4.5.3 implies `Links::binaryLinks` only for `connectorEnd->size() = 2`, which counts the inherited end. [pilot-differential.md](pilot-differential.md#binary-link-specialization-round) records how OpenSysML counts effective ends for the base | **not filed** — question drafted below, awaiting maintainer authorisation |
| `org.omg.sysml.xtext` — `SysMLValidator`/`KerMLValidator`, invocation argument count | `2026-07` (`jupyter-sysml-kernel` 0.61.0) | an invocation that leaves a default-less `in` parameter unbound validates clean in every form: positional (`F(1.0)`, `F()` against `calc def F { in x : Real; in y : Real; … }`), named (`F(x = 1.0)`, `F(y = 2.0)`), with the omitted parameter declared `[1]` or `[1..*]`, on a calc, a behavior (`Act(1.0).r`, `Act()`), a constructor (`new P()`, `new P(1.0).q`) and an invocation heading a feature chain; only an argument past the last parameter is reported, `Must correspond to one input parameter of the invoked type` (`arity.sysml:38:32` for `F(1.0, 2.0, 3.0)`; `arity.kerml:30:30` for the KerML twin). The pinned evaluator forms and evaluates the same calls: `F(1.0)`, `F(x = 1.0)`, `F(y = 2.0)` and `F()` answer the unreduced `OperatorExpression +`, `F(1.0, 2.0)` `LiteralRational 3.0`, `D(1.0)` (`in y default 1.0`) `2.0`, `Opt(1.0)` (`in y [0..1]`) `1.0` | established by running the pinned `validate-sysml-batch` and `validate-kerml` over a 20-form probe of the above and `build/pilot-evaluator/eval-sysml --cases` over its evaluable rows (transcript below); earlier, `2026-05` (0.60.1) over the whole `airbus/apollo-11-sysml-v2` model at `6e9c93f` was silent on `ln(m0 / mf)` and `calculateDeltaV(isp, initialMass, finalMass)` ([performance.md](../internals/performance.md#a-real-model-apollo-11)), and the pilot's own `kerml-examples/Simple Tests/Behaviors.kerml:14` (`A().y` against `behavior A { in x; … }`) is silent under `validate-kerml` with `ParsingTests_Behaviors.kerml.xt` declaring the file error-free. **Adjudicated as the specification's reading, not a pilot defect:** KerML 1.0 §8.3.4.8.8 lists no `InvocationExpression` constraint on the count of arguments — `validateInvocationExpressionParameterRedefinition` and `…NoDuplicateParameterRedefinition` bound each argument *written* to one input, and the pinned validator's `Must correspond to one input parameter`, `Parameter already bound` and `Must be an in parameter` are those — so the unbound parameter is a property of the instance the call describes, not of the expression. OpenSysML therefore reports the omission as the advisory `unbound-parameter` (a warning in every conformance mode) identically at a bare call and at a chain head, and refuses the evaluation at run time with `ErrUnboundParameter` | **not filed** — a question, not a defect report, drafted below; a maintainer may still want to confirm the reading |
| `org.omg.sysml.xtext` — `SysMLValidator.isDuration`/`isTime`, behind `validateTriggerInvocationActionAfterArgument` and `…AtArgument` | `2026-07` (`jupyter-sysml-kernel` 0.61.0) | with `d : DurationValue` and `t : TimeInstantValue`, `accept after d * d` and `accept at t * t` validate clean although the product has dimension T², while `accept after 10 [m] / 2 [m/s]`, whose quotient has dimension T, is refused | established from the pinned `SysMLValidator` class: an operator argument is a duration or an instant when its operator is one of `-`, `+`, `*`, `%`, `^`, `**` (`isQuantityOperator`) and every operand is itself one — `/` is not in the list and no dimension is computed; reproduced with the pinned batch validator, transcript below | **not filed** — question drafted below, awaiting maintainer authorisation |
| `org.omg.sysml.interactive` — the expression evaluator over `OccurrenceFunctions` | `2026-07` (`jupyter-sysml-kernel` 0.61.0) | `OccurrenceFunctions::'==='(w1, w1)` evaluates to `false` while `w1 === w1` and `BaseFunctions::'==='(w1, w1)` evaluate to `true`; `isDuring(1)` and `isDuring("x")` evaluate to `true`; `create`, `destroy`, `addNew` and `addNewAt` answer their `occ` argument for any argument, an out-of-range `addNewAt` index included | established by evaluating the calls through the pinned pilot's own headless evaluator (`build/pilot-evaluator/eval-sysml --cases`, transcript below): the evaluator folds each declared body over the *declarations* (`x.portionOfLife == y.portionOfLife` over features no value has, `notEmpty(during)` over the function's own feature) rather than over occurrences, so its answers contradict its own operator | **not filed** — question drafted below, awaiting maintainer authorisation |

### `Type::ownedDisjoining` does not contain a `Disjoining` whose `owningType` is that `Type` (pilot `2026-05`)

Filed as
[Systems-Modeling/SysML-v2-Pilot-Implementation#790](https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation/issues/790);
the body below is what was submitted, and the supporting analysis is
[the disjoining diagnostics, one by one](pilot-differential.md#k6-diagnostic-by-diagnostic-f33).
Closed upstream by
[#791](https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation/pull/791), which
confirmed the cause named below — `Type_ownedDisjoining_SettingDelegate.basicGet` filtered on
`getTypeDisjoined() == this`, the delegate, rather than the owning type, so `ownedDisjoining` was
always empty — and the `2026-08` release carries the fix: the six differential rows are gone and
the three-line reproducer validates clean.

````markdown
### Every `disjoint from` clause in a type declaration reports an unpaired bidirectional reference

**Version:** `2026-05` (validated through `jupyter-sysml-kernel` 0.60.1, the KerML
standalone setup + `SysMLUtil`).

#### Minimal reproduction

`Decl.kerml`, complete — no imports, no library references:

```kerml
package Decl {
    classifier A;
    classifier B disjoint from A;
}
```

Validate it on its own, in a fresh resource set.

#### Expected

No diagnostics. `disjoint from` in a type declaration is
`DisjoiningPart` (`org.omg.sysml.xtext/src/org/omg/sysml/xtext/KerML.xtext:344`,
reached from `TypeRelationshipPart` at `:340`), and this is how the shipped
example models write it — six of the `.kerml` files under
`org.omg.sysml.examples`/`kerml-examples` use exactly this clause
(`Simple Tests/Types.kerml:31`, `Simple Tests/Classifiers.kerml:13`,
`Simple Tests/Features.kerml:20`, `Simple Tests/Inverses.kerml:3`,
`Simple Tests/FeatureChains.kerml:31`,
`KerML Spec Annex A Examples/A-2-ModelingInstances.kerml:9`).

#### Actual

One error per clause, on the clause's line:

```
The opposite features 'owningType' of 'org.omg.sysml.lang.sysml.impl.DisjoiningImpl{Simple Tests/Types.kerml#//@ownedRelationship.0/@ownedRelatedElement.0/@ownedRelationship.14/@ownedRelatedElement.0/@ownedRelationship.1}' and 'ownedDisjoining' of 'org.omg.sysml.lang.sysml.impl.TypeImpl{Simple Tests/Types.kerml#//@ownedRelationship.0/@ownedRelatedElement.0/@ownedRelationship.14/@ownedRelatedElement.0}' do not refer to each other
```

This is EMF's `_UI_UnpairedBidirectionalReference_diagnostic`, raised by
`EObjectValidator` over an `EReference` pair — not a `KerMLValidator` rule — so
it is a statement about the loaded object graph rather than about the model.
All six example files above report it; the parse itself succeeds, and the
standalone form `disjoint b.f.a from b.a;`
(`Simple Tests/FeatureChains.kerml:28`) does not report it. It is not a batching
artifact: each file reproduces the diagnostic when validated alone in a fresh
resource set.

#### Mechanism

`Disjoining::owningType` declares `eOpposite="#//Type/ownedDisjoining"` in
`org.omg.sysml/model/SysML.ecore`, and `Type::ownedDisjoining` is derived,
transient and volatile — its setting delegate selects the `Type`'s
`ownedRelationship`s that are `Disjoining`s whose `typeDisjoined` is that
`Type`. Probing the reproducer's loaded model through the pilot's own API gives:

```
Disjoining //@ownedRelationship.0/@ownedRelatedElement.0/@ownedRelationship.1/@ownedRelatedElement.0/@ownedRelationship.0
  owner                = ClassifierImpl(B)
  owningRelatedElement = ClassifierImpl(B)
  typeDisjoined        = ClassifierImpl(B)
  disjoiningType       = ClassifierImpl(A)
  owningType           = ClassifierImpl(B)
  owner.ownedDisjoining        = []
  owner.ownedRelationship size= 1
    rel DisjoiningImpl same=true
```

`B.ownedRelationship` contains the `Disjoining`, that `Disjoining`'s
`typeDisjoined` and `owningType` are both `B` — and yet the derived
`B.ownedDisjoining`, the other end of the `eOpposite` pair, is empty. So the
delegate does not return a `Disjoining` that satisfies its own documented
derivation, and EMF's check on the pair then fails for every `disjoint from`
clause written in a type declaration.

`OwnedDisjoining` (`KerML.xtext:437`) sets only `disjoiningType`; the owned form
leaves `typeDisjoined` to be the owning type, which the standalone `Disjoining`
production (`:426`) instead names explicitly — consistent with the standalone
form being unaffected.
````

#### Does a fix upstream clear the whole `kerml-examples` column?

Yes, and it was checked file by file rather than by family, since the answer
decides whether this root's pilot-only rows are ours to act on at all. Each of
the six files contributes **exactly one** pilot-only row, each row is the EMF
pair diagnostic above, and each line is a `disjoint from` clause written in a
type declaration — the form the mechanism section pins to `OwnedDisjoining`. The
per-clause table is in
[the row-by-row sweep of pilot-only diagnostics](pilot-differential.md#only-the-pilot--the-row-by-row-sweep-137). The clause appears on classifiers, plain types and features alike and the
reported EMF class tracks the declaration, so the defect is in the pair rather
than in one metaclass; the standalone form `disjoint b.f.a from b.a;`
(`Simple Tests/FeatureChains.kerml:28`) sits in the same file as one of the six
and reports nothing. No `kerml-examples` file carries a second pilot-only row of
any kind, so a fix to the derived `ownedDisjoining` delegate clears this root's
column entirely and silences nothing else it depends on.

---

### Are the `queryx/failing` query path expressions intended notation? (pilot `2026-05`)

**Not filed.** Drafted here for a maintainer to authorise; nothing has been
posted upstream. The adjudication is in
[the parser-recovery decisions](adjudications.md).

````markdown
**Question, not a bug report:** three Xpect fixtures under
`sysml/src/org/omg/sysml/xpect/tests/queryx/failing/` declare file-wide silence
(`// XPECT noErrors ---> ""`) while the pinned release's own validator rejects
them. Is the notation planned for a later version, or are the fixtures kept as a
record of a proposal that the grammar deliberately does not admit?

The forms are

```sysml
value v1_i: Integer[0..*] = .*/.*[Integer];       // QPE-Qualifier
value v_redefining: Integer = ./vehicle_1/cylinders/@redefining;  // QPE-Traversal
value vw_recursive: Integer[0..*] = .**/cylinders;  // QPE-Wildcard
```

Running the release's SysML validator (`jupyter-sysml-kernel-0.60.1-all.jar`,
tag `2026-05`) over the three model bodies reports, among others:

```
QPE-Wildcard.sysml:9:38: error: For input string: "."
QPE-Wildcard.sysml:9:40: error: no viable alternative at input '/'
QPE-Traversal.sysml:7:57: error: no viable alternative at input '@'
QPE-Qualifier.sysml:9:40: error: no viable alternative at input '/'
```

Their `XPECT_SETUP` names `org.omg.sysml.xpect.tests.query.failing.SysMLQueryFailingTest`
while the runner in the directory is `org.omg.sysml.xpect.tests.queryx.failing.SysMLQueryFailingTest`
and extends `KerMLXtextTests`.

A second implementation reading the corpus cannot tell from the fixtures alone
whether the declared silence is an obligation or an aspiration, which is the
reason for asking rather than implementing.
````

### `validateFeatureOwnedCrossSubsetting` indexes the wrong list and throws (pilot `2026-07`)

Filed as
[Systems-Modeling/SysML-v2-Pilot-Implementation#794](https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation/issues/794);
the body below is what was submitted. The reproduction is the rejection-corpus case
`cmd/pilot-reject/testdata/negative/semantic/k42-two-cross-subsettings.kerml`.
Checked against upstream `master` at `13c32ea26680323921c14e76755897fc551ec258`
(2026-09-01): `KerMLValidator.xtend` is byte-identical to the pinned `2026-07`
copy and the tag-to-master diff touches no validation or grammar source, so the
reproduction below stands for the current head as well as for the pin (no
master build was run — Maven Central was unreachable from the sandbox).

````markdown
### A feature with two `crosses` clauses reports `Error executing EValidator`

**Version:** `2026-07` (`jupyter-sysml-kernel` 0.61.0, the KerML standalone
setup); the offending lines are unchanged on `master` at `13c32ea2`.

#### Minimal reproduction

```kerml
package K42TwoCrossSubsettings {
    class A {
        feature x : A;
        feature y : A;
    }
    assoc S {
        end a : A;
        end b : A crosses a.x crosses a.y;
    }
}
```

The grammar admits the second `crosses` (`FeatureSpecializationPart` repeats
`FeatureSpecialization`), and `validateFeatureOwnedCrossSubsetting` is meant to
report it as `At most one cross subsetting is allowed`. Instead the validator
reports

```
k42-two-cross-subsettings.kerml:0:0: error: Error executing EValidator
k42-two-cross-subsettings.kerml:9:39: error: The opposite features 'crossingFeature' of '...CrossSubsettingImpl{...@ownedRelationship.2}' and 'ownedCrossSubsetting' of '...FeatureImpl{...}' do not refer to each other
```

The second line is EMF's opposite-consistency check on the extra
`CrossSubsetting` (`Feature::ownedCrossSubsetting` is single-valued), not the
intended constraint message. Dropping the second clause (`end b : A crosses a.x;`)
makes the model validate clean, so the second `crosses` is the only defect.

#### Cause

In `KerMLValidator.checkFeature` (`KerMLValidator.xtend`, the
`validateFeatureOwnedCrossSubsetting` block):

```xtend
val crossSubsettings = f.ownedRelationship.filter[r | r instanceof CrossSubsetting].toList
if (crossSubsettings.size > 1) {
    for (var i = 1; i < crossSubsettings.size; i++)
        error(INVALID_FEATURE_OWNED_CROSS_SUBSETTING_MSG, refSubsettings.get(i), null, INVALID_FEATURE_OWNED_CROSS_SUBSETTING)
}
```

`refSubsettings.get(i)` reads the reference-subsetting list collected for the
`validateFeatureOwnedReferenceSubsetting` check just above; with no `references`
clause on the feature that list is empty and the `get(1)` throws, which Xtext
surfaces as `Error executing EValidator`. The intended target is
`crossSubsettings.get(i)`.
````

---

### A non-Boolean transition guard is accepted with the full library loaded (pilot `2026-07`)

**Not filed.** Drafted here for a maintainer to authorise; nothing has been
posted upstream. The observation is recorded under [constraints the pilot
declares but does not enforce](pilot-rejection.md#constraints-the-pilot-declares-but-does-not-enforce).

````markdown
**Question, not a bug report:** `validateTransitionFeatureMembershipGuardExpression`
is implemented in `SysMLValidator.checkTransitionFeatureMembership` and
`validation/invalid/TransitionUsage_invalid.sysml.xt` expects its message:

```sysml
transition
    first S2_1
    // XPECT errors ---> "Must be a Boolean expression." at "if \"test\""
    if "test"
    then S2_2;
```

That fixture's `XPECT_SETUP` loads a reduced resource set (`Base`, `Occurrences`,
`Performances`, `States`, ... but not `ScalarValues`). Running the release's
SysML validator (`jupyter-sysml-kernel-0.61.0-all.jar`, tag `2026-07`) with the
full standard library over the same shape reports no error:

```sysml
package T2 {
    state def S2 {
        state S2_1;
        transition first S2_1 if "test" then S2_2;
        state S2_2;
    }
    state def S3 {
        state a;
        state b;
        transition t first a if 1 + 2 then b;
    }
}
```

The same silence covers every other non-Boolean guard tried: `accept … if 1 then`,
`transition if 2.5 then`, an enumeration literal, a part-typed attribute, a `String`-valued
calculation, and the guarded successions of an action body (`first a if "go" then b`, a
decision's `if e then b`).

The cause appears to be `ExpressionAdapter.getRelevantFeatures` (`EXPRESSION_GUARD_FEATURE`):
a transition guard implicitly redefines `TransitionPerformances::TransitionPerformance::guard`,
declared `bool guard[*]`, so `KerMLValidator.isBoolean` finds a `Boolean`-typed result on
every guard by construction and `checkTransitionFeatureMembership` cannot fail. The reduced fixture library
has no `TransitionPerformances`, which is why the Xpect expectation holds there.

Is the guard check intended to fire in a full-library workspace? A second
implementation that rejects `if "test"` with the full library loaded, as the
fixture suggests it should, currently disagrees with the release's validator on
the same text.
````

---

### A trigger's time arithmetic is judged by its operator, not its dimension (pilot `2026-07`)

**Not filed.** Drafted here for a maintainer to authorise; nothing has been
posted upstream. OpenSysML judges the argument by the dimension of its value
(`spec-compliance.md`, the `after`/`at` trigger rows), so the two implementations
disagree on the shapes below in both directions.

````markdown
**Question, not a bug report:** `SysMLValidator.isDuration` and `isTime`, which
`checkTriggerInvocationExpression` uses for `validateTriggerInvocationActionAfterArgument`
and `…AtArgument`, admit an `OperatorExpression` when its operator is one of
`-`, `+`, `*`, `%`, `^`, `**` and every operand is itself a duration or a time
instant. With the release's validator (`jupyter-sysml-kernel-0.61.0-all.jar`,
tag `2026-07`) and the full standard library:

```sysml
package T {
    private import ISQ::*;
    private import Time::*;
    private import SI::*;
    attribute d : DurationValue;
    attribute t : TimeInstantValue;
    state def S {
        state s1; state s2;
        transition first s1 accept after d * d then s2;             // no error: dimension T²
        transition first s1 accept at t * t then s2;                // no error: dimension T²
        transition first s1 accept after 10 [m] / 2 [m/s] then s2;  // An after expression must be a DurationValue: dimension T
    }
}
```

Is the operator list the intended reading of `checkTriggerInvocationExpressionAfterArgument`
(the argument's result conforms to `ISQ::DurationValue`)? A product of two durations
is not a duration, and a quotient of a length by a speed is one; a second
implementation that judges the value's dimension accepts the last line and
refuses the first two.
````

---

### A quantity value is accepted as the unit of a quantity (pilot `2026-07`)

**Not filed.** Drafted here for a maintainer to authorise; nothing has been
posted upstream. OpenSysML judges the nested quantity by its value type
(`validation-constraints.md`, `validateOperatorExpressionQuantity`), so the two
implementations disagree on the shape below.

````markdown
**Question, not a bug report:** `SysMLValidator.checkOperatorExpression` judges
the unit of `x [u]` with `resultConformsTo(u, TensorMeasurementReference)`,
which admits an `OperatorExpression` whose declared result is a supertype of
the wanted type when one of its arguments conforms. `BaseFunctions::'['` returns
`Anything`, so a quantity value nested as the unit passes for the measurement
reference it was built from. With the release's validator
(`jupyter-sysml-kernel-0.61.0-all.jar`, tag `2026-07`) and the full standard
library:

```sysml
package Q {
    private import ISQ::*;
    private import SI::*;
    attribute a = 10 [2 [m]];          // no warning: `m` is an argument of `[`
    attribute b = 10 [(m, 3)];         // no warning: `m` is an element
    attribute c = 10 [if true ? m else s];  // Should be a measurement reference (unit).
}
```

Is the existential reading of `resultConformsTo` intended for `[`? `2 [m]` is a
`ScalarQuantityValue`, not a measurement reference, and a second implementation
that judges the value's type warns on `a`; it agrees on `b` (a sequence whose
elements are units) and on `c` (the branches of a conditional are expression
bodies, so its result is `Anything` and no argument is a unit).
````

---

### Eight control-node succession constraints are unimplemented `TODO`s (pilot `2026-07`)

**Not filed.** Drafted here for a maintainer to authorise; nothing has been
posted upstream. The rules are implemented on our side by
`internal/core/passes/control_node.go` and refereed against the specification
text; the adjudication is in
[pilot-differential.md](pilot-differential.md#control-node-successions-the-pilot-does-not-validate).

````markdown
### `SysMLValidator` does not check the succession constraints on control nodes

**Version:** `2026-07` (`jupyter-sysml-kernel` 0.61.0, `validate-sysml-batch` over the
shipped standard library).

SysML v2 8.3.17 (`ControlNode`, `DecisionNode`, `ForkNode`, `JoinNode`, `MergeNode`)
declares nine validation constraints. `SysMLValidator.xtend` (`:857–888`) declares an
error code for each, but implements only `validateControlNodeOwningType`; the other
eight are `// TODO: Check validate… (?)` comments in otherwise empty `@Check` methods:
`validateControlNodeIncomingSuccessions`, `validateControlNodeOutgoingSuccessions`,
`validateDecisionNodeIncomingSuccessions`, `validateDecisionNodeOutgoingSuccessions`,
`validateForkNodeIncomingSuccessions`, `validateJoinNodeOutgoingSuccessions`,
`validateMergeNodeIncomingSuccessions`, `validateMergeNodeOutgoingSuccessions`.

#### Minimal reproduction

```sysml
package ForkTwoIncoming {
    action def A {
        action a;
        action b;
        action c;
        fork f;
        first a then f;
        first b then f;
        first f then c;
    }
}
```

`f` has two incoming successions; `validateForkNodeIncomingSuccessions`
(`targetConnector->selectByKind(Succession)->size() <= 1`, SysML v2 8.3.17 `ForkNode`) is
violated.

#### Expected

An error on `fork f`.

#### Actual

No diagnostics. The same holds for a join or merge node with two outgoing successions, a
decision node with two incoming ones, and for the end multiplicities — `succession s first
[0..1] a then [1] m;` into a merge is accepted where `validateMergeNodeIncomingSuccessions`
requires source multiplicity `0..1`, and `succession s first a then [0..1] f;` into a fork
is accepted where `validateControlNodeIncomingSuccessions` requires target multiplicity
`1..1`.

#### Note

The grammar admits every one of these models, and the specification says the rules "shall
be enforced in the abstract syntax, even if not shown explicitly in the concrete syntax
notation for a model" (7.17.3), so a validator is the only place they can be caught. One
reading question may be behind the `(?)` on the `TODO` lines: `multiplicityHasBounds`
requires `mult <> null`, and a connector end written without a multiplicity (`first a then
f;`) is given none by the pilot's `SuccessionAdapter`/`ConnectorAdapter`, so a literal evaluation of the four
multiplicity constraints would reject the specification's own examples. Treating an
unwritten end multiplicity as the required one, and checking only written ones, is what a
second implementation has to assume; a note in the release on the intended reading would
help.
````

---

### A bound naming a package-level feature is rejected whatever its type (pilot `2026-07`)

**Not filed.** Drafted here for a maintainer to authorise; nothing has been
posted upstream. OpenSysML reads the referenced feature's declared type
(`internal/core/passes/w8c_multiplicity_bounds.go`) wherever the feature is
owned and rejects only a bound whose type does not conform to `Integer`; the
adjudication is in
[pilot-differential.md](pilot-differential.md#multiplicity-bound-result-types-round).

````markdown
### `validateMultiplicityRangeResultTypes` rejects a bound that names a package-level feature

**Version:** `2026-07` (`jupyter-sysml-kernel` 0.61.0, `validate-kerml` over the shipped
standard library).

```kerml
package P {
    private import ScalarValues::*;
    feature k : Natural;
    feature d [k];
    class T {
        feature k : Natural;
        feature d [k];
    }
}
```

reports `Must have a Natural value` at line 4 (`feature d [k];` in the package) and nothing at
line 7 (the same declaration inside `T`). Both `k` are typed by `Natural`, so under KerML 1.1
8.3.3.6 both bounds have a Natural-conforming result. The difference is where the bound is sent
by `KerMLValidator.checkMultiplicityRange`: `k` inside `T` has a featuring type, so the reference
is not model-level evaluable and `isInteger` judges it by its type; the package-level `k` has no
featuring type and no value, so `FeatureReferenceExpression.modelLevelEvaluable` answers true,
`evaluate` yields the feature itself rather than a `LiteralInteger`, `MultiplicityRange.valueOf`
returns its `-2` null marker, and the result-types error fires. The method is annotated
`// TODO: Correct validateMultiplicityBoundResults OCL from KERML-199`. Is a package-level
feature meant to be a valid bound (judged by its type like a member), or is a bound that cannot
be evaluated to a literal meant to be rejected?
````

---

### A multiplicity is found through aliases and references (pilot `2026-07`)

**Not filed.** Drafted here for a maintainer to authorise; nothing has been
posted upstream. OpenSysML does not check `validateFeatureMultiplicityDomain`
at all — the census row in
[validation-constraints.md](validation-constraints.md) records it as not
implemented — so there is no adjudication of ours to point at, only the
pilot's behaviour against the clause.

````markdown
### `validateFeatureMultiplicityDomain` fires on an alias or a reference to a class's multiplicity, and not on a foreign featuring

**Version:** `2026-07` (`jupyter-sysml-kernel` 0.61.0, `validate-kerml` over the shipped
standard library).

```kerml
package P {
    private import ScalarValues::*;
    class E {
        multiplicity em [1..4];
        feature k : Integer { alias a for em; }
        feature v : Natural = em;
    }
}
```

reports `Multiplicity must have same featuring types as it feature` at line 5 (`feature k`) and
line 6 (`feature v`). Neither feature owns a multiplicity: `k` owns an alias `Membership` whose
`memberElement` is `em`, and `v` owns a `FeatureValue` whose `FeatureReferenceExpression` holds
`em` through a reference `Membership`. `Type_multiplicity_SettingDelegate.getMultiplicityOf` takes
the first `Multiplicity` among `ownedMembership.memberElement`, which finds `em` in both cases,
and `checkFeature` then compares `em`'s featuring types with the feature's: `em` is a classifier
multiplicity, so it has none (as `validateClassifierMultiplicityDomain` requires), while `k` and
`v` are featured by `E`, and the error is reported. KerML 1.1 8.3.3.1.10 derives `multiplicity`
from `ownedMember->selectByKind(Multiplicity)`, under which neither feature has a multiplicity
and the constraint is vacuous; the model is valid.

Conversely,

```kerml
package P {
    private import ScalarValues::*;
    class C { feature k : Integer { multiplicity m [1..2]; } }
    class D;
    featuring of C::k::m by D;
}
```

validates clean, although `m` is `k`'s multiplicity and its `featuringType` under 8.3.3.3.4
(`typeFeaturing.featuringType`, every `TypeFeaturing` counted — the one
`checkMultiplicityTypeFeaturing` implies from `k` and the written one) is `{C, D}` while `k`'s is
`{C}`. `Feature_featuringType_SettingDelegate.basicGet` reads only `ownedTypeFeaturing` plus the
adapter's implicit featuring types, so the standalone `featuring` relationship does not reach the
check. Should `Type::multiplicity` be
derived from `ownedMember` rather than from every owned membership's member, and should
`Feature::featuringType` include non-owned `TypeFeaturing`s?
````

---

### A connector inheriting a third end is given the binary base (pilot `2026-07`)

**Not filed.** Drafted here for a maintainer to authorise; nothing has been
posted upstream. OpenSysML counts a declaration's effective ends — owned and
inherited — when choosing between `Links::links` and `Links::binaryLinks`
(`internal/core/semantics/implicit.go`, `connector.go`); the adjudication is in
[pilot-differential.md](pilot-differential.md#binary-link-specialization-round).

````markdown
### `ConnectorAdapter.getDefaultSupertype` makes a connector with an inherited third end binary

**Version:** `2026-07` (`jupyter-sysml-kernel` 0.61.0, `validate-kerml` over the shipped
standard library).

```kerml
package P {
    class T;
    assoc N { end a : T; end b : T; end c : T; }
    assoc B specializes N { end redefines a : T; end redefines b : T; }
    class C {
        feature x : T; feature y : T; feature z : T;
        connector m : N { end redefines a references x; end redefines b references y; }
    }
}
```

reports `Cannot have more than two ends` at line 7 (`connector m`) and accepts line 4
(`assoc B`). Both declarations own two ends that redefine `a` and `b` and inherit `c`, so each
has three ends. `ConnectorAdapter.getDefaultSupertype` chooses between `Links::links` and
`Links::binaryLinks` by `TypeUtil.getOwnedEndFeaturesOf(target).size()`, which is two, so `m`
implicitly subsets `binaryLinks`; `checkConnectorBinarySpecialization` then finds three
`connectorEnd`s on a `BinaryLink`-conforming connector and reports the error. KerML 1.1
8.3.4.5.3 (`validateConnectorBinarySpecialization`) implies `Links::binaryLinks` only when
`connectorEnd->size() = 2`, and `connectorEnd` includes inherited ends, so the specification
leaves `m` n-ary. `AssociationAdapter` counts owned ends the same way, but
`checkAssociationBinarySpecialization` inspects owned ends only, so `B` escapes. Should the
adapters count the derived `connectorEnd`/`associationEnd` rather than the owned end features,
or is a subtype meant to redefine every end of its general?
````

---

### `OccurrenceFunctions` are evaluated over declarations rather than occurrences (pilot `2026-07`)

**Not filed.** Drafted here for a maintainer to authorise; nothing has been
posted upstream. OpenSysML's own semantics for the six functions are recorded in
[spec-compliance.md](spec-compliance.md) (the *Occurrences have a lifetime* rows).

The probe model and the pinned evaluator's verbatim answers
(`jupyter-sysml-kernel-0.61.0-all.jar`, tag `2026-07`, through
`build/pilot-evaluator/eval-sysml --cases`; the evaluator exited 0, every answer is
its own):

```sysml
package OccProbe {
    private import ScalarValues::*;
    private import OccurrenceFunctions::*;
    private import SequenceFunctions::*;
    part def Widget { attribute mass : Real = 1.0; }
    part w1 : Widget;
    part w2 : Widget;
    part group : Widget[0..*] ordered nonunique;
}
```

| Expression | Pilot answer |
|---|---|
| `OccProbe::w1 === OccProbe::w1` | `LiteralBoolean true` |
| `OccProbe::w1 === OccProbe::w2` | `LiteralBoolean false` |
| `OccProbe::w1 !== OccProbe::w2` | `LiteralBoolean true` |
| `BaseFunctions::'==='(OccProbe::w1, OccProbe::w1)` | `LiteralBoolean true` |
| `OccurrenceFunctions::'==='(OccProbe::w1, OccProbe::w1)` | `LiteralBoolean false` |
| `OccurrenceFunctions::'==='(OccProbe::w1, OccProbe::w2)` | `LiteralBoolean false` |
| `OccurrenceFunctions::'==='(null, null)` | `LiteralBoolean true` |
| `OccurrenceFunctions::isDuring(OccProbe::w1)` | `LiteralBoolean true` |
| `OccurrenceFunctions::isDuring(1)` | `LiteralBoolean true` |
| `OccurrenceFunctions::isDuring("x")` | `LiteralBoolean true` |
| `OccurrenceFunctions::create(OccProbe::w1)` | `PartUsage w1` |
| `OccurrenceFunctions::create(1)` | `LiteralInteger 1` |
| `OccurrenceFunctions::destroy(OccProbe::w1)` | `PartUsage w1` |
| `OccurrenceFunctions::destroy(null)` | (nothing) |
| `OccurrenceFunctions::addNew(OccProbe::group, OccProbe::w1)` | `PartUsage w1` |
| `OccurrenceFunctions::addNewAt(OccProbe::group, OccProbe::w1, 1)` | `PartUsage w1` |
| `OccurrenceFunctions::addNewAt(OccProbe::group, OccProbe::w1, 5)` | `PartUsage w1` |
| `OccurrenceFunctions::addNewAt((1, 2), 3, 9)` | `LiteralInteger 3` |
| `SequenceFunctions::includingAt((1, 2), 3, 5)` | `EXCEPTION:java.lang.IndexOutOfBoundsException: toIndex = 4` |

````markdown
**Question, not a bug report:** the interactive evaluator answers the
`OccurrenceFunctions` declarations by folding their declared bodies over the
model elements the arguments name, which gives answers that contradict its own
operators. With `part w1 : Widget;` in scope, `w1 === w1` and
`BaseFunctions::'==='(w1, w1)` are `true` but `OccurrenceFunctions::'==='(w1, w1)`
is `false` — the body `x.portionOfLife == y.portionOfLife` is evaluated over
features that hold no value. `isDuring(1)` and `isDuring("x")` are `true`: the
body `notEmpty(during)` is evaluated over the function's own `during` feature
rather than the argument's lifetime, so a data value that is no occurrence is
reported as happening during. `create`, `destroy`, `addNew` and `addNewAt` answer
their `occ` argument for any argument, an `addNewAt` index past the group's end
included, while `SequenceFunctions::includingAt` with the same index throws
`IndexOutOfBoundsException`. Is the evaluator intended to answer these six at all
outside an executing performance? If so, is `OccurrenceFunctions::'==='` intended
to agree with the `===` operator, and `isDuring` to reject an argument that is
not an `Occurrence`, as the declared parameter types say?
````

### An invocation leaving an input parameter unbound validates clean (pilot `2026-07`)

**Not filed.** Drafted here for a maintainer to authorise; nothing has been
posted upstream, to the pilot or to any model's repository. OpenSysML has
adjudicated the pilot's behaviour as the specification's reading (see the row
above), so this is a request to confirm that reading, not a defect report.

````markdown
**Question, not a bug report:** is an `InvocationExpression` that leaves an
input parameter of the invoked type unbound — no argument, no `default`, lower
multiplicity bound 1 — meant to validate clean? KerML 8.3.4.8.8 constrains the
arguments an invocation *writes* (`validateInvocationExpressionParameterRedefinition`,
`validateInvocationExpressionNoDuplicateParameterRedefinition`) and states no
constraint on the parameters it leaves out, so we read the omission as a property
of the instance the expression describes (the parameter has no value) rather than
of the expression, and report it as an advisory only. Is that the intended reading?

With `jupyter-sysml-kernel-0.61.0-all.jar` (tag `2026-07`), the SysML and KerML
validators report nothing for any of these:

```sysml
calc def F { in x : Real; in y : Real; return : Real = x + y; }
calc def One { in x : Real; in y : Real [1]; return : Real = x + y; }
calc def Many { in x : Real; in y : Real [1..*]; return : Real = x; }
action def Act { in a : Real; in b : Real; out r : Real; }
part def P { attribute p : Real; attribute q : Real; }

attribute f0 = F();              attribute f1 = F(1.0);
attribute fn1 = F(x = 1.0);      attribute fn2 = F(y = 2.0);
attribute one1 = One(1.0);       attribute many1 = Many(1.0);
attribute act0 = Act().r;        attribute act1 = Act(1.0).r;
ref c0 = new P();                attribute c1c = new P(1.0).q;
```

The only diagnostic on the probe is the one expected of the argument past the
last parameter:

```
arity.sysml:38:32: error: Must correspond to one input parameter of the invoked type   -- F(1.0, 2.0, 3.0)
arity.kerml:30:30: error: Must correspond to one input parameter of the invoked type   -- the KerML twin
```

The release's evaluator (`eval-sysml`) forms and evaluates the same calls, so
the expression is treated as well formed end to end:

```
F(1.0)        -> OperatorExpression +      F(1.0, 2.0)  -> LiteralRational 3.0
F(x = 1.0)    -> OperatorExpression +      D(1.0)       -> LiteralRational 2.0   (in y default 1.0)
F(y = 2.0)    -> OperatorExpression +      Opt(1.0)     -> LiteralRational 1.0   (in y [0..1])
F()           -> OperatorExpression +
```

A model author would presumably still want to hear about `F(1.0)`: the public
`airbus/apollo-11-sysml-v2` model (commit `6e9c93f`) computes
`isp * g0 * ln(m0 / mf)` against a two-input `naturalLogarithm` and
`calculateDeltaV(isp, initialMass, finalMass)` against four inputs, and both
validate clean. Is an advisory the pilot would consider, or is silence the
intended reading?
````

---

### A classifier's multiplicity is found through an alias (pilot `2026-07`)

**Not filed.** Drafted here for a maintainer to authorise; nothing has been
posted upstream. OpenSysML derives a type's multiplicity from its owned members
only, so an alias of a feature's multiplicity does not become the classifier's
own, and no diagnostic is drawn; the census row
(`validateClassifierMultiplicityDomain` in [validation-constraints.md](validation-constraints.md))
records the disagreement. The same delegate is behind the feature-side draft
[above](#a-multiplicity-is-found-through-aliases-and-references-pilot-2026-07); the two
belong in one report.

````markdown
### `Type.multiplicity` is derived through alias memberships, so `validateClassifierMultiplicityDomain` fires on a valid alias

**Version:** `2026-07` (`jupyter-sysml-kernel` 0.61.0, `validate-kerml` over the shipped
standard library).

```kerml
package P {
    class T;
    class K { feature f : T { multiplicity m [1..2]; } }
    class C { alias m for K::f::m; }
}
```

reports `Multiplicity must not have a featuring type` at line 4 (`class C`). `C` owns no
multiplicity: its only owned membership is an alias whose `memberElement` is `K::f::m`, a
multiplicity featured by `f`. KerML 1.1 8.3.3.1 derives `Type::multiplicity` as
`ownedMember->selectByKind(Multiplicity)->any(true)`, and `ownedMember` is derived from
`ownedMembership.ownedMemberElement`, which an alias membership does not have, so the
specification leaves `C.multiplicity` empty and `validateClassifierMultiplicityDomain`
satisfied. `Type_multiplicity_SettingDelegate.getMultiplicityOf` instead maps
`ownedMembership` through `Membership::getMemberElement`, which follows the alias to `f`'s
multiplicity, and `KerMLValidator.checkClassifier` then finds its featuring type. The same
alias inside a subclass (`class F :> K { alias mf for f::m; }`) and inside a `struct` are
reported alike. Should the delegate read `ownedMemberElement` (owned memberships only), as
the derivation says?
````

---

### A conjugated classifier is judged against its generic default only (pilot `2026-07`)

**Not filed.** Drafted here for a maintainer to authorise; nothing has been
posted upstream. OpenSysML checks a conjugated classifier against every default
base its kind and end count imply (`internal/core/passes/w11e_implicit_base.go`);
the census row (`validateClassifierDefaultSupertype` in
[validation-constraints.md](validation-constraints.md)) records the difference as
⚠️ approximate.

````markdown
### `checkClassifier` validates a conjugated association against `Links::Link` but not `Links::BinaryLink`

**Version:** `2026-07` (`jupyter-sysml-kernel` 0.61.0, `validate-kerml` over the shipped
standard library).

```kerml
package P {
    class T;
    assoc struct AS ~ Objects::LinkObject { end feature a : T[1]; end feature b : T[1]; }
    assoc A ~ Links::Link { end feature a : T[1]; end feature b : T[1]; }
    interaction I ~ Links::Link { end feature a : T[1]; end feature b : T[1]; }
}
```

validates clean. A conjugated type owns no specialization
(`validateSpecializationSpecificNotConjugated`) and `TypeAdapter.computeImplicitGeneralTypes`
adds none for it, so the only supertypes these three have are the ones reached through the
conjugated type. `checkClassifier` tests
`ImplicitGeneralizationMap.getDefaultSupertypeFor(c.getClass())`, the `base` entry alone —
`Objects::LinkObject`, `Links::Link`, and for `InteractionImpl` (a Java subclass of
`AssociationImpl`) again `Links::Link` — all satisfied here. KerML 1.1 8.3.4.7
`checkAssociationBinarySpecialization` and `checkAssociationStructureBinarySpecialization`
require a two-end association to specialize `Links::BinaryLink` / `Objects::BinaryLinkObject`,
and an interaction is a performance as well as a link (7.4.10.2), so it must reach
`Performances::Performance` too; none of these declarations do. Is the direct check
meant to cover only the generic default, leaving the `binary` and behavioral bases to the
implicit-specialization machinery that conjugation switches off?
````

---

### Repeated anonymous `perform a;` members are distinguishable or not by whether they have bodies (pilot `2026-07`)

**Not filed.** Drafted here for a maintainer to authorise; nothing has been
posted upstream. OpenSysML reports the two `Duplicate of … member name` warnings
on every repeated anonymous performed or exhibited use (`internal/core/resolve/distinguishability.go`),
as the pilot does when the uses have bodies; the Name Resolution map in
[spec-compliance.md](spec-compliance.md) records the bodiless case as a pilot
artefact.

````markdown
### `validateNamespaceDistinguishability` misses repeated anonymous performed uses unless they have bodies

**Version:** `2026-07` (`jupyter-sysml-kernel` 0.61.0, `validate-sysml-batch`; each file run four
times with the same result).

```sysml
package P {
    part def H { action a; }
    part h : H { perform a; }                                   // (1)
    part h2 : H { perform a; perform a; }                       // (2)
    part h3 : H { perform a; perform a; perform a; }            // (3)
    part h4 : H { perform a { attribute i; } perform a { attribute j; } }   // (4)
    part h5 : H { perform a { attribute i; } perform a; }       // (5)
    part h6 : H { exhibit s; exhibit s; }                       // (6), with `state s;` in H
}
```

Each `perform a;` is an unnamed `PerformActionUsage` whose effective name is `a`, the name of
the action it references (KerML 7.3.4.5, SysML v2 7.16.4), so every one of them duplicates the
inherited `H::a` and, where repeated, its siblings. With each `part` in a file of its own next to
`H`:

- (1) `warning: Duplicate of inherited member name 'a' from H` at the `perform` — as expected;
- (4) and (5) `Duplicate of other owned member name` **and** `Duplicate of inherited member name
  'a' from H` on each of the two uses — as expected;
- (2) and (6) **no warning at all**, the inherited duplicate of (1) included;
- (3) a single `Duplicate of inherited member name 'a' from H` on the *third* use only.

The KerML spellings behave consistently: `feature :>> a; feature :>> a;` reports the two
owned duplicates with or without bodies, and `feature ::> a;` never names anything. The
SysML result appears to depend on the order in which the effective names are computed while
the references are still being linked — computing `memberName` for one use resolves `a` in
`h2`, which asks the sibling use for *its* `memberName`, which resolves `a` again — rather than
on anything in the model. Is the bodiless outcome intended, or should (2), (3) and (6) report
what (4) and (5) do?
````

---

## The errata overlay entries for these models

The second section's rows are also entries of the declared errata overlay
(`internal/errata`, [the declared errata overlay](errata-overlay.md)): the published bytes on
disk are never edited, and a row that carries a correction has that correction
applied to the *second* figure every oracle reports, never to the headline one.
The overlay adds no category and reclassifies nothing — the two quantity rows stay the adjudicated
commensurability family recorded in the false-positive audit.

An entry is accepted only with a specification citation and a written
derivation, and only while its as-published text still matches the corpus on
disk; both are tests (`internal/errata`), not conventions. A defect with no
unambiguous intended reading is documented **without** a correction rather than
closed by a guess.

| Finding | File | Line | Citation | Overlay | Status |
|---|---|---:|---|---|---|
| dimensionless addend | `sysml-examples/Geometry Examples/VehicleGeometryAndCoordinateFrames.sysml` | 38 | SysML v2 §9.8.9.1 | corrected | **not filed** — drafted below, awaiting maintainer authorisation |
| mismatched dimensions | `sysml-examples/Analysis Examples/Turbojet Stage Analysis.sysml` | 25 | SysML v2 §9.8.9.1 | documented without a correction | **not filed** — drafted below, awaiting maintainer authorisation |
| a return typed by a dimension its value does not have | `sysml-examples/Analysis Examples/Dynamics.sysml` | 13 | KerML 7.4.9 | documented without a correction | **not filed** — drafted below, awaiting maintainer authorisation |
| non-conforming redefinition | `sysml-examples/Individuals Examples/AnalysisIndividualExample.sysml` | 86 | KerML 7.4.9, 8.3.4.2 | retired at `2026-07` | **closed** — fixed upstream, never filed by us |

Filing is the user's decision: nothing here has been posted to
`Systems-Modeling/SysML-v2-Pilot-Implementation` or any other upstream repository.

### `radius = 22/2*25.4 + 110 [mm]` adds a dimensionless value to a length (pilot `2026-05`)

**Not filed.** Drafted here for a maintainer to authorise; nothing has been
posted upstream.

Published, `Geometry Examples/VehicleGeometryAndCoordinateFrames.sysml`:38:

```sysml
:>> radius = 22/2*25.4 + 110 [mm];
```

Corrected by the overlay:

```sysml
:>> radius = (22/2*25.4 + 110) [mm];
```

`'[' SequenceExpression ']'` is a postfix on `PrimaryExpression`
(`build/pilot-grammars/KerMLExpressions.xtext:308`), below `AdditiveExpression`,
so `[mm]` qualifies `110` alone and the `+` combines a dimensionless value with
a length. **SysML v2 §9.8.9.1** requires the operands and the result of an
addition to share a quantity dimension, so the published expression is not
satisfiable under any reading, and the evident intent — a radius in millimetres —
is the parenthesised form. OpenSysML's warning at that line
(`operator '+' combines incommensurable quantities`) is therefore a true
positive. The pinned pilot performs no dimensional analysis and is silent both
on the published line and on the corrected one, so the correction changes our
verdict and not the pilot's.

### `1/(2 * Cp) * V^2 + T_static` adds L^6 to Θ (pilot `2026-05`)

**Not filed.** Drafted here for a maintainer to authorise; nothing has been
posted upstream.

Published, `Analysis Examples/Turbojet Stage Analysis.sysml`:25:

```sysml
return : TemperatureValue = 1/(2 * Cp) * V^2 + T_static;
```

`V` is declared `VolumeValue` (L^3) and `Cp` `DimensionOneValue`, so the first
operand has dimension L^6 while `T_static` is a `TemperatureValue` (Θ);
**SysML v2 §9.8.9.1** requires both operands and the result of `+` to share a
quantity dimension, and the declared return type Θ agrees with the second
operand only. The physics the calculation names (a total-temperature rise,
`V^2 / (2·Cp)`) wants `V` to be a speed rather than a volume, but the published
model does not say so anywhere: correcting it would mean choosing a type, a
unit and a dimension on the example's behalf.

So this row is **documented without a correction**. The overlay carries it for
provenance only; both figures every oracle reports keep the published text, and
OpenSysML's warning at that line stays in the differential census.

### `return a : AccelerationValue = tp * dt * tp` returns L^4·M^2·T^-5 (pilot `2026-07`)

**Not filed.** Drafted here for a maintainer to authorise; nothing has been
posted upstream.

Published, `Analysis Examples/Dynamics.sysml`:13, in `calc def Acceleration`
whose parameters are `in dt : TimeValue; in tm : MassValue; in tp: PowerValue`:

```sysml
return a : AccelerationValue = tp * dt * tp;
```

The file imports `ISQ::*`, whose definitions fix every dimension involved:
`AccelerationValue` declares `:>> mRef: AccelerationUnit[1]` and
`AccelerationUnit`'s power factors are L^1 and T^-2; `PowerValue` declares
`:>> mRef: PowerUnit[1]`, whose factors are L^2, M^1 and T^-3; and `TimeValue`
is `alias TimeValue for DurationValue`, T^1. The published product is therefore
(L^2·M·T^-3)^2 · T = L^4·M^2·T^-5, while **KerML 7.4.9** makes that expression
the value of the return feature, which answers to the `AccelerationValue` the
same line declares. The two dimensions are incommensurable, so the model is
unsatisfiable as published.

No correction is derivable. Acceleration from power is `tp / (tm * v)`, but this
calculation declares no speed among its parameters and its unused `tm` cannot
alone repair the exponents — `tp * dt / tm` is L^2·T^-2, not L·T^-2. Two of the
three plausible repairs also change the caller's contract, so the entry is
**documented without a correction** and the published text is what every oracle
reads.

The pinned pilot performs no dimensional analysis and is silent on the line, so
this is a diagnostic OpenSysML raises alone
(`cannot bind a value of dimension L^4·M^2·T^-5 to a feature typed by
AccelerationValue (dimension L·T^-2)`).

Earlier in the same file, at line 9, `calc def Power` has a second defect of the
same family that OpenSysML does **not** report:

```sysml
return tp : PowerValue = whlpwr - Cd * v - Cf * tm * v;
```

`whlpwr` is a `PowerValue` (L^2·M·T^-3), `Cd` and `Cf` are `Real`, `v` a
`SpeedValue` (L·T^-1) and `tm` a `MassValue`, so the three subtraction operands
have dimensions L^2·M·T^-3, L·T^-1 and M·L·T^-1 — which **SysML v2 §9.8.9.1**
requires to agree. The static dimensional check judges no product, by the
[documented design](spec-compliance.md), so no warning is raised here and the
overlay declares no entry for it either: an overlay entry covers one line per
file, and line 13 is the line an oracle reports. It is recorded here because a
report about this file should name both.

### `fuelConsumption : FuelEconomyAnalysis_1` redefines an action typed by `FuelConsumption` (pilot `2026-05`, fixed at `2026-07`)

**Never filed, and now closed.** The `2026-07` corpus publishes
`individual action :>> fuelConsumption : FuelConsumption_1` — the reading derived
below — so the overlay entry was retired and this section is kept as the record
of the finding.

Published at `2026-05`, `Individuals Examples/AnalysisIndividualExample.sysml`:86:

```sysml
individual action :>> fuelConsumption : FuelEconomyAnalysis_1 {
```

Corrected by the overlay, and published as such since `2026-07`:

```sysml
individual action :>> fuelConsumption : FuelConsumption_1 {
```

The redefined feature is `action fuelConsumption : FuelConsumption` of
`FuelEconomyAnalysis`, and **KerML 7.4.9** and **8.3.4.2** make a redefinition a
subsetting, whose subsetting feature's type must conform to the subsetted one's.
`FuelEconomyAnalysis_1` is the individual *analysis* definition specializing the
enclosing `FuelEconomyAnalysis`, so it conforms to nothing that `FuelConsumption`
specializes and the published model is unsatisfiable. Two lines above, the same
file declares `individual action def FuelConsumption_1 :> FuelConsumption` and
never mentions it again: the individual counterpart of the redefined feature's
type, which is what line 86 evidently meant to name. Substituting it clears our
error and leaves the rest of the file's verdict unchanged.

The pinned pilot validates subsetting conformance nowhere, so it is silent on
both texts, and the correction changes our verdict and not the pilot's.

---

## Proposed specification issue: identity annotations in the textual notation

**Filed** (maintainer-approved, 2026-09-01) against the **SysML 2.0** specification
(the textual-notation clauses, formal/26-03-02) as
[INBOX-2510](https://issues.omg.org/browse/INBOX-2510) — a temporary key that
redirects to the permanent one once the issue is assigned to a task force. The design
this draft distills is
[element-identity-annotations.md](element-identity-annotations.md); the working
prototype is OpenSysML's `IdentityMetadata` library, its validation pass, and the
RDF round trip. The body below is the submission text.

````markdown
**Title:** Textual notation cannot carry element identity, severing round trips
with the repositories the specification's own API defines

**Nature:** request for enhancement (interchange gap). **Severity:** significant.

The textual notation deliberately omits `Element::elementId`: text is treated as a
projection, and identity as the repository's concern. But the notation is the form
engineers version, diff and review, and the Systems Modeling API and Services
specification addresses every element by that id. The combination severs round
trips: any tool that serializes a model to text and reads it back has lost the
correlation with the repository it came from, so a rename — same element, new
name — is indistinguishable from a delete plus a create. Implementations are
already inventing workarounds (sidecar mapping files, IRI conventions, comment
conventions), none of which survive interchange through another conforming tool.

**Proposal:** standardize identity annotations — either a normative metadata
library, or dedicated surface syntax if the taskforce prefers. A minimal library
form, implementable today because user-defined metadata is already conforming
notation:

```sysml
standard library package IdentityMetadata {
    metadata def ElementId {
        attribute id : ScalarValues::String;
    }
    metadata def ProjectRef {
        attribute projectId : ScalarValues::String;
        attribute branch : ScalarValues::String[0..1];
        attribute org : ScalarValues::String[0..1];
    }
}
```

Applied opt-in: `@ElementId { id = "8f3a41d0-…"; }` on an element pins its
repository identity; one `@ProjectRef` on the root namespace binds the document to
a project (branch selecting a version, never contributing to identity). Elements
without an annotation keep tool-derived identity, so unannotated models are
unaffected and annotation cost is paid only where correlation matters.

**Implementation experience:** OpenSysML (github.com/Open-MBEE/OpenSysML)
implements exactly this shape: the metadata library, a validation pass (duplicate
and malformed ids, project-scope conflicts), RDF export keyed by the effective id,
and reader re-materialization closing the notation → RDF → notation round trip
byte-for-byte — all without any specification change, demonstrating that only the
*standardization* of the spelling is missing. Round-trip measurements against a
live Flexo MMS repository are maintained in the project's committed
interoperability report.
````

Submitted 2026-09-01 via the
[OMG issue reporting form](https://issues.omg.org/issues/create-new-issue); the
key above updates once a task force takes the issue.

## Proposed specification issue: diagram layout in the textual notation

**Drafted, not filed.** Against the **SysML 2.0** specification (the textual-notation
clauses, formal/26-03-02, and the graphical-notation clause 7.27 that treats a diagram
as a projection with no persisted geometry). The design this draft distills is
[diagram-layout-annotations.md](diagram-layout-annotations.md); the working prototype
is OpenSysML's `DiagramLayout` library, the geometry it carries through the view
engine, and its validation pass. The body below is the draft submission text.

````markdown
**Title:** Textual notation cannot carry diagram layout, so a diagram does not
survive a round trip through text

**Nature:** request for enhancement (interchange gap). **Severity:** significant.

The graphical notation is defined as a projection of the model, and neither the
textual notation nor the Systems Modeling API and Services specification has a
place for where a diagram draws an element. A model kept in text therefore has no
diagrams of its own: every rendering is laid out afresh, a diagram arranged in one
tool loses its arrangement when the model is exchanged as text, and a
reorganized diagram is invisible in a review of the change. Implementations keep
positions in tool-specific sidecars (per-tool diagram files, IRI conventions,
comment conventions), none of which another conforming tool reads.

**Proposal:** standardize a per-view layout vocabulary as a normative metadata
library — implementable today, since user-defined metadata is already conforming
notation — or as dedicated surface syntax if the taskforce prefers. A minimal
library form:

```sysml
standard library package DiagramLayout {
    metadata def Layout {
        attribute x : ScalarValues::Real;
        attribute y : ScalarValues::Real;
        attribute width : ScalarValues::Real[0..1];
        attribute height : ScalarValues::Real[0..1];
        attribute collapsed : ScalarValues::Boolean[0..1];
    }
    metadata def Route {
        attribute points : ScalarValues::Real[0..*] ordered nonunique;
    }
    metadata def Canvas {
        attribute unit : ScalarValues::String[0..1];
        attribute width : ScalarValues::Real[0..1];
        attribute height : ScalarValues::Real[0..1];
    }
}
```

Applied per view and opt-in: `metadata Layout about engine { x = 120; y = 80; }`
in a view's body places the element in that view; `@Layout { … }` in an element's
own body is the position every view that does not place it falls back to;
`Route` gives the edge a connection, transition, succession or flow is drawn as
its waypoints; `Canvas` in a view body sizes the drawing surface. Coordinates are
pixels from the top-left corner, y downward. Elements without an annotation are
laid out by the tool as today, so unannotated models are unaffected.

**Implementation experience:** OpenSysML (github.com/Open-MBEE/OpenSysML)
implements this shape: the metadata library, resolution of the effective
position per (view, element) pair from the view's body before the element's own
annotation, the geometry carried through its view engine to every rendering it
produces (as structured fields over its language-server render method, and kept
visible as comments and suffixes in Mermaid and text output), and a validation
pass (an annotation on an element the view's rendering does not draw, an odd
waypoint list, a canvas outside a view, two positions for one element in one
view) — all without any specification change, demonstrating that only the
*standardization* of the spelling is missing.
````

Not yet submitted; the text above is the draft a maintainer would file through the
[OMG issue reporting form](https://issues.omg.org/issues/create-new-issue).
