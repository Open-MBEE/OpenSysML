- **Document queries read the objects a session holds.** A `%run-query`/`-run-query` parameter
  written as a usage's name binds the object the session holds under it while it holds one
  (`car` after `%instantiate car`), `#2` binds an object by id and `car.wheels[2]` a nested one by
  path, and the element as before when nothing is held. Every query operation that takes an
  element takes an object and reads what it holds: `OwnedElements` and `Descendants` are the
  objects it holds as parts, each element of a collection under its own path (`wheels[1]`,
  `wheels[2]`), `Ancestors` the objects holding it, `WhereType` tests its types, `WhereName` its
  path, and `WhereFeature`, `Project`, `OrderBy` and `Column` read the values it holds now — after
  a run changed them, not the declared defaults. The new `DocumentQueries::Objects(type = T)`
  enumerates every object the session holds that is of the type; outside a session it is refused
  with an error saying to instantiate an object first. `WhereMetadata` tests the usage an object
  stands for; `RelatedElements` reads the model's relationships and refuses an object row. A document renders over objects too: `-instantiate <name>`
  is now accepted beside `-render-document`/`-render-documents` and creates the objects first, and
  a document parameter bound to a usage's name binds the object held under it. Objects render by
  path in Markdown and PDF; in HTML each carries `data-object="#<id>"` beside the `data-element`
  of the usage it stands for and is a `span.sysml-object`.
