- **A call whose required input receives no value is performed.** The SysML v1 migration
  wrote a call passing no argument, or a pin no value reaches, for a parameter with no default
  and a lower bound above zero as a placeholder performing nothing; v1 runs the callee with the
  parameter unset, so the parameter or pin is now declared admitting no value (`[0..upper]`),
  the call is written and performed, the absence propagates through the pins, nested activity
  outputs and method parameters it feeds, and a write of a feature requiring a value from one
  that may be absent is guarded (`if x->SequenceFunctions::notEmpty() { assign … }`). The report
  says on each parameter and pin why a value may fail to reach it. A send of a signal whose
  required attribute gets no value still stands in for itself, since v2 admits no such send. A
  call behavior action that names no behavior yet has pins stays unresolved with its pins and
  the reason; an «Allocate» from the action to a part is named in it as saying where the action
  runs, not what it does, and no behavior or value is made up for it.
- **«Probability» is read by provenance.** The SysML v1 migration weights a decision's
  branches only by the OMG SysML profile's «Probability», recognised by the namespace its
  application is serialised under as every standard stereotype is; a same-named stereotype from
  another profile weights nothing, and the report says which profile it comes from.
- **A run configuration is refused by name.** `-compare-results` refuses a configuration whose
  behavior was not migrated, or whose `durationSimulationMode` is no draw policy, or whose run
  fails, naming the configuration in the refusal, so the refusals of several configurations
  printed together tell which is which.
