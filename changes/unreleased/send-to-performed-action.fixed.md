- **`%send` reaches an object whose performed action is waiting at an `accept`.** The REPL used to
  refuse a signal to any object that exhibited no state machine (`runs no state machine, so
  nothing there accepts a signal`), so the only way to wake a lone object's performed action was a
  sibling's machine sending to it. `%send <signal> to <object>` now asks every behavior the object
  runs — the machines it exhibits and the actions it performs, an accept nested in a performed
  action included — and posts the signal on the runtime's message bus when one of them is parked
  for it, reporting the accept that takes it (`Accepted by performed action "main" waiting at
  accept g`); the action goes on from its accept at the next `%advance`. An object none of whose
  behaviors accepts the signal is refused with each behavior's standing, and one running no
  behavior at all with a hint to start one. With an `%action <name> <object>` session open, a bare
  `%send <signal>` goes to that object, as it goes to the `%state` session's; an `%action` session
  performing on behalf of no object still has none to send to, and says so.
