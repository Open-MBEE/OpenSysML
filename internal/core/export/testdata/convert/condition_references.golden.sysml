package ConditionReferences {
    private import ScalarValues::*;
    requirement def Resistance {
        attribute structuralIntegrityMaintained : Boolean;
        attribute a : Boolean;
        attribute b : Boolean;
        attribute x : Boolean;
        require constraint { structuralIntegrityMaintained }
        require constraint { not x }
        assume constraint { a }
        require constraint <'HLR-R059'> { x }
        require constraint <'HLR-R060'> named { a and b }
        constraint { a and b }
        constraint c1 { x }
        assert constraint { x }
        assert constraint { not x }
        assert not constraint { x }
    }
    part def Rover {
        attribute ok : Boolean;
        attribute n : Real;
        assert constraint { ok }
        assert constraint {
            n > 0
            ok
        }
        constraint { ok }
        constraint inv1 { not ok }
    }
}
