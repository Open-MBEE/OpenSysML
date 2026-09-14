# Car Checks

The assertions about the car and what it holds.

<!-- caption -->
*Verdicts on the car*

| path | name | verdict | reason |
| --- | --- | --- | --- |
| car | massOk | holds |  |
| car | fits | undecided | constraint fits: assertion evaluation failed: no value for feature capacity |
| car.engine | powerLow | violated | constraint powerLow: assertion evaluated to false: power \< 200.0 |
| car.engine |  | holds |  |
| car.engine | checkEngine | holds |  |
| car.wheels\[1\] | pressureOk | violated | constraint pressureOk: assertion evaluated to false: pressure >= 30.0 |
| car.wheels\[2\] | pressureOk | violated | constraint pressureOk: assertion evaluated to false: pressure >= 30.0 |

- assert constraint fits on car: undecided
- assert constraint powerLow on car.engine: violated
- assert constraint pressureOk on car.wheels\[1\]: violated
- assert constraint pressureOk on car.wheels\[2\]: violated
