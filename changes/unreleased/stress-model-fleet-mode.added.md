- **The satellite-network stress model can be generated as a fleet.** `tools/cmd/stress-model -fleet`
  declares each orbital plane as occurrences of one of four spacecraft blocks — `part sats :
  BlockA[400] ordered` — with the as-built values as the block's defaults and stated only on the
  units that diverge, instead of one `part def` per satellite; the spacecraft, ground segment,
  requirements and state machine are unchanged, and the ring, inter-plane and downlink connectors
  are declared once over each collection with `[1]` ends rather than once per satellite pair. `-stats`
  now reports the spacecraft definitions, the units carrying values of their own and the satisfy
  assertions in both forms, so the two can be compared: at 12 800 satellites the fleet declares 12 467 elements against
  2 354 827, and validates in 0.70 s and 184 MB rather than 331 s and 20.3 GB. A guide chapter,
  `docs/guide/modeling-fleets.md`, shows the constellation both ways and what the
  runtime does with 12 800 occurrences, and the stress-test record and performance notes carry the
  measurements. `BenchmarkFleetInstantiate` and `BenchmarkFleetSatisfy` in `tests/stressmodel`
  time the runtime over the fleet form.
