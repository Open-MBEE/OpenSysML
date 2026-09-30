# Orbiter Program Requirements Traceability

Every requirement of the program in one matrix, grouped by the team that owns it, with its lineage, refinements, satisfiers and verification count; then the gaps that remain and the checks that fail.

## Traceability matrix

*Requirements by owning team*

**team: Comms**

| team | shortName | name | priority | derivedFrom | descendants | refinedBy | satisfiedBy | verifications | satisfied |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Comms | COM-1 | linkMargin | critical | downlink | 0 |  | transmitter | 1 | true |
| Comms | COM-2 | dataRate | critical | downlink | 0 |  | transmitter | 1 | true |

**team: Power**

| team | shortName | name | priority | derivedFrom | descendants | refinedBy | satisfiedBy | verifications | satisfied |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Power | PWR-1 | arrayOutput | critical | power | 1 |  | array | 1 | true |
| Power | PWR-2 | batteryDepth | high | power | 0 |  | battery | 2 | true |
| Power | PWR-3 | cellDegradation | high | arrayOutput | 0 | ArrayDegradationAnalysis | array | 0 | true |

**team: Program**

| team | shortName | name | priority | derivedFrom | descendants | refinedBy | satisfiedBy | verifications | satisfied |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Program | ST-1 | science | critical |  | 7 |  |  | 0 | false |
| Program | ST-2 | longevity | critical |  | 2 |  |  | 0 | false |

**team: Systems**

| team | shortName | name | priority | derivedFrom | descendants | refinedBy | satisfiedBy | verifications | satisfied |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Systems | SYS-1 | downlink | critical | science | 2 |  | transmitter | 0 | true |
| Systems | SYS-2 | power | critical | science | 3 |  |  | 0 | false |
| Systems | SYS-3 | survival | high | longevity | 1 | EclipseThermalAnalysis |  | 0 | false |
| Systems | SYS-4 | telemetry | low |  | 0 |  |  | 0 | false |

**team: Thermal**

| team | shortName | name | priority | derivedFrom | descendants | refinedBy | satisfiedBy | verifications | satisfied |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Thermal | THM-1 | heaterPower | critical | survival | 0 | HeaterBank |  | 0 | false |

## Coverage gaps

*Leaf requirements with no satisfier or no verification*

| team | shortName | name | priority |
| --- | --- | --- | --- |
| Systems | SYS-4 | telemetry | low |
| Thermal | THM-1 | heaterPower | critical |
| Power | PWR-3 | cellDegradation | high |

*Of those, the critical ones*

| team | shortName | name | priority |
| --- | --- | --- | --- |
| Thermal | THM-1 | heaterPower | critical |

*Satisfied but never verified*

| team | shortName | name | priority |
| --- | --- | --- | --- |
| Power | PWR-3 | cellDegradation | high |
| Systems | SYS-1 | downlink | critical |

*Unsatisfied but already refined by a design element or analysis*

| team | shortName | name | priority |
| --- | --- | --- | --- |
| Systems | SYS-3 | survival | high |
| Thermal | THM-1 | heaterPower | critical |

## Verdicts

*Every check across the three designs*

| kind | name | path | verdict |
| --- | --- | --- | --- |
| requirement | cellDegradation | PowerDesign::powerSubsystem | undecided |
| satisfaction |  | PowerDesign::powerSubsystem.array | holds |
| verification | arrayIlluminationTest | PowerDesign::powerSubsystem.array | holds |
| satisfaction |  | PowerDesign::powerSubsystem.array | undecided |
| satisfaction |  | PowerDesign::powerSubsystem.battery | holds |
| verification | batteryCycleTest | PowerDesign::powerSubsystem.battery | holds |
| verification | eclipseCycleAnalysis | PowerDesign::powerSubsystem.battery | holds |
| satisfaction |  | CommsDesign::commsSubsystem.transmitter | violated |
| verification | linkBudgetAnalysis | CommsDesign::commsSubsystem.transmitter | violated |
| satisfaction |  | CommsDesign::commsSubsystem.transmitter | holds |
| verification | dataRateTest | CommsDesign::commsSubsystem.transmitter | holds |
| satisfaction |  | CommsDesign::commsSubsystem.transmitter | undecided |

*Checks that came out false*

| kind | name | path | condition |
| --- | --- | --- | --- |
| satisfaction |  | CommsDesign::commsSubsystem.transmitter | transmitter.linkMargin >= 3.0 |
| verification | linkBudgetAnalysis | CommsDesign::commsSubsystem.transmitter |  |
