package ClauseOrder {
    private import ScalarValues::*;
    part def Quantity {
        attribute num : Real;
    }
    part def Ratio specializes Quantity {
        attribute redefines num : Real;
    }
    part def Ctx;
    part def Mission {
        part context : Ctx;
        snapshot groundSystemAtIngress subsets context : Ctx {
        }
    }
}
