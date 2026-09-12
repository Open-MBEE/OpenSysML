package U {
    private import ScalarValues::*;
    private import SequenceFunctions::*;
    attribute u;
}

package T {
    private import ScalarValues::*;
    private import SequenceFunctions::*;
    part def D;
    part rack {
        part slots[3] : D;
        part gear[1..*] : D;
        part loose[0..2] : D;
        part lone : D;
    }
    enum def Mode { ON; OFF; }
}
