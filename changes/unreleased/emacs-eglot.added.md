- **The Emacs package works with Eglot as well as lsp-mode.** `sysml-eglot.el` registers
  `sysml-lsp` with Eglot 1.20 or later and handles `opensysml/renderChanged` and
  `opensysml/stdlibContent`, so `M-x sysml-diagram-show` draws, redraws and visits through
  either client, and `(require 'sysml-diagram)` no longer needs lsp-mode installed. The
  `emacs-package` CI job runs the suites, the live ones included, once per client. Code lenses
  and `sysml.strictConformance` stay lsp-mode's only.
