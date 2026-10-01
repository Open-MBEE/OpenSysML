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

```mermaid
---
config:
  theme: base
  themeCSS: ".edgeLabel rect { opacity: 1 !important; }"
  themeVariables:
    fontFamily: "Helvetica, Arial, sans-serif"
    fontSize: "14px"
    primaryColor: "#FFFFFF"
    secondaryColor: "#FFFFFF"
    tertiaryColor: "#FFFFFF"
    background: "#FFFFFF"
    primaryBorderColor: "#181818"
    primaryTextColor: "#000000"
    lineColor: "#181818"
    textColor: "#000000"
    noteBkgColor: "#FEFFDD"
    noteBorderColor: "#181818"
    noteTextColor: "#000000"
    clusterBkg: "#FFFFFF"
    clusterBorder: "#181818"
    edgeLabelBackground: "#FFFFFF"
  flowchart:
    subGraphTitleMargin:
      bottom: 24
---
%% Observatory::interconnectView — interconnection rendering (render asInterconnectionDiagram)
flowchart LR
  subgraph n0 ["`**imagingChain**
*«part»*`"]
    direction LR
    n1("`**camera : Camera**
*«part»*`")
    n2("`**recorder : Recorder**
*«part»*`")
  end
  n1 ===|"link"| n2
  linkStyle 0 stroke-width:3px
```

*Observatory states, left to right*

```mermaid
---
config:
  theme: base
  themeVariables:
    fontFamily: "Helvetica, Arial, sans-serif"
    fontSize: "14px"
    primaryColor: "#FFFFFF"
    secondaryColor: "#FFFFFF"
    tertiaryColor: "#FFFFFF"
    background: "#FFFFFF"
    primaryBorderColor: "#181818"
    primaryTextColor: "#000000"
    lineColor: "#181818"
    textColor: "#000000"
    noteBkgColor: "#FEFFDD"
    noteBorderColor: "#181818"
    noteTextColor: "#000000"
    stateBkg: "#FFFFFF"
    stateBorder: "#181818"
    stateLabelColor: "#000000"
    compositeBackground: "#FFFFFF"
    compositeBorder: "#181818"
    compositeTitleBackground: "#FFFFFF"
    compositeTitleBorder: "#181818"
    transitionColor: "#181818"
    transitionLabelColor: "#000000"
    labelBackgroundColor: "#FFFFFF"
    specialStateColor: "#181818"
---
%% state rendering (the diagram states kind "state")
stateDiagram-v2
  direction LR
  state "operatingStates : ObservatoryStates<br>«state»" as n0 {
    state "idle<br>«state»<br>initial" as n1
    state "observing<br>«state»" as n2
    [*] --> n1
  }
  n1 --> n2
  n2 --> n1
```

## Declared Types

The declared type of the telescope, by relationship traversal.

*Type of telescope*

| element |
| --- |
| Assembly \*frame\* |

- Assembly \*frame\*
