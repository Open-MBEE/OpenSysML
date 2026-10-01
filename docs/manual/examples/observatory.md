# Telescope Mass Report

Mass rollup and requirement status for the telescope assembly.

This report is *generated* from the model by `sysml -render-document` [(OpenSysML)](<https://opensysml.org/>) — masses are tabulated in [Subsystem Masses](#breakdown)

<a id="breakdown"></a>

## Subsystem Masses

*All subsystems by mass*

| name | mass |
| --- | --- |
| mount | 15 |
| optics | 8.5 |
| segmentControl | 20 |

*Subsystems grouped by zone*

**zone: support**

| zone | name | mass |
| --- | --- | --- |
| support | mount | 15 |

**zone: payload**

| zone | name | mass |
| --- | --- | --- |
| payload | optics | 8.5 |
| payload | segmentControl | 20 |

### Heavy Subsystems

Subsystems at or above 10 kg:

1. mount
2. segmentControl

## Mass Requirement

*Parts satisfying the mass requirement*

| name | qualifiedName |
| --- | --- |
| telescope | Observatory::telescope |

*Verifications of the mass requirement*

| qualifiedName |
| --- |
| Observatory::massVerification |

## Diagrams

*Imaging chain interconnection*

```mermaid
---
config:
  themeCSS: ".cluster-label .nodeLabel { text-align: center; }"
  flowchart:
    subGraphTitleMargin:
      bottom: 24
---
%% Observatory::interconnectView — interconnection rendering (render asInterconnectionDiagram)
flowchart LR
  subgraph n0 ["«part»<br>imagingChain"]
    direction LR
    subgraph n1 ["«part»<br>camera : Camera"]
      direction LR
      n1.0["output"]
    end
    subgraph n2 ["«part»<br>recorder : Recorder"]
      direction LR
      n2.0["input"]
    end
  end
  n1.0 ---|"link"| n2.0
```

*Telescope part tree, left to right*

```mermaid
%% tree rendering (the diagram states kind "tree")
flowchart LR
  n0["«part»<br>telescope"]
  n1["«part»<br>optics : Subsystem"]
  n2["«attribute»<br>mass"]
  n1 --- n2
  n3["«attribute»<br>zone"]
  n1 --- n3
  n0 --- n1
  n4["«part»<br>segmentControl : Subsystem"]
  n5["«attribute»<br>mass"]
  n4 --- n5
  n6["«attribute»<br>zone"]
  n4 --- n6
  n0 --- n4
  n7["«part»<br>mount : Subsystem"]
  n8["«attribute»<br>mass"]
  n7 --- n8
  n9["«attribute»<br>zone"]
  n7 --- n9
  n0 --- n7
```
