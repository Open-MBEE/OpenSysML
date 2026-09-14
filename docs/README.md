# OpenSysML documentation

## Using OpenSysML

**[The guide](guide/)** — a handbook, in reading order: [install](guide/01-install.md),
[your first model](guide/02-first-model.md), [the command line](guide/03-command-line.md),
[the REPL](guide/04-repl.md), [checks](guide/05-checking.md), [behavior](guide/06-behavior.md),
[saving and RDF](guide/07-saving-and-rdf.md), [editors](guide/08-editors.md),
[from your own program](guide/09-clients.md), [troubleshooting](guide/10-troubleshooting.md).

Runnable models live in [examples/](../examples/), with a catalog describing what
each one demonstrates.

**[Document generation manual](manual/README.md)** — how to generate documents from your models using
the built-in document queries: [concepts](manual/introduction.md), [getting started](manual/getting-started.md),
a [query cookbook](manual/query-cookbook.md), [document authoring](manual/authoring.md),
[outputs](manual/outputs.md) (Markdown, HTML and PDF), [interfaces](manual/interfaces.md),
a [complete worked example](manual/worked-example.md) and
[limitations and troubleshooting](manual/troubleshooting.md).

## Reference

- **[CLI](reference/cli.md)** — every `sysml` flag, the modes, and the exit codes
- **[REPL commands](reference/repl-commands.md)** — every `%` command and its arguments
- **[LSP extensions](reference/lsp.md)** — the custom render requests `sysml-lsp` offers diagram clients
- **[Environment variables](reference/environment.md)** — resource limits for a single run, and file paths
- **[Client libraries](reference/clients.md)** — the Go, Python, Node, Java and Rust libraries, and how to choose between them
- **[Go packages](reference/api.md)** — `client/opensysml` and the packages behind it, type by type
- **[Python API](reference/python-api.md)** — the `opensysml` package, its generated typed classes, and what calls cost
- **[Service transports](reference/service-transports.md)** — what `sysml-grpc` serves on its single port, and what happens when a capability is missing
- **[RDF mapping](reference/rdf-mapping.md)** — the triples a model becomes, which constructs are not mapped, and why the mapping is experimental
- **[Grammar](reference/grammar/README.md)** — how grammar productions map to the parser

## Internals

- **[Architecture](internals/architecture.md)** — the pipeline, the validation tiers, and the test contracts
- **[Testing](internals/testing.md)** — the tests each kind of change must satisfy
- **[Performance](internals/performance.md)** — profiling, and what a large model costs
- **[Design notes](internals/design/)** and **[plans](internals/notes/)** — maintainer material

## Project status

- **[Spec compliance](project/spec-compliance.md)** — each rule marked faithful, approximate or not implemented
- **[Training examples](project/training-examples.md)** — the OMG corpus, 100/100 clean
- **[Pilot corpora gate](project/pilot-corpora.md)** — OpenSysML's diagnostics on the three pinned OMG pilot corpora, with CI preventing regressions
- **[Pilot differential](project/pilot-differential.md)** — OpenSysML's diagnostics compared against the OMG pilot implementation's
- **[Grammar coverage](project/grammar-coverage.md)** — which OMG grammar productions the project's inputs exercise
- **[Roadmap](project/roadmap.md)** — the known gaps, in the order we plan to address them
- **[Nightly snapshots](project/nightly.md)** — `develop` built every night as a prerelease, and how to verify one
- **[Releasing](project/releasing.md)** — the pre-tag gate, tagging, artifacts and Homebrew
- **[macOS distribution](project/macos-distribution.md)** — Gatekeeper and the signing decision
- **[Demo](project/demo.md)** — a scripted walkthrough of the full surface

For contribution guidance, including where a new page belongs, see
[CONTRIBUTING.md](../CONTRIBUTING.md).
