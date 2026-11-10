# OSLC Query text

OpenSysML accepts OSLC Query 3.0 text as an element-identification query over
the symbols in a parsed model, and over the objects a session holds of them. It
is not an OSLC server: this implementation does not provide query capabilities,
result containers, service providers, or resource shapes.

The text is a *translation* into the document-query plans of
[QVTS](../guide/qvts.md): `internal/doc/querytranslate` parses it into a
transport-neutral query, compiles that to the plan IR of `internal/ir/queryplan`
(`WhereType`, `WhereName`, `WhereFeature`, `WhereText`, `Project`, `OrderBy`,
`Objects`, `Except`), and `internal/doc/queryexec` executes the plan exactly as it
executes a `DocumentQueries` query. The structured API `Query` of the gRPC
service is translated by the same package onto the same operations, so the
three query spellings read the model with one semantics and refuse with one
record, the [query outcome](../internals/design/query-outcomes.md).

## Grammar

The supported filter is a compound term:

```text
compound   ::= term (" and " term)*
term       ::= identifier comparison_op value
             | identifier " in " "[" value ("," value)* "]"
             | identifier "{" compound "}"
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

A scoped term `sysml:owner{sysml:name="Platform"}` constrains a related
element: the term inside the braces is evaluated over the element the property
names, so the example keeps the elements owned by something named `Platform`.
Scoped terms nest (`sysml:owner{sysml:owner{sysml:name="Robot"}}`) and take
compound terms, and they apply to any element-valued property, a scoped
`rdf:type` reading the related element's metaclass.

The `oslc.where`, `oslc.select`, `oslc.orderBy`, `oslc.searchTerms`, and
`oslc.prefix` parameters may be supplied as a URL-encoded query-parameter
string. Standard decoding applies: `+` means a space and percent-encoded
sequences are decoded. A bare where-clause remains accepted. `oslc.orderBy`
accepts `+` and `-` prefixes. Selection and ordering are comma-separated, and
both take nested properties: `oslc.select=sysml:name,sysml:owner{sysml:name}`
reports the owner's name as the column `owner.name`, and
`oslc.orderBy=-sysml:owner{sysml:name}` orders by it. `*` in `oslc.select`,
alone or inside a nested selection, is every property the selected rows could
name, read of the reflective table `Project` reads (see [query
kinds](../manual/query-kinds.md)): the properties of the element rows'
metaclasses the evaluator serves (`isComposite` and `portionKind` of a
`PartUsage`, `body` of a `Comment`) with the evaluator's own (`rdf:type`,
`sysx:multiplicityUpper`), or the features an object row's model declares; a
column no row carries is left out, and a nested `sysml:owner{*}` reads the
related element's. A query that selects nothing reports every property but
`sysml:elementId`, the ones of particular metaclasses absent on the other rows.
`oslc.searchTerms="wheel","axle"` keeps the elements whose text properties
contain any of the quoted terms, case-insensitively, as `WhereText` matches.

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

`sysml:`, `sysx:`, `elmt:` and `rdf:` are bound by default to the namespaces
the RDF export writes (`internal/translate/rdf`: the OMG SysML namespace,
OpenSysML's `urn:opensysml:sysml:`, the `urn:sysmlv2:element:` element
identifiers, and RDF). `oslc.prefix` adds or rebinds a prefix. An unbound
prefix is an error.

A property is one of three things, told apart by its namespace:

| Spelling | Property |
| --- | --- |
| `rdf:type` | The element's metaclass (`@type`); the `elmt:` form selects objects, below |
| `sysml:<property>` | A property the SysML metamodel declares, read reflectively from the model's metaclass declarations — `sysml:name`, `sysml:qualifiedName`, `sysml:owner`, `sysml:isAbstract`, `sysml:documentation`, `sysml:type`, `sysml:satisfiedRequirement`, … every property `Project` can read of the element |
| `sysx:<property>` | What the metamodel does not declare: the two evaluator-derived properties `sysx:multiplicityLower` and `sysx:multiplicityUpper`, and the features of the model itself (`sysx:pressure`, `sysx:wheels.diameter`), read of each row as a document query's `WhereFeature` reads them |

An element-valued property reads as the element's identity, and a scoped term
or nested selection continues through it. An expression element the metamodel
derives — a send's `sysml:payloadArgument`, `sysml:senderArgument` or
`sysml:receiverArgument` — has no identity of its own, so it reads, compares
and orders as written (`new Alarm(3)`), while a path through it reaches what
it refers to: `sysml:senderArgument{rdf:type}` is `FeatureReferenceExpression`,
`sysml:senderArgument{sysml:referent{sysml:name}}` the port it names.

A property the metamodel declares for some metaclass is never unknown: selected
of rows none of which carry it (`oslc.select=sysml:body` over a model without
comments), it is absent on every row; compared, it keeps no row; ordered by, it
leaves the order. So is a metamodel property OpenSysML does not derive yet,
such as an accept's `sysml:payloadArgument`. A `sysml:` spelling of a property
the metamodel does not declare is refused before any element is read, as a QVTS
path failure whose diagnostic lists the nearest spellings under the query's own
`oslc.prefix` bindings, so they are always writable as they read:
`sysml:multiplicityUpper` names `sysx:multiplicityUpper`, `sysml:nmae` names
`sysml:name`. A `sysx:` feature the rows do not carry is refused by the
evaluator the same way, naming the features the rows do have. `@type` and `@id`
are not OSLC query text: the first is written `rdf:type`, and identity is
reported for every result rather than asked for.

## Elements and objects

`rdf:type=sysml:PartUsage` selects the *declared* elements of that metaclass
(`WhereType`, exact), over the model's elements in declaration order.
`rdf:type=elmt:<id>` selects the *held objects* — the instances a session
created with `-instantiate`, `%instantiate` or the `Instantiate` RPC — whose
type is the element `<id>` identifies, over the objects the model holds
exactly as a document query's `Objects(type)` does. The id is the one the RDF
export writes for the element (`rdf.EncodeElementID` of its qualified name:
`elmt:Garage__Wheel`), and the full `<urn:sysmlv2:element:…>` IRI selects
alike. The two forms do not mix in one `in [...]` list, and an object term is
not negated or nested.

An object row answers under the path a session reaches it by
(`Garage::car.wheels[2]`), with the usage it instantiates as its identity and
metaclass; the gRPC `QueryResultElement.path` field carries the path, and the
REPL prints it in place of the qualified name. `sysx:` features
(`sysx:pressure`, `sysx:engine{sysx:power}` — the column `engine.power`) read
the object's values, through the objects its features hold, so
`rdf:type=elmt:Garage__Wheel and sysx:pressure<25` keeps the under-inflated
wheels and `rdf:type=elmt:Garage__Car and sysx:engine{sysx:power>50}` the
cars with a strong engine. Two object terms joined by `and` keep the objects
both select.

## Semantics and choices

Every term is one plan operation over the rows the terms before it kept, so
`and` is nesting and the terms compare as the document-query evaluator
compares: `sysml:name` terms are `WhereName`, `rdf:type` terms `WhereType`,
every other term a `WhereFeature` with `tolerate=true`, which keeps a row whose
property is absent out of the answer without failing the query. Equality,
inequality and `in` compare typed values — a number against a number, a
boolean against `true`/`false`, a string lexically — and a multi-valued
property matches when any value equals an operand. `in [...]` keeps the rows
any alternative keeps, in source order (`Except` of the rows no alternative
kept). Ordered comparisons take exactly one numeric operand and apply to the
numeric properties: the two multiplicity bounds, where `*` is positive
infinity for model values and operands alike (`sysx:multiplicityUpper=*`
identifies the unbounded usages), and the model's own attributes. An ordered
comparison on a text property, a boolean operand that is not `true`/`false`,
and a metaclass the library does not declare are refused before any element
is read.

Results keep declaration order, with parents before their children, unless
`oslc.orderBy` is supplied. `oslc.select` is a `Project`; identity and
metamodel type are always present on every result. `oslc.orderBy=+a,-b` is a
nested `OrderBy` per term, applied in reverse precedence so the stable sort
keeps `b` within equal `a`; it compares values as the evaluator orders them
(numbers numerically, `*` as positive infinity, text lexically); rows missing
the property sort as the smallest value would — first ascending, last
descending — and keep their existing order among themselves.

OSLC compound terms have no `or`, so OSLC text and the structured API Query
are deliberately not interchangeable: an `or` in OSLC text is refused with a
typed error, while the API's composite `or` constraint translates to the same
plan algebra. [Which query is
which](../manual/query-kinds.md#the-api-query-over-a-project) places the two
beside the project's other query surfaces.

## Construct coverage

| Construct | Behavior |
| --- | --- |
| `term and term` | Supported: nested plan operations. |
| `identifier in [...]` | Supported: the rows any alternative keeps, in source order. |
| `scoped_term` / nested property query | Supported: the inner compound is evaluated over the related element the property names, to any depth. |
| Nested `oslc.select` / `oslc.orderBy` (`sysml:owner{sysml:name}`) | Supported: reported and ordered as the property path `owner.name`. |
| Property wildcard `*` in `oslc.select` | Supported: every property the selected rows could name — their metaclasses' that the evaluator serves, the evaluator's own, an object's declared features — at any nesting level, sorted; a column no row carries is left out. |
| Property wildcard `*` in `oslc.where` or `oslc.orderBy` | Typed error. A term compares one property and an order has one key; the wildcard names every property. |
| Value wildcard `*` on the multiplicity bounds | Supported: `*` is the model's own infinity value, not a wildcard. |
| Value wildcard `*` on any other property | Typed error. A `*` value would be a wildcard, which no plan operation evaluates; refusing it avoids a lexical comparison that silently matches nothing. |
| `oslc.searchTerms` | Supported: `WhereText` over the result's text properties, any quoted term matching, case-insensitively. Terms are quoted strings; an empty parameter or an unquoted term is refused. |
| `rdf:type=elmt:<id>` | Supported: the held objects of that element, under their paths. |
| `or` | Typed error. OSLC Query 3.0 defines no disjunction; the structured API `Query` does. |
| `oslc.properties` | Typed error naming `oslc.select`, which reports the property projection. |
| Any other `oslc.*` parameter, or a repeated one | Typed error. |
| Language-tagged literals | Typed error; model properties do not carry language tags. |
| Non-`xsd:` or unsupported `xsd:` datatypes | Typed error rather than a potentially misleading comparison. |

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
gRPC response instead keys properties by their query property names (`name`,
`owner.name`, `pressure`), which the structured `query` field also uses.
A refused query carries the [query outcome](../internals/design/query-outcomes.md)
as a `QueryOutcome` status detail on every transport, as `RunDocumentQuery`
does, so a client reads the failing step, the frontier and the nearest spelling
rather than parsing prose.

The answer can be written as a `QVTS::QueryResult` view instead
([Write the answer as a view](../guide/qvts.md#write-the-answer-as-a-view)): `-query … -as-view
<file|->`, `%query -as-view …`, and `as_view` on the `Query` RPC, which fills
`QueryResponse.result_view` under the `query_result_view` capability. The view's `query`
attribute is the OSLC text as given, its exposed elements are the matched ones, and a refused
query — an unknown property — still yields a view, with `outcome = Outcome::pathError`, carried
as a `QueryResultView` status detail beside the `QueryOutcome`.
The REPL's `%query` and `sysml -query` identify unnamed elements with the same
positional `@N` IDs as the service query; see [the query model](api.md#the-query-model).
A query that matches nothing says so: the REPL prints `no
elements matched`, and the command reports it on standard error, so the result
rows on standard output stay one line per match. `-query` with empty text is
treated as a mistake rather than as an absent flag: it is refused instead of
starting the interactive REPL.
