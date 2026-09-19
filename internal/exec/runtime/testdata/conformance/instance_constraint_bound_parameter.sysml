// A constraint usage binds the parameters of the definition it is typed by, and
// the inherited condition sees those bindings.
package test {
    constraint def MassLimit {
        in m;
        in limit;
        m <= limit
    }

    part def Vehicle {
        attribute mass = 1200.0;

        constraint withinLimit : MassLimit {
            in m = mass;
            in limit = 1500.0;
        }

        constraint overLimit : MassLimit {
            in m = mass;
            in limit = 1000.0;
        }
    }
}
