# MOSA demo

[`mosa-demo.sysml`](mosa-demo.sysml) works a modular ground vehicle — a chassis, mission
computer, sensor pod, radio and power unit, with a severable autonomy module — through the
Modular Open Systems Approach, using the bundled [`MOSA`](../../docs/project/mosa-library.md)
library. Each package is stamped with the MOSA pillar it serves, and each artefact carries the
statutory keyword that both types it and adds it to the approach's base usage:

```sysml
@MOSAPackage { pillar = MOSAPillar::modularDesign; }
package 'Modular Design' {
    #majorSystemPlatform part vehicle {
        #majorSystemComponent part sensorPod : 'Sensor Pod' {
            @DataRights { kind = DataRightsKind::limited; asserter = "Sensor vendor"; }
        }
        #keyInterface interface sensorLink : 'Sensor Data Link'
            connect sensorPod.data to missionComputer.sensorIn {
            @InterfaceControl { authority = "Program office interface control working group"; }
        }
    }
}
```

| Package | MOSA pillar | What it holds |
| --- | --- | --- |
| `Open Standards` | open standards | the `#consensusStandard`s (IEEE 802.3, ISO 11898-1) and `#technicalStandard`s (MIL-STD-1275, VICTORY) the interfaces are verified against, with their registry entries |
| `Modular System Interfaces` | key interfaces | five `#modularSystemInterface` definitions spanning the data, networking, electrical, mechanical and software characteristics of `InterfaceCharacteristicKind` |
| `Modular Design` | modular design | the `#majorSystemPlatform`, its `#majorSystemComponent`s and the `#modularSystem` autonomy module; the interface usages joining them (one as a `#keyInterface`), each with its `@InterfaceControl` authority; `@DataRights` on the components and `@Proprietary` on the vendor's mount and perception library; and the `#conformance` connections naming the standard each interface meets |
| `MOSA Requirements` | modular design | a `#mosaObjective` with its `#moe`, a `#modularityRequirement` with its `#mop`, and `#interfaceRequirement`s, chained by `#derivation` |
| `Conformance Assessment` | conformance | a `MOSAConformance` assessment of the vehicle and the `satisfy` relationships tracing interfaces to their requirements |
| `MOSA Views` | enabling environment | one usage of each MOSA view definition |
| `Interface Control Document` | enabling environment | an `InterfaceControlDocument` whose tables run the library's interface, standards, data-rights and proprietary registers over the vehicle |

## From the command line

Analyse the model:

```bash
./bin/sysml examples/mosa-demo/mosa-demo.sysml -validate
```

The model has no errors, and `-validate` also runs the MOSA checks, which report the six gaps
the model leaves open on purpose (each is marked with a `doc` comment):

| Warning | Gap |
| --- | --- |
| `mosa-component-no-data-rights` | the `radio` records no `@DataRights` |
| `mosa-proprietary-no-rationale` | the vendor's `perception` library is `@Proprietary` without a `rationale` |
| `mosa-interface-not-traced` | the `radioLink` satisfies no interface requirement |
| `mosa-interface-no-control` | the `powerBus` names no `@InterfaceControl` authority |
| `mosa-interface-no-standard` | the `autonomyLink` conforms to no standard and is not proprietary |
| `mosa-boundary-not-designated` | the radio-to-chassis antenna `connect` is not a modular system interface |

Record the missing fact — a `@DataRights`, a `rationale`, a `satisfy`, an `@InterfaceControl`,
a `#conformance` connection, a `#modularSystemInterface` keyword — and the warning goes away.

The `MOSA Views` package holds one usage of each MOSA view definition; render them all with

```bash
./bin/sysml examples/mosa-demo/mosa-demo.sysml -render-all rendered/
```

which writes the modular decomposition as a Mermaid tree, the interfaces between the components
as a Mermaid interconnection diagram, and the interface, standards, proprietary-element,
data-rights, requirements and conformance registers as Markdown tables.

The interface control document renders to Markdown with

```bash
./bin/sysml examples/mosa-demo/mosa-demo.sysml \
    -render-document "'Modular Ground Vehicle'::'Interface Control Document'::'Ground Vehicle Interface Control Document'"
```

(add `-doc-form html` or `-doc-form pdf` for the other backends).
