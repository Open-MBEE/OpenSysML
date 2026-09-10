package ClauseOrder {
    private import ScalarValues::*;
    part def Quantity { attribute num : ScalarValues::Real; }
    part def Ratio :> Quantity { attribute :>> num : ScalarValues::Real; }
    part def Ctx;
    part def Mission {
        part context : Ctx;
        snapshot groundSystemAtIngress :> context : Ctx { }
    }
}
