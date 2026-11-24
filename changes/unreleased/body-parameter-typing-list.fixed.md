- A body parameter's typing clause now lists its types (`{ in a : A, B; a }`) as any other
  declaration's does; the parser read one type after the colon and reported the comma, so the
  list could only be written `defined by A, B`, and a graph stating two typings for a body
  parameter was refused when read back as notation.
  The list's bound is read after its last type (`in a : A, B[1]`), and a body parameter typed by
  a conjugate (`in a : ~P`) parses after the colon and is exported as a `ConjugatedPortTyping`,
  as a usage typed so is; it was written as an ordinary typing by `P`.
