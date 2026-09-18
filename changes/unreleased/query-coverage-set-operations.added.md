- **Document queries express requirement coverage gaps.** The new
  `DocumentQueries::WhereRelated(source, relationshipKind, direction, maxDepth, exists = true)`
  keeps each row by whether at least one element is reachable from it over a named relationship —
  every kind `RelatedElements` accepts, through the same edge tables, typed errors and visit
  budget — and `exists = false` keeps the rows with none, so "which requirements does nothing
  satisfy or verify" is one filter over incoming `satisfaction` or `verification` edges. The new
  ordered set operations `Except(source, exclude)` and `Union(source, other)` combine query results
  by the identity traversal already deduplicates by (a model element by its declaration, a held
  object by the object, a verdict by its assertion and the object it was checked on), keeping
  source order and projected columns. The query cookbook gains a Coverage section
  (`UnsatisfiedRequirements`, `UnverifiedRequirements`, their union and difference) and a
  Requirement hierarchy recipe that lists nested requirement usages and definitions under a root
  in tree order with `shortName`, `name` and `documentation`, which the requirements example
  renders as a table of its report.
