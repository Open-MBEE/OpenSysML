package test {
    private import ScalarValues::*;

    // The parameters an action is invoked with include the ones it inherits:
    // Double and Triple state flows of their own but declare no parameters
    // beyond Double's redefinition of factor, which gives it a default.
    action def Scale {
        in factor : Integer;
        in amount : Integer;
        inout tally : Integer;
        out scaled : Integer;
        out result : Integer;
    }

    action def Double :> Scale {
        in redefines factor = 2;
        first start;
        then action go {
            assign scaled := amount * factor;
            assign tally := tally + scaled;
            assign result := scaled;
        }
        then done;
    }

    action def Triple :> Scale {
        first start;
        then action go {
            assign scaled := amount * factor;
            assign tally := tally + scaled;
            assign result := scaled + 1;
        }
        then done;
    }

    action outer {
        attribute tally : Integer = 100;
        out doubled : Integer;
        out tripled : Integer;
        out viaResult : Integer;
        out viaCall : Integer;
        first start;
        then action twice : Double { in amount = 5; }
        then action thrice : Triple { in factor = 3; in amount = 4; }
        then action called = Triple(10, 2, tally);
        then action fin {
            assign doubled := twice.scaled;
            assign tripled := thrice.scaled;
            assign viaResult := thrice.result;
            assign viaCall := called;
        }
        then done;
    }
}
