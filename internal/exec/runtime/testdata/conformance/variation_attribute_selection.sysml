// A variation attribute is bound to one of its variants, and the value the
// selected variant declares is read through the variation feature.
package test {
    private import ScalarValues::Real;

    attribute def Characteristic {
        attribute cost : Real;
    }
    attribute def Cut :> Characteristic;

    part def Diamond {
        attribute cut : Cut;
        attribute diamondCost : Real;
    }

    abstract part diamondFamily : Diamond {
        attribute :>> diamondCost = cut.cost;

        variation attribute :>> cut {
            variant attribute cutShallow { :>> cost = 200.0; }
            variant attribute cutIdeal { :>> cost = 250.0; }
            variant attribute cutDeep { :>> cost = 220.0; }
        }
    }

    part idealDiamond :> diamondFamily {
        attribute :>> cut = cut::cutIdeal;

        assert constraint isIdeal {
            cut == cut::cutIdeal
        }
        assert constraint notShallow {
            cut == cut::cutShallow
        }
    }
}
