// Package baseline stores a referee's committed baseline as a directory of
// fragments instead of one generated file, so that two branches re-recording
// it conflict only where they record the same source file differently.
//
// A baseline is a per-source-file ratchet: the referee measures every model
// file of its corpus roots and records what it found. Kept in one file, every
// re-recording rewrites the whole file and any two branches that re-record it
// conflict on merge, whatever they changed. Kept as this package lays it out,
// the directory holds
//
//   - one fragment per compared source file, at <dir>/<root>/<relative path>.json,
//     carrying the file's own summary and its rows, one row per line in key
//     order, so that a change to one file's verdicts touches that file's
//     fragment and nothing else;
//   - index.json, carrying the run's provenance (the pinned reference, the
//     bridge sources, every compared input's digest), the list of recorded
//     files per root and the referee's aggregate figures, which are a fold
//     over the fragments and are held to them by Check and Verify.
//
// The index is the one file every re-recording may touch: the provenance moves
// only when an input or the pin moves, and the aggregate moves whenever any
// verdict does. Two branches that both move a verdict therefore still meet in
// index.json, but the conflict is mechanical: the fragments merge on their own
// and Rewrite with no replaced roots re-folds the index from them without the
// reference toolchain. Rewrite vouches only for what it re-measured: of the
// committed provenance, the inputs named like the replaced roots may move and
// nothing else, so a moved pin, tool, errata digest or other input needs a run
// over every root. Inside a replaced root the index and the files need not
// agree — two branches that each added a file of one root merge that way, and
// the root's digest having moved, re-recording that root is the resolution.
//
// Only storage and comparison are shared. A referee keeps its own row type R,
// its per-file summary S and its aggregate A, and its own policy on what a
// movement means: Check names every moved, added or removed row per fragment,
// every fragment missing or left behind and every stale index figure, and
// leaves the adjudication to the referee's record.
//
// To migrate a referee: give each compared file a Fragment[S, R] whose Rows
// are keyed by the row's name, write a fold from the fragments to A, record
// with Write (or Rewrite for a subset of roots), hold a run to the committed
// directory with Check, and hold the committed directory to itself, without
// the toolchain, with Verify.
package baseline
