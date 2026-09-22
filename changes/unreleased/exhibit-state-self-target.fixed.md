- **An `exhibit state` usage naming nothing exhibits itself.** Per SysML v2
  §8.3.17 (`ExhibitStateUsage::exhibitedState` redefines `performedAction` — the
  reference feature of the owned reference subsetting, or the usage itself when
  there is none), `exhibit state modes { in cmd = port.cmd; … }` and
  `exhibit state idle;` are state usages with their own (possibly empty) body,
  not references to one held elsewhere; `exhibit state modes;` no longer fails
  instantiation with `classifier behavior names no body`, and a body declaring
  no initial state fails with `ErrNoInitialState` at initialization like any
  machine stating an empty body. An `exhibit` declaration that does name an
  element — the `exhibit m;` reference form, a `references`/`::>` clause, or a
  typing — that resolves to no behavior body is still reported.
