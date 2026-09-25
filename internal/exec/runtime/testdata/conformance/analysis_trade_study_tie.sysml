package test {
    private import ScalarValues::*;
    private import TradeStudies::*;

    part def Engine { attribute mass : Real; }
    part a : Engine { attribute :>> mass = 30.0; }
    part b : Engine { attribute :>> mass = 10.0; }
    part c : Engine { attribute :>> mass = 10.0; }

    analysis tied : TradeStudy {
        subject : Engine[1..*] = (a, b, c);
        objective : MinimizeObjective;
        calc :>> evaluationFunction {
            in part e :>> alternative : Engine;
            return :>> result : Real = e.mass;
        }
        return part :>> selectedAlternative : Engine;
    }
}
