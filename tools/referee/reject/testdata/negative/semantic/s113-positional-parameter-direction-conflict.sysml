// Invalid: the first parameter of a step redefines its behavior's first parameter by position, so an `out` written first under `in input; out output;` conflicts with `input` (KerML 8.4.4.6 checkFeatureParameterRedefinition, 8.3.3.3.8 validateRedefinitionDirectionConformance; pilot validateRedefinitionDirectionConformance).
package S113PositionalParameterDirectionConflict {
    private import ScalarValues::Integer;
    action def Step { in input : Integer; out output : Integer; }
    action def Chain {
        action stepA : Step { out :>> output; }
    }
}
