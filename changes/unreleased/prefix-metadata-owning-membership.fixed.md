- A `#M` prefix on a definition, usage or package is now reflected as the `OwningMembership` the
  annotated element owns its metadata usage through, first among its owned relationships, as the
  grammar and the RDF conversion have it; a query read an `Annotation` with no `elementId` there
  before. A relationship such as `dependency` still owns its prefix through an `Annotation`.
