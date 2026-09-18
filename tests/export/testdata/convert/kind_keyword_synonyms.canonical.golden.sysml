package Synonyms {
    datatype Temperature;
    feature reading : Temperature;
    function def Average;
    occurrence base;
    snapshot atStart;
    timeslice during;
    flow note;
    allocate reading to base;
    use case def Drive;
    constraint def Safe;
    state def Idle;
    part def Vehicle {
        exhibit idling : Idle;
        assert safe : Safe;
        inv alsoSafe : Safe;
    }
    rendering def AsTree;
    view def Dashboard {
        render display;
        render rendering : AsTree;
    }
}
