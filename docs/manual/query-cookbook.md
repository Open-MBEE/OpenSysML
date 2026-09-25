# Query Cookbook

Every recipe in this chapter runs against one model,
[`examples/cookbook.sysml`](examples/cookbook.sysml), and every output shown
is what the `sysml` binary printed. The model is a small observatory:

```sysml
package Cookbook {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;

	part def Subsystem {
		attribute mass : Real;
		assert constraint massKnown { mass > 0.0 }
	}
	part def OpticalSubsystem :> Subsystem;
	part def MirrorAssembly :> OpticalSubsystem {
		attribute :>> mass = 10.0;
		assert constraint lightweight { mass < 5.0 }
	}

	metadata def Critical;

	port def OpticalPort;
	port def DataPort;

	part telescope {
		part primaryMirror : MirrorAssembly {
			@Critical;
			port opticalOut : OpticalPort;
		}
		part instrumentCluster : Subsystem {
			attribute redefines mass = 4.5;
			port opticalIn : OpticalPort;
			port dataOut : DataPort;
		}
		part mountControl : Subsystem {
			attribute redefines mass = 15.0;
			port dataIn : DataPort;
		}
		connection opticalPath connect primaryMirror.opticalOut to instrumentCluster.opticalIn;
		connection dataPath connect instrumentCluster.dataOut to mountControl.dataIn;
	}

	part def Computer;
	part scienceComputer : Computer;
	allocation processing allocate telescope.instrumentCluster to scienceComputer;

	requirement massRequirement;
	part observatory {
		satisfy massRequirement by telescope;
	}

	verification def MassTest;
	verification massVerification : MassTest {
		objective {
			verify massRequirement;
		}
	}

	// ... the recipe queries below ...

	requirement def PointingRequirement {
		doc /* The telescope holds a target within the stated accuracy. */
	}
	requirement <'REQ-2'> pointingRequirement : PointingRequirement {
		doc /* The telescope points at a target to within 2 arcseconds. */
		requirement <'REQ-2.1'> slewRequirement {
			doc /* The mount reaches a new target within 60 seconds. */
		}
		requirement <'REQ-2.2'> trackingRequirement {
			doc /* The mount tracks a target for 30 minutes without drift. */
		}
	}
	verification def PointingTest;
	verification pointingVerification : PointingTest {
		objective {
			verify pointingRequirement;
		}
	}

	// ... and the coverage recipes ...
}
```

Each recipe is a `calc def` specializing `DocumentQueries::Query` declared in
the same package. Run one with:

```console
$ sysml docs/manual/examples/cookbook.sysml -run-query "<name> [<parameter>=<expression> ...]"
```

A binding whose expression is a name binds the element it denotes; anything
else is evaluated as an expression (strings in quotes, numbers as literals).

## Anatomy of a query

```sysml
calc def MassTable :> Query {
	in root : Element;                 // entry parameters, bound by the caller
	Project(                            // operations compose inside-out
		source = PartsByMass(root = root),  // ... and queries invoke queries
		properties = ("name", "mass", "qualifiedName")
	)
}
```

- A query is a `calc def` specializing `DocumentQueries::Query`.
- Its `in` parameters are the entry bindings a caller supplies — an element,
  a string, a number, a boolean, or a sequence of them.
- Its body is one expression composing the library operations; `source`
  arguments chain them, innermost first. A name in an argument reads the
  query's parameter of that name, or binds the model element it refers to
  (`OwnedElements(source = telescope)` starts from that part), just as a
  `%run-query` binding does. The element is checked against the parameter's
  type when the query is planned.
- A query can invoke another query by name, with its own bindings. Invocation
  is dependency-ordered and cycle-checked, with depth and count budgets.

Results are ordered element sequences. Order is the model's declaration order
until an `OrderBy` says otherwise, and elements are deduplicated by identity,
so a query is deterministic by construction.

## Parameter defaults

An `in` parameter may declare a default, and a caller that leaves it unbound
gets that default — from `%run-query`, `-run-query`, `RunDocumentQuery`, a
document's content block, or another query's invocation alike:

```sysml
calc def HeavySubsystems :> Query {
	in root : Element = telescope;         // a name binds the element it refers to
	in threshold : String default "10";    // anything else is evaluated
	WhereFeature(
		source = Descendants(source = root, maxDepth = 3),
		'feature' = "mass", operator = ">=", value = threshold
	)
}

calc def LightSubsystems :> HeavySubsystems {
	in redefines threshold default "5";    // a redefining default wins
}
```

- A default follows the binding rule of `%run-query <p>=<expr>`: a default that
  names a model element binds that element; any other default is an expression.
  The rule applies wherever a value is expected, so `in roots : Element[0..*] =
  (telescope, groundStation);` binds both elements, a list may mix element names
  with parameters and query invocations, and `in candidates : Element[0..*] =
  OwnedElements(source = telescope);` starts the traversal from that part.
- An expression default is evaluated once per query execution, before any row is
  produced, in the scope of the query that declared it — it may name that
  query's other parameters (`in candidates : Element[0..*] = OwnedElements(source = root);`)
  or invoke another query, within the usual visit and invocation budgets.
  Defaults are filled in parameter order after the explicit bindings, so a
  default may read a parameter bound explicitly or defaulted before it; one that
  reads a later, still unbound parameter fails as a missing binding.
- Defaults are inherited: `LightSubsystems` keeps `root = telescope` from
  `HeavySubsystems`, and its own `threshold` default replaces the inherited one.
  The nearest default along the redefinition chain wins.
- A default is checked against the parameter's type and multiplicity exactly
  like an explicit binding, and an explicit binding always overrides the
  default. What the default's text already settles is refused when the query is
  planned, as `document-query-default-type` or
  `document-query-default-multiplicity` naming the parameter: a literal or
  named element of the wrong type, and a list or invocation whose size cannot
  fit (`in source : Element = (telescope, groundStation);`). A named element is
  never a data value — `= label` is refused for a `String` parameter even when
  `label` is a `String` attribute — except an enumeration literal, which is a
  value of its enumeration: `in hue : Color = Color::red;` binds the literal,
  and a literal of another enumeration is refused. What only the values decide
  — a parameter reference whose multiplicity is not known statically — is
  checked when the default is evaluated, with the failures an explicit binding
  gets.
- A default the plan cannot represent (a form the query expression language has
  no operation for) is a planning error naming the parameter, reported with the
  other `document-query-*` diagnostics rather than at execution time.

## Collection

### Direct children: `OwnedElements`

```sysml
calc def Children :> Query {
	in root : Element;
	OwnedElements(source = root)
}
```

```console
$ sysml cookbook.sysml -run-query "Cookbook::Children root=Cookbook::telescope"
✓ Query Cookbook::Children returned 5 rows
  Row 1: Cookbook::telescope::primaryMirror
  Row 2: Cookbook::telescope::instrumentCluster
  Row 3: Cookbook::telescope::mountControl
  Row 4: Cookbook::telescope::opticalPath
  Row 5: Cookbook::telescope::dataPath
```

Everything the element owns is returned — here the three parts *and* the two
connections, in declaration order. Filter afterwards to narrow.

### Descendants to a depth: `Descendants`

```sysml
calc def AllParts :> Query {
	in root : Element;
	WhereType(
		source = Descendants(source = root, maxDepth = 10),
		type = "PartUsage"
	)
}
```

```console
$ sysml cookbook.sysml -run-query "Cookbook::AllParts root=Cookbook::telescope"
✓ Query Cookbook::AllParts returned 5 rows
  Row 1: Cookbook::telescope::primaryMirror
  Row 2: Cookbook::telescope::instrumentCluster
  Row 3: Cookbook::telescope::mountControl
  Row 4: Cookbook::telescope::opticalPath
  Row 5: Cookbook::telescope::dataPath
```

