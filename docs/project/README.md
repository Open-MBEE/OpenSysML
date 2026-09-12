# Project

The current state of the project: what is implemented, what is known to be missing, and
how a release is produced.

The conformance records below are engineering records rather than user documentation: they state
what each oracle measures, which divergences are deliberate, and which rows are still open. Where
the divergence census still keys its rows by a short row number, that number is a cross-reference
within these records and means nothing outside this repository.

- **[Spec compliance](spec-compliance.md)** — faithful, approximate, or not implemented,
  rule by rule
- **[Training examples](training-examples.md)** — the OMG corpus, adjudicated per file
- **[Pilot corpora gate](pilot-corpora.md)** — OpenSysML diagnostics on the three pinned OMG pilot
  corpora, ratcheted in CI
- **[RDF corpus round-trip gate](rdf-corpus-roundtrip.md)** — the notation → Turtle → notation →
  Turtle verdict of every model under `examples/`, ratcheted per file in CI
- **[Pilot differential](pilot-differential.md)** — OpenSysML diagnostics compared against the OMG
  pilot implementation, advisory
- **[Pilot execution referee](pilot-execution-referee.md)** — how far the pinned pilot's
  execution surface reaches, and which behavior rows it can adjudicate
- **[Behavior semantic oracle](behavior-semantic-oracle.md)** — action and state-machine
  conformance cases whose expected outcomes are derived by hand from the Kernel Semantic and
  Systems Library text rather than recorded from the executor, and the three the executor fails
- **[Pilot Xpect expectations](pilot-xpect.md)** — the pilot's own inline `.xt` assertions as a
  declared-intent oracle, advisory
- **[Grammar coverage](grammar-coverage.md)** — which OMG grammar productions the project's inputs
  exercise, judged by whether an input for each exists, advisory
- **[Validation-constraint census](validation-constraints.md)** — which of the pilot's named
  validation constraints OpenSysML reports, each mapped to the pass and message that reports it,
  with a violating model as evidence; the figures and the name list are gated in CI
- **[Adjudications](adjudications.md)** — the divergences from the pinned pilot implementation we
  keep, the rows still open against it, and the reading behind each one
- **[Element-scoped tier gating](element-scoped-tier-gating.md)** — how a pass opts out of
  document-wide skipping and gates itself per subject, and what that measured
- **[Declared errata overlay](errata-overlay.md)** — how a defect in the published reference
  material is declared, cited and applied to what the oracles read a second time
- **[Lossless library records](lossless-library-records.md)** — the record format
  and version-compatibility surface roadmap L3 needs, the measurements that chose it, and the rows
  it leaves open
- **[Exact-rational evaluation](exact-rational-evaluation.md)** — whether the evaluator should
  compute `Real`/`Rational` arithmetic exactly rather than in binary64, adjudicated against the
  pinned pilot and the specification text, and declined
- **[HTML document backend](html-document-backend.md)** — the design for rendering documents as
  semantic, styleable HTML straight from the document IR
- **[OOSEM library](oosem-library.md)** — the object-oriented systems engineering method as a
  bundled OpenSysML library: what it defines, what it reuses from the standard domain libraries,
  and the prior art it was checked against
- **[MOSA library](mosa-library.md)** — the Modular Open Systems Approach as a bundled OpenSysML
  library: the statute's vocabulary, openness as metadata over the standard model, the
  interface control document and the warning-only checks
- **[Roadmap](roadmap.md)** — the known gaps, in the order they should be picked up
- **[Releasing](releasing.md)** — the pre-tag gate, tagging, artifacts, Homebrew
- **[macOS distribution](macos-distribution.md)** — Gatekeeper and the signing decision
- **[Bugs in the OMG materials](omg-issues.md)** — defects in the vendored specification
  libraries
- **[Demo](demo.md)** — a scripted walkthrough of the whole surface
