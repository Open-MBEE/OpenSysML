// A small model whose Turtle predates the membership end statements.

package Vehicle {
    // Imports come first.
    private import ScalarValues::*;

    /* The chassis carries everything else. */
    part def Chassis {
        attribute mass : Real;

        // Wheels hang off the chassis.
        part wheel : Wheel[4];
    }

    part def Wheel;

    alias Frame for Chassis;

    part car : Chassis {
        // Two tonnes.
        attribute :>> mass = 2000.0;
    }

    port def Plug;

    part charger {
        port plug : Plug;
    }

    part battery {
        port plug : Plug;
    }

    // Chargers meet batteries here.
    connect charger.plug to battery.plug;
}
