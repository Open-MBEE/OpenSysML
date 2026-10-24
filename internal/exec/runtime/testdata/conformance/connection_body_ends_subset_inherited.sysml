// A connection usage may declare its ends in its body, each attached by
// reference subsetting (`end :> causes ::> a;`), in place of a `connect`
// clause (SysML v2 §7.13.2): the usage is a connector object whose ends hold
// the owner's features they name, and each end's value is a value of every
// feature the end subsets (KerML 1.0 §7.3.4.4), so an inherited `[1..*]`
// collection the ends subset is filled by them, named or not.
package test {
    private import ScalarValues::*;
    abstract connection def Gate {
        abstract ref occurrence causes[1..*] :> participant;
        abstract ref occurrence effects[1..*] :> participant;
    }
    connection def Pair { end one : Occurrence; end two : Occurrence; }
    occurrence tree {
        occurrence a;
        occurrence b;
        occurrence top;
        connection gate : Gate {
            end :> causes ::> a;
            end :> causes ::> b;
            end :> effects ::> top;
        }
        connection namedGate : Gate {
            end c1 :> causes ::> a;
            end e1 :> effects ::> top;
        }
        connection pair : Pair {
            end :>> one ::> a;
            end :>> two ::> b;
        }
        assert constraint gateCauses { SequenceFunctions::size(gate.causes) == 2 }
        assert constraint gateEffects { SequenceFunctions::size(gate.effects) == 1 }
        assert constraint gateParticipants { SequenceFunctions::size(gate.participant) == 3 }
        assert constraint namedGateCauses { SequenceFunctions::size(namedGate.causes) == 1 }
        assert constraint namedGateEnd { namedGate.c1 == a }
        assert constraint pairOne { pair.one == a }
        assert constraint pairTwo { pair.two == b }
    }
}
