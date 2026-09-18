# Getting Started

## The smallest working document

A document is a `part def` specializing `DocumentQueries::Document` with a
literal `title` and content blocks as nested parts. This is the whole of the
smallest one:

```sysml
package Hello {
	private import DocumentQueries::*;
	private import ScalarValues::*;

	part def HelloReport :> Document {
		attribute redefines title = "Hello, Documents";

		part greeting : Paragraph {
			attribute redefines text = "This document was generated from a SysML v2 model.";
		}
	}
}
```

Save it as `hello.sysml` and render it:

```console
$ sysml hello.sysml -render-document Hello::HelloReport
✓ package Hello
# Hello, Documents

This document was generated from a SysML v2 model.
```

The status line goes to stderr; the Markdown goes to stdout, or to a file with
`-o hello.md`.

Three things to note:

- `DocumentQueries` is bundled with the binary — no extra files to import.
- `title` is required and must be a literal string; a document without one is
  a planning error, not an empty heading.
- Content renders in declaration order.

## Adding model data

A document earns its keep when its content comes from the model. That takes a
*query* — a `calc def` specializing `DocumentQueries::Query` — and a block
that invokes it:

```sysml
package Hello {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;

	part rover {
		part chassis;
		part arm;
		part mast;
	}

	calc def PartNames :> Query {
		in root : Element;
		Project(
			source = OrderBy(
				source = OwnedElements(source = root),
				property = "name",
				direction = "ascending",
				missing = "last",
				multiple = "error"
			),
			properties = ("name")
		)
	}

	part def RoverReport :> Document {
		attribute redefines title = "Rover Parts";

		part intro : Paragraph {
			attribute redefines text = "The rover's top-level parts, from the model:";
		}

		part partsTable : Table {
			attribute redefines caption = "Top-level parts";
			calc rows : PartNames {
				in root = rover;
			}
		}
	}
}
```

The query reads inside-out: `OwnedElements` collects `rover`'s children,
`OrderBy` sorts them by name, and `Project` turns each element into a row with
one `name` column. The `in root : Element;` parameter makes the query
reusable; the table's `calc rows : PartNames { in root = rover; }` binds it to
a concrete element.

```console
$ sysml rover.sysml -render-document Hello::RoverReport
✓ package Hello
# Rover Parts

The rover's top-level parts, from the model:

*Top-level parts*

| name |
| --- |
| arm |
| chassis |
| mast |
```

Add a part to `rover` and rerun — the table updates. That is the entire
workflow.

## Running a query on its own

While writing a query it is faster to run it directly than to render a
document around it. `-run-query` takes the query's qualified name and its
bindings:

```console
$ sysml rover.sysml -run-query "Hello::PartNames root=Hello::rover"
✓ package Hello
✓ Query Hello::PartNames returned 3 rows
  Columns: name
  Row 1: Hello::rover::arm
    name = "arm"
  ...
```

The same commands exist in the REPL as `%render-document` and `%run-query`,
over gRPC, and in the VS Code extension — see [Interfaces](interfaces.md).

## Where to go from here

- The [query cookbook](query-cookbook.md) covers every query operation with a
  runnable recipe.
- [Document authoring](authoring.md) covers every content block, including
  inline runs, cross-references, grouped tables and diagrams.
- The [worked example](worked-example.md) puts it all together in one report.
