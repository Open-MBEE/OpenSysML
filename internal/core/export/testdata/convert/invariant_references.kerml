package InvariantReferences {
    private import ScalarValues::*;
    class Tank {
        feature ready : Boolean;
        feature sealed : Boolean;
        feature level : Real;
        inv { ready }
        inv holds { not sealed }
        inv { sealed and ready }
        inv bounded {
            level > 0
            ready
        }
    }
}
