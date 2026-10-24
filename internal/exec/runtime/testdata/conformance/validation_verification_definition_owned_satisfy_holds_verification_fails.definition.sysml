package DefinitionPkg {
    private import ScalarValues::Real;
    private import RequirementPkg::costReq;

    part def Definition {
        attribute cost : Real;
        satisfy costReq;

        verification def CostTest {
            subject product : Definition;
            objective costObjective { verify costReq; }
            return verdict = VerificationCases::PassIf(product.cost <= 50.0);
        }
            verification costTest : CostTest {
            subject product = this;
            }
    }
}
