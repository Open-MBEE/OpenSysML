package U {
    private import ScalarValues::*;
    private import SequenceFunctions::*;
    attribute u;
    attribute s : String;
    attribute b : Boolean;
    attribute r : Real;
    attribute xs : Real[2..4];
}

package T {
    private import ScalarValues::*;
    private import SequenceFunctions::*;
    part def D { attribute tag : String default = "d"; attribute mass : Real; }
    part rack {
        part slots[3] : D;
        part gear[1..*] : D;
        part loose[0..2] : D;
        part lone : D;
        part many[10001..*] : D;
        part fixed : D :> gear;
        part vacant[0] : D;
        part tagged : D :> gear { attribute :>> tag = "x"; }
    }
    enum def Mode { ON; OFF; }
}
