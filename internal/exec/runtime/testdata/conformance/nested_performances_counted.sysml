package test {
    private import ScalarValues::Integer;
    private import ScalarValues::Boolean;

    action def Act {
        action a1;
        action a2;
        attribute subperformanceCount : Integer = subperformances->SequenceFunctions::size();
        attribute enclosedPerformanceCount : Integer = enclosedPerformances->SequenceFunctions::size();
        attribute firstSubperformanceIsA1 : Boolean = subperformances#(1) == a1;
        attribute a1ContextIsThis : Boolean = a1.this == self.this;
    }
    part def P3 {
        part pa;
        port q1;
        action a1;
        action a2;
        state s1;
        attribute ownedActionCount : Integer = ownedActions->SequenceFunctions::size();
        attribute ownedStateCount : Integer = ownedStates->SequenceFunctions::size();
        attribute ownedPerformanceCount : Integer = ownedPerformances->SequenceFunctions::size();
        attribute involvingPerformanceCount : Integer = involvingPerformances->SequenceFunctions::size();
        attribute suboccurrenceCount : Integer = suboccurrences->SequenceFunctions::size();
        attribute timeEnclosedCount : Integer = timeEnclosedOccurrences->SequenceFunctions::size();
        attribute firstOwnedActionIsA1 : Boolean = ownedActions#(1) == a1;
        attribute thirdOwnedActionIsS1 : Boolean = ownedActions#(3) == s1;
        attribute onlyOwnedStateIsS1 : Boolean = ownedStates#(1) == s1;
        attribute firstSuboccurrenceIsPa : Boolean = suboccurrences#(1) == pa;
        attribute secondTimeEnclosedIsQ1 : Boolean = timeEnclosedOccurrences#(2) == q1;
        attribute lastTimeEnclosedIsS1 : Boolean = timeEnclosedOccurrences#(5) == s1;
        attribute a1ContextIsOwner : Boolean = a1.this == self;
        attribute s1ContextIsOwner : Boolean = s1.this == self;
        attribute a1HasNoSubperformances : Boolean = a1.subperformances->SequenceFunctions::isEmpty();
    }
    item def Box {
        action ia;
        attribute ownedPerformanceCount : Integer = ownedPerformances->SequenceFunctions::size();
        attribute suboccurrenceCount : Integer = suboccurrences->SequenceFunctions::size();
        attribute iaContextIsOwner : Boolean = ia.this == self;
    }
    part def Forms {
        action one;
        action two [2];
        action opt [0..1];
        action many [*];
        attribute ownedActionCount : Integer = ownedActions->SequenceFunctions::size();
        attribute secondOfTwoIsHeld : Boolean = ownedActions#(3) == two#(2);
    }
    part def Rig {
        part p3 : P3;
        item box : Box;
        part forms : Forms;
        action act : Act;
        attribute actContextIsRig : Boolean = act.this == self;
        attribute actA1ContextIsRig : Boolean = act.a1.this == self;
    }
    part rig : Rig;
}
