- **A succession outside a behavior body now orders the performances it relates.** Examples
  are `first b.g then b.m;` in a package, `first grip then move;` in a part def, and
  `first r::move then r::grip;`, which orders every `Robot`.
  - The succession is featured as KerML §8.3.4.5.3 derives: by its owning type, or by the
    innermost common featuring type of its ends.
  - A behavior the runtime starts when an object is materialized is held until its predecessor
    has ended. Unrelated performances keep every interleaving, and snapshots, held images and
    checked state keys carry what a held behavior waits for.
  - An explicit `perform x.beh.start` or `-action` start that would break the order fails with
    `succession-order-violated`, and a cycle fails with `succession-order-cycle`; the start is
    never reordered.
  - A succession with a behavior end that orders nothing in a run is reported with
    `succession-orders-nothing` and the reason. Examples are a requirement end, or a missing or
    repeated performance, which is open under KERML-29.
  - Ends with no common featuring type are rejected with `connector-type-featuring`.
