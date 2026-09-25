package W10F {
    private import ScalarValues::*;
    private import VectorFunctions::*;

    attribute def Cost {
        attribute v : Real default 5.0;
        attribute w : Real default 2.0;
    }

    attribute template : Cost;

    part def Ring {
        attribute cost : Cost = template;
    }

    part plain : Ring;

    part r : Ring {
        attribute :>> cost {
            attribute :>> v = 11.0;
        }
    }

    calc def InheritedValues {
        return : Real[*] = (plain.cost.v, plain.cost.w, r.cost.v);
    }

    calc def VectorElements {
        return : Real[*] = 'cartesian+'(
            CartesianThreeVectorOf((1.0, 2.0, 3.0)),
            CartesianThreeVectorOf((4.0, 5.0, 6.0))).elements;
    }
}
