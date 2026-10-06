- **`%view` checks every concern a viewpoint frames against what the view exposes, without the view
  restating the framing.** A view no longer has to write `frame concern …;` for each concern of the
  viewpoints it `satisfy`s — SysML v2 admits `frame` in a requirement, concern or viewpoint body, never
  in a view body — so the `violated (framed by the viewpoint but not by the view)` verdict is gone and
  each framed concern is evaluated, in the viewpoint's order, against the exposed elements its subject
  admits; the other verdicts (`conforms`, `violated` naming the element and condition, `unevaluable`
  for no condition or an unresolved reference, `(from <view>)` on an inherited `satisfy`) are as before.
  A `frame` written in a view definition or usage body is still parsed but draws the
  `nonstandard-notation` warning, which `-strict` escalates to an error as the pinned OMG pilot
  rejects it. The views, disposal-robot and self-model demos and the REPL handbook transcript drop
  their view-body framings; their `%view` output is unchanged.
