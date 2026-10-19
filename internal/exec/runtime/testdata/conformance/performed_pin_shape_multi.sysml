// A pin written [0..*] holds a sequence whatever count it is bound to: one value is a
// one-element sequence, several stay several, none is the empty sequence, and a [1] pin
// assigned from a one-element pin holds the element.
package test {
    private import ScalarValues::*;
    private import SequenceFunctions::*;

    action def Gather {
        in items : Integer[0..*];
        out count : Integer[1];
        out kept : Integer[0..*];
        first start;
        then action c { assign count := size(items); }
        then action k { assign kept := items; }
        then done;
    }

    action probe {
        out one : Integer[0..*];
        out several : Integer[0..*];
        out none : Integer[0..*];
        out oneCount : Integer[1];
        out noneCount : Integer[1];
        out head : Integer[1];
        first start;
        then action g1 : Gather { in items = 7; }
        then action g2 : Gather { in items = (1, 2, 3); }
        then action g3 : Gather { in items = (); }
        then action read {
            assign one := g1.kept;
            assign several := g2.kept;
            assign none := g3.kept;
            assign oneCount := g1.count;
            assign noneCount := g3.count;
            assign head := g1.kept;
        }
        then done;
    }
}
