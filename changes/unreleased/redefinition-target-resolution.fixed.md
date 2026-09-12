- **A redefinition's target is resolved from the owning type's generals and then the enclosing
  namespace, never from the owning type's own scope (KerML 8.2.3.5.2).** `attribute z :>> x;`
  used to bind a sibling `x`, a member the owning type imported (`private import Lib::*;`) or an
  alias it declared (`alias y for x;`), and `:>> C::nope` or `:>> w.x` started at a sibling `C` or
  `w`; the pinned OMG pilot leaves all of these unresolved, and so does OpenSysML now, so the
  downstream reports those bindings drew (a featuring-type conflict, a conformance error) no
  longer appear. A target the generals lack is still found in the enclosing namespace and
  outward, as before.
  The general-type search itself is completed so that no case the pilot accepts moved: a
  member a general acquires through its `public import`, the `source`/`target` ends of a
  redefined flow or transfer, a namesake reached through the type of a redefined feature, a
  nested metadata body at any depth, and a qualified chain through an inherited feature all
  resolve from the generals. The standard library snapshot is regenerated: a flow definition
  now specializes `Flows::MessageAction` (`Flows::Message` when it declares two ends) rather
  than `Flows::Flow`, `Flows::Flow::source`/`target` no longer list themselves as their own
  generals, and `ShapeItems::CuboidOrTriangularPrism::ff`/`rf`'s `faces::edges` binds through
  `Polygon`'s inherited `faces`, as the pilot binds it.
