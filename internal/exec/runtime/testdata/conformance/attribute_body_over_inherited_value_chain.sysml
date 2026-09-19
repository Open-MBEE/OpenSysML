// Bodies over a value the base declaration wrote merge down the specialization
// chain the way bodies over a valueless feature do: each level keeps what the
// level above it valued and the innermost restatement wins.
package test {
    private import ScalarValues::Real;

    attribute def Cost { attribute v : Real = 1.0; attribute w : Real = 2.0; }

    part def Ring {
        attribute template : Cost { attribute :>> v = 9.0; }
        attribute cost : Cost = template;
    }

    part base : Ring {
        attribute :>> cost { attribute :>> v = 11.0; }
    }

    part leaf :> base {
        attribute :>> cost { attribute :>> w = 4.0; }

        assert constraint innermostBodyWins { cost.w == 4.0 }
        assert constraint keepsTheBodyAboveIt { cost.v == 11.0 }
    }
}
