# Lamp Report

Where the lamps stand and what they did.

*Active states of every lamp*

| path | machine | statePath | region | enclosing |
| --- | --- | --- | --- | --- |
| lamp1 | lp | on.dim | light | on |
| lamp1 | lp | on.fast | fan | on |
| lamp2 | lp | off |  |  |

*Lamps that are on*

| qualifiedName |
| --- |
| lamp1 |

*What lamp1 did from 1 s up to 2.5 s*

| time | kind | event | from | to | target | payload |
| --- | --- | --- | --- | --- | --- | --- |
| 1 \[s\] | accept | Dim |  |  |  | level = 3 |
| 1 \[s\] | transition | accept Dim | run | dim |  |  |
| 2 \[s\] | accept | Boost |  |  |  |  |
| 2 \[s\] | send | Report |  |  | panel |  |
| 2 \[s\] | transition | accept Boost | slow | fast |  |  |

- t=0 lamp1.lp: enter: on
- t=2 lamp2.lp: enter: on

- lamp1.lp in on.dim
- lamp1.lp in on.fast
- lamp2.lp in off
