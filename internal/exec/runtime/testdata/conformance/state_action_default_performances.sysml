package test {
    private import ScalarValues::Integer;
    private import ScalarValues::Boolean;

    state def Entered {
        entry action en {} then idle;
        state idle;
        attribute entryCount : Integer = entryAction->SequenceFunctions::size();
        attribute doCount : Integer = doAction->SequenceFunctions::size();
        attribute exitCount : Integer = exitAction->SequenceFunctions::size();
        attribute doIsEmpty : Boolean = doAction->SequenceFunctions::isEmpty();
        attribute triggerCount : Integer = incomingTransitionTrigger->SequenceFunctions::size();
        attribute acceptedCount : Integer = accepted->SequenceFunctions::size();
    }
    state def Declared {
        entry action en {} then idle;
        state idle;
        do action run {}
        exit action leave {}
        attribute doCount : Integer = doAction->SequenceFunctions::size();
        attribute exitCount : Integer = exitAction->SequenceFunctions::size();
    }
    part def Machine {
        exhibit state entered : Entered;
        exhibit state declared : Declared;
    }
    view def Sheet {
        attribute renderingCount : Integer = viewRendering->SequenceFunctions::size();
    }
    part def Rig {
        part m : Machine;
        view sheet : Sheet;
    }
}
