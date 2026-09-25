// A constraint usage that takes its conditions from a definition is evaluated
// against those inherited conditions, rather than reported as vacuously
// satisfied because the usage declares no condition of its own.
package test {
    constraint def PositiveMargin {
        attribute margin = 5.0;
        margin > 0.0
    }

    constraint def NegativeMargin {
        attribute margin = -5.0;
        margin > 0.0
    }

    part def Vehicle {
        attribute mass = 1500.0;

        constraint headroom : PositiveMargin;
        constraint overrun : NegativeMargin;
    }
}
