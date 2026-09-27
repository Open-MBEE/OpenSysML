# Lander Requirement Traceability

The lander specification as a tree, what satisfies and verifies each requirement, where coverage is missing, and which checks fail.

## Requirement hierarchy

**L-1** — The lander dry mass shall not exceed 1800 kg.

**L-2** — The lander shall descend under powered control to touchdown.

**L-2.1** — The descent engine shall deliver at least 3000 N.

**L-2.2** — The landing radar shall range the surface from 5000 m.

**L-3** — The lander shall survive touchdown at 2 m/s.

**L-3.1** — Each landing leg shall absorb 0.3 m of stroke.

**L-3.2** — The lander shall not tip over on a 15 degree slope.

**L-4** — The lander shall transmit a recovery beacon after landing.

## Traceability matrix

*Every requirement with its satisfiers and verifiers*

| shortName | name | satisfiedBy | verifiedBy |
| --- | --- | --- | --- |
| L-1 | mass | lander | weighLander |
| L-2 | descent |  |  |
| L-2.1 | thrust | engine | hotFire |
| L-2.2 | altimetry | radar | radarRangeTest |
| L-3 | touchdown |  |  |
| L-3.1 | legStroke | leg | dropTest |
| L-3.2 | tipOver |  |  |
| L-4 | beacon | transponder |  |

## Coverage gaps

*Requirements no part satisfies*

| shortName | name |
| --- | --- |
| L-2 | descent |
| L-3 | touchdown |
| L-3.2 | tipOver |

*Requirements no case verifies*

| shortName | name |
| --- | --- |
| L-2 | descent |
| L-3 | touchdown |
| L-3.2 | tipOver |
| L-4 | beacon |

*Requirements with a gap of either kind*

| shortName | name |
| --- | --- |
| L-2 | descent |
| L-3 | touchdown |
| L-3.2 | tipOver |
| L-4 | beacon |

*Requirements satisfied but never verified*

| shortName | name |
| --- | --- |
| L-4 | beacon |

## Verdicts

*Every check of the lander against its requirements*

| kind | name | path | verdict |
| --- | --- | --- | --- |
| satisfaction |  | LanderHierarchy::lander | holds |
| verification | weighLander | LanderHierarchy::lander | holds |
| satisfaction |  | LanderHierarchy::lander.propulsion.engine | violated |
| verification | hotFire | LanderHierarchy::lander.propulsion.engine | violated |
| satisfaction |  | LanderHierarchy::lander.gear.leg | holds |
| verification | dropTest | LanderHierarchy::lander.gear.leg | holds |
| satisfaction |  | LanderHierarchy::lander.avionics.radar | holds |
| verification | radarRangeTest | LanderHierarchy::lander.avionics.radar | holds |
| satisfaction |  | LanderHierarchy::lander.avionics.transponder | undecided |

*Checks that came out false*

| kind | name | path | condition |
| --- | --- | --- | --- |
| satisfaction |  | LanderHierarchy::lander.propulsion.engine | engine.thrust >= 3000.0 |
| verification | hotFire | LanderHierarchy::lander.propulsion.engine |  |
