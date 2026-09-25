package test {
    private import ScalarValues::*;
    private import TradeStudies::*;

    part def Engine { attribute mass : Real; attribute cylinders : Real; }
    part light : Engine { attribute :>> mass = 10.0; attribute :>> cylinders = 2.0; }
    part heavy : Engine { attribute :>> mass = 20.0; attribute :>> cylinders = 0.0; }

    analysis perCylinder : TradeStudy {
        subject : Engine[1..*] = (light, heavy);
        objective : MinimizeObjective;
        calc :>> evaluationFunction {
            in part e :>> alternative : Engine;
            return :>> result : Real = e.mass / e.cylinders;
        }
        return part :>> selectedAlternative : Engine;
    }
}
