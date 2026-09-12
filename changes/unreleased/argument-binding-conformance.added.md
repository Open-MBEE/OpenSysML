- **Non-conforming operator and invocation arguments warn `Bound features should have conforming
  types`, as the reference does.** Each argument of an operator or invocation expression is bound to
  the parameter it fills (KerML 1.1 §8.3.4.8.3), and the type checker now judges that implied binding
  with the rule an explicit `bind` gets: an argument whose static type conforms neither to nor from
  the selected function's parameter type — `rearWheel + 1` with `rearWheel : Wheel`, `sum(robots.mass)`
  passing `MassValue`s to `RealFunctions::sum` — draws the warning, at the argument of an invocation
  and at the whole operator expression, where the reference puts it. Positional, named and receiver
  arguments are judged alike, through feature chains and nested invocations. Nothing is reported where
  either side is unknown — an unresolved or ambiguous callee, an untyped, unbound or collection-valued
  argument, a parameter typed by a `Collection` or `Element` — or where a precise type error already
  covers the argument, and a conforming argument (`Integer` into `Real`, `MassValue` into
  `ScalarQuantityValue`) stays silent. The pilot Xpect `warnings` kind agrees 113 of 113, and the two
  reference-only rows on `examples/disposal-team-demo/team.sysml:29` are agreed.
