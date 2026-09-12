- **A design note for an embedded target at NASA's highest software class**
  (`docs/internals/design/embedded-target.md`). It proposes how a state machine, an action and
  the calcs they reach compile to C that runs under an RTOS in a form Class A software assurance
  can accept: a closed, serializable behavior IR whose written semantics, not the interpreter,
  is the requirement basis; a freestanding ISO C profile over static tables — no allocation, no
  recursion, a static bound on every loop, no compiler extensions or non-local exits, one
  decision per branch so structural coverage is measurable — with a small prelude verified once;
  a static refusal of every model with an admissible scheduling choice or an unbounded resource;
  the IR, trace map, resource report and budget file as reproducible configuration items; a
  fixed-step host interface designed with the planned C ABI and proved under Zephyr on QEMU and
  as an F´ component; and the evidence a tool qualification argument consumes. Nothing is
  implemented; the note exists to be reviewed before code is written.
