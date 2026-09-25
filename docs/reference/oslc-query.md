# OSLC Query text

OpenSysML accepts OSLC Query 3.0 text as an element-identification query over
the symbols in a parsed model. It is not an OSLC server: this implementation
does not provide query capabilities, result containers, service providers, or
resource shapes.

## Grammar

The supported filter is a compound term:

```text
term (" and " term)*
term ::= identifier comparison_op value
       | identifier " in " "[" value ("," value)* "]"
comparison_op ::= "=" | "!=" | "<" | ">" | "<=" | ">="
```

Identifiers are prefixed names. Values may be `<uri>`, a prefixed name,
quoted strings (with `\"` and `\\` escapes), `true`, `false`, or a decimal.
The supported typed literal suffixes are `^^xsd:string`, `^^xsd:boolean`,
`^^xsd:integer`, `^^xsd:decimal`, and `^^xsd:double`.

A `<uri>` in the SysML namespace and the prefixed name that expands to it are
the same value, so `rdf:type=<https://www.omg.org/spec/SysML#PartUsage>`,
`rdf:type=sysml:PartUsage`, and `rdf:type="PartUsage"` select alike. A model
qualified name is a literal, not a prefixed name: write
`sysml:qualifiedName="Robot::Platform::battery"` with the quotes. The same
holds for `sysml:type`, whose values are the qualified names of typing
definitions. A bare name is not a value: `sysml:name=battery` is refused, and
the diagnostic names the quoted form to write.

The `oslc.where`, `oslc.select`, `oslc.orderBy`, and `oslc.prefix` parameters
may be supplied as a URL-encoded query-parameter string. Standard decoding
applies: `+` means a space and percent-encoded sequences are decoded. A bare
where-clause remains accepted. `oslc.orderBy` accepts `+` and `-` prefixes.
Selection and ordering are comma-separated.

A parameter this implementation does not recognize is a typed error, not something
to ignore, because ignoring it would answer a different question than the one asked:
a misspelt `oslc.wheree` would otherwise select the whole model. A parameter
given twice is refused for the same reason, and so is a parameter with no
value: `oslc.select=` narrows nothing, so rather than quietly returning every
property (which is what omitting it means), it is refused. A property named
twice in one `oslc.select`, whether by the same prefix or by two prefixes that
name the same namespace, is refused as well: it is reported once, so the second
name could not be honored.

## Prefixes and properties

`sysml:` and `rdf:` are bound by default to the OMG SysML and RDF namespaces.
`oslc.prefix` adds or rebinds a prefix. An unbound prefix is an error, as is a
bound predicate that is not in this table:

| OSLC predicate | Query property |
| --- | --- |
| `rdf:type` | `@type` |
| `sysml:qualifiedName` | `qualifiedName` and element identity |
| `sysml:name` | `name` |
| `sysml:declaredName` | `declaredName` |
| `sysml:shortName` | `shortName` |
| `sysml:declaredShortName` | `declaredShortName` |
| `sysml:documentation` | `documentation` |
| `sysml:owner` | `owner` |
| `sysml:isAbstract` | `isAbstract` |
| `sysml:isIndividual` | `isIndividual` |
| `sysml:type` | `type` |
| `sysml:multiplicityLower` | `multiplicityLower` |
| `sysml:multiplicityUpper` | `multiplicityUpper` |
| `sysml:satisfiedRequirement` | `satisfiedRequirement` |
| `sysml:satisfyingFeature` | `satisfyingFeature` |

Unknown properties fail the query instead of silently returning no matches, and
the diagnostic lists the OSLC predicates of the left column, since the query
property names of the right column are the Go API's. `@type` and `@id` are not
OSLC query text: the first is written `rdf:type`, and identity is reported for
every result rather than asked for. The listed predicates follow the query's own
`oslc.prefix` bindings, so they are always writable as they read: rebinding
`sysml` to another namespace lists the properties under whichever prefix names
the SysML namespace, or reports them as unnamed if none does.

## Semantics and choices

Equality, inequality, and `in` compare lexical values. A multi-valued property
matches when any value equals an operand. Ordered comparisons are numeric and
are limited to the two multiplicity properties; `*` is positive infinity for
those existing model values and operands. Ordered comparisons require exactly
one operand.

Results keep declaration order, with parents before their children, unless
`oslc.orderBy` is supplied. `oslc.select` controls which properties are reported;
identity and metamodel type are always present on every result.
`oslc.orderBy` compares multiplicity properties numerically, with `*` as
positive infinity, and compares other properties lexically. Elements missing the
property keep their existing order.
The two multiplicity properties also accept `*` as a value in equality comparisons:
`sysml:multiplicityUpper=*` identifies the unbounded usages. On every other property
a `*` value would be a wildcard, which this implementation does not evaluate, so it
is refused rather than compared lexically and silently matching nothing.

OSLC compound terms have no `or`, so OSLC text and the structured API Query
are deliberately not interchangeable; [Which query is
which](../manual/query-kinds.md#the-api-query-over-a-project) places the two
beside the project's other query surfaces.

## Unsupported constructs

| Construct | Behavior |
| --- | --- |
| `scoped_term` / nested property query | Typed error. The OSLC rationale is to match a resource using a related resource's property; graph-pattern traversal is not implemented by this symbol-index evaluator. |
| Property wildcard (`*` in `oslc.where`, `oslc.select`, or `oslc.orderBy`) | Typed error. Generic property wildcards are not implemented. |
| Value wildcard (`*` compared against any property but the two multiplicity bounds) | Typed error. On the multiplicity bounds `*` is the model's own infinity value, not a wildcard. |
| `oslc.searchTerms` | Typed error. Free-text search is distinct from property identification. |
| `oslc.properties` | Typed error naming `oslc.select`, which reports the property projection. |
| Any other `oslc.*` parameter, or a repeated one | Typed error. |
| Language-tagged literals | Typed error; model properties do not carry language tags. |
| Non-`xsd:` or unsupported `xsd:` datatypes | Typed error rather than a potentially misleading lexical comparison. |

## API and command surfaces

The gRPC `QueryRequest.oslc_query` field carries this text and is mutually
exclusive with the structured `query` field. The service advertises the
`oslc_query` capability in addition to `query`.

The REPL accepts `%query <oslc-query>`. The `sysml` command accepts
`-query <oslc-query>` alongside the other model modes. Both print one matched
element per line as qualified name and metamodel type, followed by selected
properties. A reported property carries the name the query asked for it by, so
`oslc.select=rdf:type` reports `rdf:type=PartUsage` and a reported row can be
written back into a query; rebinding a prefix renames it in the answer too. The
gRPC response instead keys properties by the query property names of the table
above, which the structured `query` field also uses.
A query that matches nothing says so: the REPL prints `no
elements matched`, and the command reports it on standard error, so the result
rows on standard output stay one line per match. `-query` with empty text is
treated as a mistake rather than as an absent flag: it is refused instead of
starting the interactive REPL.
