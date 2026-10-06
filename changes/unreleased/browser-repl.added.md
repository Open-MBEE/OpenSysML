- **The CLI page runs the real `sysml` REPL in the browser, with guided walkthroughs.** The
  page's toy shell over `sysml-engine` is replaced by `cmd/sysml` itself, built for js/wasm
  with `-tags sysml_prod` (about 12 MB gzipped, downloaded when started), on an in-memory
  filesystem holding the runtime-showcase models. Seven walkthroughs type their commands into
  the prompt: units, the Saturn V mass rollup, the delta-v analysis and sweep, failing
  requirements, stepping an action, a state machine on a clock and a user-written one. Up and
  Down recall earlier entries, kept across visits, and Tab completes through the REPL's own
  completer. `TestBrowserREPLWalkthroughs` runs every walkthrough through the page's host.
- **The landing-page diagram can be edited.** "Edit the model" opens the diagram's SysML
  source; the engine re-parses and re-instantiates it as you type and redraws the parts and
  interfaces, a parse error is shown with its line and column while the last good diagram
  stays up, and "Run the model" executes the edited model.
