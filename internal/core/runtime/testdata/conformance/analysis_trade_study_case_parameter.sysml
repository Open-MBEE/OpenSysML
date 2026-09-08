package test {
    private import ScalarValues::*;
    private import TradeStudies::*;

    part def Engine { attribute mass : Real; attribute power : Real; }
    part strong : Engine { attribute :>> mass = 30.0; attribute :>> power = 300.0; }
    part light : Engine { attribute :>> mass = 10.0; attribute :>> power = 50.0; }

    analysis weighted : TradeStudy {
        subject : Engine[1..*] = (strong, light);
        in attribute powerWeight : Real;
        objective : MaximizeObjective;
        calc :>> evaluationFunction {
            in part e :>> alternative : Engine;
            return :>> result : Real = e.power * powerWeight - e.mass;
        }
        return part :>> selectedAlternative : Engine;
    }
}
