# The OpenSysML guide

Read the chapters in order the first time through; each one builds on the ones before it.

1. [Install](01-install.md) — a release build, Homebrew, or from source
2. [Your first model](02-first-model.md) — declare, instantiate, evaluate
3. [From the command line](03-command-line.md) — checking a file, exit status, scripting
4. [The REPL](04-repl.md) — the session model and the meta-commands
5. [Expressions, calculations, constraints and requirements](05-checking.md) — what a model asserts, and how to check it
6. [Behavior: actions and state machines](06-behavior.md) — running and debugging behavior
7. [Saving, and converting to RDF](07-saving-and-rdf.md) — `%save`, `-convert`, round trips
8. [Editors](08-editors.md) — `sysml-lsp` and the VS Code extension
9. [From your own program](09-clients.md) — the Go, Python, Node, Java and Rust clients
10. [Troubleshooting](10-troubleshooting.md) — diagnosing a run that stops early
11. [Migrating a SysML v1 model](11-migrating-from-sysml-v1.md) — `-convert` from XMI, reading the report, finishing by hand

Chapter 9 does one task in all five clients side by side — Go, Python, Node/TypeScript, Java and
Rust, in tabs — and then has a section per client for what only that one has.
[Client libraries](../reference/clients.md) explains which to choose and what each covers.

If you want to look up a specific detail rather than read through, the [reference](../reference/)
covers the CLI flags, the REPL commands, the environment variables, the service API, the client
libraries, each client's API and the RDF mapping.
