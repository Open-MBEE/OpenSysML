package ClauseOrder {
    private import ScalarValues::*;
    part def Quantity {
        attribute num : ScalarValues::Real;
    }
    part def Ratio specializes Quantity {
        attribute : ScalarValues::Real redefines num;
    }
    part def Ctx;
    part def Mission {
        part context : Ctx;
        snapshot groundSystemAtIngress : Ctx subsets context {
        }
    }
}
