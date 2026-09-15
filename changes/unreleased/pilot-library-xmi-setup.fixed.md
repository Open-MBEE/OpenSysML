- **The pilot corpus downloader refuses to report success over an empty corpus.**
  `pilot_fetch_subtrees` in `scripts/pilot-pin.sh` now fails, installing nothing, when a subtree
  of the pinned release holds no file of the kinds asked for, and re-fetches a destination that
  is stamped at the current pin but holds no such file instead of reporting it present; before,
  either left a stamped, empty directory that a required corpus gate would then fail on with no
  hint of why. `scripts/pilot-pin-test.sh` checks the downloader against a throwaway release
  repository and runs in CI before any corpus is fetched. The contributor docs now list all
  three download scripts beside the `OPENSYSML_REQUIRE_*` variables that make their gates
  mandatory.
