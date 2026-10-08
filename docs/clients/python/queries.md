# Queries and documents

The standard query API expresses a query with `scope`, `select` and `where`.
`Model.query` accepts either a standard payload or keyword arguments:

```python
result = model.query({
    "@type": "Query",
    "where": {
        "@type": "PrimitiveConstraint",
        "operator": "=",
        "property": "@type",
        "value": ["PartUsage"],
    },
})

result = model.query(
    scope=["Demo::vehicle"],
    select=["name", "qualifiedName"],
    where={"operator": "=", "property": "@type", "value": ["PartUsage"]},
)
```

Each result is a `QueryElement` with the element's qualified name, metamodel
type and selected properties. A property that an element does not have is
absent, not empty. The `scope` covers named elements and their nested contents;
an empty scope covers the loaded model. This standard query surface has no
graph traversal or transitive-closure operator.

A payload the standard does not describe raises `QueryError` before it is sent.
A property the service does not provide raises `InvalidRequestError` naming
available properties. A service without the `query` capability raises
`MissingCapabilityError`.

## Native document queries

`run_document_query` runs a `calc def` specializing
`DocumentQueries::Query`; `render_document` renders a `part def` specializing
`DocumentQueries::Document`.

```python
result = model.run_document_query("Observatory::SubsystemTable")
result.columns
for row in result.rows:
    print(row.element.id, row.cells)

result = model.run_document_query(
    "Observatory::HeavierThan",
    bindings={"threshold": 10.0},
)

markdown = model.render_document("Observatory::SubsystemReport")
html = model.render_document("Observatory::SubsystemReport", form="html")
```

Bindings may be element references, strings, integers, floats,
`fractions.Fraction` values, booleans or lists of those values. Other binding types raise `DocumentQueryError` before a
request is made. Results use those Python types, `ElementRef` for model
elements, and `opensysml.INFINITY` for unbounded multiplicities.

`render_document` takes no bindings because its queries receive parameters from
the document model. It returns Markdown by default or standalone HTML with
`form="html"`; PDF rendering remains available through the CLI and its
converter toolchain. The service must advertise `document_query`,
`render_document`, and `render_document_html` for their corresponding calls.

For the full supported property and query surface, see the
[Go API reference](../../reference/api.md) and
[Python API reference](../../reference/python-api.md).
