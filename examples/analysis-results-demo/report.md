# Recorded analysis runs

An analysis run's printed results are discarded unless they are recorded — by -record-run or by hand, in the vocabulary the AnalysisRecords library defines. Every row below is a declared record typed by a run definition on that vocabulary, annotated with the command that produced it — except the last table, whose verdicts are recomputed live.

## Every recorded run

*Every @AnalysisRecords::RecordedRun-annotated AnalysisRun in the Records package*

| name | caseName | kind | subjectName | objective |
| --- | --- | --- | --- | --- |
| scoutRun | Descent::scoutBudget | run | Landers::scout | satisfied |
| haulerRun | Descent::haulerBudget | run | Landers::hauler | satisfied |
| relayRun | Descent::relayBudget | run | Landers::relay | satisfied |
| scoutSweep40 | Descent::scoutBudget | sweep | Landers::scout | satisfied |
| scoutSweep60 | Descent::scoutBudget | sweep | Landers::scout | satisfied |
| scoutSweep80 | Descent::scoutBudget | sweep | Landers::scout | not satisfied |
| lightestRun | Selection::lightest | trade |  | satisfied |

## Recorded fuel budgets

*One record per run, grouped by subject*

**subjectName: Landers::scout**

| name | subjectName | kind | burnTime | fuelUsed | wetMass | fuelLeft | objective |
| --- | --- | --- | --- | --- | --- | --- | --- |
| scoutRun | Landers::scout | run | 40 | 120 | 730 | 130 | satisfied |
| scoutSweep40 | Landers::scout | sweep | 40 | 120 | 730 | 130 | satisfied |
| scoutSweep60 | Landers::scout | sweep | 60 | 180 | 670 | 70 | satisfied |
| scoutSweep80 | Landers::scout | sweep | 80 | 240 | 610 | 10 | not satisfied |

**subjectName: Landers::hauler**

| name | subjectName | kind | burnTime | fuelUsed | wetMass | fuelLeft | objective |
| --- | --- | --- | --- | --- | --- | --- | --- |
| haulerRun | Landers::hauler | run | 40 | 320 | 1980 | 580 | satisfied |

**subjectName: Landers::relay**

| name | subjectName | kind | burnTime | fuelUsed | wetMass | fuelLeft | objective |
| --- | --- | --- | --- | --- | --- | --- | --- |
| relayRun | Landers::relay | run | 40 | 100 | 530 | 80 | satisfied |

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
| relayRun | Landers::relay | 40 | 80 | 110 | 30 |

## Provenance

*Every element annotated @AnalysisRecords::RecordedRun*

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

*The recorded selection and verdict*

| name | caseName | selected | objective |
| --- | --- | --- | --- |
| lightestRun | Selection::lightest | Landers::relay | satisfied |

*Each alternative's evaluation, as recorded*

| alternative | score | selected | tied |
| --- | --- | --- | --- |
| Landers::scout (object \#1) | 850 | false | false |
| Landers::hauler (object \#2) | 2300 | false | false |
| Landers::relay (object \#3) | 630 | true | false |

## Live verdicts

*Assertions about the scout, evaluated at render time*

| kind | name | verdict | reason |
| --- | --- | --- | --- |
| satisfaction |  | holds |  |
| verification | checkScout | holds |  |
