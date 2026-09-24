# Recorded analysis runs

Analysis runs are printed and discarded; nothing writes them back. Every row below is a declared record — a part typed by a run definition whose attributes hold the inputs, outputs and objective a run printed, annotated with the command that produced it — except the last table, whose verdicts are recomputed live.

## Recorded fuel budgets

*One record per run, grouped by subject*

**subjectName: scout**

| name | subjectName | kind | burnTime | fuelUsed | wetMass | fuelLeft | objective |
| --- | --- | --- | --- | --- | --- | --- | --- |
| scoutRun | scout | run | 40 | 120 | 730 | 130 | satisfied |
| scoutSweep40 | scout | sweep | 40 | 120 | 730 | 130 | satisfied |
| scoutSweep60 | scout | sweep | 60 | 180 | 670 | 70 | satisfied |
| scoutSweep80 | scout | sweep | 80 | 240 | 610 | 10 | not satisfied |

**subjectName: hauler**

| name | subjectName | kind | burnTime | fuelUsed | wetMass | fuelLeft | objective |
| --- | --- | --- | --- | --- | --- | --- | --- |
| haulerRun | hauler | run | 40 | 320 | 1980 | 580 | satisfied |

**subjectName: relay**

| name | subjectName | kind | burnTime | fuelUsed | wetMass | fuelLeft | objective |
| --- | --- | --- | --- | --- | --- | --- | --- |
| relayRun | relay | run | 40 | 100 | 530 | 80 | satisfied |

## Sweep of scoutBudget

*One record per sweep row*

| name | caseName | burnTime | fuelUsed | fuelLeft | objective |
| --- | --- | --- | --- | --- | --- |
| scoutSweep40 | Descent::scoutBudget | 40 | 120 | 130 | satisfied |
| scoutSweep60 | Descent::scoutBudget | 60 | 180 | 70 | satisfied |
| scoutSweep80 | Descent::scoutBudget | 80 | 240 | 10 | not satisfied |

## Runs that no longer match the model

*Records whose saved fuelLeft differs from the value the model now derives*

| name | subjectName | burnTime | fuelLeft | liveFuelLeft | drift |
| --- | --- | --- | --- | --- | --- |
| relayRun | relay | 40 | 80 | 110 | 30 |

## Provenance

*Every element annotated @RecordedRun*

| name | kind | caseName | command |
| --- | --- | --- | --- |
| scoutRun | run | Descent::scoutBudget | ./bin/sysml examples/analysis-results-demo/lander-results.sysml -analysis Descent::scoutBudget |
| haulerRun | run | Descent::haulerBudget | ./bin/sysml examples/analysis-results-demo/lander-results.sysml -analysis Descent::haulerBudget |
| relayRun | run | Descent::relayBudget | ./bin/sysml examples/analysis-results-demo/lander-results.sysml -analysis Descent::relayBudget |
| scoutSweep40 | sweep | Descent::scoutBudget | ./bin/sysml examples/analysis-results-demo/lander-results.sysml -analysis Descent::scoutBudget -sweep burnTime=40.0..80.0:20.0 |
| scoutSweep60 | sweep | Descent::scoutBudget | ./bin/sysml examples/analysis-results-demo/lander-results.sysml -analysis Descent::scoutBudget -sweep burnTime=40.0..80.0:20.0 |
| scoutSweep80 | sweep | Descent::scoutBudget | ./bin/sysml examples/analysis-results-demo/lander-results.sysml -analysis Descent::scoutBudget -sweep burnTime=40.0..80.0:20.0 |
| lightestRun | trade | Selection::lightest | ./bin/sysml examples/analysis-results-demo/lander-results.sysml -analysis Selection::lightest |

## Trade study

*The recorded selection and scores*

| name | caseName | selected | scoutScore | haulerScore | relayScore | objective |
| --- | --- | --- | --- | --- | --- | --- |
| lightestRun | Selection::lightest | relay | 850 | 2300 | 630 | satisfied |

## Live verdicts

*Assertions about the scout, evaluated at render time*

| kind | name | verdict | reason |
| --- | --- | --- | --- |
| satisfaction |  | holds |  |
| verification | checkScout | holds |  |
