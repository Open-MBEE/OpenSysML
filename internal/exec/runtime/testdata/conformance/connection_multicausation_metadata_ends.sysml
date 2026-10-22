// The Cause and Effect library's form: `#multicausation connection` ends tagged
// `#cause`/`#effect` subset `causes`/`effects` through semantic metadata whose
// baseType the library spells with an `as` cast (`causes as SysML::Usage`,
// SysML v2 §7.27.4), and `Multicausation::causes [1..*]` redefines the base, so
// the ends fill it with or without an explicit `:> causes`.
package test {
    private import CauseAndEffect::*;
    occurrence tree {
        #cause event occurrence a;
        #cause event occurrence b;
        #effect event occurrence top;
        #multicausation connection gate {
            end #cause ::> a;
            end #cause ::> b;
            end #effect ::> top;
        }
        #multicausation connection explicitGate {
            end #cause :> causes ::> a;
            end #effect :> effects ::> top;
        }
        connection typedGate : Multicausation {
            end #cause ::> b;
            end #effect ::> top;
        }
        assert constraint gateCauses { SequenceFunctions::size(gate.causes) == 2 }
        assert constraint gateEffects { SequenceFunctions::size(gate.effects) == 1 }
        assert constraint explicitGateCauses { SequenceFunctions::size(explicitGate.causes) == 1 }
        assert constraint typedGateCauses { SequenceFunctions::size(typedGate.causes) == 1 }
        assert constraint typedGateEffects { SequenceFunctions::size(typedGate.effects) == 1 }
    }
}
