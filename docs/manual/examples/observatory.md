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
  themeVariables:
    fontFamily: "Helvetica, Arial, sans-serif"
    fontSize: "14px"
    primaryColor: "#FFFFFF"
    secondaryColor: "#FFFFFF"
    tertiaryColor: "#FFFFFF"
    background: "#FFFFFF"
    clusterBkg: "#FFFFFF"
    edgeLabelBackground: "#FFFFFF"
    primaryBorderColor: "#181818"
    lineColor: "#181818"
    clusterBorder: "#181818"
    noteBorderColor: "#181818"
    primaryTextColor: "#000000"
    textColor: "#000000"
    noteTextColor: "#000000"
    noteBkgColor: "#FEFFDD"
    stateBkg: "#FFFFFF"
    stateBorder: "#181818"
    stateLabelColor: "#000000"
    compositeBackground: "#FFFFFF"
    compositeBorder: "#181818"
    compositeTitleBackground: "#FFFFFF"
    compositeTitleBorder: "#181818"
    actorBkg: "#FFFFFF"
    actorBorder: "#181818"
    actorTextColor: "#000000"
    signalColor: "#181818"
    signalTextColor: "#000000"
    labelBoxBkgColor: "#FFFFFF"
    labelBoxBorderColor: "#181818"
    labelTextColor: "#000000"
    actorLineColor: "#181818"
    loopTextColor: "#000000"
    activationBorderColor: "#181818"
    activationBkgColor: "#FFFFFF"
    sequenceNumberColor: "#000000"
    transitionColor: "#181818"
    transitionLabelColor: "#000000"
    labelBackgroundColor: "#FFFFFF"
    specialStateColor: "#181818"
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
  themeVariables:
    fontFamily: "Helvetica, Arial, sans-serif"
    fontSize: "14px"
    primaryColor: "#FFFFFF"
    secondaryColor: "#FFFFFF"
    tertiaryColor: "#FFFFFF"
    background: "#FFFFFF"
    clusterBkg: "#FFFFFF"
    edgeLabelBackground: "#FFFFFF"
    primaryBorderColor: "#181818"
    lineColor: "#181818"
    clusterBorder: "#181818"
    noteBorderColor: "#181818"
    primaryTextColor: "#000000"
    textColor: "#000000"
    noteTextColor: "#000000"
    noteBkgColor: "#FEFFDD"
    stateBkg: "#FFFFFF"
    stateBorder: "#181818"
    stateLabelColor: "#000000"
    compositeBackground: "#FFFFFF"
    compositeBorder: "#181818"
    compositeTitleBackground: "#FFFFFF"
    compositeTitleBorder: "#181818"
    actorBkg: "#FFFFFF"
    actorBorder: "#181818"
    actorTextColor: "#000000"
    signalColor: "#181818"
    signalTextColor: "#000000"
    labelBoxBkgColor: "#FFFFFF"
    labelBoxBorderColor: "#181818"
    labelTextColor: "#000000"
    actorLineColor: "#181818"
    loopTextColor: "#000000"
    activationBorderColor: "#181818"
    activationBkgColor: "#FFFFFF"
    sequenceNumberColor: "#000000"
    transitionColor: "#181818"
    transitionLabelColor: "#000000"
    labelBackgroundColor: "#FFFFFF"
    specialStateColor: "#181818"
  flowchart:
    subGraphTitleMargin:
      bottom: 24
---
%% tree rendering (the diagram states kind "tree")
flowchart LR
  subgraph n0 ["`**telescope**
*«part»*`"]
    direction LR
    subgraph n1 ["`**optics : Subsystem**
*«part»*`"]
      direction LR
      mermaid_anchor_n1((" "))
      n2("`**mass**
*«attribute»*`")
      n3("`**zone**
*«attribute»*`")
  mermaid_anchor_n1 --- n2
  mermaid_anchor_n1 --- n3
    end
    subgraph n4 ["`**segmentControl : Subsystem**
*«part»*`"]
      direction LR
      mermaid_anchor_n4((" "))
      n5("`**mass**
*«attribute»*`")
      n6("`**zone**
*«attribute»*`")
  mermaid_anchor_n4 --- n5
  mermaid_anchor_n4 --- n6
    end
    subgraph n7 ["`**mount : Subsystem**
*«part»*`"]
      direction LR
      mermaid_anchor_n7((" "))
      n8("`**mass**
*«attribute»*`")
      n9("`**zone**
*«attribute»*`")
  mermaid_anchor_n7 --- n8
  mermaid_anchor_n7 --- n9
    end
  end
  n0 --- n1
  n0 --- n4
  n0 --- n7
  classDef treeAnchor fill:transparent,stroke:transparent,color:transparent
  class mermaid_anchor_n1,mermaid_anchor_n4,mermaid_anchor_n7 treeAnchor
```
