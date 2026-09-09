package test {
    private import ScalarValues::*;
    private import TradeStudies::*;

    part def Engine { attribute power : Real; }
    part weak : Engine { attribute :>> power = 100.0; }
    part strong : Engine { attribute :>> power = 250.0; }
    part medium : Engine { attribute :>> power = 180.0; }

    analysis def Strongest :> TradeStudy {
        subject :>> studyAlternatives : Engine[1..*];
        objective : MaximizeObjective;
        calc :>> evaluationFunction {
            in part e :>> alternative : Engine;
            return :>> result : Real = e.power;
        }
        return part :>> selectedAlternative : Engine;
    }

    analysis strongest : Strongest {
        subject = (weak, strong, medium);
    }
}
