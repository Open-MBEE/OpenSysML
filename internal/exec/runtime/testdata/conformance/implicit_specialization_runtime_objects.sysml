package test {
    private import ScalarValues::Integer;
    private import ScalarValues::Boolean;
    private import ShapeItems::Rectangle;

    part def K {
        attribute n : Integer = 0;
        perform action loopy {
            first start;
            then loop action body {
                first start;
                then action inc assign n := n + 1;
                then done;
            } until n >= 3;
            then done;
        }
        attribute loopEnclosedCount : Integer = loopy.loops#(1).enclosedPerformances->SequenceFunctions::size();
        attribute loopSubperformanceCount : Integer = loopy.loops#(1).subperformances->SequenceFunctions::size();
        attribute loopSuboccurrenceCount : Integer = loopy.loops#(1).suboccurrences->SequenceFunctions::size();
        attribute loopTimeEnclosedCount : Integer = loopy.loops#(1).timeEnclosedOccurrences->SequenceFunctions::size();
        attribute firstEnclosedIsFirstBody : Boolean = loopy.loops#(1).enclosedPerformances#(1) == loopy.loops#(1).body#(1);
        attribute bodyEnclosedCount : Integer = loopy.loops#(1).body#(1).enclosedPerformances->SequenceFunctions::size();
        attribute bodySubperformanceCount : Integer = loopy.loops#(1).body#(1).subperformances->SequenceFunctions::size();
        attribute bodyEnclosedIsInc : Boolean = loopy.loops#(1).body#(1).enclosedPerformances#(1) == loopy.loops#(1).body#(1).inc;
        attribute loopyEnclosedCount : Integer = loopy.enclosedPerformances->SequenceFunctions::size();
        attribute loopySubperformanceCount : Integer = loopy.subperformances->SequenceFunctions::size();
        attribute loopStartTimeEnclosedCount : Integer = loopy.loops#(1).start.timeEnclosedOccurrences->SequenceFunctions::size();
        exhibit state sm { entry action en { } then idle; state idle; }
        attribute entryActionIsEn : Boolean = sm.entryAction == sm.en;
    }
    part def Crate {
        item face : Rectangle;
        attribute faceSubitemCount : Integer = face.subitems->SequenceFunctions::size();
        attribute edgeSubitemCount : Integer = face.edges#(1).subitems->SequenceFunctions::size();
    }
    part def Rig {
        part k : K;
        part crate : Crate;
    }
    part rig : Rig;
}
