package MC {
    private import ScalarValues::*;
    private import RandomFunctions::*;
    action def Settle { out t : Real; first start; then assign t := uniform(1.0, 5.0); then done; }
    part def Probe { attribute t : Real; action settle { first start; then assign t := uniform(1.0, 5.0); then done; } }
    individual def probe :> Probe;
    analysis def Mc :> Simulation::MonteCarlo {
        subject analysed : Probe;
        perform action run ::> analysed.settle;
        attribute :>> observed : Real = analysed.t;
        return Mean : Real = mean;
    }
    part def P { attribute m : Real = 2.0; }
    part p : P;
    analysis def Cost { subject s : P; return total : Real = s.m * 3.0; }
}
