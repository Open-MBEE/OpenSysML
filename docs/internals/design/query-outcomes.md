# Query outcomes: value, path error, model error

How a document query's failures are told apart, and why. The normative reference is the
QVTS-Core specification ([Open-MBEE/qvts](https://github.com/Open-MBEE/qvts), `docs/qvts-core.md`
§3 and §5), whose native semantics the document-query engine (`internal/doc/queryexec`) is.

## The three outcomes

A query step either produces a **value** — possibly an empty collection, when the step's result
type admits one: nothing owned, nothing related, no row selected — or fails with an error about
one of two things:

- a **path error** says the *query* is wrong about this model: a name no element carries, a
  feature the element lacks, a metaclass or relationship kind the metamodel does not define. The
  error states where the path stopped resolving and what the model offers there (the *frontier*),
  so the reader can tell a typo from a model that lacks the element;
- a **model error** says the *model* violates a constraint the step relied on: a name several
  elements carry, more values than a feature's multiplicity admits. The error is a finding about
  the model, located at the element, to be fixed in the model rather than in the query.

What the engine never does is answer an empty collection for either: OCL's `invalid` collapsing
to `∅` is the ambiguity QVTS exists to remove.

## Where the engine stands

| Step | Value | Path error | Model error |
|:--|:--|:--|:--|
| `Named` | the element | `unknown-element`: the longest prefix that resolves, the segment it lacks, the nearest spellings; an alias that denotes nothing | `ambiguous-element`: the elements, by qualified name |
| `WhereType`, `WhereMetadata` | the selected rows | `unknown-classification` | — |
| `RelatedElements` | the neighbours, possibly none | `unknown-relationship` | — |
| `Project`, `WhereFeature` on a feature the element lacks | an empty cell, with `tolerate = true` | `missing-feature`: the element (`Target`), the feature (`Property`), the features it does have (`Candidates`); `unknown-property` when no row carries it | — |
| `Project`, `WhereFeature` on a metadata path (`Tag::key`) the element lacks | *today an empty cell* | *to do:* decide whether an absent annotation is a path error | — |
| `Project` on a `[1]` feature with two values | *today the first value* | — | *to do:* `multiplicity` |

`Error` carries the frontier in `Target` (the namespace that resolved), `Expected` (the segment it
lacks) and `Candidates` (the nearest spellings, or the elements an ambiguous name denotes), so a
surface can render it as more than a line of text — the REPL, the gRPC service and the SPARQL
differential in `qvts/sparql` all read the same record.
