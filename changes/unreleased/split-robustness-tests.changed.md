- The runtime robustness suite is one `robustness_<feature>_test.go` file per feature: the
  cases of the former single `TestRuntimeRobustness` registry in `robustness_test.go` now run
  under `TestRuntimeRobustness<Feature>` functions beside the newer per-feature files, and
  `robustness_test.go` keeps only the shared helpers and fixtures. The documentation pointers
  follow the cases to their files.
