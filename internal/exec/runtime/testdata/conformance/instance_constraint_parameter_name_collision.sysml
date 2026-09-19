// A constraint usage named after the parameter it binds is still evaluated: the
// parameter is declared inside the usage, so it masks the usage's own name.
package test {
    constraint def MassLimit {
        in m;
        in limit;
        m <= limit
    }

    part def Vehicle {
        attribute mass = 1200.0;

        constraint limit : MassLimit {
            in m = mass;
            in limit = 1500.0;
        }
    }
}
