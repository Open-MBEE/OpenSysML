# Spacecraft Requirement Traceability

Every requirement, what satisfies and verifies it, and whether the spacecraft meets it.

## Requirements

**SC-1** — The bus dry mass shall not exceed 1200 kg.

**SC-2** — The high-gain antenna shall provide at least 40 dBi.

**SC-3** — The radiator shall provide at least 2 m² of rejection area.

**SC-4** — The spacecraft shall be passivated at end of mission.

## Traceability Matrix

*Requirements with their satisfiers and verifiers*

| shortName | name | satisfiedBy | verifiedBy | verifications |
| --- | --- | --- | --- | --- |
| SC-1 | massLimit | bus | massMeasurement | 1 |
| SC-2 | downlinkGain | antenna | gainTest, gainAnalysis | 2 |
| SC-3 | heatRejection | radiator |  | 0 |
| SC-4 | passivation |  |  | 0 |

## Verdicts

*Checks of the spacecraft against its requirements*

| kind | name | path | verdict | condition |
| --- | --- | --- | --- | --- |
| satisfaction |  | Traceability::spacecraft.bus | holds |  |
| verification | massMeasurement | Traceability::spacecraft.bus | holds |  |
| satisfaction |  | Traceability::spacecraft.antenna | holds |  |
| verification | gainTest | Traceability::spacecraft.antenna | holds |  |
| verification | gainAnalysis | Traceability::spacecraft.antenna | holds |  |
| satisfaction |  | Traceability::spacecraft.radiator | violated | radiator.area >= 2.0 |
