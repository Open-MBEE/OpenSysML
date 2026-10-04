# Telescope Mass Report

Mass rollup for the telescope assembly. Masses are in kg \| not \#grams, \*not\* \_lbs\_, \`raw\`, \<b>\&plain\</b>.

Reading order: *em\*ph\*asis* **bold\_move** `` mass >= `limit` `` [the \[spec\]](<https://example.com/spec(v2).md>) [Subsystem Masses \| by \*name\*](#breakdown) [the zone groups](#zones)

<a id="zones"></a>

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

*All subsystems by mass*

| name | mass |
| --- | --- |
| baffle\|shroud \*tricky\* | 1.5 |
| mount | 15 |
| optics | 8.5 |
| segmentControl | 20 |

*Mass margins (allocated - estimated)*

| name | label | margin |
| --- | --- | --- |
| baffle\|shroud \*tricky\* | subsystem: baffle\|shroud \*tricky\* | -1.5 |
| mount | subsystem: mount | 0 |
| optics | subsystem: optics | 1.5 |
| segmentControl | subsystem: segmentControl | 5.5 |

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

*Imaging chain interconnection*

```d2
# Observatory::interconnectView — interconnection rendering (render asInterconnectionDiagram)
classes: {
  usage: { style: { fill: white; stroke: "#181818"; stroke-width: 1; font-color: black; font-size: 14; border-radius: 8 } }
  pin: { style: { fill: white; stroke: "#181818"; stroke-width: 1; font-color: black; font-size: 10 } }
  connection: { style: { stroke: "#181818"; font-size: 13; font-color: black; stroke-width: 3 } }
}
n0: "«part»\nimagingChain" {
  class: usage
  n1: "«part»\ncamera : Camera" {
    class: usage
    "n1.0": "output" { class: pin }
  }
  n2: "«part»\nrecorder : Recorder" {
    class: usage
    "n2.0": "input" { class: pin }
  }
}
n0.n1."n1.0" -- n0.n2."n2.0": "link" { class: connection }
```

*Observatory states, left to right*

```d2
# state rendering (the diagram states kind "state")
direction: right
classes: {
  usage: { style: { fill: white; stroke: "#181818"; stroke-width: 1; font-color: black; font-size: 14; border-radius: 8 } }
  initial: { shape: oval; width: 18; height: 18; style: { fill: black; stroke: black } }
  edge: { style: { stroke: "#181818"; font-size: 13; font-color: black; stroke-width: 1 } }
}
n0: "«state»\noperatingStates : ObservatoryStates" {
  class: usage
  n3: "" { class: initial }
  n1: "«state»\nidle\ninitial" { class: usage }
  n2: "«state»\nobserving" { class: usage }
}
n0.n3 -> n0.n1: { class: edge }
n0.n1 -> n0.n2: { class: edge }
n0.n2 -> n0.n1: { class: edge }
```

## Declared Types

The declared type of the telescope, by relationship traversal.

*Type of telescope*

| element |
| --- |
| Assembly \*frame\* |

- Assembly \*frame\*
