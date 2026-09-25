// A variation feature no variant was selected for has no value: reading it, and
// anything computed from it, reports that rather than a wrong number.
package test {
    private import ScalarValues::Real;

    attribute def Cut {
        attribute cost : Real;
    }

    part def Diamond {
        attribute cut : Cut;
        attribute diamondCost : Real;
    }

    part unconfiguredDiamond : Diamond {
        attribute :>> diamondCost = cut.cost;

        variation attribute :>> cut {
            variant attribute cutShallow { :>> cost = 200.0; }
            variant attribute cutIdeal { :>> cost = 250.0; }
        }
    }
}
