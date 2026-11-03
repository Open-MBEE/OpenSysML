// A state's `entryAction`, `doAction` and `exitAction` each hold one
// performance whether or not the state declares an `entry`, `do` or `exit`
// action: `States::StateAction` redefines `StatePerformance`'s `entry[1]`,
// `do[1]` and `exit[1]` steps, so an undeclared one is the default performance
// of a `[1]` composite feature, not an empty value. The library's genuinely
// optional `[0..1]` features beside them — `incomingTransitionTrigger`,
// `accepted`, a view's `viewRendering` — hold nothing when unset.
package test {
    private import ScalarValues::Integer;
    private import ScalarValues::Boolean;

    state def Entered {
        entry action en {} then idle;
        state idle;
    }
    state def Declared {
        entry action en {} then idle;
        state idle;
        do action run {}
        exit action leave {}
    }
    part def Machine {
        exhibit state entered : Entered;
        exhibit state declared : Declared;
        attribute entryCount : Integer = entered.entryAction->SequenceFunctions::size();
        attribute doCount : Integer = entered.doAction->SequenceFunctions::size();
        attribute exitCount : Integer = entered.exitAction->SequenceFunctions::size();
        attribute doIsEmpty : Boolean = entered.doAction->SequenceFunctions::isEmpty();
        attribute triggerCount : Integer = entered.incomingTransitionTrigger->SequenceFunctions::size();
        attribute acceptedCount : Integer = entered.accepted->SequenceFunctions::size();
        attribute declaredDoCount : Integer = declared.doAction->SequenceFunctions::size();
        attribute declaredExitCount : Integer = declared.exitAction->SequenceFunctions::size();
    }
    view def Sheet;
    part def Rig {
        part m : Machine;
        view sheet : Sheet;
        attribute renderingCount : Integer = sheet.viewRendering->SequenceFunctions::size();
    }
}
