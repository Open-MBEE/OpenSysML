# Rover Requirement Derivation

How the mission requirements flow down to the subsystems, what refines them, and where the flow-down ends without a part to satisfy it.

## Lineage

<!-- caption -->
*Every requirement with its parent, children, descendant count, refinements and satisfiers*

| shortName | name | derivedFrom | derives | descendants | refinedBy | satisfiedBy |
| --- | --- | --- | --- | --- | --- | --- |
| M-1 | range |  | RangeDerivation::specification::system::dailyRange, RangeDerivation::specification::system::energyBudget | 4 |  |  |
| M-2 | lifetime |  | RangeDerivation::specification::system::nightSurvival | 2 |  |  |
| S-1 | dailyRange | RangeDerivation::specification::mission::range |  | 0 |  |  |
| S-2 | energyBudget | RangeDerivation::specification::mission::range | RangeDerivation::specification::subsystem::batteryCapacity, RangeDerivation::specification::subsystem::driveEfficiency | 2 | RangeDerivation::EnergyBudgetAnalysis |  |
| S-3 | nightSurvival | RangeDerivation::specification::mission::lifetime | RangeDerivation::specification::subsystem::batteryHeater | 1 |  |  |
| B-1 | batteryCapacity | RangeDerivation::specification::system::energyBudget |  | 0 |  | RangeDerivation::rover::battery |
| B-2 | batteryHeater | RangeDerivation::specification::system::nightSurvival |  | 0 | RangeDerivation::HeaterLoop |  |
| D-1 | driveEfficiency | RangeDerivation::specification::system::energyBudget |  | 0 |  | RangeDerivation::rover::drive |

## Following a chain

<!-- caption -->
*Everything derived from M-1, at any depth*

| shortName | name | documentation |
| --- | --- | --- |
| S-1 | dailyRange | The rover shall drive 500 m per sol. |
| S-2 | energyBudget | The rover shall store 1.2 kWh for each sol's drive. |
| B-1 | batteryCapacity | The battery shall hold 1.5 kWh. |
| D-1 | driveEfficiency | The drive shall convert 85 percent of electrical energy to motion. |

<!-- caption -->
*Derived directly from M-1*

| shortName | name | documentation |
| --- | --- | --- |
| S-1 | dailyRange | The rover shall drive 500 m per sol. |
| S-2 | energyBudget | The rover shall store 1.2 kWh for each sol's drive. |

<!-- caption -->
*The chain above B-1*

| shortName | name | documentation |
| --- | --- | --- |
| S-2 | energyBudget | The rover shall store 1.2 kWh for each sol's drive. |
| M-1 | range | The rover shall traverse 20 km over the mission. |

## Chain ends

<!-- caption -->
*Requirements at the top of a chain*

| shortName | name | documentation |
| --- | --- | --- |
| M-1 | range | The rover shall traverse 20 km over the mission. |
| M-2 | lifetime | The rover shall operate for 90 sols. |

<!-- caption -->
*Leaves no part satisfies*

| shortName | name | documentation |
| --- | --- | --- |
| S-1 | dailyRange | The rover shall drive 500 m per sol. |
| B-2 | batteryHeater | The battery shall be kept above -20 C. |
