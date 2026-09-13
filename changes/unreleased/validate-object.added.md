- **An object is validated as a whole.** `%validate <object>` at the prompt, `-validate=<object>`
  on the command line and the `ValidateInstance` RPC evaluate every assertion about an object and
  the objects it holds — each `assert constraint` the carrier's type declares or inherits, each
  requirement usage it carries, and each `satisfy` assertion whose subject is in the tree — against
  the concrete object carrying it, nested parts and every element of a collection included, and
  report one verdict per assertion per object, labelled by the path from the object validated
  (`car.wheels[2]`), then one verdict about the object itself. A condition that evaluated false is
  violated; one that could not be evaluated is undecided with the reason, and leaves the object
  not shown valid rather than valid; a walk cut short by an object graph without end is reported
  bounded and not valid; an object no assertion is about decides nothing and is not shown valid
  either. A constraint declared without `assert` is not swept, and a symbol with no object to
  validate — a package, an attribute — is refused as the wrong kind. The object is named
  as every prompt command names one: by the name it was instantiated under, by id, or by a path
  into what it holds. `-validate` without an object still checks only that the model analyses
  cleanly. Over the wire the response carries each verdict's `instance_path` and a `summary`
  verdict of kind `object`; the Go client answers `Client.ValidateInstance` with a `Validation`
  (`Valid()`, `Violated()`, `Bounded`) and the Python client `Model.validate_instance` with a
  `Validation` (`valid`, `violated`, `undecided`, `bounded`), each verdict carrying its
  `instance_path`.
