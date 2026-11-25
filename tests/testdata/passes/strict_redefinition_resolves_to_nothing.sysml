package Repro {
    private import ISQBase::*;
    part def NuclearReactor {
        part def HotLegPipe {
            attribute diameter;
        }
    }
    part def Holder {
        part pipe : NuclearReactor::HotLegPipe {
            attribute :>> length = 10.0 [SI::m];
        }
    }
}
