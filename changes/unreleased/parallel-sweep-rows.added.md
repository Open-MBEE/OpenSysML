- **Sweep rows run in parallel under `-jobs`, each in a context of its own, with the same table
  whatever the count.** `-sweep`/`-samples`, `%sweep`/`%samples` and the `RunSweep` RPC run
  their rows `-jobs` (`%jobs`, `OPENSYSML_JOBS`) at a time. Every row is a run of its own: it
  instantiates the case's subject and the `self` of a nested case in a fresh context over the
  plan's worker and carries the arguments' values in — evaluated once at the prompt, so a held
  feature a run wrote reads as written, an argument naming a held object bound to the object
  the row makes for it — so a case that writes a feature of its subject writes its own row's
  object and no row sees another's; the table comes out in range order whatever order the rows
  finish in, so the rows, their outputs, verdicts, evaluations and errors are those of
  `-jobs 1`, only each row's `time` and the report's `workers`/`warming` varying with the
  count; the RPC's response numbers every row's objects apart in its `instances` table. A
  `%sweep` on an object the session holds runs each row on a fresh object of the held object's
  declaration (one reached through a feature of another on the like of its root, walked along
  the same path) and leaves the held object as it was, which stands for it while it is as the
  declaration made it; an object named by `#<id>`, one a run has written a feature of or
  destroyed, or one running a behavior its type exhibits or performs — one fresh from
  `%instantiate` included, for now — is refused naming the reason rather than swept on the
  session's context, as is an argument naming such an object or holding a value bound to the
  run that made it, and the session's readers are no longer held up while the rows run. A
  deadline met mid-sweep starts no further row, discards the rows in flight and reports the
  deadline and no table, as one job does when it meets the deadline between two rows.
