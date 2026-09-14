- **A document query over gRPC binds an object the service holds.** `Instantiate` now keeps
  the object it creates, in one runtime per cached model, for as long as the model stays cached;
  instantiating the same usage again denotes the new object and keeps the earlier one by id.
  `RunDocumentQuery` binds a parameter to such an object through the new `object` arm of
  `DocumentValue` — a `DocumentObject` naming it by `instance_id`, by `path` (`car`,
  `Garage::car`, `#2`, `car.wheels[2]`: what `%run-query` accepts) or by both — and runs the
  query in that runtime over the held population, so `DocumentQueries::Objects(type = T)`
  enumerates what the model holds (no rows before the first `Instantiate`) and `Verdicts`
  checks a bound object's current values. A row that is an object, and an object-valued cell,
  is answered as the `object` arm with the object's id, the path it is reached under and the
  usage it stands for; `RenderDocument` renders over the same population, as `-render-document`
  does beside `-instantiate`. A binding while nothing is held or naming an unknown id or usage
  is `NOT_FOUND`; a path that does not reach an object, an out-of-range index, an object bound
  to a non-`Element` parameter or an id its path disagrees with is `INVALID_ARGUMENT`, with the
  REPL's wording. The Go client binds with `opensysml.ObjectByID`/`ObjectByPath` and decodes
  `Object` (`ID`, `Path`, `Element`) as a cell and as `Row.Object`; the Python client binds with
  `ObjectRef(id=…)`/`ObjectRef(path=…)` and decodes `ObjectRef` as a cell and as
  `DocumentRow.object`; the Node, Java and Rust clients carry the regenerated stubs.
