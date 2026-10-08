// A lower bound of thousands holds exactly that many values, counted and addressed
// without making the objects up front: each is made when first reached, under one
// identity, and a roll-up over all of them reads each once (KerML 1.0 §7.4.12).
package test {
    private import ScalarValues::*;
    private import SequenceFunctions::*;
    private import RealFunctions::*;

    part def Component {
        attribute unitMass : Real = 2.0;
    }

    part def Assembly {
        attribute k : Integer = 3;
        attribute gridBound : Integer = k * 1000;
        part cell : Component[2000];
        part grid : Component[gridBound];
        part spare : Component :> grid {
            attribute :>> unitMass = 5.0;
        }

        attribute cellCount : Integer = size(cell);
        attribute gridCount : Integer = size(grid);
        attribute cellMass : Real = sum(cell.unitMass);
        attribute gridMass : Real = sum(grid.unitMass);
        attribute sameMember : Boolean = cell#(1500) == cell#(1500);
        attribute distinctMembers : Boolean = cell#(1500) == cell#(1501);
        attribute spareFirst : Boolean = grid#(1) == spare;
        attribute holdsSpare : Boolean = includes(grid, spare);
    }

    part asm : Assembly;
}
