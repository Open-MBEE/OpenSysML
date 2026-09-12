// `all T` is ordered as T's declarations are written: an enumeration's values and
// a variation's variants come in declaration order, whatever order they are named
// in elsewhere, so the sequence an extent yields is the same in every run.
package test {
    private import ScalarValues::*;
    private import SequenceFunctions::*;

    enum def Size { small; medium; large; }

    part def Engine {
        attribute cylinders : Integer;
    }

    variation part engineChoice : Engine {
        variant part fourCyl { attribute :>> cylinders = 4; }
        variant part sixCyl { attribute :>> cylinders = 6; }
        variant part eightCyl { attribute :>> cylinders = 8; }
    }

    part order {
        ref part preferred : Engine = engineChoice::eightCyl;
        attribute largest : Size = Size::large;
    }

    calc cylindersOffered {
        attribute sizes : Size[*] = all Size;
        return : Integer[*] = (all engineChoice).cylinders;
    }
}
