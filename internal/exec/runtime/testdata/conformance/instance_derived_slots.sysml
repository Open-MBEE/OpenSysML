// A default that is an expression over sibling features is derived per
// instance: it is evaluated against the object's slots, not constant-folded.
package test {
    part def Engine {
        attribute power = 300.0;
        attribute derated = power * 0.9;
    }

    part def Vehicle {
        attribute mass = 1500.0;
        attribute folded = 3.0 * 4.0;
        attribute doubled = mass * 2.0;
        part engine : Engine;
        attribute total = mass + engine.derated;
    }
}
