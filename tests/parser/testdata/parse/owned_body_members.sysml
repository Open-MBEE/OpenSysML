// Members only one body kind offers, in every body kind that owns them.
package OwningBodies {
    part def Vehicle;
    part def Person;
    action def Init;
    view def Table;

    requirement def Safe {
        subject v : Vehicle;
        actor driver : Person;
        stakeholder owner : Person;
        frame c { stakeholder s : Person; }
    }

    use case def Drive {
        subject v : Vehicle;
        actor driver : Person;
        objective { require constraint { true } }
        include park { actor driver : Person; }
    }

    state def Running {
        entry action init : Init;
        do action run;
        exit;
        entry; then idle;
        state idle;
    }

    view def Overview {
        render asTable : Table;
    }
}
