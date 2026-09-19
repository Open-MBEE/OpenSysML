- **The counting gates and the remaining unreleased programs live in the tools module.** The
  validation-constraint census (`cmd/validation-census`), the grammar-coverage harness
  (`cmd/grammar-coverage`) and the documentation-figure gate (`cmd/doc-counts` with
  `internal/doccounts`) are `tools/census/{validation,grammar,doccounts}`, the conformance runner
  (`cmd/conformance`) and the stress-model writer (`cmd/stress-model`) are `tools/cmd/conformance`
  and `tools/cmd/stress-model`, and the baseline provenance and JUnit writers they share
  (`internal/baseline`, `internal/junit`) are `tools/oracle/baseline` and `tools/oracle/junit`;
  every one runs with `go run -C tools ./cmd/<name>`. The analysis-library census schema that the
  runtime's own test writes is `tests/fixtures`, which the doc-counts gate reads from there;
  the model tests load the stress model from `tests/stressmodel`. No figure moved.
