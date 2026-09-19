// A `[n]` feature holds n objects, so a roll-up over it counts each once — the
// subsetting parts are among the n, anonymous objects make up the rest.
package test {
    private import ScalarValues::*;
    private import RealFunctions::*;

    part def Component {
        attribute unitMass : Real = 1.0;
        attribute basicMass : Real = unitMass;
    }

    part def Assembly {
        part cell : Component[4] {
            attribute :>> unitMass = 2.0;
        }
        attribute cellMass : Real = sum(cell.basicMass);

        part module : Component[3];
        attribute moduleMass : Real = sum(module.basicMass);
    }

    part asm : Assembly {
        part heavy : Component :> module {
            attribute :>> unitMass = 5.0;
        }
    }
}
