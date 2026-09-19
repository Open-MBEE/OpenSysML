package test {
    private import ScalarValues::*;
    private import TradeStudies::*;
    private import ControlFunctions::*;

    part def Engine { attribute mass : Real; }
    part heavy : Engine { attribute :>> mass = 30.0; }

    analysis none : TradeStudy {
        subject : Engine[1..*] = (heavy)->select { in e : Engine; e.mass < 0.0 };
        objective : MinimizeObjective;
        calc :>> evaluationFunction {
            in part e :>> alternative : Engine;
            return :>> result : Real = e.mass;
        }
        return part :>> selectedAlternative : Engine;
    }
}
