- **`sysml-diagram.el` no longer loads lsp-mode.** It sends its requests through the client
  managing the model buffer and reads their answers as plists, so `(require 'sysml-diagram)`
  works without lsp-mode installed; load `sysml-lsp` beside it, as the README's setup does.
