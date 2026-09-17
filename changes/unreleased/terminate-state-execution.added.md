- **`terminate` ends an occurrence, a state's behavior or the state machine.**
  `terminate <occurrence>;` evaluates its target — `this`, a part's feature chain
  (`terminate vehicle.engine;`), a nested action node's own occurrence — and ends that
  occurrence's lifetime: its owned parts, the behaviors it exhibits or performs, and any action
  or state performance running on it stop where they are, keeping their values; a name that
  denotes no occurrence, one already ended, and one `destroy` emptied are each a typed error.
  A `terminate;` in a state's `entry`, `do` or `exit` body ends that behavior at the statement,
  the state stays active and the machine keeps dispatching. A transition whose target is a
  terminate action (`transition first idle accept Abort then stop; action stop terminate;`) ends
  the state machine's performance as SysML v2 §7.18.3 and the PSSM's terminate pseudostate
  both prescribe: the source exits and the transition's effect run, then no further state is
  exited, running do behaviors are abandoned and no state remains active — reached directly, or
  through a choice, junction or join, from inside a composite state or one region of an
  orthogonal one. Such a run reports `Outcome.Terminated` with no final state, the REPL says
  `State machine terminated` and `Execution state: Terminated`, the LSP debug snapshot's `state`
  is `terminated`, `%instances` lists a terminated object as `ended`, and `explore`/`check`
  count the terminated run as one outcome. The PSSM referee translates the suite's terminate
  pseudostates the same way, so its `terminate-gap` bucket is retired: *Terminate 003* passes,
  and *Terminate 001/002* fail on the order an orthogonal state's regions are entered in, which
  the referee's record already attributes to an open finding.
