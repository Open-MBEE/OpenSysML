package U {
    private import ScalarValues::*;
    private import SequenceFunctions::*;
    attribute u;
    attribute s : String;
    attribute b : Boolean;
    attribute r : Real;
    attribute xs : Real[2..4];
    attribute ss : String[2..*];
    attribute bs : Boolean[2];
    attribute os : Real[0..4];
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
    part store {
        part base[0..*] : D;
        part sub[10001..*] : D :> base;
        part cap[0..3] : D;
        part three[3..*] : D :> cap;
        part pool[0..*] : D;
        part inner[2] : D :> pool;
        part outer[5..*] : D :> pool;
    }
    enum def Mode { ON; OFF; }
}
