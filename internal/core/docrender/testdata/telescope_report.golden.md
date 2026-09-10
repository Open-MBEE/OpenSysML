# Telescope Mass Report

Mass rollup for the telescope assembly. Masses are in kg \| not \#grams, \*not\* \_lbs\_, \`raw\`, \<b>\&plain\</b>.

Reading order: *em\*ph\*asis* **bold\_move** `` mass >= `limit` `` [the \[spec\]](<https://example.com/spec(v2).md>) [Subsystem Masses \| by \*name\*](#breakdown) [the zone groups](#zones)

<a id="zones"></a>

<!-- caption -->
*Subsystems grouped by zone*

**zone: support \| \*frame\***

| zone | name | mass |
| --- | --- | --- |
| support \| \*frame\* | baffle\|shroud \*tricky\* | 1.5 |
| support \| \*frame\* | mount | 15 |

**zone: payload**

| zone | name | mass |
| --- | --- | --- |
| payload | optics | 8.5 |
| payload | segmentControl | 20 |

<a id="breakdown"></a>

## Subsystem Masses \| by \*name\*

<!-- caption -->
*All subsystems by mass*

| name | mass |
| --- | --- |
| baffle\|shroud \*tricky\* | 1.5 |
| mount | 15 |
| optics | 8.5 |
| segmentControl | 20 |

<!-- caption -->
*Mass margins (allocated - estimated)*

| name | label | margin |
| --- | --- | --- |
| baffle\|shroud \*tricky\* | subsystem: baffle\|shroud \*tricky\* | -1.5 |
| mount | subsystem: mount | 0 |
| optics | subsystem: optics | 1.5 |
| segmentControl | subsystem: segmentControl | 5.5 |

<!-- caption -->
*Subsystem notes*

| shortName | name | documentation |
| --- | --- | --- |
|  | baffle\|shroud \*tricky\* |  |
| M3\|\* | mount |  |
| M1 | optics | The primary mirror assembly., Collects light \| not \*heat\*<br>from the target. |
|  | segmentControl | Actuators that phase the mirror segments. |

**M3\|\***

**M1** — The primary mirror assembly. Collects light \| not \*heat\* from the target.

Actuators that phase the mirror segments.

### Heavy Subsystems

mount segmentControl

1. mount
2. segmentControl

**mount** [mount](<https://example.com/parts#mount>) **segmentControl** [segmentControl](<https://example.com/parts#segmentControl>)

- `mount`
- `segmentControl`

### Missing Subsystems

| name | mass |
| --- | --- |

## Diagrams

<!-- caption -->
*Imaging chain interconnection*

```mermaid
%% Observatory::interconnectView — interconnection rendering (render asInterconnectionDiagram)
flowchart LR
  subgraph n0 ["part Observatory::imagingChain"]
    n1["part camera (Camera)"]
    n2["part recorder (Recorder)"]
  end
  n1 ---|"link"| n2
```

<!-- caption -->
*Observatory states, left to right*

```mermaid
%% state rendering (the diagram states kind "state")
stateDiagram-v2
  direction LR
  state "state Observatory::operatingStates (ObservatoryStates)" as n0 {
    state "state idle (initial)" as n1
    state "state observing" as n2
    [*] --> n1
  }
  n1 --> n2
  n2 --> n1
```

## Declared Types

The declared type of the telescope, by relationship traversal.

<!-- caption -->
*Type of telescope*

| element |
| --- |
| Observatory::Assembly \*frame\* |

- Assembly \*frame\*
