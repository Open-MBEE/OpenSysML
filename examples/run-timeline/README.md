# Run timeline and message sequence

This example runs two object state machines on one clock. The sender's parallel
transmission and power regions make a deterministic scheduling choice, then its
timed transition sends `Ping` to its receiver part. Both renderings are built
from the recorded run, not from model views.

From the repository root:

```bash
sysml examples/run-timeline/run-timeline.sysml \
  -instantiate RunTimeline::mission \
  -state "RunTimeline::Sender::modes RunTimeline::mission.sender" \
  -state "RunTimeline::Receiver::modes RunTimeline::mission.sender.receiver" \
  -advance 6 \
  -render-run timeline=timeline.mmd \
  -render-run sequence=sequence.puml
```

Use `.txt`, `.mmd`/`.mermaid`, or `.puml`/`.plantuml` for text, Mermaid, or
PlantUML output. Run renderings do not accept DOT.
