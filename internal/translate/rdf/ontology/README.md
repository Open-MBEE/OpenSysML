# SysML v2 metamodel term table

`table.go` is generated from the OMG SysML v2 metamodel as the pilot implementation
([Systems-Modeling/SysML-v2-Pilot-Implementation](https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation))
publishes it: `org.omg.sysml/model/SysML.ecore`, at the release and commit
[`scripts/pilot-pin.sh`](../../../../scripts/pilot-pin.sh) pins for every other pilot input.
The metamodel version is the date in the ecore's namespace URI
(`https://www.omg.org/spec/SysML/20250201` → `20250201`). The ecore is not vendored
(~620 KB of third-party XMI); the generated header records the pilot tag, commit and namespace
URI, so the table is reproducible from the header alone.

Each ecore `EStructuralFeature` becomes one property, named as the OWL rendering of the
metamodel names it — `https://www.omg.org/spec/SysML#<DefiningClass>_<name>` — and holding:

| Field | From the ecore |
|-------|----------------|
| `Name`, `DefiningClass`, `IRI` | the feature's name and the `EClass` declaring it |
| `Kind` | `ObjectProperty` for an `EReference`, `DatatypeProperty` for an `EAttribute` |
| `Range` | the referenced class, the attribute's enumeration, or the XSD/OWL datatype of its primitive (`Boolean` → `xsd:boolean`, `String` → `xsd:string`, `Integer` → `xsd:int`, `Real` → `owl:real`) |
| `Many` | `upperBound="-1"`: the API JSON form spells it as an array |
| `Ordered` | a multi-valued feature without `ordered="false"` (ecore's default is ordered) |
| `Derived` | `derived="true"` |
| `Redefines`, `Subsets` | the `redefines` / `subsets` annotations' references, as `Class::feature` |
| `Opposite` | `eOpposite`, as `Class::feature` |

Each ecore `EEnum` becomes an `Enumeration`. Its literals keep their declaration order;
enumerations are ordered by name in the generated table. Datatype properties whose range is
in the SysML namespace refer to one of these declarations.

`Property.QualifiedName` gives the same `Class::feature` spelling, so the three reference fields
can be looked up with `PropertyOf`. Every `EClass` becomes a `Class` with its `eSuperTypes`, which
`IsAncestorOrSelf` walks for the domain check in `validate.go` and `PropertyOf` uses to pick the
declaration a metaclass inherits a name from, and `Abstract` for `abstract="true"`, which the check
reports as an `abstract-class` violation.

The table's contents are counted in one place,
[docs/project/roadmap.md](../../../../docs/project/roadmap.md) § D8, together with what the gate
finds. Some unqualified names are declared by more than one metaclass, so `LookupProperty` returns
every declaration and `AmbiguousNames` reports those names rather than one being picked silently.

## Regenerating and checking

From the repository root:

```bash
make ontology-table          # ./scripts/download-pilot-metamodel.sh && go run -C tools ./gen/ontology
make ontology-table-check    # the same with -check, as CI runs it
```

`scripts/download-pilot-metamodel.sh` fetches the pinned pilot's `org.omg.sysml/model` into
`build/pilot-metamodel/` and stamps it with the pin; the generator refuses a download whose stamp
is not the current pin. It also refuses an ecore whose namespace URI is not
`https://www.omg.org/spec/SysML/<yyyymmdd>`, a feature whose type, opposite, redefined or subsetted
feature names nothing in the ecore, a primitive it has no datatype for, and a name containing `_`,
which would make an IRI ambiguous. `-check` writes nothing and fails when the committed table
differs from what the pin generates. `TestTableFollowsThePilotPin` (`tests/ontology`) fails, with
no download needed, when the pin moves and the table's header still names the previous one.

## Bumping

The table follows the pilot pin; there is no separate ontology pin to bump.

1. Move `PILOT_TAG` and `PILOT_COMMIT` in `scripts/pilot-pin.sh` (see
   [docs/project/pilot-corpora.md](../../../../docs/project/pilot-corpora.md) for what else that
   moves).
2. `make ontology-table`, then review the diff of `table.go`: classes and properties added,
   removed or renamed, and new flags.
3. Update the counts in `tests/ontology/ontology_test.go` and `docs/project/roadmap.md` § D8.
4. `go test ./internal/translate/... ./tests/ontology ./tests/export ./tests/migrate` and the
   RDF and API-JSON corpus round-trip gates (`docs/project/rdf-corpus-roundtrip.md`). The export
   gate `TestGoldenGraphsMatchOntology` compares the golden graphs against the new table: a
   metaclass the export writes that the metamodel renamed, or a property it dropped, appears as a
   new violation, and an entry of `tests/export/testdata/ontology-known-violations.txt` the new
   table resolves as one to remove. Fix the mapping rather than listing a new violation, and keep
   graphs earlier releases wrote readable.
