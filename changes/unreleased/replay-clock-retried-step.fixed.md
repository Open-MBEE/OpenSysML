- **A witness `explore` writes for a step the clock retries replays under `replay:`.** When a
  step leaves every remaining token waiting on the clock, the executor advances it and retries
  the step, and an exploring run draws its token order at the retry, where the tokens are due
  together. Replay consumed the order's move on the first pass, found the token not yet due
  unable to act and refused the exploration's own witness (`step 3: 3@direct is not able to
  act (able to act: 2@performed)`). A token-order move is now kept for the retry while at most
  one token is able to act and every alternative it names is a token present in the step,
  parked or held; the one able to act moves, and the move is taken at the retry. A move naming
  a token absent from the step is refused at once as before, and one kept for a retry that
  never comes — the step ending in progress, a message wait or a deadlock — is refused with the
  same message. The `-schedule replay:` flag, `%replay` and the checker's replay of a witness
  share the fix; what `explore` records, the witness format and the step numbering are unchanged.
