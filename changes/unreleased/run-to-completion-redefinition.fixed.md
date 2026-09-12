- **A state machine redefining `isRunToCompletion` or `runToCompletionScope` away from the
  Kernel Semantic Library default is refused instead of run under the default.** The runtime
  implements only the defaults (`isRunToCompletion = true`, the whole machine as the scope) and
  never read a model's redefinition, so `attribute :>> isRunToCompletion = false` or a
  `runToCompletionScope` narrowed to a substate executed silently as if the default held.
  Lowering now refuses such a machine with the typed `lower.RunToCompletionRedefinition`, an
  `ErrUnsupportedStateContent` naming the feature, the declaring body and the value written —
  on the executed machine, on a state definition it specializes, on a substate and on an
  orthogonal region's substate alike — and refuses a value it cannot read as the default, saying
  the default cannot be verified. A redefinition restating the default (`= true`, `= self` on the
  machine) runs unchanged, and so does a machine restating it over the redefinition it inherits
  from a specialized definition: only the redefinition a body makes effective is judged. The
  target is resolved as a symbol, so an alias of the library feature is refused too. Every
  surface that starts a machine — the REPL's `%state`, an object exhibiting it, the analysis
  engines — reports the refusal through the lowering error it already shows, and a state
  rendering reports it as a machine that does not lower. Neither a non-run-to-completion
  scheduling nor a narrowed scope is implemented.
