package Cost {
    private import ScalarValues::Real;

    requirement def CostRequirement {
        subject product : Definition;
        attribute maximumCost : Real;
        require constraint costOk { product.cost <= maximumCost }
    }

    requirement costReq : CostRequirement {
        :>> maximumCost = 100.0;
    }

    part def Definition {
        attribute cost : Real;
        satisfy costReq;

        verification costTest {
            objective costObjective { verify costReq; }
            return verdict = VerificationCases::PassIf(costReq(this));
        }
    }

    part cheap : Definition {
        attribute :>> cost = 80.0;
    }

    part dear : Definition {
        attribute :>> cost = 120.0;
    }
}
