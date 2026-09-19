// A constraint a part carries is evaluated against the instance's slots, so it
// sees a derived value as well as a declared one.
package test {
    part def Vehicle {
        attribute mass = 1500.0;
        attribute payload = 300.0;
        attribute gross = mass + payload;

        constraint withinLimit {
            gross <= 2000.0
        }

        constraint overLimit {
            gross > 2000.0
        }
    }
}
