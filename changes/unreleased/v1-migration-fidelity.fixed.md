- **A case's timed steps run on the clock.** A case body's action flow waited on the clock for
  its own `accept after`, but the clock did not list its waits, so a timed step in an analysis
  or verification case deadlocked; the flow is now on the clock for the run. A case an action
  body performs as a step pauses that body, whose executor lists the case's waits among its own
  and resumes the step when the instant comes, so a wait for a message nothing posts is the
  typed `ErrAcceptDeadlock` of the performing action, and an expression reading a case's output
  while it waits is the typed `ErrCaseReadWaits` naming the wait (`analysis_steps_wait_on_clock`,
  `action_case_step_waits_on_clock`).
- **A state behavior of no content executes as nothing.** A state's entry, do or exit behavior,
  or a transition's effect, written as an action usage with neither a body nor an action
  performed (`entry action hello;`, `do action log`) was refused at run time as performing no
  action; it now executes as nothing, as a bodyless nested action of an action body does
  (`state_behavior_action_of_no_content`).
- **A binding end at a performed action's node reads the body's names.** A `bind` written at a
  node of a `perform action` resolved a simple name to the performing part's feature before the
  enclosing action's same-named parameter, so a parameter given no value read the part's value
  instead of being empty; the name now resolves in the body's scope first, as an expression of
  the body does. A pin valued by its own name (`inout log = log`) reads the feature it masks
  around the usage owning the pin rather than itself, which was refused as a cyclic feature
  value (`performed_action_binding_end_names_parameter`).
