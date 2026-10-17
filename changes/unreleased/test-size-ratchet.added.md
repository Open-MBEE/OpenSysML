- The hygiene suite caps test files at 2,000 lines and test fixtures (files under a
  `testdata/` directory or named `*.golden.*`) at 200 KiB. The files already over a cap are
  allowlisted at their current size with a reason, so none of them can grow further and an
  entry is retired as soon as its file shrinks back under the cap.
