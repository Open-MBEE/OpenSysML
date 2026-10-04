# Run timeline and message sequence

This example runs two sibling object state machines on one clock. The controller's
parallel communication and power regions make a deterministic scheduling choice.
It sends payload-bearing `Ping` messages to the instrument, which answers with
`Ack` messages over the connected ports. Both renderings are built
from the recorded run, not from model views.

From the repository root:

```bash
sysml examples/run-timeline/run-timeline.sysml \
  -instantiate RunTimeline::mission \
  -state "RunTimeline::Controller::modes RunTimeline::mission.controller" \
  -state "RunTimeline::Instrument::modes RunTimeline::mission.instrument" \
  -advance 6 \
  -render-run timeline=timeline.mmd \
  -render-run sequence=sequence.puml
```

Use `.txt`, `.mmd`/`.mermaid`, or `.puml`/`.plantuml` for text, Mermaid, or
PlantUML output. Run renderings do not accept DOT.
