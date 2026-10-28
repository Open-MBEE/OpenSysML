# APE reference model migration fixture

This directory holds the APE reference model and its captured migration artifacts for future full-model regression work. Small, focused fixtures remain the automated contract for individual defects, including nested-requirement references in use-case views.

## Layout

```text
APE-ReferenceModel.mdzip       Original source archive, unchanged
manifest.json                 Sizes and SHA-256 checksums at relocation
generated/
  APE-ReferenceModel.sysml
  APE-ReferenceModel.migration-report.json
  APE-ReferenceModel.migration-results.json
  APE-ReferenceModel.validation.json
  APE-ReferenceModel.migration-summary.md
  images/                     67 extracted images, beside the generated model
  diagnostics/                Raw diagnostic output and incomplete verification logs
```

The source archive and generated artifacts are local and Git-ignored: the archive is about 21 MB and several outputs exceed the repository's 200 KiB fixture limit. The README, ignore rules, and small manifest can be tracked without adding an oversized fixture exception. A fresh checkout needs the matching source archive supplied separately before replaying the migration. There is no download source configured for this model.

The manifest records file identity, not expected migration correctness. It must not be used to accept output changes blindly. The original source SHA-256 identifies the input to use when comparing runs; output hashes identify the historical artifacts already present locally. New runs should be compared semantically and adjudicated before becoming a regression baseline.

## Replay from the repository root

Run the automated regression with the archive present:

```text
OPENSYSML_REQUIRE_APE_REFERENCE_MODEL=1 go test -count=1 ./tests/migrate -run '^TestAPEReferenceModelMigration$' -v
```

In PowerShell, set `$env:OPENSYSML_REQUIRE_APE_REFERENCE_MODEL = '1'` first, then run the `go test` command. The test verifies the archive checksum, migrates it afresh through the conversion entry point and its syntax gate, checks qualified nested-requirement references and report identities, and verifies all extracted images against the inventory. It does not read the captured generated files and does not assert that the model is semantically clean.

Without the require variable, a missing archive produces an explicit skip. With it set to `1`, a missing archive fails. An unreadable or checksum-mismatched archive always fails. This is a local full-model regression until a separate source-provisioning mechanism is configured for CI; ignoring the archive does not prevent Go tests from reading it when supplied.

Build the CLI using the Go toolchain required by the repository, then run:

```text
go build -o bin/sysml ./cmd/sysml
bin/sysml tests/migrate/testdata/ape-reference-model/APE-ReferenceModel.mdzip -migrate sysml -o tests/migrate/testdata/ape-reference-model/generated/APE-ReferenceModel.sysml -migration-report tests/migrate/testdata/ape-reference-model/generated/APE-ReferenceModel.migration-report.json -migration-results tests/migrate/testdata/ape-reference-model/generated/APE-ReferenceModel.migration-results.json
bin/sysml tests/migrate/testdata/ape-reference-model/generated/APE-ReferenceModel.sysml -validate -json -o tests/migrate/testdata/ape-reference-model/generated/APE-ReferenceModel.validation.json
```

On Windows, build and invoke `bin/sysml.exe` instead. Ensure the `generated` directory exists first. The migration writes extracted images into `generated/images`, preserving the model's relative image references. Copy the captured artifacts elsewhere first when retaining the historical comparison is useful.

## Captured result and known limitations

The captured migration produced syntax accepted by the CLI, with 4,991 mapped, 2,349 approximated, 484 unmapped, and 849 skipped elements. It laid out 347 of 360 diagrams and wrote 67 image files. Validation reported 48 unresolved-reference errors and 1,123 name-conflict warnings. These are historical observations, not accepted error counts that a future test should silently ratchet.

The run used the workspace CLI before the work moved onto the develop-based feature branch. Regenerate with the current branch before drawing conclusions about its present behavior. The diagnostic logs are incomplete and do not establish that the full repository verification gate passed. Paths in the captured reports and summary describe their original invocation and have not been rewritten.

## Future regression use

- Supply the source archive matching the manifest, migrate it afresh, and check generated notation through the public CLI conversion path so its syntax gate remains active.
- Compare source-element identities, verdicts and reasons, reference resolution, diagram accounting, and attached-file preservation. Do not use aggregate counts alone as the contract.
- Reduce any discovered defect to a small committed XMI fixture with explicit expected behavior and failure-mode coverage.
- Keep the complete archive and bulky generated results outside the tracked fixture set. If this model becomes a provisioned test corpus, add an explicit provisioning and integrity-check mechanism before wiring it into required CI.
- Resolve or adjudicate the existing semantic diagnostics before declaring this model fully validated.
