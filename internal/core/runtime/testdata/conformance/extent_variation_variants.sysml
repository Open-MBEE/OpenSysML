// `all V` over a variation part or definition is the sequence of the variants it
// declares, in declaration order: each stands for an object of the variant, so the
// features the variant declares are read through it, as through a selected one.
package test {
    private import ScalarValues::*;
    private import SequenceFunctions::*;
    private import IntegerFunctions::sum;

    part def Engine {
        attribute cylinders : Integer;
    }

    variation part engineChoice : Engine {
        variant part fourCyl { attribute :>> cylinders = 4; }
        variant part sixCyl { attribute :>> cylinders = 6; }
        variant part eightCyl { attribute :>> cylinders = 8; }
    }

    variation part def Gearbox {
        variant part manual;
        variant part automatic;
    }

    part vehicle {
        attribute gearboxCount : Natural = size(all Gearbox);
        attribute firstGearbox = (all Gearbox)#(1);
        attribute choices : Engine[*] = all engineChoice;
        attribute choiceCount : Natural = size(all engineChoice);
        attribute firstChoice = (all engineChoice)#(1);
        attribute lastChoice = (all engineChoice)#(3);
        attribute lastCylinders : Integer = ((all engineChoice)#(3)).cylinders;
        attribute totalCylinders : Integer = sum((all engineChoice).cylinders);

        assert constraint offersSix { includes(all engineChoice, engineChoice::sixCyl) }
    }
}
