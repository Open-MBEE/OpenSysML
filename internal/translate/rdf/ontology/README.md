# SysML v2 ontology term table

`table.go` is generated from the OMG SysML v2 metamodel as
[Open-MBEE/sysmlv2-rdf-ontology](https://github.com/Open-MBEE/sysmlv2-rdf-ontology)
renders it in OWL — `sysml2/owl/www.omg.org/spec/SysML.owl`, version `202407`,
generated there from `SysML.ecore` — plus the checkout's `sysml2/ecore/SysML.ecore`
itself, which supplies each property's upper multiplicity. Neither file is
vendored here (the OWL alone is ~624 KB of third-party RDF/XML): the generator
reads them from a local checkout and records the upstream commit SHA in the
generated header, so the table is reproducible from a header alone.

The table holds, per property, the unqualified local name this tool's encoder
writes (`declaredName`), the metaclass that defines it (`Element`), the full
ontology IRI (`https://www.omg.org/spec/SysML#Element_declaredName`), whether it
is an `owl:ObjectProperty` or an `owl:DatatypeProperty`, its declared
`rdfs:range`, and whether its ecore upper bound is unbounded (`Many` — the
multiplicity the API JSON form spells as an array). It also holds every
`owl:Class` and its named `rdfs:subClassOf` parents, which `IsAncestorOrSelf`
walks for the domain check in `validate.go` and which `PropertyOf` uses to
pick the declaration a given metaclass inherits a name from.

The table's contents are counted in one place,
[docs/project/roadmap.md](../../../../docs/project/roadmap.md) § D8, together with
what the gate finds. Some unqualified names are declared by more than one
metaclass, so `LookupProperty` returns every declaration and `AmbiguousNames`
reports those names rather than one being picked silently.

## What the ontology does not carry

`SysML.owl` records no ecore abstractness. The only abstractness in the file is
the metamodel's own `Type::isAbstract` property (`sysml:Type_isAbstract`, a
`Type`-domained `owl:DatatypeProperty` about modeled types, not about
metaclasses); the `owl:Class` declarations themselves carry no
abstract/concrete marker. A "the metaclass must be concrete" check is therefore
not possible from this table and is not implemented — see
`docs/project/roadmap.md` § D7 for the abstract-`sysml:Import` finding that check
would otherwise have caught.

## Regenerating

Clone the ontology (any revision; the SHA is recorded, not pinned):

```bash
git clone https://github.com/Open-MBEE/sysmlv2-rdf-ontology.git
```

Then, from the repository root:

```bash
go run -C tools ./gen/ontology -ontology /path/to/sysmlv2-rdf-ontology
```

or, equivalently, with the checkout in `$SYSMLV2_RDF_ONTOLOGY`:

```bash
SYSMLV2_RDF_ONTOLOGY=/path/to/sysmlv2-rdf-ontology go generate ./internal/translate/rdf/ontology
```

The generator lives in the `tools/` module and writes the table under the
repository root it finds above its working directory; pass the checkout as an
absolute path. It reads the ontology version from the checkout's `sysml2/README.md`
and the commit SHA from the checkout's on-disk Git metadata. It does not need a
`git` binary, refuses to write a table if any property has no `rdfs:domain` or
if a property IRI disagrees with its domain or names no eStructuralFeature in
the ecore, and overwrites `table.go` in place.
Review the diff, then run
`go test ./internal/translate/rdf/ontology/... ./internal/translate/export/...` — the
export gate compares the golden graphs against the new table and will report
anything the ontology bump changed.
