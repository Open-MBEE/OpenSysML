- Development moved to a `develop` integration branch; `main` now carries releases only.
  Feature and fix pull requests target `develop`, releases reach `main` through
  `release/x.y.z` pull requests and are tagged there, and CircleCI builds and tests both
  branches. `make proto-breaking` compares against `origin/develop` by default.
