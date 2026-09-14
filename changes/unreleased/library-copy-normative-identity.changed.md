- **A copy of a library file rooted at the library's packages converts as the library.** The
  encoder read only the byte-identical bundled file as the standard library, so the notation
  rebuilt from a source-free graph — not the bundled bytes — came back as a user file: its
  normative ids were written as `@IdentityMetadata::ElementId` annotations, and its second
  Turtle gained `sysx:declaredId`, `sysx:hasBody` and derived `_om` owning-membership IRIs;
  a `standard library package` sitting beside the bundled one also resolved a few inherited
  names (`portionOf` in a respaced `Occurrences.kerml`) against the bundled package. A
  document whose roots are the top-level packages of one bundled library file — same
  qualified names and normative ids, or the library's own package modifiers — is now
  analyzed in that file's place on the encoder side as the decoder already did: its elements
  carry normative element and owning-membership ids, no annotation and no `sysx:declaredId`,
  and its names resolve to its own declarations. An annotation restating a normative id
  declares nothing. `succession all a then b` no longer gains `first` when rebuilt. Every
  bundled library file now converts `notation → Turtle → strip source text → notation →
  Turtle` with the second Turtle equal to the first, source predicates aside, from the first
  hop. A user `package Actions { … }` keeps its encoded ids, an element carrying a library
  UUID under another name keeps it as declared, and a workspace copy of a library file in
  the editor is still the user's file.
