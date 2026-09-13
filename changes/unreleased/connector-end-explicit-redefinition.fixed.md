- **A connector end that redefines an end by name also redefines the end at its own position.**
  `connection def Link :> BinaryConnection { end source : Foo :>> BinaryLinkObject::source; end
  target : Foo :>> BinaryLinkObject::target; }` has two ends again, not four: an explicit `:>>` on
  an end adds to the positional redefinition of each general connector's end (KerML 7.4.6,
  SysML v2 7.13.2) instead of suppressing it, as it does in the pilot implementation. Such a
  declaration, and the usages typed by it, are no longer reported as specializing a binary link
  with more than two ends; a genuine third end still is.
