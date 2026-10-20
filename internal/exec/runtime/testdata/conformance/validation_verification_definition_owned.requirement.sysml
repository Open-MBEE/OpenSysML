package RequirementPkg {
    private import ScalarValues::Real;
    private import DefinitionPkg::Definition;

    requirement def CostRequirement {
        subject product : Definition;
        attribute maximumCost : Real;
        require constraint costOk { product.cost <= maximumCost }
    }
    requirement costReq : CostRequirement {
        :>> maximumCost = 100.0;
    }
}
