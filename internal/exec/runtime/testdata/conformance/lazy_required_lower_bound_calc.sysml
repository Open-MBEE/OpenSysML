// A calc reads one member of a lower bound of thousands and the count of all of them;
// the trace shows the collection held by its unmade identities, not as objects made.
package test {
    private import ScalarValues::*;
    private import SequenceFunctions::*;

    part def Component {
        attribute unitMass : Real = 2.0;
    }

    part def Assembly {
        part cell : Component[2000];
    }

    part asm : Assembly;

    calc pickMass {
        return : Real = asm.cell#(1999).unitMass * size(asm.cell);
    }
}
