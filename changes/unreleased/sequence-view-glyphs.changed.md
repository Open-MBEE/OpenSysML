Sequence renderings draw the SysML v2.1 §8.2.3.9 glyphs: a `message` is its own edge kind, solid
with an open head (PlantUML `->>`, Mermaid `-)`), labelled `name of payload`; the successions
between events are dashed open-headed rows of the sequence (`-->>`, `--)`), labelled with their
name; the events a message or succession ends at are proxies on their lifelines (filled points on
the canvas, spans in D2, `producer.sent` in text), and a port a message ends at is a port on the
lifeline's border. An `event occurrence` is headed «event occurrence» rather than «occurrence»,
and `individual` usages fold into the `individuals` compartment with their keyword kept on the
row. The rendered output of every sequence view changes accordingly.
