package test {
    private import ScalarValues::Integer;

    part def Leaf {
        part inner;
    }
    part def P1 {
        part pa : Leaf;
        part pb;
        attribute suboccurrenceCount : Integer = self.suboccurrences->SequenceFunctions::size();
        attribute subobjectCount : Integer = self.subobjects->SequenceFunctions::size();
        attribute bareCount : Integer = suboccurrences->SequenceFunctions::size();
        attribute grandchildCount : Integer = self.suboccurrences.suboccurrences->SequenceFunctions::size();
    }
    part def Mixed {
        item ia;
        part pb;
        attribute suboccurrenceCount : Integer = self.suboccurrences->SequenceFunctions::size();
        attribute subitemCount : Integer = self.subitems->SequenceFunctions::size();
    }
    action def Act {
        action a1;
        action a2;
        attribute subperformanceCount : Integer = self.subperformances->SequenceFunctions::size();
    }
    part def Bare {
        attribute suboccurrenceCount : Integer = self.suboccurrences->SequenceFunctions::size();
        attribute subobjectCount : Integer = self.subobjects->SequenceFunctions::size();
    }
    action def Idle {
        attribute subperformanceCount : Integer = self.subperformances->SequenceFunctions::size();
    }
    part def Rig {
        part p1 : P1;
        part mixed : Mixed;
        part bare : Bare;
        action act : Act;
        action idle : Idle;
    }
    part rig : Rig;
}
