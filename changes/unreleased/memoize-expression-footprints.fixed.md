- The execution runtime computes what an expression reads — the footprint a `while` or `until`
  condition, an `if` guard, a transition guard, a `for` collection or a `send` message stands
  on for order dependence — once per model instead of on every pass: a loop's every iteration
  rebuilt its condition's footprint, two allocations per pass that the 0.11.0 performance
  record found on every loop row (`IntegerArithmeticLoop` 9 072 → 11 073 allocations per run,
  280 → 483 KiB). A pass of the loop is back to the cost of its body.
