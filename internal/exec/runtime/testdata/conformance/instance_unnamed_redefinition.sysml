// An unnamed redefinition in a nested usage body takes the name of what it
// redefines, so the value it writes governs the object's slot.
package test {
    part def Engine {
        attribute power = 200.0;
    }

    part def Car {
        part engine : Engine {
            :>> power = 250.0;
        }

        attribute enginePower = engine.power;
    }
}
