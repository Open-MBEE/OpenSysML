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

```plantuml
@startuml
' Observatory::interconnectView — interconnection rendering (render asInterconnectionDiagram)
<style>
root {
  BackGroundColor white
  FontName SansSerif
  FontSize 14
  FontColor black
  LineColor #181818
  HorizontalAlignment left
}
element {
  BackGroundColor white
  LineColor #181818
  LineThickness 0.5
  RoundCorner 0
  Shadowing 0.0
}
start, end, activityBar {
  BackGroundColor black
}
arrow {
  LineColor #181818
  LineThickness 1
  FontSize 13
}
note {
  BackGroundColor #FEFFDD
  FontSize 13
}
.usage {
  RoundCorner 20
}
.package {
  LineThickness 1.5
}
.region {
  LineStyle 4
}
</style>
skinparam wrapWidth 300
hide stereotype
rectangle "**Observatory::imagingChain**\n<size:10>//«part»//</size>" as n0 <<part>> <<usage>> {
  rectangle "**camera : Camera**\n<size:10>//«part»//</size>" as n1 <<part>> <<usage>>
  rectangle "**recorder : Recorder**\n<size:10>//«part»//</size>" as n2 <<part>> <<usage>>
}
n1 -[thickness=3]- n2 : link
@enduml
```

*Observatory states, left to right*

```plantuml
@startuml
' state rendering (the diagram states kind "state")
<style>
root {
  BackGroundColor white
  FontName SansSerif
  FontSize 14
  FontColor black
  LineColor #181818
  HorizontalAlignment left
}
element {
  BackGroundColor white
  LineColor #181818
  LineThickness 0.5
  RoundCorner 0
  Shadowing 0.0
}
start, end, activityBar {
  BackGroundColor black
}
arrow {
  LineColor #181818
  LineThickness 1
  FontSize 13
}
note {
  BackGroundColor #FEFFDD
  FontSize 13
}
.usage {
  RoundCorner 20
}
.package {
  LineThickness 1.5
}
.region {
  LineStyle 4
}
</style>
skinparam wrapWidth 300
hide stereotype
left to right direction
hide empty description
state "**Observatory::operatingStates : ObservatoryStates**\n<size:10>//«state»//</size>" as n0 <<state>> <<usage>> {
  state "**idle**\n<size:10>//«state»//</size>\ninitial" as n1 <<state>> <<usage>>
  state "**observing**\n<size:10>//«state»//</size>" as n2 <<state>> <<usage>>
  [*] --> n1
}
n1 --> n2
n2 --> n1
@enduml
```

## Declared Types

The declared type of the telescope, by relationship traversal.

*Type of telescope*

| element |
| --- |
| Observatory::Assembly \*frame\* |

- Assembly \*frame\*
