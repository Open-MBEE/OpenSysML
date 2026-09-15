- **The satellite-network stress model can be generated as a fleet.** `cmd/stress-model -fleet`
  declares each orbital plane as occurrences of one of four spacecraft blocks — `part sats :
  BlockA[400] ordered` — with the as-built values as the block's defaults and stated only on the
  units that diverge, instead of one `part def` per satellite; the spacecraft, ground segment,
  requirements and state machine are unchanged, and the ring, inter-plane and downlink connectors
  are declared once over each collection with `[1]` ends rather than once per satellite pair. `-stats`
  now reports the spacecraft definitions and the units carrying values of their own in both forms,
  so the two can be compared: at 12 800 satellites the fleet declares 12 467 elements against
  2 354 827, and validates in 0.57 s and 175 MB rather than 301 s and 20.1 GB. A guide chapter,
  `docs/guide/modeling-fleets.md`, shows the constellation both ways and what the
  runtime does with 12 800 occurrences, and the stress-test record and performance notes carry the
  measurements. `BenchmarkFleetInstantiate` and `BenchmarkFleetSatisfy` in `internal/stressmodel`
  time the runtime over the fleet form.
