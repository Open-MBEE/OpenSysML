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

```dot
// view: Observatory::interconnectView
// kind: interconnection
// stated: render asInterconnectionDiagram
// layout: dot
digraph "Observatory::interconnectView" {
  graph [fontname="Helvetica"];
  node [shape=box, style=filled, fillcolor=white, color="#181818", fontname="Helvetica", fontsize=14, penwidth=0.5];
  edge [color="#181818", fontname="Helvetica", fontsize=13, penwidth=1];
  subgraph "cluster_n0" {
    label=<<b>Observatory::imagingChain</b><br/><font point-size="10"><i>«part»</i></font>>;
    color=black;
    penwidth=0.5;
    "n0" [shape=point, style=invis, width=0, height=0, label=""];
    "n1" [style="rounded,filled", label=<<b>camera : Camera</b><br/><font point-size="10"><i>«part»</i></font>>];
    "n2" [style="rounded,filled", label=<<b>recorder : Recorder</b><br/><font point-size="10"><i>«part»</i></font>>];
  }
  "n1" -> "n2" [label="link", arrowhead=none, penwidth=3];
}
```

*Observatory states, left to right*

```dot
// kind: state
// stated: the diagram states kind "state"
// layout: dot
digraph {
  graph [fontname="Helvetica", rankdir=LR];
  node [shape=box, style=filled, fillcolor=white, color="#181818", fontname="Helvetica", fontsize=14, penwidth=0.5];
  edge [color="#181818", fontname="Helvetica", fontsize=13, penwidth=1];
  subgraph "cluster_n0" {
    label=<<b>Observatory::operatingStates : ObservatoryStates</b><br/><font point-size="10"><i>«state»</i></font>>;
    color=black;
    penwidth=0.5;
    "n0" [shape=point, style=invis, width=0, height=0, label=""];
    "n3" [shape=point, fillcolor=black, label=""];
    "n1" [style="rounded,filled", label=<<b>idle</b><br/><font point-size="10"><i>«state»</i></font><br/>initial>];
    "n2" [style="rounded,filled", label=<<b>observing</b><br/><font point-size="10"><i>«state»</i></font>>];
  }
  "n3" -> "n1";
  "n1" -> "n2";
  "n2" -> "n1";
}
```

## Declared Types

The declared type of the telescope, by relationship traversal.

*Type of telescope*

| element |
| --- |
| Observatory::Assembly \*frame\* |

- Assembly \*frame\*
