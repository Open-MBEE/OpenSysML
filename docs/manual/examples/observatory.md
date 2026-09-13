# Telescope Mass Report

Mass rollup and requirement status for the telescope assembly.

This report is *generated* from the model by `sysml -render-document` [(OpenSysML)](<https://opensysml.org/>) — masses are tabulated in [Subsystem Masses](#breakdown)

<a id="breakdown"></a>

## Subsystem Masses

<!-- caption -->
*All subsystems by mass*

| name | mass |
| --- | --- |
| mount | 15 |
| optics | 8.5 |
| segmentControl | 20 |

<!-- caption -->
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

<!-- caption -->
*Parts satisfying the mass requirement*

| name | qualifiedName |
| --- | --- |
| telescope | Observatory::telescope |

<!-- caption -->
*Verifications of the mass requirement*

| qualifiedName |
| --- |
| Observatory::massVerification |

## Diagrams

<!-- caption -->
*Imaging chain interconnection*

```mermaid
%% Observatory::interconnectView — interconnection rendering (render asInterconnectionDiagram)
flowchart LR
  subgraph n0 ["Observatory::imagingChain<br>«part»"]
    n1["camera : Camera<br>«part»"]
    n2["recorder : Recorder<br>«part»"]
  end
  n1 ---|"link"| n2
```

<!-- caption -->
*Telescope part tree, left to right*

```mermaid
%% tree rendering (the diagram states kind "tree")
flowchart LR
  n0["Observatory::telescope<br>«part»"]
  n1["optics : Subsystem<br>«part»"]
  n2["mass<br>«attribute»"]
  n1 --- n2
  n3["zone<br>«attribute»"]
  n1 --- n3
  n0 --- n1
  n4["segmentControl : Subsystem<br>«part»"]
  n5["mass<br>«attribute»"]
  n4 --- n5
  n6["zone<br>«attribute»"]
  n4 --- n6
  n0 --- n4
  n7["mount : Subsystem<br>«part»"]
  n8["mass<br>«attribute»"]
  n7 --- n8
  n9["zone<br>«attribute»"]
  n7 --- n9
  n0 --- n7
```
