package test {
    private import ScalarValues::*;
    private import TradeStudies::*;

    part def Engine { attribute mass : Real; }
    part light : Engine { attribute :>> mass = 10.0; }
    part heavy : Engine { attribute :>> mass = 20.0; }

    analysis single : TradeStudy {
        subject : Engine[1] = (light, heavy);
        objective : MinimizeObjective;
        calc :>> evaluationFunction {
            in part e :>> alternative : Engine;
            return :>> result : Real = e.mass;
        }
        return part :>> selectedAlternative : Engine;
    }
}
