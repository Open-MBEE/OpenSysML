- **The `meta` cast evaluates.** `x meta T` (KerML 1.0 §7.4.9.2 MetaCastExpression) is now a
  value rather than the refusal `unsupported operator: 'meta'`: as the shorthand for
  `x.metadata as T` it answers, of the element `x` names, the metadata annotations whose type
  conforms to `T` in model order, then the element's reflective metaobject when its own metaclass
  conforms to `T` (§8.3.4.8.15), and `()` when neither does (`seatBelt meta SysML::PartDefinition`
  for a part usage). The metaobject is a value of its own kind: the element together with the
  reflective metaclass that classifies it, equal to and identical with every other metaobject of
  the same element whatever it was cast to, and rendered `meta(Pkg::x : SysML::Systems::PartUsage)`
  in the REPL and in traces. The metaclass's features read off it through ordinary member access —
  `declaredName`, `name`, `qualifiedName`, `shortName`, `documentation`, `isAbstract`, `isComposite`,
  `isDerived`, `isEnd`, `isOrdered`, `isUnique`, `isVariable`, `isConstant`, `isPortion`,
  `isSufficient`, the element-valued `owner`, `ownedMember`, `ownedFeature` and `type` (metaobjects
  in turn, `definition` for a SysML usage) and `direction` (a `FeatureDirectionKind` literal) —
  each shaped by the feature's declared multiplicity; a feature the metaclass declares but the
  runtime does not derive (`ownedRelationship`, …) is a typed error naming the feature and the
  element, and a name the metaclass does not declare is the ordinary missing-member error.
  `sysml -compile` refuses a `meta` cast by name, a metaobject having no native representation;
  `@@`, `@` and the static reading of `meta` in a `SemanticMetadata::baseType` are unchanged.
- **`x.metadata` ends with the element's reflective metaobject.** A MetadataAccessExpression
  yields the referenced element's metadata annotations followed by one metaobject of the
  element's own metaclass (KerML 1.0 §8.3.4.8.15), so `seatBelt.metadata->size()` on a part
  annotated once is `2` and the metadata of an element nothing annotates is that metaobject
  alone rather than `()`. The pilot evaluator answers the same counts; the two conformance
  fixtures and the pilot referee record moved with it.
- **Metaobjects cross the gRPC boundary.** `Value.metaobject` carries `element_id`, the
  qualified name of the element (its identity), and `metaclass_id`, the qualified name of the
  metaclass that classifies it, in both directions of `Evaluate`, `EvaluateCalc`, `ExecuteAction`
  and `RunAnalysis`; a metaobject sent as an argument is rebound to the named model's element,
  `metaclass_id` resolved when omitted and refused in band when it names another metaclass, and
  one naming no element is refused in band rather than read as null. The service advertises the
  `metaobject_values` capability; without it a metaobject in a response is an unsupported null and
  one in a request is `UNIMPLEMENTED`. The Go, Python, Node, Java and Rust clients read the arm as
  a typed value equal by element (`opensysml.Metaobject`, `{ kind: "metaobject" }`,
  `Value.MetaobjectValue`, `Value::Metaobject`), reject an empty `element_id`, and refuse to send
  one to a service lacking the capability; the conformance suite pins the arm over gRPC, Connect
  protobuf and Connect JSON.
