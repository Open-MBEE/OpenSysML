- **The SysML v1 migration reads past the UML metaclass where the tool's own encoding hides the
  v2 form.** A Signal is an `item def`, and properties typed by one are `item` / `ref item`, not
  `attribute def` and `attribute`. A constraint block's parameters are public `in attribute`s
  whether the tool stores them as UML Properties or, as MagicDraw does under a
  «ConstraintParameter» marker, as UML Ports, and a `private` parameter loses its visibility so
  the block's binding connectors can reach it — as does any private feature a connector, slot,
  redefinition or subset reaches from outside, the report naming what reached it. A type
  referenced by href into the SysML or UML primitive library resolves to `ScalarValues::Real` /
  `Integer` / `Boolean` / `String` from a plain (`PrimitiveTypes.xmi#Real`) or dotted
  (`SysML.xmi#SysML_dataType.Real`) fragment, or from the qualified name MagicDraw records
  beside an opaque id (`referentPath`) into a module named for that library; the tool library's `float`, `double`, `int`, `long`,
  `short`, `byte` and `boolean` are written as the matching scalar and reported as
  approximations. A nested connector end's `propertyPath` given as one whitespace-separated
  attribute is split into its ids rather than failing to resolve. An opaque expression is copied
  only when it parses as v2 and every name it uses is a written element visible where it is
  written, so a JavaScript body, a bare enumeration literal or a call to an operation stays a
  comment, and a private inherited feature an expression names is exposed like one a connector
  reaches. An instance of a value type is
  an `attribute` typed by it rather than an `individual def` that cannot specialize an attribute
  def; a slot contradicting its feature — more values than the multiplicity allows, a repeated
  value of a unique feature, a feature of a classifier the instance is not written to
  specialize — is left as a comment; a real
  literal on an `Integer` feature and a numeric string on a scalar feature take the feature's
  scalar, a literal on a value type or enumeration with no scalar base is not bound, and a
  default naming an instance of a block types the usage by that individual — its only type when
  the property is untyped — instead of being written as a value. An undirected part or item property of an interface block is a `ref`,
  since a port owns no composite parts, and a specializing block's property named like an
  inherited one redefines it when both are the same kind of usage, and is reported when they
  are not. Migrating the current TMT observatory model now yields notation
  with no analysis errors, down from a hundred.
