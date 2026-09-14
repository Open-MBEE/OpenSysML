- **Document queries report which constraints and requirements hold.** The new
  `DocumentQueries::Verdicts(source, kind = "all")` checks the object behind each row as a whole —
  the object the session holds when the binding is one (`%instantiate car`, `-instantiate`), the
  element's declared object otherwise — and answers one **verdict row** per assertion about it and
  the objects it holds: every `assert constraint`, every requirement carried, every `satisfy` whose
  subject it is, and each verification case verifying such a requirement, a collection's members
  under their own paths (`car.wheels[2]`). A verdict row stands for the assertion (so `name`,
  `WhereName` and `WhereType` read the constraint or requirement) and adds `kind`, `carrier`,
  `path`, `verdict` (`holds`, `violated`, `undecided`), `condition`, `reason` and `verification`,
  which `Project`, `WhereFeature`, `OrderBy` and `Column` read; `kind = "constraint"`
  (`requirement`, `satisfaction`, `verification`) keeps one kind. A row that is no object, and a
  walk the runtime could not complete, are typed errors rather than a table missing rows. Verdicts
  print in `%run-query`/`-run-query` as `<assertion> on <path>: <verdict>`, render in Markdown and
  PDF as that text and in HTML as a `span.sysml-verdict` (`data-verdict`, `data-path`,
  `data-object`), and `RunDocumentQuery` answers them as the new `verdict` arm of `DocumentValue`
  (`DocumentVerdict`), decoded by the Go and Python clients as `DocumentVerdict`; a verdict bound
  as a parameter is refused.
