- **The census and referee tools bound every helper program they run.** `-timeout 0` on the
  `api-json` referee and the derived-property census now means no limit instead of a deadline
  that has already passed, and both stop a launcher that floods standard error. The `pdftotext`
  run of the formal census, the launcher's `--jvm` answer and the `git` queries of the oracle
  and the Hideo suite pin now run under a time limit and an output limit, so a helper that
  hangs or never stops writing fails the tool with a named error instead of stalling it.
