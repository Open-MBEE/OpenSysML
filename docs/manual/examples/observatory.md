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

*Telescope part tree, left to right*

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
---
%% tree rendering (the diagram states kind "tree")
flowchart LR
  n0("`**telescope**
*«part»*`")
    n1("`**optics : Subsystem**
*«part»*`")
      n2("`**mass**
*«attribute»*`")
      n3("`**zone**
*«attribute»*`")
  n1 --- n2
  n1 --- n3
    n4("`**segmentControl : Subsystem**
*«part»*`")
      n5("`**mass**
*«attribute»*`")
      n6("`**zone**
*«attribute»*`")
  n4 --- n5
  n4 --- n6
    n7("`**mount : Subsystem**
*«part»*`")
      n8("`**mass**
*«attribute»*`")
      n9("`**zone**
*«attribute»*`")
  n7 --- n8
  n7 --- n9
  n0 --- n1
  n0 --- n4
  n0 --- n7
```
