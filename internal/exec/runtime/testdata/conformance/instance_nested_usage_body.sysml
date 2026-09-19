// A nested part usage is shaped by its own body: what the body redeclares wins
// over what the usage's type declares, and an untyped nested part is still an
// object whose members hold values.
package test {
    part def Engine {
        attribute power = 200.0;
    }

    part def Car {
        part engine : Engine {
            attribute power redefines Engine::power = 250.0;
        }

        part cabin {
            attribute seats = 4.0;
        }

        attribute enginePower = engine.power;
        attribute seatCount = cabin.seats;
    }
}
