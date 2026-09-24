package ClauseOrder {
    private import ScalarValues::*;
    part def Quantity {
        attribute num : Real;
    }
    part def Ratio specializes Quantity {
        attribute : Real redefines num;
    }
    part def Ctx;
    part def Mission {
        part context : Ctx;
        snapshot groundSystemAtIngress : Ctx subsets context {
        }
    }
}
