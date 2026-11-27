- An action node whose declaration states more than a name (`action n : T assign p.w := 1;`,
  `action n[1] if …`, `#M action n :> m send …`, and the `while` and `for` forms) is one element
  `n` of the node's metaclass, typed and specialized as declared, on the Turtle and API JSON
  export and on reflection, as the name-only form is. `n` was written as an `ActionUsage` owning
  an unnamed node (`n___400`) of that metaclass, and reflection reported two elements. A graph
  states the declaration, so the notation comes back from it without `sysx:sourceText`.
