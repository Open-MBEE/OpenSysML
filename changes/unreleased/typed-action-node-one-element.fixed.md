- An action node whose declaration states more than a name (`action n : T assign p.w := 1;`,
  `action n[1] if …`, `#M action n :> m send …`, and the `while` and `for` forms) is one element
  `n` of the node's metaclass, typed and specialized as declared, on the Turtle and API JSON
  export and on reflection, as the name-only form is. `n` was written as an `ActionUsage` owning
  an unnamed node (`n___400`) of that metaclass, and reflection reported two elements. A graph
  states the declaration, so the notation comes back from it without `sysx:sourceText`.
- A named assignment, conditional or loop node whose id an `ElementId` annotation declares converts
  back from a graph without `sysx:sourceText`: the annotation is written `about` the node in the body
  the node is a member of, since the node takes no body of its own. The conversion was refused.