`maxDepth` bounds the walk; each level is visited in declaration order. Omit
it (or pass `null`) to walk the whole subtree — `Ancestors` likewise walks to
the root when unbounded.
Note that the connections are still here: a `connection` usage *is* a
`PartUsage` in the SysML metamodel (its metaclass conforms to it). Use
a feature or name filter, or `type = "ConnectionUsage"`, to separate them —
see [Type filters](#type-filters).

### Ancestors: `Ancestors`

```sysml
calc def Enclosing :> Query {
	in leaf : Element;
	Ancestors(source = leaf, maxDepth = 2)
}
```

```console
$ sysml cookbook.sysml -run-query "Cookbook::Enclosing leaf=Cookbook::telescope::primaryMirror::opticalOut"
✓ Query Cookbook::Enclosing returned 2 rows
  Row 1: Cookbook::telescope::primaryMirror
  Row 2: Cookbook::telescope
```

Owners are returned nearest-first, up to `maxDepth` levels.

### Elements by qualified name: `Named`

```sysml
calc def NamedParts :> Query {
	WhereType(
		source = Descendants(source = Named(qualifiedName = ("Cookbook::telescope", "Cookbook::Traceability"))),
		type = "PartUsage"
	)
}
```

```console
$ sysml cookbook.sysml -run-query "Cookbook::NamedParts"
✓ Query Cookbook::NamedParts returned 12 rows
  Row 1: Cookbook::telescope::primaryMirror
  Row 2: Cookbook::telescope::instrumentCluster
  ...
  Row 6: Cookbook::Traceability::gimbal
  ...
```

A query parameter must be bound to a feature, so a walk rooted at a *package*
or a *definition* has nothing to bind `root` to. `Named` resolves qualified
names — spelled as strings, like the types `WhereType` takes — to the elements
they name, in the order given, and any element may be named, a package or
definition included. A name that resolves to nothing, or to more than one
element, fails the query with the name quoted rather than returning fewer rows.
The SysML v1 migration roots every table scope this way.

## Type filters

`WhereType` keeps elements whose *metamodel* type matches — `"PartUsage"`,
`"ConnectionUsage"`, `"RequirementUsage"`, `"AttributeUsage"`, `"PortUsage"`,
`"PartDefinition"` and so on — including metaclass conformance, so
`type = "Usage"` keeps every kind of usage. Several names keep the elements of
any of them: `type = ("PartUsage", "PortUsage")`. A name that is neither a
known metamodel type nor resolvable in the model is a typed
`unknown-classification` error rather than a silently-empty result.

```sysml
calc def Connections :> Query {
	in root : Element;
	WhereType(
		source = Descendants(source = root, maxDepth = 10),
		type = "ConnectionUsage"
	)
}
```

```console
$ sysml cookbook.sysml -run-query "Cookbook::Connections root=Cookbook::telescope"
✓ Query Cookbook::Connections returned 2 rows
  Row 1: Cookbook::telescope::opticalPath
  Row 2: Cookbook::telescope::dataPath
```

To select by a *model-defined* classification — "every part typed by
`Subsystem`" — filter on what distinguishes those elements instead: a
metadata annotation ([below](#metadata-filters)) or a characteristic
attribute ([property filters](#property-filters)).

## Metadata filters

`WhereMetadata` keeps elements annotated with a metadata definition, matching
specializations of it too; several names keep the elements annotated with any
of them. The model marks `primaryMirror` with `@Critical`:

```sysml
calc def CriticalParts :> Query {
	in root : Element;
	WhereMetadata(
		source = AllParts(root = root),
		'metadata' = "Cookbook::Critical"
	)
}
```

```console
$ sysml cookbook.sysml -run-query "Cookbook::CriticalParts root=Cookbook::telescope"
✓ Query Cookbook::CriticalParts returned 1 row
  Row 1: Cookbook::telescope::primaryMirror
```

(`'metadata'` is quoted because `metadata` is a SysML keyword.)

## Name filters

`WhereName` compares each element's effective name against a value:

```sysml
calc def MirrorParts :> Query {
	in root : Element;
	WhereName(
		source = AllParts(root = root),
		operator = "contains",
		value = "Mirror"
	)
}
```

```console
$ sysml cookbook.sysml -run-query "Cookbook::MirrorParts root=Cookbook::telescope"
✓ Query Cookbook::MirrorParts returned 1 row
  Row 1: Cookbook::telescope::primaryMirror
```

Text operators: `=`/`==`, `!=`/`<>`, `contains`, `startsWith`, `endsWith`
(also spelled `starts-with`/`ends-with`), and `matches` with a regular
expression.

## Property filters

`WhereFeature` compares an attribute's constant value. The comparison is
typed: numbers compare numerically (`<`, `<=`, `>`, `>=` and equality, with
`*` accepted as infinity), booleans by equality, strings with the text
operators above, and an element-valued feature — a verdict's `assertion`, a
`RelatedColumn` list — as the qualified name it prints by, with the text
operators. An element without the attribute simply does not match; a
property no element in the source has is a typed `unknown-property` error.

```sysml
calc def HeavyParts :> Query {
	in root : Element;
	in threshold : String;
	WhereFeature(
		source = AllParts(root = root),
		'feature' = "mass",
		operator = ">=",
		value = threshold
	)
}
```

```console
$ sysml cookbook.sysml -run-query "Cookbook::HeavyParts root=Cookbook::telescope threshold=\"10\""
✓ Query Cookbook::HeavyParts returned 2 rows
  Row 1: Cookbook::telescope::primaryMirror
  Row 2: Cookbook::telescope::mountControl
```

Two details worth noting: `value` is always written as a string and parsed by
the operator's type, and `primaryMirror` matches through its *definition* —
`MirrorAssembly` fixes `mass = 10.0`, and the usage inherits it.

The built-in properties of the [projection table](#projection) are features
too, so `feature = "shortName"` with `startsWith` selects the requirements
whose identifier shares a prefix, and `feature = "documentation"` with
`contains` selects the elements whose `doc` text mentions a word — any one of
an element's several bodies matching is enough.

### Quantities

An attribute declared with a unit — `attribute :>> mass = 2290000 [kg];` — is
a *quantity*: a magnitude carried with its unit, never a bare number. The
filter's `value` is a bare number, and it compares against the magnitude **in
the attribute's own unit**: `mass >= "1000000"` matches `2290000 [kg]`, and
would match `1500000 [g]` too, because the threshold is read in each element's
unit. Choose the threshold for the unit your model declares, or normalize the
unit in the model. A `value` carrying a unit of its own (`"1000 [kg]"`) is
not a number and is refused as a typed `invalid-argument` error.

```sysml
calc def HeavyStages :> Query {
	in root : Element;
	WhereFeature(
		source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "PartUsage"),
		'feature' = "mass",
		operator = ">=",
		value = "1000000"
	)
}
```

```console
$ sysml units.sysml -run-query "UnitsRepro::HeavyStages root=UnitsRepro::rocket"
✓ Query UnitsRepro::HeavyStages returned 1 row
  Row 1: UnitsRepro::rocket::s1
```

The attribute compared need not be a literal: a mass declared as `dryMass +
propellantMass` is evaluated for each element before the comparison, as
[derived values](#derived-values) describes. Filtering and sorting then follow
the same commensurability rules as literal quantities.

## Sorting

`OrderBy` sorts by a property with every policy explicit — there are no
defaults to guess:

- `direction`: `"ascending"` or `"descending"`.
- `missing`: where elements without the property go — `"first"`, `"last"`,
  or `"error"` to refuse them.
- `multiple`: which value to sort by when the property has several —
  `"first"`, `"last"`, or `"error"`.

The sort is stable, so equal keys keep their declaration order. Mixing
incomparable value types across elements is a typed `invalid-order` error.

Quantities sort by converted magnitude when their units are commensurable:
`500000 [g]` orders below `119000 [kg]`. Two Integer magnitudes compare
exactly, so neighbours a Real cannot tell apart (`9007199254740993 [kg]`
against `9007199254740992 [kg]`) keep their order. Quantities of different dimensions
(`2290000 [kg]` against `42 [m]`) are not ordered — that is an
`invalid-order` error naming both units, never a silent comparison of the
bare magnitudes.

```sysml
calc def PartsByMass :> Query {
	in root : Element;
	OrderBy(
		source = AllParts(root = root),
		property = "mass",
		direction = "descending",
		missing = "last",
		multiple = "error"
	)
}
```

```console
$ sysml cookbook.sysml -run-query "Cookbook::PartsByMass root=Cookbook::telescope"
✓ Query Cookbook::PartsByMass returned 5 rows
  Row 1: Cookbook::telescope::mountControl
  Row 2: Cookbook::telescope::primaryMirror
  Row 3: Cookbook::telescope::instrumentCluster
  Row 4: Cookbook::telescope::opticalPath
  Row 5: Cookbook::telescope::dataPath
```

The connections have no `mass`, so `missing = "last"` places them after the
sorted parts.

## Projection

`Project` turns elements into rows of named, typed cells — what a document
table renders. Beyond the model's own attributes, these built-in properties
are always projectable:

| Property | Value |
|---|---|
| `name` | The effective name |
| `declaredName` | The declared name, absent when the name is derived |
| `shortName` | The effective short name (`<'HLR-R001'>`), absent when the element has none |
| `declaredShortName` | The declared short name, absent when the short name is derived |
| `documentation` | The body of each `doc` comment in declaration order, delimiters and indentation removed; absent when undocumented |
| `qualifiedName`, `@id` | The fully-qualified name |
| `owner` | The owner's qualified name |
| `@type` | The metamodel type (`PartUsage`, ...) |
| `type` | The declared type's qualified name |
| `isAbstract` | Boolean |
| `isIndividual` | Boolean: whether a definition or usage carries the `individual` modifier |
| `multiplicityLower`, `multiplicityUpper` | Integers, `*` as unbounded |

```sysml
calc def MassTable :> Query {
	in root : Element;
	Project(
		source = PartsByMass(root = root),
		properties = ("name", "mass", "qualifiedName")
	)
}
```

```console
$ sysml cookbook.sysml -run-query "Cookbook::MassTable root=Cookbook::telescope"
✓ Query Cookbook::MassTable returned 5 rows
  Columns: name, mass, qualifiedName
  Row 1: Cookbook::telescope::mountControl
    name = "mountControl"
    mass = 15.0
    qualifiedName = "Cookbook::telescope::mountControl"
  Row 2: Cookbook::telescope::primaryMirror
    name = "primaryMirror"
    mass = 10.0
    qualifiedName = "Cookbook::telescope::primaryMirror"
  Row 3: Cookbook::telescope::instrumentCluster
    name = "instrumentCluster"
    mass = 4.5
    qualifiedName = "Cookbook::telescope::instrumentCluster"
  Row 4: Cookbook::telescope::opticalPath
    name = "opticalPath"
    mass = (none)
    qualifiedName = "Cookbook::telescope::opticalPath"
  Row 5: Cookbook::telescope::dataPath
    name = "dataPath"
    mass = (none)
    qualifiedName = "Cookbook::telescope::dataPath"
```

A cell for a property the element lacks is empty (`(none)` in the CLI's row
listing, an empty table cell in a document).

### Quantity cells

A quantity-valued attribute projects as the runtime prints it — magnitude,
then the unit in brackets — in the CLI row listing, a Markdown cell (brackets
escaped as `\[kg\]`, so they read as text) and an HTML cell, whose
`<span class="sysml-value" data-value-kind="quantity">` also carries
`data-magnitude` and `data-unit` apart. The unit is the one the model spelt
(`kg`, `km/h`), not a reduction to base units.

```sysml
part def Stage {
	attribute mass :> ISQ::mass;
}
part def FirstStage :> Stage {
	attribute :>> mass = 2290000 [kg];
}
part def UpperStage :> Stage {
	attribute :>> mass = 119000 [kg];
}
part rocket {
	part s1 : FirstStage;
	part s2 : UpperStage;
}

calc def Masses :> Query {
	in root : Element;
	Project(
		source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "PartUsage"),
		properties = ("name", "mass")
	)
}
```

```console
$ sysml units.sysml -run-query "UnitsRepro::Masses root=UnitsRepro::rocket"
✓ Query UnitsRepro::Masses returned 2 rows
  Columns: name, mass
  Row 1: UnitsRepro::rocket::s1
    name = "s1"
    mass = 2290000 [kg]
  Row 2: UnitsRepro::rocket::s2
    name = "s2"
    mass = 119000 [kg]
```

### Derived values

A value that is a literal, a quantity, or an expression over those alone
(`2 [kg] * 3` is `6 [kg]`, `1 [km] + 500 [m]` is `1.5 [km]`) is folded once,
statically, when the model is analysed. A value written over *other features*
— the common shape in a mass or power budget — is instead evaluated by the
runtime **as seen from the row's element**: each leaf is read through the
redefinition chain of that concrete carrier, so a type-level `:>>` and a
usage-level `:>>` both win over the definition's own value, a `default` applies
where nothing binds the feature, and a feature chain (`s1.mass`) reads the
owned part's value. Arithmetic, comparisons, conditionals and the library
functions the runtime provides (`sum`, `size`, indexing with `#`, `->collect`)
all apply, with the runtime's rules: units are kept and converted, `Integer`
stays `Integer`, and a collection-valued attribute projects as one value per
element.

```sysml
part def Stage {
	attribute dryMass :> ISQ::mass;
	attribute propellantMass :> ISQ::mass;
	attribute mass :> ISQ::mass = dryMass + propellantMass;
}
part def FirstStage :> Stage {
	attribute :>> dryMass default = 130000 [kg];
	attribute :>> propellantMass = 2160000 [kg];
}
part def Vehicle {
	part s1 : FirstStage;
	part s2 : FirstStage {
		attribute :>> dryMass = 120000 [kg];
	}
	attribute liftoffMass :> ISQ::mass = s1.mass + s2.mass;
}
part rocket : Vehicle;
```

```console
$ sysml derived.sysml -run-query "DerivedRepro::Masses root=DerivedRepro::Vehicle"
✓ Query DerivedRepro::Masses returned 2 rows
  Columns: name, dryMass, mass
  Row 1: DerivedRepro::Vehicle::s1
    name = "s1"
    dryMass = 130000 [kg]
    mass = 2290000 [kg]
  Row 2: DerivedRepro::Vehicle::s2
    name = "s2"
    dryMass = 120000 [kg]
    mass = 2280000 [kg]
$ sysml derived.sysml -run-query "DerivedRepro::Vehicles root=DerivedRepro"
✓ Query DerivedRepro::Vehicles returned 1 row
  Columns: name, liftoffMass
  Row 1: DerivedRepro::rocket
    name = "rocket"
    liftoffMass = 4570000 [kg]
```

The query and the REPL agree: `rocket.s1.mass` prints `= 2290000 [kg]` too.
Note that `s2`'s `dryMass` overrides a `default =`; a value written with a
plain `=` is fixed for every redefinition, and the analyser refuses the
override before any query runs.

What the runtime cannot turn into a value is reported, never guessed:

- A leaf **unbound** anywhere in the carrier's chain (an abstract
  `attribute mass :> ISQ::mass;` that nothing ever binds) makes the derived
  value *absent* — an empty cell, as for a value-less feature — and
  `WhereFeature` does not match it, `OrderBy` places it by its `missing`
  policy.
- A value that genuinely depends on the model running — an `in` parameter of
  a calculation, an action's state, a non-constant function — or that the
  runtime rejects — a cycle (`a = b; b = a;`), operands of different
  dimensions (`mass + length`), a result no cell can hold such as a part
  (`attribute heart = engine.core;`) — is a typed `unevaluable-feature`
  error naming the query, the property, the row element and the runtime's
  reason. A table never shows a wrong number or a silently empty cell for a
  value the model does declare, and a `??` default does not cover it — the
  feature is present, not absent.

The roll-up a library writes over a possibly empty collection evaluates as
written: `sum` over no quantities is the zero of the collection's declared
kind, in its coherent SI unit, so `mass + sum(subcomponents.totalMass)` on a
component whose `subcomponents : MassedComponent [*] default null` holds
nothing is `mass` (`100 [kg] + 0 [kg]`), and a `default null` collection holds
the parts that subset it (`part b1 : Bolt :> subcomponents;`, or with
`subsets`) — the default is only its value where nothing populates it — so
the sum rolls up through them recursively. The kind is the collection's
declared one, and survives a `select`, `reject` or `collect` that leaves
nothing: a collection typed `Real[*]`, or an empty one mapped through an
untyped body parameter (`->collect { in x; x * x }`), still sums to the
number `0`, and `10 [kg] + 0 [m]` or `10 [kg] + 5` remain the
`incommensurable units` error above.

A value written with `=` holds for as long as the object does, not just on
the first read: it is derived from what the object holds *now*. When a run
assigns a feature the expression read (`assign a := 9;`), when a binding
propagates a new value into it, or when a `default null` collection is
superseded by a part that subsets it, the derived value is dropped and
derived again the next time it is read — through a part or a binding the
expression read through, and on through the values that read *it*. Nothing is
recomputed until something asks, and a value a run assigned is never
recomputed: `assign d := 100;` fixes `d` whatever `a` does afterwards, while
a `dd = d + 1` beside it keeps following `d`. A probe or transaction that
wrote such a feature is rolled back with the values that read it.

The parameters of a calculation or action usage are the boundary. They are
bound once, when the invocation starts, and stay bound while its outputs are
read — an assignment to a feature an `in` named does not rebind it for a
later output read of the same invocation. Only the `=` value of an object's
own feature follows what it read.

## Computed columns

A projection may also derive columns: each `Column(name, expression)` entry
appends a named column whose expression is evaluated once per row over the
row element's declared features. Arithmetic (`+`, `-`, `*`, `/`), string
concatenation with `+` and `??` defaults for absent values are supported:

```sysml
calc def MassBudget :> Query {
	in root : Element;
	Project(
		source = PartsByMass(root = root),
		properties = ("name", "mass"),
		columns = (
			Column(name = "massLbs", expression = (Subsystem::mass ?? 0.0) * 2.2),
			Column(name = "label", expression = "part: " + Element::name)
		)
	)
}
```

```console
$ sysml cookbook.sysml -run-query "Cookbook::MassBudget root=Cookbook::telescope"
✓ Query Cookbook::MassBudget returned 5 rows
  Columns: name, mass, massLbs, label
  Row 1: Cookbook::telescope::mountControl
    name = "mountControl"
    mass = 15.0
    massLbs = 33.0
    label = "part: mountControl"
  ...
```

Feature references name the declaring definition (`Subsystem::mass`,
`Element::name`); a row element that lacks the feature makes the expression
fail with a typed error naming the query, column and row — unless a `??`
default covers it, which is why `MassBudget`'s `massLbs` defaults to `0.0`
for the two connections in its results. Computed names join the projection:
`OrderBy` can sort by them and a table's `groupBy` can group by them.

Every built-in property is reachable the same way — `Element::shortName`,
`Element::declaredShortName` and `Element::documentation` included — so
`(Element::shortName ?? "—") + ": " + Element::name` labels a row by its
identifier. A column holds as many values as the feature it reads declares:
`Element::documentation` is `[0..*]`, so an element carrying two `doc` bodies
fills the cell with both in order (comma-joined in a document table) and one
carrying none leaves it empty, while a feature declared without a multiplicity
is one value per row — a row binding two fails the column with a typed
`column-cardinality` error naming the declared bound, and a row binding none
with `column-absent` unless `??` supplies a default — a value, or `null` to
leave that row's cell empty (`Stage::mass ?? null`). Operators always take one
value per operand, so `Element::documentation + "."` over two bodies fails.

A column expression may also be a **feature chain** — `'Monte Carlo'.runs`,
`stat.runs`, `outer.inner.value` — reading a feature of a member nested in
the row element. Each segment names a member of the element the previous one
reached: the row's own members answer first, then inherited ones, and the
last segment reads that member's feature. A segment that is not a basic name
is quoted, as in the notation. A row lacking a segment entirely makes the
path absent on that row — an empty cell, or a `??` default — and the path is
an unknown-property error only when no row reaches it; a member the path
finds that declares no value is an empty cell, and a multi-valued member
fills the cell with all of its values. The same path works as a `properties`
or `property` string (`"stat.runs"`), and `OrderBy` sorts by it. This is how
an individual's nested usage — an analysis the migrator writes, for instance —
contributes a column:

```sysml
analysis def 'Template Group 1 Monte Carlo' {
	out runs : ScalarValues::Natural;
	out mean : ScalarValues::Real;
}
individual part def 'template Group 11' :> 'Template Group 1' {
	analysis 'Monte Carlo' : 'Template Group 1 Monte Carlo' {
		out :>> runs = 5;
		out :>> mean = 18.0;
	}
}
calc def RunCounts :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		properties = ("name"),
		columns = (Column(name = "runs", expression = 'Monte Carlo'.runs ?? 0))
	)
}
```

```console
$ sysml cookbook.sysml -run-query "Cookbook::RunCounts root=Cookbook::Results"
✓ Query Cookbook::RunCounts returned 3 rows
  Columns: name, runs
  Row 1: Cookbook::Results::'template Group 11'
    name = "template Group 11"
    runs = 5
  ...
```

Quantities take part in column arithmetic with the runtime's rules, so a
column keeps its unit: `Stage::mass * 2` is `4580000 [kg]`, `Stage::mass /
1000` is `2290 [kg]`, `Stage::mass / Stage::length` is `54523.8… [kg/m]`,
and a ratio of like quantities (`Stage::length / Stage::length`) is a bare
number. Adding or subtracting quantities converts the right operand into the
left operand's unit (`1 [km] + 500 [m]` is `1.5 [km]`); operands of
different dimensions — `Stage::mass +
Stage::length`, or a quantity plus a bare number — are a typed
`column-incommensurable` error naming the column, the row and both units.

## Query invokes query

`MassTable` above already shows it: `PartsByMass(root = root)` invokes the
other query with its own bindings, and `AllParts` invokes `Children`'s
sibling the same way. Factoring collection into one base query and deriving
filtered/sorted/projected variants from it is the intended style. The engine
compiles the invocation graph up front: an unknown name, a cycle, or blowing
the depth/count budget is a typed error at that point.

## Relationship traversal

`RelatedElements` walks one named relationship kind from each source element:

```sysml
RelatedElements(
	source = <elements>,
	relationshipKind = "<kind>",   // specialization, subsetting, redefinition,
	                                // typing, connection, allocation,
	                                // satisfaction, verification,
	                                // derivation or refinement
	direction = "<direction>",     // outgoing or incoming
	maxDepth = <n>                 // omit, or null, for no bound
)
```

Direction is from the relationship's own point of view — `outgoing` follows
it as declared, `incoming` follows it backwards. Traversal is breadth-first
to `maxDepth` (unbounded when omitted or `null`), deduplicated, in
declaration order, and bounded by a visit budget so a pathological model
terminates with a typed error rather than hanging. The budget pays only for
the elements reached: the edge table a relationship kind reads is built once
per model (again after an edit), from every declaration in the workspace, and is not charged to it —
so a matrix over a large model costs what its rows relate to, not the model's size.

### Connections

Connection edges run **port to port** — traverse from the connector's
endpoint, not from the part that owns it:

```sysml
calc def ConnectedTo :> Query {
	in origin : Element;
	RelatedElements(
		source = origin,
		relationshipKind = "connection",
		direction = "outgoing",
		maxDepth = 1
	)
}
```

```console
$ sysml cookbook.sysml -run-query "Cookbook::ConnectedTo origin=Cookbook::telescope::primaryMirror::opticalOut"
✓ Query Cookbook::ConnectedTo returned 1 row
  Row 1: Cookbook::telescope::instrumentCluster::opticalIn
```

`outgoing` follows `connect A to B` from A's endpoint to B's; `incoming`
follows it the other way. Untyped `connect` clauses carry connection edges
too.

### Allocations

`allocate X to Y` is outgoing from X, incoming to Y:

```sysml
calc def AllocatedTargets :> Query {
	in origin : Element;
	RelatedElements(
		source = origin,
		relationshipKind = "allocation",
		direction = "outgoing",
		maxDepth = 1
	)
}
```

```console
$ sysml cookbook.sysml -run-query "Cookbook::AllocatedTargets origin=Cookbook::telescope::instrumentCluster"
✓ Query Cookbook::AllocatedTargets returned 1 row
  Row 1: Cookbook::scienceComputer
```

### Satisfy relationships

`satisfy R by P` points from the satisfying element to the requirement, so
"who satisfies this requirement" is an **incoming** traversal from the
requirement:

```sysml
calc def SatisfiedBy :> Query {
	in req : Element;
	RelatedElements(
		source = req,
		relationshipKind = "satisfaction",
		direction = "incoming",
		maxDepth = 1
	)
}
```

```console
$ sysml cookbook.sysml -run-query "Cookbook::SatisfiedBy req=Cookbook::massRequirement"
✓ Query Cookbook::SatisfiedBy returned 1 row
  Row 1: Cookbook::telescope
```

### Verify relationships

Likewise, "which verifications cover this requirement" is incoming from the
requirement; the result is the verification usage whose objective `verify`s
it:

```sysml
calc def VerifiedBy :> Query {
	in req : Element;
	RelatedElements(
		source = req,
		relationshipKind = "verification",
		direction = "incoming",
		maxDepth = 1
	)
}
```

```console
$ sysml cookbook.sysml -run-query "Cookbook::VerifiedBy req=Cookbook::massRequirement"
✓ Query Cookbook::VerifiedBy returned 1 row
  Row 1: Cookbook::massVerification
```

### Derive relationships

A requirement derivation is a connection conforming to the domain library's
`RequirementDerivation::Derivation` — typed by it, or written with the
`#derivation` semantic metadata. The cookbook model derives three requirements
from `massRequirement`, one of them at second hand:

```sysml
requirement mirrorMassRequirement;
requirement segmentMassRequirement;
requirement instrumentMassRequirement;
connection deriveMirrorMass : RequirementDerivation::Derivation
	connect massRequirement to mirrorMassRequirement;
#RequirementDerivation::derivation connection deriveInstrumentMass
	connect massRequirement to instrumentMassRequirement;
#RequirementDerivation::derivation connection deriveSegmentMass
	connect mirrorMassRequirement to segmentMassRequirement;
```

The `derivation` kind runs from the original requirement to each derived one,
so the requirements derived from an original — transitively, to `maxDepth` —
are an **outgoing** traversal, and the original(s) a derived requirement traces
back to are an incoming one:

```sysml
calc def DerivedFrom :> Query {
	in req : Element;
	RelatedElements(
		source = req,
		relationshipKind = "derivation",
		direction = "outgoing",
		maxDepth = 2
	)
}
```

```console
$ sysml cookbook.sysml -run-query "Cookbook::DerivedFrom req=Cookbook::massRequirement"
✓ Query Cookbook::DerivedFrom returned 3 rows
  Row 1: Cookbook::mirrorMassRequirement
  Row 2: Cookbook::instrumentMassRequirement
  Row 3: Cookbook::segmentMassRequirement
```

Which end is the original is read from the derivation itself: an end
subsetting `originalRequirements` or tagged `#original` is the original, one
subsetting `derivedRequirements` or tagged `#derive` is derived, and a
connection typed by a `connection def` specializing `Derivation` inherits the
roles its definition's ends state. An end that states no role takes the one
left over: it is the original when no other end is, and derived otherwise —
so `connect (a, b, c)` with no stated roles derives `b` and `c` from `a`, and
an unmarked end beside an `#original` end is derived. A `connection def`
specializing `Derivation` whose ends are typed by requirement definitions —
the form the v1 migrator writes — relates those definitions the same way,
through the ends it inherits from a general definition as well as its own; an
end that redefines an inherited end keeps that end's role and, when it declares
no type, its type. A plain connection between two requirements is not a
derivation.

### Refine relationships

A refinement is a `dependency` annotated `@ModelingMetadata::Refinement`,
as a prefix (`#refinement dependency ...`) or in its body (`{ @Refinement; }`).
The cookbook model states one from a part definition to the requirement it
refines:

```sysml
#ModelingMetadata::refinement dependency mirrorRefinesMass
	from MirrorAssembly to mirrorMassRequirement;
```

The `refinement` kind runs from each client of the dependency to each of its
suppliers, so "what refines this requirement" is an **incoming** traversal
from the requirement:

```sysml
calc def RefinedBy :> Query {
	in req : Element;
	RelatedElements(
		source = req,
		relationshipKind = "refinement",
		direction = "incoming",
		maxDepth = 1
	)
}
```

```console
$ sysml cookbook.sysml -run-query "Cookbook::RefinedBy req=Cookbook::mirrorMassRequirement"
✓ Query Cookbook::RefinedBy returned 1 row
  Row 1: Cookbook::MirrorAssembly
```

A dependency with several clients or suppliers relates every client to every
supplier. A dependency without the `Refinement` metadata states no refinement
edge.

### Specialization (and the other structural kinds)

`specialization`, `subsetting`, `redefinition` and `typing` traverse the
declaration hierarchy. Incoming specialization from a general type finds what
specializes it, transitively to `maxDepth`:

```sysml
calc def Specializers :> Query {
	in general : Element;
	RelatedElements(
		source = general,
		relationshipKind = "specialization",
		direction = "incoming",
		maxDepth = 2
	)
}
```

```console
$ sysml cookbook.sysml -run-query "Cookbook::Specializers general=Cookbook::Subsystem"
✓ Query Cookbook::Specializers returned 2 rows
  Row 1: Cookbook::OpticalSubsystem
  Row 2: Cookbook::MirrorAssembly
```

Traversal results are elements like any others — feed them into `Project` for
a traceability table, as the [worked example](worked-example.md) does for its
requirement section.

## Coverage

`RelatedElements` answers "what satisfies this requirement"; a traceability
report also has to answer "which requirements does **nothing** satisfy".
`WhereRelated` is that filter: it keeps each source row by whether at least
one element is reachable from it over a relationship kind, and takes the same
`relationshipKind`, `direction` and `maxDepth` as `RelatedElements` — every
kind it accepts, the same typed errors for an unknown kind or direction, the
same edge tables and visit budget:

```sysml
WhereRelated(
	source = <elements>,
	relationshipKind = "<kind>",
	direction = "<direction>",
	maxDepth = <n>,
	exists = true            // keep rows with a related element (default),
	                         // false keeps the rows with none
)
```

The rows to check are the requirements under a root. `WhereType` matches
nested requirement usages as well as top-level ones, and — because a
`satisfy` usage *is* a `RequirementUsage` in the metamodel — the satisfaction
assertions too, so the base recipe subtracts those with `Except`:

```sysml
calc def Requirements :> Query {
	in root : Element;
	Except(
		source = Union(
			source = WhereType(source = Descendants(source = root, maxDepth = 10), type = "RequirementDefinition"),
			other = WhereType(source = Descendants(source = root, maxDepth = 10), type = "RequirementUsage")
		),
		exclude = WhereType(source = Descendants(source = root, maxDepth = 10), type = "SatisfyRequirementUsage")
	)
}

calc def UnsatisfiedRequirements :> Query {
	in root : Element;
	WhereRelated(
		source = Requirements(root = root),
		relationshipKind = "satisfaction",
		direction = "incoming",
		maxDepth = 1,
		exists = false
	)
}
```

The model's `pointingRequirement` (declared at the end of the package with
its definition and two nested requirements) is verified by
`pointingVerification` but satisfied by nothing, and neither are its
children or the three requirements derived from `massRequirement`:

```console
$ sysml cookbook.sysml -run-query "Cookbook::UnsatisfiedRequirements root=Cookbook"
✓ Query Cookbook::UnsatisfiedRequirements returned 7 rows
  Row 1: Cookbook::PointingRequirement
  Row 2: Cookbook::pointingRequirement
  Row 3: Cookbook::mirrorMassRequirement
  Row 4: Cookbook::segmentMassRequirement
  Row 5: Cookbook::instrumentMassRequirement
  Row 6: Cookbook::pointingRequirement::slewRequirement
  Row 7: Cookbook::pointingRequirement::trackingRequirement
```

`UnverifiedRequirements` is the same recipe with
`relationshipKind = "verification"`; rooted at the package it also finds the
nested `Traceability` package's `downlinkRequirement`, which the
[traceability matrix](#traceability-matrix) below shows with no verifier:

```console
$ sysml cookbook.sysml -run-query "Cookbook::UnverifiedRequirements root=Cookbook"
✓ Query Cookbook::UnverifiedRequirements returned 7 rows
  Row 1: Cookbook::PointingRequirement
  Row 2: Cookbook::mirrorMassRequirement
  Row 3: Cookbook::segmentMassRequirement
  Row 4: Cookbook::instrumentMassRequirement
  Row 5: Cookbook::pointingRequirement::slewRequirement
  Row 6: Cookbook::pointingRequirement::trackingRequirement
  Row 7: Cookbook::Traceability::downlinkRequirement
```

Rows keep their order and any projected columns, so `WhereRelated` composes
with `Project` and `OrderBy` like the other filters. Omitting `exists`
keeps the covered rows instead; `maxDepth` bounds how far the walk looks
for a related element, and each element it reaches charges the visit budget.

### Set operations: `Except` and `Union`

`Except(source, exclude)` keeps the rows of `source` not among `exclude`, in
source order; `Union(source, other)` is every row of `source` followed by the
rows of `other` not already present. Both emit each row once and identify a row the
way `RelatedElements` de-duplicates: a model element by its declaration, an
object the session holds by the object itself, a verdict by its assertion and
the object it was checked on, a state by its object, machine and state path, and
an event by its place in the trace — so `Verdicts`, `States` and `Events` tables
can be combined too.
Combining the two coverage queries gives the requirements with a gap of
either kind, and subtracting one from the other the requirements with exactly
one:

```sysml
calc def UncoveredRequirements :> Query {
	in root : Element;
	Union(
		source = UnsatisfiedRequirements(root = root),
		other = UnverifiedRequirements(root = root)
	)
}

calc def VerifiedButUnsatisfied :> Query {
	in root : Element;
	Except(
		source = UnsatisfiedRequirements(root = root),
		exclude = UnverifiedRequirements(root = root)
	)
}
```

```console
$ sysml cookbook.sysml -run-query "Cookbook::UncoveredRequirements root=Cookbook"
✓ Query Cookbook::UncoveredRequirements returned 8 rows
  Row 1: Cookbook::PointingRequirement
  Row 2: Cookbook::pointingRequirement
  Row 3: Cookbook::mirrorMassRequirement
  Row 4: Cookbook::segmentMassRequirement
  Row 5: Cookbook::instrumentMassRequirement
  Row 6: Cookbook::pointingRequirement::slewRequirement
  Row 7: Cookbook::pointingRequirement::trackingRequirement
  Row 8: Cookbook::Traceability::downlinkRequirement
```

```console
$ sysml cookbook.sysml -run-query "Cookbook::VerifiedButUnsatisfied root=Cookbook"
✓ Query Cookbook::VerifiedButUnsatisfied returned 1 row
  Row 1: Cookbook::pointingRequirement
```

`Except` keeps the source's projected columns; `Union` requires both inputs
to be unprojected or to project the same columns, and is a typed error
otherwise, since its rows share one table.

## Requirement hierarchy

A requirement tree is the requirement definitions and usages under a root —
`Requirements` above — in hierarchy order. `Descendants` visits level by
level, so sort by `qualifiedName`: an element's qualified name prefixes its
children's, which puts each requirement directly above the ones nested in
it. Projecting `shortName`, `name` and `documentation` gives the table a
document renders:

```sysml
calc def RequirementTree :> Query {
	in root : Element;
	Project(
		source = OrderBy(
			source = Requirements(root = root),
			property = "qualifiedName",
			direction = "ascending",
			missing = "last",
			multiple = "error"
		),
		properties = ("shortName", "name", "documentation")
	)
}
```

```console
$ sysml cookbook.sysml -run-query "Cookbook::RequirementTree root=Cookbook::pointingRequirement"
✓ Query Cookbook::RequirementTree returned 2 rows
  Columns: shortName, name, documentation
  Row 1: Cookbook::pointingRequirement::slewRequirement
    shortName = "REQ-2.1"
    name = "slewRequirement"
    documentation = "The mount reaches a new target within 60 seconds."
  Row 2: Cookbook::pointingRequirement::trackingRequirement
    shortName = "REQ-2.2"
    name = "trackingRequirement"
    documentation = "The mount tracks a target for 30 minutes without drift."
```

Rooted at the package, the same query lists `PointingRequirement`, the
`Traceability` package's three requirements, `massRequirement` and its three
derived requirements, and `pointingRequirement` with its two children beneath
it.
The [requirements example](examples/requirements.sysml) renders such a tree
as the last table of its report, [`requirements.md`](examples/requirements.md).

## Traceability matrix

`RelatedElements` answers one requirement at a time. To put every requirement
in one table with its satisfiers and verifiers beside it, derive the columns
from the relationships instead: a `RelatedColumn(name, relationshipKind,
direction, maxDepth, aggregate = "list", targets)` entry of `columns`
traverses the named relationship from each row's element — the same kinds,
directions and depth bound as `RelatedElements` — and fills a cell with what
it reaches. The `aggregate` chooses the cell's shape: `"list"` (the default)
holds the related elements, `"count"` how many there are, `"any"` whether
there is at least one — an existence test that stops at the first element it
reaches. `targets`, when given, keeps only the reached elements among them:
a dependency matrix whose columns are one query and whose rows are another
is `Project(source = <rows>, columns = (RelatedColumn(..., targets = <columns>)))`.

The cookbook model's `Traceability` package holds three requirements, a
`spacecraft` whose parts satisfy them and three verification cases, two of
which verify the pointing requirement and none the downlink one:

```sysml
calc def TraceMatrix :> Query {
	in root : Element;
	Project(
		source = WhereType(
			source = Descendants(source = root, maxDepth = 1),
			type = "RequirementUsage"
		),
		properties = ("shortName", "name"),
		columns = (
			RelatedColumn(name = "satisfiedBy", relationshipKind = "satisfaction", direction = "incoming", maxDepth = 1),
			RelatedColumn(name = "verifiedBy", relationshipKind = "verification", direction = "incoming", maxDepth = 1),
			RelatedColumn(
				name = "verifications",
				relationshipKind = "verification",
				direction = "incoming",
				maxDepth = 1,
				aggregate = "count"
			)
		)
	)
}
```

```console
$ sysml cookbook.sysml -run-query "Cookbook::TraceMatrix root=Cookbook::Traceability"
✓ Query Cookbook::TraceMatrix returned 3 rows
  Columns: shortName, name, satisfiedBy, verifiedBy, verifications
  Row 1: Cookbook::Traceability::pointingRequirement
    shortName = "TR-1"
    name = "pointingRequirement"
    satisfiedBy = Cookbook::Traceability::gimbal
    verifiedBy = [Cookbook::Traceability::pointingTest, Cookbook::Traceability::pointingAnalysis]
    verifications = 2
  Row 2: Cookbook::Traceability::thermalRequirement
    shortName = "TR-2"
    name = "thermalRequirement"
    satisfiedBy = Cookbook::Traceability::radiator
    verifiedBy = Cookbook::Traceability::thermalTest
    verifications = 1
  Row 3: Cookbook::Traceability::downlinkRequirement
    shortName = "TR-3"
    name = "downlinkRequirement"
    satisfiedBy = Cookbook::Traceability::transmitter
    verifiedBy = (none)
    verifications = 0
```

A list cell is genuinely multi-valued: the report brackets several elements
and prints `(none)` for an empty cell, a document table renders each element
as it renders a multi-valued `documentation` projection (comma-joined in
Markdown, one linked value each in HTML), and the gRPC `run_query` response
carries every element. The elements keep the traversal's order, so the
verification declared first comes first.

Related columns join the projection like computed ones: `OrderBy` sorts by
them, a table's `groupBy` groups by them, and `WhereFeature` filters on them.
A list cell's elements compare and sort as their qualified names, so
`WhereFeature(feature = "satisfiedBy", operator = "endsWith", value = "::gimbal")`
keeps the requirements the gimbal satisfies and `OrderBy(property =
"satisfiedBy", multiple = "first")` sorts by each row's first satisfier.
Uncovered requirements are the rows whose count is zero:

```sysml
calc def Unverified :> Query {
	in root : Element;
	WhereFeature(
		source = TraceMatrix(root = root),
		'feature' = "verifications",
		operator = "=",
		value = "0"
	)
}
```

```console
$ sysml cookbook.sysml -run-query "Cookbook::Unverified root=Cookbook::Traceability"
✓ Query Cookbook::Unverified returned 1 row
  Columns: shortName, name, satisfiedBy, verifiedBy, verifications
  Row 1: Cookbook::Traceability::downlinkRequirement
    shortName = "TR-3"
    name = "downlinkRequirement"
    satisfiedBy = Cookbook::Traceability::transmitter
    verifiedBy = (none)
    verifications = 0
```

A relationship kind or direction `RelatedElements` would refuse is refused
here too, with the same typed error naming the column; so is an `aggregate`
other than the three above. The [traceability example](examples/traceability.md)
renders such a matrix as a document table beside the requirement list and
the requirements' verdicts.

## Objects the session holds

Every recipe so far reads the model: its elements and what they declare. A
query can also read the **objects** a session holds — the ones `-instantiate`
(or `%instantiate` in the REPL) created — with the same operations. A binding
written as a usage's name binds the object the session holds under that name
while it holds one, and the element otherwise; `#2` binds an object by the id
the instantiation report printed, and `telescope.primaryMirror` a nested object
by its path. Over an object, `OwnedElements` and `Descendants` are the objects
it holds as its parts, `Ancestors` the objects holding it, `WhereType` tests
the object's types, and `WhereFeature`, `Project` and `OrderBy` read the values
the object holds **now** — after a run changed them, not the declared defaults.
An object's `name` is its path from the object it was bound through
(`primaryMirror`, `wheels[2]` for the second of a collection), its
`qualifiedName` the whole path (`Cookbook::telescope.primaryMirror`), and the
report prints its id beside each row.

```sysml
calc def HeldParts :> Query {
	in root : Element;
	Project(
		source = WhereType(source = Descendants(source = root, maxDepth = 2), type = "Subsystem"),
		properties = ("qualifiedName", "mass")
	)
}
```

```console
$ sysml cookbook.sysml -instantiate Cookbook::telescope -run-query "Cookbook::HeldParts root=telescope"
✓ Created instance of Cookbook::telescope
  ID: 1
  Use %features Cookbook::telescope to inspect
✓ Query Cookbook::HeldParts returned 3 rows
  Columns: qualifiedName, mass
  Row 1: Cookbook::telescope.primaryMirror (#2)
    qualifiedName = "Cookbook::telescope.primaryMirror"
    mass = 10.0
  Row 2: Cookbook::telescope.instrumentCluster (#4)
    qualifiedName = "Cookbook::telescope.instrumentCluster"
    mass = 4.5
  Row 3: Cookbook::telescope.mountControl (#7)
    qualifiedName = "Cookbook::telescope.mountControl"
    mass = 15.0
```

Without `-instantiate` the same invocation binds the element `telescope` and
returns its three declared subsystems, as the recipes above do.

`Objects(type = "<type>")` enumerates every object the session holds that is of
the type — the objects bound at the top and every object they hold, each under
its path — and needs no binding at all:

```sysml
calc def HeldSubsystems :> Query {
	OrderBy(
		source = Project(source = Objects(type = "Subsystem"), properties = ("qualifiedName", "mass")),
		property = "mass",
		direction = "descending",
		missing = "last",
		multiple = "first"
	)
}
```

```console
$ sysml cookbook.sysml -instantiate Cookbook::telescope -run-query "Cookbook::HeldSubsystems"
✓ Created instance of Cookbook::telescope
  ID: 1
  Use %features Cookbook::telescope to inspect
✓ Query Cookbook::HeldSubsystems returned 3 rows
  Columns: qualifiedName, mass
  Row 1: Cookbook::telescope.mountControl (#7)
    qualifiedName = "Cookbook::telescope.mountControl"
    mass = 15.0
  Row 2: Cookbook::telescope.primaryMirror (#2)
    qualifiedName = "Cookbook::telescope.primaryMirror"
    mass = 10.0
  Row 3: Cookbook::telescope.instrumentCluster (#4)
    qualifiedName = "Cookbook::telescope.instrumentCluster"
    mass = 4.5
```

A session holding no object returns no rows from `Objects`; outside any
session the operation is refused with a typed error, and `RelatedElements` is
refused over an object row — [Which query is
which](query-kinds.md#object-rows-and-verdict-rows) draws these boundaries.

A document renders the same way: `-instantiate <name> -render-document <doc>`
creates the object first, and every table or list whose query is bound to that
usage's name, or enumerates `Objects`, renders the objects by path. In HTML each
such row carries its `data-object="#<id>"` beside the `data-element` of the
usage it stands for, and an object-valued cell is a `span.sysml-object`. See
[Rendering a document over objects](../reference/cli.md#rendering-a-document-over-objects).

## Which constraints and requirements hold

`Verdicts(source = <rows>)` checks the object behind each row — the object the
session holds when the binding is one, the row's declared object otherwise —
and returns one row per assertion about it or about the objects it holds:
every `assert constraint`, every requirement the object carries, every
`satisfy` whose subject it is, and the verification cases that verify those
requirements. Each row is a **verdict**: its `verdict` is `holds`, `violated`
or `undecided`, its `path` names the object the assertion was checked on
(`Cookbook::telescope.primaryMirror`), its `kind` is `constraint`,
`requirement`, `satisfaction` or `verification`, and its `reason` explains a
violation or why nothing could be decided. The row still stands for the
assertion element, so `name`, `qualifiedName`, `WhereName` and `WhereType`
read the constraint or requirement itself; on a violated row `condition` is the
condition that came out false, as written.

The cookbook's `Subsystem` asserts `massKnown { mass > 0.0 }`, and the
`MirrorAssembly` redefining `mass = 10.0` also asserts `lightweight { mass < 5.0 }`:

```sysml
calc def Checks :> Query {
	in root : Element;
	Project(source = Verdicts(source = root), properties = ("path", "name", "verdict", "reason"))
}
```

```console
$ sysml cookbook.sysml -run-query "Cookbook::Checks root=telescope"
✓ Query Cookbook::Checks returned 6 rows
  Columns: path, name, verdict, reason
  Row 1: satisfy massRequirement by telescope on Cookbook::telescope: undecided
    path = "Cookbook::telescope"
    name = (none)
    verdict = "undecided"
    reason = "satisfaction satisfy massRequirement by telescope: no condition to evaluate"
  Row 2: verification Cookbook::massVerification on Cookbook::telescope: undecided
    path = "Cookbook::telescope"
    name = "massVerification"
    verdict = "undecided"
    reason = "the case body bound no VerdictKind value"
  Row 3: assert constraint massKnown on Cookbook::telescope.primaryMirror: holds
    path = "Cookbook::telescope.primaryMirror"
    name = "massKnown"
    verdict = "holds"
    reason = (none)
  Row 4: assert constraint lightweight on Cookbook::telescope.primaryMirror: violated
    path = "Cookbook::telescope.primaryMirror"
    name = "lightweight"
    verdict = "violated"
    reason = "constraint lightweight: assertion evaluated to false: mass < 5.0"
  Row 5: assert constraint massKnown on Cookbook::telescope.instrumentCluster: holds
    ...
  Row 6: assert constraint massKnown on Cookbook::telescope.mountControl: holds
    ...
```

Rows come in the order the objects are walked — the root first, then each
part in declaration order — with the assertions on one object together. The
`massRequirement` has no `require constraint`, so satisfying it decides
nothing, and its verification case binds no verdict; both are `undecided` with
the reason saying so. Written over the element `telescope`, the query checks
the declared object — definition defaults and `:>>` redefinitions — exactly as
the derived `mass` recipes above read it. With `-instantiate Cookbook::telescope`
the same binding is the held object and the verdicts are about its values
**now**, so a run that changed `mass` changes the table.

`kind = "constraint"` (or `requirement`, `satisfaction`, `verification`)
keeps one kind of assertion; the default `"all"` keeps every kind. To list
only what fails, filter on the verdict:

```sysml
calc def Violated :> Query {
	in root : Element;
	WhereFeature(source = Verdicts(source = root), 'feature' = "verdict", operator = "=", value = "violated")
}
```

```console
$ sysml cookbook.sysml -run-query "Cookbook::Violated root=telescope"
✓ Query Cookbook::Violated returned 1 row
  Row 1: assert constraint lightweight on Cookbook::telescope.primaryMirror: violated
```

`OrderBy(property = "verdict")` sorts the table by outcome, `WhereFeature` on
`path` or `kind` narrows it, and `Project` reads any verdict property beside
the assertion's own (`shortName`, `documentation`) — `assertion` and `carrier`
project the assertion element and the object it was checked on themselves.
A verdict row's
`verification` property lists the outcomes (`pass`, `fail`, `inconclusive`,
`error`) of the verification cases that verify its requirement — on a
`requirement` or `satisfaction` row — while a `verification` row carries one
case's own outcome as its `verdict`.

Two things a verdict table refuses rather than approximates. `Verdicts` over a
row that is not an object — a package, an attribute usage — is a typed error
naming the element, as `-validate=<object>` is. And when the object graph
cannot be walked whole (a part that holds another of its own type without
end, or one that exceeds the materialization budget), the query fails with an
`incomplete-validation` error instead of returning a table missing rows;
`Ancestors`, `Descendants` and `OwnedElements` are likewise refused over
verdict rows, which are assertions checked on an object, not elements owning
others.

In a document, a `Verdicts` table renders each cell as
`<assertion> on <path>: <verdict>`; in HTML a verdict cell is a
`span.sysml-verdict` carrying `data-verdict`, `data-path` and, for an object
the session holds, `data-object`, beside the `data-element` of the assertion.

## Where the objects stand and what they did

Objects the session holds may be *running*: an object whose type exhibits a
state machine starts it when the object is created, and `%send` and
`%advance` (or `-state` with `-advance` on the command line) drive it. Three
operations read the run. `States(source = <rows>)` answers the state each
object's machine is in now — one row per active leaf state, so a `parallel`
state contributes a row per region; `InState(name = "<state>")` is the
inverse, the held objects whose machine is in that state; and
`Events(source, kind, since, before)` reads the trace the session records as
rows. All three read the session as `Objects` does, and are refused with the
same `no-runtime` error where there is none;
[Which query is which](query-kinds.md#runtime-state-and-event-queries) has
the boundaries in full.

The cookbook model's `Dome` exhibits a `DomeControl` machine: `closed` until
an `Open` arrives, then `open` with two regions — `pointing`, which slews on a
`Slew(azimuth)` signal, and `shutter`, which takes a second to reach `opened`
— and back to `closed` on `Close`:

```sysml
calc def DomeStates :> Query {
	in root : Element;
	Project(source = States(source = root), properties = ("machine", "statePath", "region", "enclosing"))
}

calc def Opened :> Query {
	Project(source = InState(name = "open"), properties = ("qualifiedName"))
}

calc def Accepted :> Query {
	in root : Element;
	Project(
		source = Events(source = root, kind = "accept", since = 1 [s], before = 2.5 [s]),
		properties = ("time", "event", "payload")
	)
}
```

Sending a signal needs the prompt, so this recipe runs there. `%trace on`
first, since `Events` reads the trace the session records and refuses
(`no-trace`) in one that records none — and `%trace off` discards it:

```console
$ sysml docs/manual/examples/cookbook.sysml
sysml> %trace on
sysml> %instantiate dome
✓ Created instance of Cookbook::dome
  ID: 1
sysml> %instantiate spareDome
✓ Created instance of Cookbook::spareDome
  ID: 3
sysml> %state control dome
sysml> %send Open
sysml> %advance 1
sysml> %send Slew(azimuth = 120.0)
sysml> %advance 1
sysml> %send Open to spareDome
sysml> %advance 0.5
sysml> %send Close to spareDome
sysml> %advance 0.5
```

*Which state is `#1.control` in?* — one row per active leaf, with the
region each runs in and the composite state enclosing both:

```console
sysml> %run-query DomeStates root=#1
✓ Query Cookbook::DomeStates returned 2 rows
  Columns: machine, statePath, region, enclosing
  Row 1: #1.control in open.slewing
    machine = "control"
    statePath = "open.slewing"
    region = "pointing"
    enclosing = "open"
  Row 2: #1.control in open.opened
    machine = "control"
    statePath = "open.opened"
    region = "shutter"
    enclosing = "open"
```

A state row's `name` is the leaf's own (`slewing`), `statePath` its name
qualified by the states enclosing it, `object` and `path` the object, and the
row answers the state declaration's own properties too, so `WhereName` and
`WhereFeature` on any of them narrow the table and `OrderBy` orders it. A row
prints as `<object>.<machine> in <statePath>`, the object as the binding named
it. `States` over an object exhibiting no state machine is a typed
`no-state-machine` error, not an empty table.

*Which objects are in `open`?* — `spareDome` opened at `t = 2` and closed
again at `t = 2.5`, so only `dome` is:

```console
sysml> %run-query Opened
✓ Query Cookbook::Opened returned 1 row
  Columns: qualifiedName
  Row 1: Cookbook::dome (#1)
    qualifiedName = "Cookbook::dome"
```

The name is a leaf or a state enclosing one, by name or dotted path
(`open.slewing`), and each object is one row however many of its leaves match;
a name no held object's machine declares is a typed `unknown-state` error.
The rows are object rows, so `Descendants`, `WhereFeature` and `Verdicts`
read them as [above](#objects-the-session-holds).

*What did `#1` accept between `t = 1 [s]` and `t = 2.5 [s]`?* — the interval
is inclusive at `since` and exclusive at `before`, so the `Open` accepted at
`t = 0` is out, the `Slew` at `t = 1` in, and for `spareDome` the `Open` at
`t = 2` is in while the `Close` at `t = 2.5` is out:

```console
sysml> %run-query Accepted root=#1
✓ Query Cookbook::Accepted returned 1 row
  Columns: time, event, payload
  Row 1: t=1 Cookbook::dome.control: accept Slew
    time = 1.0 [s]
    event = "Slew"
    payload = "azimuth = 120.0"
sysml> %run-query Accepted root=spareDome
✓ Query Cookbook::Accepted returned 1 row
  Columns: time, event, payload
  Row 1: t=2 Cookbook::spareDome.control: accept Open
    time = 2.0 [s]
    event = "Open"
    payload = (none)
```

`kind` names the records to keep — `accept`, `send`, `transition`, `entry`,
`exit`, `do`, `choice` (a due order or region order the run drew, with
`alternatives` and `taken`) or `guard` (one it could not evaluate), several
separated by commas, `all` by default — and a `source` left out reads every
object's records. `since` and `before` take a duration or a bare number of
the clock's seconds; a bound that is not a duration (`1 [m]`), or an interval
with `before` at or before `since`, is a typed `invalid-interval` error. Each
row's `time` is the instant read from the runtime clock, `state`, `from` and
`to` the states an entry, exit or transition touched, `target` the object a
send was addressed to, and `text` the line `%trace` prints — the rows are the
record it prints from, so the two never disagree. The rows come in the order
the run made them; `OrderBy(property = "time", direction = "descending", ...)`
reverses it.

In a document, a state cell renders as `<path>.<machine> in <statePath>` and
an event cell as `t=<instant> <path>.<machine>: <text>`; in HTML they are a
`span.sysml-state` with `data-machine`, `data-state` and `data-region`, and a
`span.sysml-event` with `data-event-kind` and `data-time`, each carrying the
object's `data-object`.
